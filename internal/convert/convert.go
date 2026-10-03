package convert

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
)

// maxTokens is the output budget for one input file. Requests are streamed,
// so a large budget doesn't risk an HTTP timeout.
const maxTokens = 64000

// Converter transcribes sheet music images and PDFs into Muse scores
type Converter struct {
	Client  anthropic.Client
	Model   string
	Effort  string    // low, medium, high, xhigh or max; empty uses the model's default
	Retries int       // how many times to ask for fixes when a page fails the checks
	Log     io.Writer // progress messages; nil for none
}

// Result is a converted score
type Result struct {
	YAML     string
	Problems []string // checks that still failed after all retries
}

// pageContext carries what later pages need to know about earlier ones
type pageContext struct {
	header   Header
	channels []string
	bars     int
	tail     string // the last few bars, for continuity
}

// Convert transcribes the input files, in order, as consecutive pages of
// one piece. snd sets the instrument (or envelope and harmonic), the melody
// instrument for three-staff music, and volume;
// a zero volume is picked by AutoVolume. name, if set, replaces the title
// read from the music. If a page fails after earlier pages succeeded, the
// result holds the score up to that page along with the error.
func (c *Converter) Convert(ctx context.Context, files []string, snd Sound, name string) (res Result, err error) {
	var (
		header   Header
		sections []string
		prev     *pageContext
	)
	for i, file := range files {
		c.logf("[%d/%d] transcribing %s", i+1, len(files), file)
		var p Page
		var probs []string
		p, probs, err = c.page(ctx, file, i+1, len(files), prev, snd)
		if err != nil {
			err = fmt.Errorf("%s: %w", file, err)
			if i > 0 {
				res.YAML = c.render(header, snd, name, files[:i], sections)
			}
			return res, err
		}
		for _, pr := range probs {
			res.Problems = append(res.Problems, fmt.Sprintf("%s: %s", filepath.Base(file), pr))
		}
		if i == 0 {
			header = p.Header
		}
		sections = append(sections, p.Sections)
		prev = nextContext(header, sections)
		c.logf("[%d/%d] %d bars so far", i+1, len(files), prev.bars)
	}
	res.YAML = c.render(header, snd, name, files, sections)
	return res, nil
}

// render assembles the transcribed pages into the final score
func (c *Converter) render(header Header, snd Sound, name string, files, sections []string) string {
	if name != "" {
		header.Name = name
	}
	var base []string
	for _, f := range files {
		base = append(base, filepath.Base(f))
	}
	layout := "Channels: C1 is the top staff, C2 the next and C3 the third."
	if snd.Melody != "" && usesC3(header, sections) {
		layout = fmt.Sprintf("Channels: C1 is the top staff (the melody, played by %s), C2 and C3 the two piano staves.", snd.Melody)
	}
	comment := fmt.Sprintf("%s\nConverted by muse-convert from %s.\n%s", header.Name, strings.Join(base, ", "), layout)
	if snd.Volume == 0 {
		snd.Volume = AutoVolume(Render("", header, snd, sections...))
	}
	return Render(comment, header, snd, sections...)
}

// page transcribes one input file, asking the model to fix whatever fails
// the checks up to c.Retries times. It returns the last attempt and the
// problems it still has.
func (c *Converter) page(ctx context.Context, file string, index, total int, prev *pageContext, snd Sound) (p Page, probs []string, err error) {
	source, err := sourceBlock(file)
	if err != nil {
		return
	}
	msgs := []anthropic.MessageParam{anthropic.NewUserMessage(
		source,
		anthropic.NewTextBlock(pageRequest(filepath.Base(file), index, total, prev)),
	)}

	for attempt := 0; ; attempt++ {
		var reply anthropic.Message
		if reply, err = c.sendRetrying(ctx, msgs); err != nil {
			return
		}
		text := replyText(reply)

		var perr error
		p, perr = SplitPage(ExtractYAML(text))
		if perr != nil {
			probs = []string{perr.Error()}
		} else {
			// check this page on its own, with the first page's header
			h := p.Header
			if prev != nil {
				h = prev.header
			}
			probs = Problems(Render("", h, withVolume(snd), p.Sections))
			if len(probs) == 0 {
				return p, nil, nil
			}
		}

		if attempt >= c.Retries {
			if perr != nil {
				err = fmt.Errorf("no usable transcription after %d attempts: %v", attempt+1, perr)
			}
			return
		}
		c.logf("  %d problem(s), asking for a fix: %s", len(probs), summarise(probs))
		msgs = append(msgs, reply.ToParam(), anthropic.NewUserMessage(anthropic.NewTextBlock(fixRequest(probs))))
	}
}

// sendAttempts is how many times one request is tried. The SDK retries
// failures before a stream starts; this also covers errors that arrive
// mid-stream, such as a proxy's intermittent content filter.
const sendAttempts = 3

// sendRetrying is send, retried on failure
func (c *Converter) sendRetrying(ctx context.Context, msgs []anthropic.MessageParam) (msg anthropic.Message, err error) {
	for try := 1; ; try++ {
		if msg, err = c.send(ctx, msgs); err == nil || ctx.Err() != nil || try == sendAttempts {
			return
		}
		c.logf("  request failed (%v), retrying", err)
	}
}

// send streams one request and returns the accumulated reply
func (c *Converter) send(ctx context.Context, msgs []anthropic.MessageParam) (msg anthropic.Message, err error) {
	params := anthropic.MessageNewParams{
		Model:        anthropic.Model(c.Model),
		MaxTokens:    maxTokens,
		System:       []anthropic.TextBlockParam{{Text: systemPrompt}},
		Messages:     msgs,
		CacheControl: anthropic.NewCacheControlEphemeralParam(),
	}
	if c.Effort != "" {
		params.OutputConfig = anthropic.OutputConfigParam{Effort: anthropic.OutputConfigEffort(c.Effort)}
	}

	stream := c.Client.Messages.NewStreaming(ctx, params)
	defer stream.Close()
	for stream.Next() {
		if err = msg.Accumulate(stream.Current()); err != nil {
			return
		}
	}
	if err = stream.Err(); err != nil {
		var apierr *anthropic.Error
		if errors.As(err, &apierr) {
			err = fmt.Errorf("API error %d: %s", apierr.StatusCode, strings.TrimSpace(apierr.RawJSON()))
		}
		return
	}

	switch msg.StopReason {
	case anthropic.StopReasonMaxTokens:
		err = fmt.Errorf("the transcription ran past %d tokens - split the input into fewer pages per file", maxTokens)
	case anthropic.StopReasonRefusal:
		err = fmt.Errorf("the model declined the request (%s)", msg.StopDetails.Category)
	}
	return
}

// imageTypes maps the image file extensions the API accepts to media types
var imageTypes = map[string]string{
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".gif":  "image/gif",
	".webp": "image/webp",
}

// sourceBlock reads an image or PDF into a content block
func sourceBlock(file string) (anthropic.ContentBlockParamUnion, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return anthropic.ContentBlockParamUnion{}, err
	}
	ext := strings.ToLower(filepath.Ext(file))
	if ext == ".pdf" {
		b64 := base64.StdEncoding.EncodeToString(data)
		return anthropic.NewDocumentBlock(anthropic.Base64PDFSourceParam{Data: b64}), nil
	}
	if mediaType, ok := imageTypes[ext]; ok {
		if data, mediaType, err = fitImage(data, mediaType); err != nil {
			return anthropic.ContentBlockParamUnion{}, err
		}
		return anthropic.NewImageBlockBase64(mediaType, base64.StdEncoding.EncodeToString(data)), nil
	}
	return anthropic.ContentBlockParamUnion{}, fmt.Errorf("unsupported file type %q - use a PDF, PNG, JPEG, GIF or WebP", filepath.Ext(file))
}

// nextContext summarises the pages so far for the next page's request
func nextContext(h Header, sections []string) *pageContext {
	all := strings.Join(sections, "\n")
	doc := Render("", h, Sound{Instrument: "piano", Volume: 1}, all)
	pc := &pageContext{header: h, channels: []string{"C1"}}

	// count bars and find where the last two start
	lines := strings.Split(all, "\n")
	var starts []int
	for i, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "- ") && !strings.HasPrefix(l, "    ") {
			starts = append(starts, i)
		}
	}
	pc.bars = len(starts)
	if n := len(starts); n > 0 {
		from := starts[max(0, n-2)]
		// keep the "# bar" comment above the first kept bar
		if from > 0 && strings.HasPrefix(strings.TrimSpace(lines[from-1]), "#") {
			from--
		}
		pc.tail = strings.Join(lines[from:], "\n")
	}
	if used := usedChannelsIn(doc); len(used) > 0 {
		pc.channels = used
	}
	return pc
}

func replyText(m anthropic.Message) string {
	var b strings.Builder
	for _, block := range m.Content {
		if t, ok := block.AsAny().(anthropic.TextBlock); ok {
			b.WriteString(t.Text)
		}
	}
	return b.String()
}

// withVolume gives a Sound a placeholder volume so a page can be checked
// before the real volume is picked
func withVolume(s Sound) Sound {
	if s.Volume == 0 {
		s.Volume = 1000
	}
	return s
}

func summarise(probs []string) string {
	s := probs[0]
	if len(probs) > 1 {
		s += fmt.Sprintf(" (+%d more)", len(probs)-1)
	}
	return s
}

func (c *Converter) logf(format string, args ...any) {
	if c.Log != nil {
		fmt.Fprintf(c.Log, format+"\n", args...)
	}
}

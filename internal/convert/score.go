package convert

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/sausheong/gomuse/internal/muse"
	"gopkg.in/yaml.v3"
)

// beatTolerance is how far apart two channels of one section may be, in
// beats, before they count as different lengths. Triplets are written as
// 0.333 or 0.334, so three of them don't always add up to exactly 1.
const beatTolerance = 0.02

// channels lists every channel a section can have, in order
var channels = []string{"C1", "C2", "C3"}

// Header holds the score-level settings the model reads from the page
type Header struct {
	Name   string  `yaml:"name"`
	Key    string  `yaml:"key"`
	Length float64 `yaml:"length"`
}

// Sound holds the score-level settings chosen on the command line
type Sound struct {
	Instrument string
	Envelope   string
	Harmonic   string
	// Melody is the instrument for the top staff (C1) when the music has
	// three staves - a voice or lead over a piano. Empty leaves C1 on
	// Instrument.
	Melody string
	Volume int // 0 picks one from the widest chords, see AutoVolume
}

// Page is the model's transcription of one input file
type Page struct {
	Header   Header
	Sections string // the YAML list items under "sections:", comments kept
}

// ExtractYAML returns the last ```yaml fenced block in a reply, or the whole
// reply if there is no fence.
func ExtractYAML(reply string) string {
	const fence = "```"
	end := strings.LastIndex(reply, fence)
	if end < 0 {
		return strings.TrimSpace(reply)
	}
	start := strings.LastIndex(reply[:end], fence)
	if start < 0 {
		return strings.TrimSpace(reply)
	}
	block := reply[start+len(fence) : end]
	// drop the info string ("yaml") on the opening fence line
	if nl := strings.IndexByte(block, '\n'); nl >= 0 {
		block = block[nl+1:]
	}
	return strings.TrimSpace(block)
}

// SplitPage splits a transcribed page into its header and the raw text of
// its sections list.
func SplitPage(doc string) (p Page, err error) {
	lines := strings.Split(doc, "\n")
	at := -1
	for i, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "sections:") && !strings.HasPrefix(l, " ") {
			at = i
			break
		}
	}
	if at < 0 {
		return p, fmt.Errorf("no top-level 'sections:' key in the transcription")
	}
	if rest := strings.TrimSpace(strings.TrimPrefix(lines[at], "sections:")); rest != "" {
		return p, fmt.Errorf("write sections as a block list, one '- ' item per bar, not %q", rest)
	}
	if err = yaml.Unmarshal([]byte(strings.Join(lines[:at], "\n")), &p.Header); err != nil {
		return p, fmt.Errorf("cannot read the score header > %v", err)
	}
	p.Sections = strings.TrimRight(strings.Join(lines[at+1:], "\n"), " \n")
	return p, nil
}

// Render builds a complete score document from a header, the sound settings
// and the sections text of one or more pages.
func Render(comment string, h Header, snd Sound, sections ...string) string {
	var b strings.Builder
	for _, l := range strings.Split(strings.TrimSpace(comment), "\n") {
		if l != "" {
			fmt.Fprintf(&b, "# %s\n", l)
		}
	}
	name, _ := yaml.Marshal(h.Name)
	fmt.Fprintf(&b, "name: %s", name)
	fmt.Fprintf(&b, "key: %s\n", h.Key)
	length := strconv.FormatFloat(h.Length, 'f', -1, 64)
	if !strings.Contains(length, ".") {
		length += ".0"
	}
	fmt.Fprintf(&b, "length: %s\n", length)
	if snd.Instrument != "" {
		fmt.Fprintf(&b, "instrument: %s\n", snd.Instrument)
	} else {
		fmt.Fprintf(&b, "envelope: %s\n", snd.Envelope)
		fmt.Fprintf(&b, "harmonic: %s\n", snd.Harmonic)
	}
	fmt.Fprintf(&b, "volume: %d\n", snd.Volume)
	if snd.Melody != "" && usesC3(h, sections) {
		fmt.Fprintf(&b, "instruments: {C1: %s}\n", snd.Melody)
	}
	b.WriteString("sections:\n")
	for _, s := range sections {
		if s = strings.TrimRight(s, " \n"); s != "" {
			b.WriteString(s)
			b.WriteString("\n")
		}
	}
	return b.String()
}

// usesC3 reports whether the sections use the third channel, which is how
// a score of voice and piano (three staves) is told from one for piano alone
func usesC3(h Header, sections []string) bool {
	doc := Render("", h, Sound{Instrument: "piano", Volume: 1}, sections...)
	return slices.Contains(usedChannelsIn(doc), "C3")
}

// Problems checks a complete score document and returns everything wrong
// with it, as sentences the model can act on. It checks that the score
// parses, that muse accepts every note, that each channel used anywhere is
// used in every section, and that all channels of a section last the same
// number of beats.
func Problems(doc string) []string {
	var s muse.Score
	if err := yaml.Unmarshal([]byte(doc), &s); err != nil {
		return []string{fmt.Sprintf("the YAML does not parse: %v", err)}
	}
	var probs []string
	if len(s.Sections) == 0 {
		probs = append(probs, "there are no sections")
	}
	if err := muse.Check(&s); err != nil {
		probs = append(probs, strings.TrimSpace(err.Error()))
	}

	used := usedChannels(s)
	for i, sec := range s.Sections {
		where := fmt.Sprintf("section %d", i+1)
		notes := channelNotes(sec)
		beats := map[string]float64{}
		for _, c := range used {
			if len(notes[c]) == 0 {
				probs = append(probs, fmt.Sprintf("%s: %s is empty but the score uses %s elsewhere - fill the bar with rests, e.g. '4:z'", where, c, c))
				continue
			}
			b, err := Beats(notes[c])
			if err != nil {
				probs = append(probs, fmt.Sprintf("%s: %s: %v", where, c, err))
				continue
			}
			beats[c] = b
		}
		if len(beats) < 2 {
			continue
		}
		lo, hi := math.Inf(1), math.Inf(-1)
		for _, b := range beats {
			lo, hi = math.Min(lo, b), math.Max(hi, b)
		}
		if hi-lo > beatTolerance {
			var parts []string
			for _, c := range used {
				if b, ok := beats[c]; ok {
					parts = append(parts, fmt.Sprintf("%s = %s beats", c, strconv.FormatFloat(b, 'f', -1, 64)))
				}
			}
			probs = append(probs, fmt.Sprintf("%s: channels have different lengths (%s); every channel in a bar must add up to the same number of beats", where, strings.Join(parts, ", ")))
		}
	}
	return probs
}

// Beats adds up the lengths of a list of notes, in beats
func Beats(notes []string) (total float64, err error) {
	for _, n := range notes {
		l := 1.0
		if prefix, _, ok := strings.Cut(n, ":"); ok {
			if l, err = strconv.ParseFloat(prefix, 64); err != nil {
				return 0, fmt.Errorf("bad length in %q", n)
			}
		}
		total += l
	}
	return math.Round(total*1000) / 1000, nil
}

// AutoVolume picks a volume that keeps the loudest moment of the score
// below clipping. Notes in a chord add up, so the worst case is the widest
// C1 or C2 chord plus the widest C3 chord, each turned down by muse's part
// balance (C2 to 0.8 and C3 to 0.7 of C1 when there's more than one part).
// The default stereo layout is somewhat louder than plain mixing, since a
// centred channel (C1) goes to both sides at about 1.41x, and C3 sits at
// about 1.23x on its own side. A worst case with four-note chords on all
// three channels peaked near 22600 of 32767 against 17700 plain, so the
// 30000 budget still absorbs it; if that budget is raised, scale the C1
// load by 1.41 when the pans are active.
func AutoVolume(doc string) int {
	var s muse.Score
	if err := yaml.Unmarshal([]byte(doc), &s); err != nil {
		return 4000
	}
	widest := map[string]int{}
	for _, sec := range s.Sections {
		for c, notes := range channelNotes(sec) {
			for _, n := range notes {
				_, p, ok := strings.Cut(n, ":")
				if !ok {
					p = n
				}
				if p == "z" {
					continue
				}
				widest[c] = max(widest[c], strings.Count(p, "-")+1)
			}
		}
	}
	w1, w2, w3 := float64(widest["C1"]), float64(widest["C2"]), float64(widest["C3"])
	if w2+w3 > 0 {
		w2, w3 = w2*0.8, w3*0.7
	}
	load := max(w1, w2) + w3
	if load == 0 {
		return 4000
	}
	v := int(30000 / load)
	return min(8000, v/100*100)
}

// usedChannelsIn lists the channels a score document uses
func usedChannelsIn(doc string) []string {
	var s muse.Score
	if err := yaml.Unmarshal([]byte(doc), &s); err != nil {
		return nil
	}
	return usedChannels(s)
}

// usedChannels lists the channels that have notes in any section
func usedChannels(s muse.Score) []string {
	var used []string
	for _, c := range channels {
		for _, sec := range s.Sections {
			if len(channelNotes(sec)[c]) > 0 {
				used = append(used, c)
				break
			}
		}
	}
	return used
}

func channelNotes(sec muse.Section) map[string][]string {
	return map[string][]string{"C1": sec.C1, "C2": sec.C2, "C3": sec.C3}
}

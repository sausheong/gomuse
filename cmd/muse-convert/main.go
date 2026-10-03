// Command muse-convert transcribes sheet music (images or PDFs) into a Muse
// YAML score using Claude's vision. Each staff of a system becomes a
// channel: the top staff C1, the next C2 and, when there are three staves,
// the third C3.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/sausheong/gomuse/internal/convert"
)

const defaultModel = "claude-opus-5-5"

func main() {
	out := flag.String("o", "", "output score `file` (default: <first input>.yaml)")
	envFile := flag.String("env", ".env", "`file` with ANTHROPIC_* settings; real environment variables win")
	model := flag.String("model", "", "Claude model (default: $ANTHROPIC_MODEL, else "+defaultModel+")")
	effort := flag.String("effort", "high", "thinking effort: low, medium, high, xhigh or max; empty for the model's default")
	retries := flag.Int("retries", 2, "times to ask the model to fix a page that fails the checks")
	name := flag.String("name", "", "score name (default: the title on the music)")
	instrument := flag.String("instrument", "piano", "instrument; set to empty to use -envelope and -harmonic")
	melody := flag.String("melody", "voice", "instrument for the top staff when there are three staves (voice and piano); empty to use -instrument")
	envelope := flag.String("envelope", "tempered", "envelope, used when -instrument is empty")
	harmonic := flag.String("harmonic", "stringed", "harmonic, used when -instrument is empty")
	volume := flag.Int("volume", 0, "volume (default: the loudest that won't clip)")
	quiet := flag.Bool("q", false, "don't print progress")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: muse-convert [flags] <page>...")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Transcribes sheet music into a Muse YAML score. Pages are PDF, PNG, JPEG,")
		fmt.Fprintln(os.Stderr, "GIF or WebP files of one piece, given in order. The top staff becomes C1,")
		fmt.Fprintln(os.Stderr, "the next C2 and, if there are three staves, the third becomes C3.")
		fmt.Fprintln(os.Stderr, "")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(2)
	}
	files := flag.Args()

	if err := convert.LoadEnv(*envFile); err != nil {
		fail("cannot read %s - %v", *envFile, err)
	}
	if *model == "" {
		*model = os.Getenv("ANTHROPIC_MODEL")
	}
	if *model == "" {
		*model = defaultModel
	}
	opts := []option.RequestOption{option.WithRequestTimeout(30 * time.Minute)}
	if base := os.Getenv("ANTHROPIC_BASE_URL"); base != "" {
		opts = append(opts, option.WithBaseURL(base))
	}
	switch {
	case os.Getenv("ANTHROPIC_API_KEY") != "":
		opts = append(opts, option.WithAPIKey(os.Getenv("ANTHROPIC_API_KEY")))
	case os.Getenv("ANTHROPIC_AUTH_TOKEN") != "":
		opts = append(opts, option.WithAuthToken(os.Getenv("ANTHROPIC_AUTH_TOKEN")))
	default:
		fail("no credentials - set ANTHROPIC_API_KEY or ANTHROPIC_AUTH_TOKEN (in the environment or %s)", *envFile)
	}

	if *out == "" {
		first := files[0]
		*out = strings.TrimSuffix(filepath.Base(first), filepath.Ext(first)) + ".yaml"
	}

	c := &convert.Converter{
		Client:  anthropic.NewClient(opts...),
		Model:   *model,
		Effort:  *effort,
		Retries: *retries,
	}
	if !*quiet {
		c.Log = os.Stderr
	}
	snd := convert.Sound{Instrument: *instrument, Envelope: *envelope, Harmonic: *harmonic, Melody: *melody, Volume: *volume}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	t := time.Now()
	res, err := c.Convert(ctx, files, snd, *name)
	if err != nil && res.YAML == "" {
		fail("%v", err)
	}
	if werr := os.WriteFile(*out, []byte(res.YAML), 0o644); werr != nil {
		fail("cannot write %s - %v", *out, werr)
	}

	fmt.Printf("Wrote %s in %s\n", *out, time.Since(t).Round(time.Second))
	if err != nil {
		fail("stopped early, so the score only has the pages before this one - %v", err)
	}
	if len(res.Problems) > 0 {
		fmt.Fprintf(os.Stderr, "%d problem(s) remain - fix them by hand before rendering:\n", len(res.Problems))
		for _, p := range res.Problems {
			fmt.Fprintln(os.Stderr, "  -", p)
		}
		os.Exit(1)
	}
	fmt.Printf("Render it with: muse %s\n", strings.TrimSuffix(*out, ".yaml"))
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "muse-convert: "+format+"\n", args...)
	os.Exit(1)
}

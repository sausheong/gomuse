package muse

import (
	"io/fs"

	"github.com/sausheong/gomuse/scores"

	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-audio/wav"
)

// -- pitch and frequency --------------------------------------------------

func TestFrequency(t *testing.T) {
	if f := frequency(0); f != 440.0 {
		t.Fatalf("frequency(0) = %v, want 440", f)
	}
	if pitch["a4"] != 0 {
		t.Fatalf("pitch[a4] = %d, want 0", pitch["a4"])
	}
	if pitch["c4"] != -9 {
		t.Fatalf("pitch[c4] = %d, want -9", pitch["c4"])
	}
	if got, want := frequency(pitch["a4"]), 440.0; got != want {
		t.Fatalf("frequency(pitch[a4]) = %v, want %v", got, want)
	}
}

// -- exact sample counts ---------------------------------------------------

func TestNoteDataSampleCount(t *testing.T) {
	data := noteData(440.0, 1.0, flat, first, 1000)
	if want := sampleCount(1.0); len(data) != want {
		t.Fatalf("noteData length = %d, want %d", len(data), want)
	}

	data = noteData(440.0, 0.5, flat, first, 1000)
	if want := sampleCount(0.5); len(data) != want {
		t.Fatalf("noteData length = %d, want %d", len(data), want)
	}
}

func TestRestSampleCount(t *testing.T) {
	data := rest(0.5)
	want := sampleCount(0.5)
	if len(data) != want {
		t.Fatalf("rest length = %d, want %d", len(data), want)
	}
	for i, v := range data {
		if v != 0 {
			t.Fatalf("rest sample %d = %d, want 0", i, v)
		}
	}
}

// -- chord summation ---------------------------------------------------

func TestConcatSumsChords(t *testing.T) {
	a := []int{1, 2, 3}
	b := []int{10, 20, 30}
	c := []int{100, 200, 300}
	data, err := concat(a, b, c)
	if err != nil {
		t.Fatalf("concat returned error: %v", err)
	}
	want := []int{111, 222, 333}
	for i := range want {
		if data[i] != want[i] {
			t.Fatalf("concat[%d] = %d, want %d", i, data[i], want[i])
		}
	}
}

func TestConcatMismatchedLengths(t *testing.T) {
	_, err := concat([]int{1, 2}, []int{1, 2, 3})
	if err == nil {
		t.Fatal("expected error for mismatched note lengths, got nil")
	}
}

func TestChainConcatenates(t *testing.T) {
	data, err := chain([]int{1, 2}, []int{3, 4, 5})
	if err != nil {
		t.Fatalf("chain returned error: %v", err)
	}
	want := []int{1, 2, 3, 4, 5}
	if len(data) != len(want) {
		t.Fatalf("chain length = %d, want %d", len(data), len(want))
	}
	for i := range want {
		if data[i] != want[i] {
			t.Fatalf("chain[%d] = %d, want %d", i, data[i], want[i])
		}
	}
}

// -- clipping ---------------------------------------------------------

func TestClampLimitsRange(t *testing.T) {
	cases := []struct{ in, want int }{
		{100, 100},
		{-100, -100},
		{maxSample, maxSample},
		{minSample, minSample},
		{maxSample + 1, maxSample},
		{minSample - 1, minSample},
		{1000000, maxSample},
		{-1000000, minSample},
	}
	for _, c := range cases {
		if got := clamp(c.in); got != c.want {
			t.Fatalf("clamp(%d) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestStereoClampsSummedChords(t *testing.T) {
	// a "chord" that overflows 16 bits once summed
	c1, err := concat([]int{20000, 20000}, []int{20000, 20000})
	if err != nil {
		t.Fatalf("concat returned error: %v", err)
	}
	data, err := stereo(c1, c1)
	if err != nil {
		t.Fatalf("stereo returned error: %v", err)
	}
	for i, v := range data {
		if v > maxSample || v < minSample {
			t.Fatalf("sample %d = %d, outside 16-bit range", i, v)
		}
		if v != maxSample {
			t.Fatalf("sample %d = %d, want clamped to %d", i, v, maxSample)
		}
	}
}

func TestStereoDoesNotAlterInRangeAudio(t *testing.T) {
	c1 := []int{100, -200, 300}
	c2 := []int{-50, 60, -70}
	data, err := stereo(c1, c2)
	if err != nil {
		t.Fatalf("stereo returned error: %v", err)
	}
	want := []int{100, -50, -200, 60, 300, -70}
	for i := range want {
		if data[i] != want[i] {
			t.Fatalf("data[%d] = %d, want %d", i, data[i], want[i])
		}
	}
}

func TestStereoRejectsTooDifferentChannels(t *testing.T) {
	c1 := make([]int, 100)
	c2 := make([]int, 100+tolerance+1)
	if _, err := stereo(c1, c2); err == nil {
		t.Fatal("expected error for channel lengths too different, got nil")
	}
}

// -- bad durations ---------------------------------------------------

func TestMakeNoteBadDuration(t *testing.T) {
	score := Score{Key: "C", Length: 1.0, Envelope: "flat", Harmonic: "first", Volume: 1000}
	section := Section{}

	cases := []string{"x:c4", "NaN:c4", "Inf:c4", "0:c4", "-1:c4"}
	for _, c := range cases {
		if _, err := makeNote(c, section, score); err == nil {
			t.Fatalf("makeNote(%q) expected error, got nil", c)
		}
	}
}

func TestMakeNoteGoodDuration(t *testing.T) {
	score := Score{Key: "C", Length: 1.0, Envelope: "flat", Harmonic: "first", Volume: 1000}
	section := Section{}
	n, err := makeNote("2:c4", section, score)
	if err != nil {
		t.Fatalf("makeNote returned error: %v", err)
	}
	if n.length != 2.0 {
		t.Fatalf("note length = %v, want 2.0", n.length)
	}
}

// -- bad notes ---------------------------------------------------

func TestProcessBadNotes(t *testing.T) {
	cases := []string{"c4x", "h4", "c"}
	for _, c := range cases {
		n := note{}
		if err := process(&n, c); err == nil {
			t.Fatalf("process(%q) expected error, got nil", c)
		}
	}
}

func TestProcessRest(t *testing.T) {
	n := note{}
	if err := process(&n, "z"); err != nil {
		t.Fatalf("process(z) returned error: %v", err)
	}
	if len(n.pitch) != 0 {
		t.Fatalf("rest should not add a pitch, got %v", n.pitch)
	}
}

func TestProcessAccidentals(t *testing.T) {
	n := note{}
	for _, tok := range []string{"f4", "f4#", "f4b", "f4n"} {
		if err := process(&n, tok); err != nil {
			t.Fatalf("process(%q) returned error: %v", tok, err)
		}
	}
	wantAcc := []int{0, 1, -1, 0}
	wantExplicit := []bool{false, true, true, true}
	for i := range wantAcc {
		if n.accidental[i] != wantAcc[i] {
			t.Fatalf("accidental[%d] = %d, want %d", i, n.accidental[i], wantAcc[i])
		}
		if n.explicit[i] != wantExplicit[i] {
			t.Fatalf("explicit[%d] = %v, want %v", i, n.explicit[i], wantExplicit[i])
		}
	}
}

func TestChordDetection(t *testing.T) {
	score := Score{Key: "C", Length: 1.0, Envelope: "flat", Harmonic: "first", Volume: 1000}
	section := Section{}

	// single note with an accidental is not a chord
	n, err := makeNote("c4#", section, score)
	if err != nil {
		t.Fatalf("makeNote(c4#) returned error: %v", err)
	}
	if len(n.pitch) != 1 {
		t.Fatalf("c4# should be a single note, got %d pitches", len(n.pitch))
	}

	// a chord of plain notes
	n, err = makeNote("c4-e4", section, score)
	if err != nil {
		t.Fatalf("makeNote(c4-e4) returned error: %v", err)
	}
	if len(n.pitch) != 2 {
		t.Fatalf("c4-e4 should be a chord of 2, got %d pitches", len(n.pitch))
	}

	// a chord of notes with accidentals
	n, err = makeNote("c4#-e4b", section, score)
	if err != nil {
		t.Fatalf("makeNote(c4#-e4b) returned error: %v", err)
	}
	if len(n.pitch) != 2 {
		t.Fatalf("c4#-e4b should be a chord of 2, got %d pitches", len(n.pitch))
	}
	if n.accidental[0] != 1 || n.accidental[1] != -1 {
		t.Fatalf("c4#-e4b accidentals = %v, want [1 -1]", n.accidental)
	}
}

// -- unknown envelope / harmonic / key ---------------------------------------------------

func TestMakeNoteUnknownEnvelope(t *testing.T) {
	score := Score{Key: "C", Length: 1.0, Envelope: "nope", Harmonic: "first", Volume: 1000}
	_, err := makeNote("c4", Section{}, score)
	if err == nil || !strings.Contains(err.Error(), "nope") {
		t.Fatalf("expected error mentioning envelope name 'nope', got %v", err)
	}
}

func TestMakeNoteUnknownHarmonic(t *testing.T) {
	score := Score{Key: "C", Length: 1.0, Envelope: "flat", Harmonic: "nope", Volume: 1000}
	_, err := makeNote("c4", Section{}, score)
	if err == nil || !strings.Contains(err.Error(), "nope") {
		t.Fatalf("expected error mentioning harmonic name 'nope', got %v", err)
	}
}

func TestUnknownKeySignature(t *testing.T) {
	tu := tune{
		key: "H",
		ch1: []note{{pitch: []int{pitch["c4"]}, accidental: []int{0}, explicit: []bool{false}, length: 0.1, env: flat, har: first, vol: 100}},
		ch2: []note{},
	}
	_, err := tu.encode()
	if err == nil || !strings.Contains(err.Error(), "H") {
		t.Fatalf("expected error mentioning unknown key 'H', got %v", err)
	}
}

// -- key signatures across octaves, naturals and explicit accidentals ------------------------

func TestKeySignatureCoversAllOctaves(t *testing.T) {
	// D major has F# and C#; before the fix this only covered octaves 2-6
	if !inNote(tuneKey["D"], pitch["f1"]) {
		t.Fatal("key D should affect F in octave 1")
	}
	if !inNote(tuneKey["D"], pitch["f7"]) {
		t.Fatal("key D should affect F in octave 7")
	}
	if !inNote(tuneKey["D"], pitch["c1"]) {
		t.Fatal("key D should affect C in octave 1")
	}
	if !inNote(tuneKey["D"], pitch["c7"]) {
		t.Fatal("key D should affect C in octave 7")
	}
}

func TestKeySignatureAppliesWithoutExplicitAccidental(t *testing.T) {
	n := note{pitch: []int{pitch["f4"]}, accidental: []int{0}, explicit: []bool{false}, length: 0.1, env: flat, har: first, vol: 100}
	tu := tune{key: "D", ch1: []note{n}, ch2: []note{}}
	if _, err := tu.encode(); err != nil {
		t.Fatalf("encode returned error: %v", err)
	}
	if tu.ch1[0].accidental[0] != 1 {
		t.Fatalf("plain f4 in key D should become sharp, accidental = %d", tu.ch1[0].accidental[0])
	}
}

func TestExplicitAccidentalReplacesKeyAccidental(t *testing.T) {
	// explicit flat on a note the key would otherwise sharpen: stays flat,
	// it must not stack into a natural (0) or worse
	flatNote := note{pitch: []int{pitch["f4"]}, accidental: []int{-1}, explicit: []bool{true}, length: 0.1, env: flat, har: first, vol: 100}
	// explicit sharp on a note the key would otherwise sharpen: stays a
	// single sharp, it must not stack into a double-sharp
	sharpNote := note{pitch: []int{pitch["f4"]}, accidental: []int{1}, explicit: []bool{true}, length: 0.1, env: flat, har: first, vol: 100}
	// an explicit natural cancels the key signature entirely
	naturalNote := note{pitch: []int{pitch["f4"]}, accidental: []int{0}, explicit: []bool{true}, length: 0.1, env: flat, har: first, vol: 100}

	tu := tune{key: "D", ch1: []note{flatNote, sharpNote, naturalNote}, ch2: []note{}}
	if _, err := tu.encode(); err != nil {
		t.Fatalf("encode returned error: %v", err)
	}
	if got := tu.ch1[0].accidental[0]; got != -1 {
		t.Fatalf("explicit flat should stay flat, got %d", got)
	}
	if got := tu.ch1[1].accidental[0]; got != 1 {
		t.Fatalf("explicit sharp should stay a single sharp, got %d", got)
	}
	if got := tu.ch1[2].accidental[0]; got != 0 {
		t.Fatalf("explicit natural should cancel the key, got %d", got)
	}
}

// -- maxSamples rejection ---------------------------------------------------

func TestParseRejectsTooLongScore(t *testing.T) {
	score := []byte(`
name: Huge
key: C
length: 1.0
envelope: flat
harmonic: first
volume: 100
sections:
  - C1: ["1000000:c4"]
    C2: []
`)
	var s Score
	out := filepath.Join(t.TempDir(), "huge")
	_, err := Parse(&s, score, out, 1000)
	if err == nil {
		t.Fatal("expected a 'too long' error, got nil")
	}
	if !strings.Contains(err.Error(), "too long") {
		t.Fatalf("expected error to mention 'too long', got %v", err)
	}
	if _, statErr := os.Stat(out + ".wav"); statErr == nil {
		t.Fatal("no wav file should have been written for a rejected score")
	}
}

func TestParseUnlimitedWhenMaxSamplesIsZero(t *testing.T) {
	score := []byte(`
name: Small
key: C
length: 0.01
envelope: flat
harmonic: first
volume: 100
sections:
  - C1: ["c4"]
    C2: ["c4"]
`)
	var s Score
	out := filepath.Join(t.TempDir(), "small")
	if _, err := Parse(&s, score, out, 0); err != nil {
		t.Fatalf("Parse with maxSamples=0 should be unlimited, got error: %v", err)
	}
}

// -- chord width (a wide chord multiplies allocation, not just duration) ----

// channelSamples must count a chord's width, not just its duration:
// note.encode (encoder.go) allocates one full-length sample slice per pitch
// in a chord, so a chord of N pitches costs N times what a single note of
// the same duration costs.
func TestChannelSamplesAccountsForChordWidth(t *testing.T) {
	score := Score{Key: "C", Length: 1.0, Envelope: "flat", Harmonic: "first", Volume: 100}
	single, err := makeNote("1:c4", Section{}, score)
	if err != nil {
		t.Fatalf("makeNote returned error: %v", err)
	}
	chord, err := makeNote("1:c4-e4-g4", Section{}, score)
	if err != nil {
		t.Fatalf("makeNote returned error: %v", err)
	}
	single1 := channelSamples([]note{single})
	chord3 := channelSamples([]note{chord})
	if chord3 != 3*single1 {
		t.Fatalf("channelSamples for a 3-note chord = %d, want %d (3x a single note)", chord3, 3*single1)
	}
}

// a chord wide enough to allocate an unreasonable amount of memory must be
// rejected outright, regardless of how short its duration is - this is what
// actually bounds encode()'s allocation, since duration alone does not.
func TestMakeNoteRejectsChordTooWide(t *testing.T) {
	pitches := make([]string, maxChordPitches+1)
	for i := range pitches {
		pitches[i] = "c4"
	}
	wide := strings.Join(pitches, "-")

	score := Score{Key: "C", Length: 0.01, Envelope: "flat", Harmonic: "first", Volume: 100}
	if _, err := makeNote(wide, Section{}, score); err == nil {
		t.Fatal("expected an error for a chord wider than maxChordPitches, got nil")
	}
}

func TestMakeNoteAllowsChordAtMaxWidth(t *testing.T) {
	pitches := make([]string, maxChordPitches)
	for i := range pitches {
		pitches[i] = "c4"
	}
	ok := strings.Join(pitches, "-")

	score := Score{Key: "C", Length: 0.01, Envelope: "flat", Harmonic: "first", Volume: 100}
	n, err := makeNote(ok, Section{}, score)
	if err != nil {
		t.Fatalf("makeNote at exactly maxChordPitches should succeed, got error: %v", err)
	}
	if len(n.pitch) != maxChordPitches {
		t.Fatalf("chord width = %d, want %d", len(n.pitch), maxChordPitches)
	}
}

// end-to-end: a tiny, well-under-the-request-body-cap score with a very wide
// chord must be rejected by Parse before any audio is synthesised, rather
// than allocating memory out of proportion to its declared sample count.
func TestParseRejectsWideChordEvenWhenShort(t *testing.T) {
	pitches := make([]string, 5000)
	for i := range pitches {
		pitches[i] = "c4"
	}
	score := []byte(`
name: Wide chord
key: C
length: 0.01
envelope: flat
harmonic: first
volume: 100
sections:
  - C1: ["` + strings.Join(pitches, "-") + `"]
    C2: []
`)
	var s Score
	out := filepath.Join(t.TempDir(), "wide")
	if _, err := Parse(&s, score, out, 6000000); err == nil {
		t.Fatal("expected an error for an oversized chord, got nil")
	}
	if _, statErr := os.Stat(out + ".wav"); statErr == nil {
		t.Fatal("no wav file should have been written for a rejected score")
	}
}

// -- every sample score parses and produces a valid WAV ---------------------------------------------------

func TestSampleScoresProduceValidWAV(t *testing.T) {
	files, err := fs.Glob(scores.FS, "*.yaml")
	if err != nil {
		t.Fatalf("glob failed: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("no sample scores embedded from scores/")
	}

	for _, f := range files {
		t.Run(f, func(t *testing.T) {
			data, err := fs.ReadFile(scores.FS, f)
			if err != nil {
				t.Fatalf("cannot read %s: %v", f, err)
			}

			var s Score
			out := filepath.Join(t.TempDir(), "out")
			if _, err := Parse(&s, data, out, 0); err != nil {
				t.Fatalf("Parse(%s) returned error: %v", f, err)
			}

			wf, err := os.Open(out + ".wav")
			if err != nil {
				t.Fatalf("cannot open generated wav: %v", err)
			}
			defer wf.Close()

			dec := wav.NewDecoder(wf)
			buf, err := dec.FullPCMBuffer()
			if err != nil {
				t.Fatalf("cannot decode generated wav: %v", err)
			}
			if buf.Format.SampleRate != sampleRate {
				t.Fatalf("sample rate = %d, want %d", buf.Format.SampleRate, sampleRate)
			}
			if buf.Format.NumChannels != numChannels {
				t.Fatalf("num channels = %d, want %d", buf.Format.NumChannels, numChannels)
			}
			if buf.SourceBitDepth != bitDepth {
				t.Fatalf("bit depth = %d, want %d", buf.SourceBitDepth, bitDepth)
			}
			if len(buf.Data) == 0 {
				t.Fatal("generated wav has no samples")
			}
		})
	}
}

// sanity check that sampleCount rounds rather than truncates
func TestSampleCountRounds(t *testing.T) {
	got := sampleCount(1.0 / 3.0)
	want := int(math.Round((1.0 / 3.0) * float64(sampleRate)))
	if got != want {
		t.Fatalf("sampleCount = %d, want %d", got, want)
	}
}

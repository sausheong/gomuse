package muse

import (
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// timedTune parses yaml, builds the tune (running applyTiming) and
// returns the score and tune.
func timedTune(t *testing.T, src string) (*Score, tune) {
	t.Helper()
	var s Score
	if err := yaml.Unmarshal([]byte(src), &s); err != nil {
		t.Fatal(err)
	}
	tn, err := buildTune(&s)
	if err != nil {
		t.Fatal(err)
	}
	return &s, tn
}

func secs(n note) (start, end float64) {
	start = float64(n.start) / sampleRate
	return start, start + n.hold
}

const humanizeOff = "humanize: 0\n"
const tbase = "key: C\nlength: 0.5\nvolume: 1000\nenvelope: flat\nharmonic: first\n"

func TestTempoMapRitardandoIntegrates(t *testing.T) {
	s := &Score{Length: 1, Sections: []Section{{Ritardando: 0.5}, {}}}
	tn := tune{ch1: []note{{length: 4}, {length: 4}}}
	s.Sections[0].C1 = []string{"4:c4"}
	s.Sections[1].C1 = []string{"4:c4"}
	tm := buildTempoMap(s, &tn)
	// section 0: D(1+r/2) = 4*1.25
	if got := tm[0].duration; math.Abs(got-5) > 1e-9 {
		t.Fatalf("duration = %v, want 5", got)
	}
	// halfway through, written 2 takes 2 + 0.5*4/(8) = 2.25
	if got := tm[0].at(2, false); math.Abs(got-2.25) > 1e-9 {
		t.Fatalf("at(2) = %v, want 2.25", got)
	}
	// next section is back in tempo and starts at the end of the first
	if tm[1].perf != 5 {
		t.Fatalf("section 1 starts at %v, want 5", tm[1].perf)
	}
	// negative speeds up
	if d := (tempoSection{written: 4, rit: -0.2}).at(4, true); math.Abs(d-3.6) > 1e-9 {
		t.Fatalf("accelerando end = %v, want 1.8", d)
	}
}

func TestFermataAddsExactBeats(t *testing.T) {
	_, tn := timedTune(t, tbase+humanizeOff+`sections:
  - C1: [1:c4, 1:d4]
    fermata: 2
  - C1: [1:e4, 1:f4]
`)
	a, b := tn.ch1[1], tn.ch1[2]
	_, aEnd := secs(a)
	bStart, _ := secs(b)
	// section 0 is 2 s written, +2 beats of 0.5 s... length is the beat
	if want := 1.0 + 2*0.5; math.Abs(aEnd-want) > 1e-3 {
		t.Fatalf("held note ends at %v, want %v", aEnd, want)
	}
	if math.Abs(bStart-aEnd) > 1e-3 {
		t.Fatalf("next section starts at %v, want %v", bStart, aEnd)
	}
	// the note before the last one isn't held
	_, firstEnd := secs(tn.ch1[0])
	if math.Abs(firstEnd-0.5) > 1e-3 {
		t.Fatalf("first note ends at %v, want 1", firstEnd)
	}
}

func TestChannelsStayAligned(t *testing.T) {
	_, tn := timedTune(t, tbase+`swing: 0.6
sections:
  - C1: [1:c4, 1:d4]
    C2: [0.5:c3, 0.5:d3, 1:e3, 0.5:f3, 0.5:g3]
    C3: [2:c2, 2:g2]
    ritardando: 0.3
    fermata: 1
  - C1: [2:e4]
    C2: [2:e3]
    C3: [2:e2]
`)
	end := func(ch []note) float64 { _, e := secs(ch[len(ch)-1]); return e }
	e1, e2, e3 := end(tn.ch1), end(tn.ch2), end(tn.ch3)
	if math.Abs(e1-e2) > 1e-3 || math.Abs(e1-e3) > 1e-3 {
		t.Fatalf("channel ends differ: %v %v %v", e1, e2, e3)
	}
	// the section boundary lands at the same moment in every channel
	s1, _ := secs(tn.ch1[2])
	s2, _ := secs(tn.ch2[5])
	s3, _ := secs(tn.ch3[2])
	if math.Abs(s1-s2) > 0.03 || math.Abs(s1-s3) > 0.03 {
		t.Fatalf("section starts differ: %v %v %v", s1, s2, s3)
	}
}

func TestSwingPositions(t *testing.T) {
	var tn tune
	_, tn = timedTune(t, "key: C\nlength: 1\nvolume: 1000\nenvelope: flat\nharmonic: first\nswing: 1\nhumanize: 0\nsections:\n  - length: 1\n    C1: [0.5:c4, 0.5:d4, 0.5:e4, 0.5:f4]\n  - C1: [1:c4]\n")
	d := 1.0 / 6
	wantStarts := []float64{0, 0.5 + d, 1, 1.5 + d}
	for i, w := range wantStarts {
		if got, _ := secs(tn.ch1[i]); math.Abs(got-w) > 2e-3 {
			t.Fatalf("note %d starts at %v, want %v", i, got, w)
		}
	}
	// on-beat note lengthens, off-beat note shortens; no overlap
	for i := 0; i < 3; i++ {
		_, e := secs(tn.ch1[i])
		s, _ := secs(tn.ch1[i+1])
		if math.Abs(e-s) > 1e-3 {
			t.Fatalf("notes %d,%d overlap or gap: end %v start %v", i, i+1, e, s)
		}
	}
	_, e0 := secs(tn.ch1[0])
	if math.Abs(e0-(0.5+d)) > 2e-3 {
		t.Fatalf("on-beat note ends at %v", e0)
	}
	_, e1 := secs(tn.ch1[1])
	if math.Abs(e1-1) > 2e-3 {
		t.Fatalf("off-beat note ends at %v, want 1", e1)
	}
}

func TestSwingZeroLeavesGrid(t *testing.T) {
	_, tn := timedTune(t, "key: C\nlength: 1\nvolume: 1000\nenvelope: flat\nharmonic: first\nhumanize: 0\nsections:\n  - C1: [0.5:c4, 0.5:d4]\n  - C1: [1:c4]\n")
	if got, _ := secs(tn.ch1[1]); math.Abs(got-0.5) > 2e-3 {
		t.Fatalf("straight off-beat at %v", got)
	}
}

func TestHumanizeBoundedDeterministicAndOff(t *testing.T) {
	src := tbase + "%ssections:\n  - C1: [1:c4, 1:d4, 1:e4, 1:f4, 1:g4, 1:a4]\n  - C1: [1:c4]\n"
	grid := func(h string) []note {
		_, tn := timedTune(t, sprintf(src, h))
		return tn.ch1
	}
	off := grid("humanize: 0\n")
	for i, n := range off[:6] {
		if got, _ := secs(n); math.Abs(got-float64(i)*0.5) > 1e-3 {
			t.Fatalf("humanize 0 moved note %d to %v", i, got)
		}
	}
	a, b := grid("humanize: 1\n"), grid("humanize: 1\n")
	moved := false
	for i := range a {
		if a[i].start != b[i].start || a[i].hold != b[i].hold {
			t.Fatal("not deterministic")
		}
		sa, ea := secs(a[i])
		so, eo := secs(off[i])
		if i > 0 && math.Abs(sa-so) > jitterMax+1e-3 {
			t.Fatalf("note %d jitter %v too big", i, sa-so)
		}
		if sa < 0 {
			t.Fatal("negative start")
		}
		if i > 0 && sa != so {
			moved = true
		}
		if math.Abs(ea-eo) > 1e-3 {
			t.Fatalf("note %d end moved (%v vs %v): jitter must not drift", i, ea, eo)
		}
	}
	if !moved {
		t.Fatal("humanize 1 changed nothing")
	}
}

func TestChordSpread(t *testing.T) {
	src := tbase + "instrument: %s\n%ssections:\n  - C1: [2:c4-e4-g4, 1:c4]\n  - C1: [1:c4]\n"
	spread := func(ins, h string) (float64, float64) {
		_, tn := timedTune(t, sprintf(src, ins, h))
		return tn.ch1[0].spread, tn.ch1[1].spread
	}
	p, single := spread("piano", "")
	if p < 0.003 || p > 0.0085 {
		t.Fatalf("piano spread %v out of range", p)
	}
	if single != 0 {
		t.Fatalf("single note spread %v", single)
	}
	g, _ := spread("guitar", "")
	if g < 0.011 || g > 0.021 {
		t.Fatalf("guitar spread %v out of range", g)
	}
	if z, _ := spread("guitar", "humanize: 0\n"); z != 0 {
		t.Fatalf("humanize 0 spread %v", z)
	}
	// per-channel instrument map picks the guitar
	_, tn := timedTune(t, tbase+"instruments: {C1: guitar}\nsections:\n  - C1: [2:c4-e4-g4]\n  - C1: [1:c4]\n")
	if tn.ch1[0].spread < 0.011 {
		t.Fatalf("instruments map not honoured: %v", tn.ch1[0].spread)
	}
}

func TestPlainIsNoOp(t *testing.T) {
	_, tn := timedTune(t, tbase+"plain: true\nsections:\n  - C1: [1:c4-e4, 1:d4]\n    fermata: 1\n    ritardando: 0.3\n")
	for _, n := range tn.ch1 {
		if n.placed || n.start != 0 || n.hold != 0 || n.spread != 0 {
			t.Fatalf("plain note was timed: %+v", n)
		}
	}
}

func TestFinalRitardandoDefault(t *testing.T) {
	_, tn := timedTune(t, tbase+humanizeOff+"sections:\n  - C1: [4:c4]\n")
	if _, e := secs(tn.ch1[0]); math.Abs(e-2*1.1) > 1e-3 {
		t.Fatalf("final section ends at %v, want 2.2", e)
	}
	_, tn = timedTune(t, tbase+humanizeOff+"sections:\n  - C1: [4:c4]\n    ritardando: -0.2\n")
	if _, e := secs(tn.ch1[0]); math.Abs(e-1.8) > 1e-3 {
		t.Fatalf("explicit accelerando ends at %v, want 1.8", e)
	}
}

func TestValidateTiming(t *testing.T) {
	bad := []string{
		"swing: 1.5\n", "swing: -0.1\n", "humanize: 2\n", "humanize: -1\n",
	}
	for _, b := range bad {
		var s Score
		if err := yaml.Unmarshal([]byte(tbase+b+"sections:\n  - C1: [c4]\n"), &s); err != nil {
			t.Fatal(err)
		}
		if validateTiming(&s) == nil {
			t.Fatalf("%q accepted", b)
		}
	}
	for _, sec := range []string{"ritardando: -0.5", "ritardando: 1.1", "fermata: -1", "fermata: 9"} {
		var s Score
		if err := yaml.Unmarshal([]byte(tbase+"sections:\n  - C1: [c4]\n    "+sec+"\n"), &s); err != nil {
			t.Fatal(err)
		}
		err := validateTiming(&s)
		if err == nil || !contains(err.Error(), "section 1") {
			t.Fatalf("%q: err = %v", sec, err)
		}
	}
	var ok Score
	_ = yaml.Unmarshal([]byte(tbase+"swing: 1\nhumanize: 0\nsections:\n  - C1: [c4]\n    ritardando: 1\n    fermata: 8\n"), &ok)
	if err := validateTiming(&ok); err != nil {
		t.Fatal(err)
	}
}

func TestParseThreeChannelsRenders(t *testing.T) {
	score := []byte(`name: t
key: C
length: 0.25
volume: 6000
instrument: piano
swing: 0.5
sections:
  - C1: [1:c5, 0.5:d5, 0.5:e5]
    C2: [0.5:c4-e4, 0.5:e4, 0.5:g4, 0.5:e4, 1:c4-e4]
    C3: [2:c2, 2:g2]
    ritardando: 0.2
  - C1: [2:c5-e5]
    C2: [2:g4]
    C3: [2:c3]
    fermata: 2
  - C1: [0.5:c5, 1.5:d5]
    C2: [2:b3]
    C3: [2:g2]
`)
	var s Score
	out := filepath.Join(t.TempDir(), "x")
	if _, err := Parse(&s, score, out, 0); err != nil {
		t.Fatalf("render failed: %v", err)
	}
}

func sprintf(f string, a ...any) string { return fmt.Sprintf(f, a...) }
func contains(s, sub string) bool       { return strings.Contains(s, sub) }

// A plain score ignores the timing fields, so it must not be rejected
// for out-of-range values of them.
func TestValidateTimingPlainIgnores(t *testing.T) {
	var s Score
	src := tbase + "plain: true\nswing: 5\nhumanize: 9\nsections:\n  - C1: [c4]\n    ritardando: 7\n    fermata: 99\n"
	if err := yaml.Unmarshal([]byte(src), &s); err != nil {
		t.Fatal(err)
	}
	if err := validateTiming(&s); err != nil {
		t.Fatalf("plain score rejected: %v", err)
	}
}

// Fermatas lengthen the performed audio beyond what the sample cap in
// Parse counts, so their total is bounded: many near-cap fermatas fail.
func TestFermataTotalBounded(t *testing.T) {
	var b strings.Builder
	b.WriteString("key: C\nlength: 1\nsections:\n")
	for i := 0; i < 120; i++ {
		b.WriteString("  - C1: [1:c4]\n    fermata: 8\n")
	}
	var s Score
	if err := yaml.Unmarshal([]byte(b.String()), &s); err != nil {
		t.Fatal(err)
	}
	err := Check(&s)
	if err == nil || !contains(err.Error(), "fermatas") {
		t.Fatalf("err = %v, want fermata total error", err)
	}
	// a few fermatas are fine
	var ok Score
	_ = yaml.Unmarshal([]byte("key: C\nlength: 1\nsections:\n  - C1: [1:c4]\n    fermata: 8\n  - C1: [1:c4]\n    fermata: 8\n"), &ok)
	if err := validateTiming(&ok); err != nil {
		t.Fatal(err)
	}
}

package muse

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// every drum is rendered at its length, peaks at 1, ends in silence, and
// its variants differ
func TestDrumKitRendered(t *testing.T) {
	for name, d := range drumKit {
		for v, x := range d.samples {
			if len(x) != sampleCount(d.length) {
				t.Errorf("%s variant %d is %d samples, want %d", name, v, len(x), sampleCount(d.length))
			}
			hi := 0.0
			for _, s := range x {
				hi = math.Max(hi, math.Abs(s))
			}
			if math.Abs(hi-1) > 1e-9 {
				t.Errorf("%s variant %d peaks at %v, want 1", name, v, hi)
			}
			if math.Abs(x[len(x)-1]) > 1e-9 {
				t.Errorf("%s variant %d ends at %v, want 0", name, v, x[len(x)-1])
			}
		}
		same := true
		for k := range d.samples[0] {
			if d.samples[0][k] != d.samples[1][k] {
				same = false
				break
			}
		}
		if same && name != "cowbell" { // the cowbell has no noise to vary
			t.Errorf("%s variants are identical, want slight differences", name)
		}
	}
}

// bands splits x's energy into below and above roughly 700 Hz: a moving
// average over 32 samples keeps the low part, and what it removes is the
// high part. Single-frequency measurements don't suit noisy sounds, whose
// sudden start smears energy across the whole spectrum.
func bands(x []float64) (low, high float64) {
	const w = 32
	sum := 0.0
	for k, v := range x {
		sum += v
		if k >= w {
			sum -= x[k-w]
		}
		avg := sum / w
		low += avg * avg
		high += (v - avg) * (v - avg)
	}
	return low, high
}

// the kick's energy is low, the hi-hat's high, and the snare has both
func TestDrumsSitWhereTheyShould(t *testing.T) {
	if lo, hi := bands(drumKit["kick"].samples[0]); lo < 10*hi {
		t.Errorf("kick: low %.3g, high %.3g - want it mostly low", lo, hi)
	}
	if lo, hi := bands(drumKit["hat"].samples[0]); hi < 10*lo {
		t.Errorf("hat: low %.3g, high %.3g - want it mostly high", lo, hi)
	}
	if lo, hi := bands(drumKit["snare"].samples[0]); lo < 0.05*hi || hi < 0.05*lo {
		t.Errorf("snare: low %.3g, high %.3g - want both its body and its wires", lo, hi)
	}
}

func TestMakeDrumNote(t *testing.T) {
	score := Score{Length: 0.5, Volume: 1000}
	n, err := makeDrumNote("0.5:k-HH-snare", Section{}, score)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(n.kit, ",") != "kick,hat,snare" || n.length != 0.25 || len(n.pitch) != 0 {
		t.Fatalf("got kit %v length %v pitch %v", n.kit, n.length, n.pitch)
	}
	if r, err := makeDrumNote("2:z", Section{}, score); err != nil || len(r.kit) != 0 || r.length != 1 {
		t.Fatalf("rest: kit %v length %v err %v", r.kit, r.length, err)
	}
	for _, bad := range []string{"bongo", "x:kick", "-1:kick", "kick-"} {
		if _, err := makeDrumNote(bad, Section{}, score); err == nil {
			t.Errorf("makeDrumNote(%q) should fail", bad)
		}
	}
}

const drumScore = `
name: Groove
key: C
length: 0.5
instrument: piano
volume: 8000
swing: 0.5
sections:
  - C1: [c4, e4, g4, e4]
    drums: [0.5:kick-hat, 0.5:hat, 0.5:snare-hat, 0.5:hat, 0.5:kick-hat, 0.5:kick-hat, 0.5:snare-hat, 0.5:hat]
  - C1: [4:c5]
    ritardando: 0.3
    fermata: 1
    drums: [kick-crash, 3:z]
`

// a melody and a drum part render together; the drums follow the same
// tempo map (swing, ritardando, fermata), so they end with the melody
func TestScoreWithDrums(t *testing.T) {
	var s Score
	if err := yaml.Unmarshal([]byte(drumScore), &s); err != nil {
		t.Fatal(err)
	}
	tu, err := buildTune(&s)
	if err != nil {
		t.Fatal(err)
	}
	if len(tu.drums) != 10 {
		t.Fatalf("got %d drum notes, want 10", len(tu.drums))
	}
	// the crash at the start of bar 2 lands with the melody's c5
	crash, c5 := tu.drums[8], tu.ch1[4]
	if d := math.Abs(float64(crash.start-c5.start)) / sampleRate; d > 2*jitterMax {
		t.Errorf("crash at %d, c5 at %d - want them together", crash.start, c5.start)
	}
	// the written ends of both parts agree through the ritardando and fermata
	last := tu.drums[len(tu.drums)-1]
	melEnd := c5.start + sampleCount(c5.hold)
	drumEnd := last.start + sampleCount(last.hold)
	if d := melEnd - drumEnd; d > tolerance || -d > tolerance {
		t.Errorf("melody ends at %d, drums at %d", melEnd, drumEnd)
	}
	// the drums are humanised like any other part, but kept off when plain
	swung := tu.drums[1].start - sampleCount(0.25)
	if swung <= 0 {
		t.Errorf("the off-beat hat should be swung late, got %d samples from straight", swung)
	}

	out := filepath.Join(t.TempDir(), "groove")
	if _, err := Parse(&Score{}, []byte(drumScore), out, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(out + ".wav"); err != nil {
		t.Fatal(err)
	}
}

// a score can be drums alone
func TestDrumsOnly(t *testing.T) {
	score := "key: C\nlength: 0.5\nvolume: 8000\nsections:\n  - drums: [k, h, s, h]\n"
	out := filepath.Join(t.TempDir(), "beat")
	if _, err := Parse(&Score{}, []byte(score), out, 0); err != nil {
		t.Fatal(err)
	}
	for _, plain := range []string{"", "plain: true\n"} {
		if _, err := Parse(&Score{}, []byte(plain+score), out, 0); err != nil {
			t.Fatalf("plain=%q: %v", plain, err)
		}
	}
}

func TestDrumMixSettings(t *testing.T) {
	ok := "key: C\nlength: 0.5\ninstrument: piano\nvolume: 1000\nlevels: {drums: 0.5}\npan: {drums: 0.2}\nsections:\n  - C1: [c4]\n    drums: [k]\n"
	parse := func(src string) *Score {
		var s Score // a fresh score each time: decoding into a used one merges maps
		if err := yaml.Unmarshal([]byte(src), &s); err != nil {
			t.Fatal(err)
		}
		return &s
	}
	s := *parse(ok)
	if err := Check(&s); err != nil {
		t.Fatalf("drums levels and pan should be allowed: %v", err)
	}
	tu, _ := buildTune(&s)
	if g := tu.drums[0].gain; g > 0.5 {
		t.Errorf("drum gain %v, want at most the 0.5 level", g)
	}
	if p := pans(&s, &tu); p[drumChannel] != 0.2 {
		t.Errorf("drum pan %v, want 0.2", p[drumChannel])
	}
	bad := strings.Replace(ok, "levels: {drums: 0.5}", "instruments: {drums: piano}", 1)
	if err := Check(parse(bad)); err == nil {
		t.Error("the drums can't be given an instrument")
	}
	unknown := strings.Replace(ok, "drums: [k]", "drums: [bongo]", 1)
	if err := Check(parse(unknown)); err == nil || !strings.Contains(err.Error(), "[drums]") {
		t.Errorf("an unknown drum should be a [drums] error, got %v", err)
	}
}

// the length cap counts each drum's full ring, not just the written note
func TestChannelSamplesCountsDrumRing(t *testing.T) {
	n, _ := makeDrumNote("0.1:crash", Section{}, Score{Length: 1, Volume: 1})
	if got := channelSamples([]note{n}); got < sampleCount(drumKit["crash"].length) {
		t.Fatalf("channelSamples = %d, want at least the crash's %d", got, sampleCount(drumKit["crash"].length))
	}
}

func TestDrumAttacksStartAtSilence(t *testing.T) {
	for name, d := range drumKit {
		for _, x := range d.samples {
			if x[0] != 0 {
				t.Errorf("%s has a discontinuous onset", name)
			}
			for _, v := range x {
				if math.IsNaN(v) || math.IsInf(v, 0) {
					t.Fatalf("%s has nonfinite samples", name)
				}
			}
		}
	}
}

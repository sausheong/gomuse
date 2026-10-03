package muse

import (
	"math"
	"path/filepath"
	"testing"
)

// brightness is how much of a note's energy lies in overtones 4 to 10,
// relative to the fundamental, over x[from:to]
func brightness(x []float64, from, to int, f float64) float64 {
	upper := 0.0
	for h := 4.0; h <= 10; h++ {
		upper += power(x, from, to, h*f)
	}
	return upper / power(x, from, to, f)
}

func ms(t float64) int { return sampleCount(t / 1000) }

// a horn opens up as it swells: the start of the attack is darker than the
// sustained note
func TestWindsBrightenWithLevel(t *testing.T) {
	for name, ins := range map[string]instrument{"trumpet": trumpet, "saxophone": saxophone} {
		x := ins(233.08, 1) // B flat 3
		early := brightness(x, 0, ms(15), 233.08)
		later := brightness(x, ms(300), ms(500), 233.08)
		if later < 2*early {
			t.Errorf("%s: overtone ratio %.3g in the first 15 ms, %.3g later - want it to open up as it swells", name, early, later)
		}
	}
}

// the trumpet is the brighter of the two
func TestTrumpetBrighterThanSaxophone(t *testing.T) {
	f := 349.23 // F4, in both ranges
	tr := brightness(trumpet(f, 1), ms(300), ms(700), f)
	sx := brightness(saxophone(f, 1), ms(300), ms(700), f)
	if tr <= sx {
		t.Fatalf("trumpet overtone ratio %.3g, saxophone %.3g - want the trumpet brighter", tr, sx)
	}
}

// pitchSwing is how far the pitch of x moves either side of f over
// x[from:to], in cents, measured by the strongest of several nearby bins
func pitchSwing(x []float64, from, to int, f float64) float64 {
	const win = 2205 // 50 ms windows
	lo, hi := math.Inf(1), math.Inf(-1)
	for at := from; at+win <= to; at += win / 2 {
		best, bestC := 0.0, 0.0
		for c := -60.0; c <= 60; c += 2 {
			if p := power(x, at, at+win, f*math.Exp2(c/1200)); p > best {
				best, bestC = p, c
			}
		}
		lo, hi = math.Min(lo, bestC), math.Max(hi, bestC)
	}
	return (hi - lo) / 2
}

// the saxophone sings with a wider vibrato than the trumpet, and both start
// a little under the pitch
func TestWindVibratoAndScoop(t *testing.T) {
	f := 440.0
	tr, sx := trumpet(f, 2), saxophone(f, 2)
	trv := pitchSwing(tr, ms(900), ms(1900), f)
	sxv := pitchSwing(sx, ms(900), ms(1900), f)
	if sxv <= trv || sxv < 10 {
		t.Fatalf("vibrato: trumpet ±%.0f cents, saxophone ±%.0f - want the saxophone's clearly wider", trv, sxv)
	}
	// the scoop: in the first 20 ms the pitch is below the note
	early := 0.0
	for c := -60.0; c <= 0; c += 2 {
		early = math.Max(early, power(sx, 0, ms(25), f*math.Exp2(c/1200)))
	}
	if early <= power(sx, 0, ms(25), f*math.Exp2(20.0/1200)) {
		t.Fatal("saxophone should start under the pitch, not above it")
	}
}

// a sustained horn carries more energy than a dying piano note at the same
// peak; the settle keeps a 1 s note close to a piano's loudness
func TestWindLoudnessNearPiano(t *testing.T) {
	f := 293.66
	n := sampleCount(1)
	p := rms(piano(f, 1), 0, n)
	for name, ins := range map[string]instrument{"trumpet": trumpet, "saxophone": saxophone} {
		if r := rms(ins(f, 1), 0, n) / p; r < 0.6 || r > 2.2 {
			t.Errorf("%s RMS is %.2f× a piano's, want roughly comparable (0.6-2.2)", name, r)
		}
	}
}

func TestScoreWithWinds(t *testing.T) {
	score := []byte(`
name: Horns
key: Bb
length: 0.5
instrument: piano
instruments: {C1: trumpet, C2: sax}
volume: 6000
sections:
  - C1: [b4, c5, 2:d5]
    C2: [d4, e4, 2:f4]
    C3: [4:b2-f3]
`)
	var s Score
	out := filepath.Join(t.TempDir(), "horns")
	if _, err := Parse(&s, score, out, 0); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"trumpet", "saxophone", "sax"} {
		if _, ok := instruments[name]; !ok {
			t.Errorf("instrument %q not registered", name)
		}
		if releases[name] <= 0 {
			t.Errorf("instrument %q has no release", name)
		}
	}
}

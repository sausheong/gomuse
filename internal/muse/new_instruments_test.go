package muse

import (
	"math"
	"path/filepath"
	"testing"
)

func TestNewInstrumentSpectra(t *testing.T) {
	f := 220.0
	from, to := sampleRate/5, sampleRate/2
	organ := drawbarOrgan(f, 2, 0.7)
	if ratio := rms(organ, sampleRate, sampleRate+sampleRate/5) / rms(organ, from, to); ratio < 0.95 || ratio > 1.05 {
		t.Fatalf("organ sustain ratio %g", ratio)
	}
	cl := clarinet(f, 1, 0.7)
	if power(cl, from, to, 2*f) >= power(cl, from, to, 3*f)/4 {
		t.Fatal("low clarinet lacks odd harmonic emphasis")
	}
	fl := flute(f, 1, 0.7)
	if power(fl, from, to, f) < 10*power(fl, from, to, 2*f) {
		t.Fatal("flute fundamental is too weak")
	}
	for name, fn := range map[string]velocityInstrument{"electric-piano": electricPiano, "vibraphone": vibraphone} {
		x := fn(f, 2, 0.7)
		if rms(x, sampleRate, sampleRate+sampleRate/10) >= rms(x, 0, sampleRate/10) {
			t.Errorf("%s fails to decay", name)
		}
	}
}

func TestNewInstrumentVelocityAndEdges(t *testing.T) {
	for name, fn := range velocityInstruments {
		t.Run(name, func(t *testing.T) {
			for _, d := range []float64{0, 1.0 / sampleRate, 0.01, 0.3} {
				for _, f := range []float64{65.4, 440, 4186} {
					x := fn(f, d, 0.3)
					if len(x) != sampleCount(d) {
						t.Fatal("wrong duration")
					}
					for _, v := range x {
						if math.IsNaN(v) || math.IsInf(v, 0) || math.Abs(v) > 1.0000001 {
							t.Fatal("invalid sample")
						}
					}
				}
			}
			n, err := makeNote("a4", Section{}, Score{Length: 1, Volume: 1000, Instrument: name})
			if err != nil || n.velocityIns == nil {
				t.Fatalf("missing velocity renderer: %v", err)
			}
			if name == "organ" {
				return
			} // drawbars are deliberately velocity insensitive
			soft, hard := fn(220, 1, 0.2), fn(220, 1, 1)
			harmonic := 2.0
			if name == "clarinet" {
				harmonic = 3
			}
			if name == "vibraphone" || name == "marimba" {
				harmonic = 4
			}
			brightness := func(x []float64) float64 { return power(x, 100, 8000, 220*harmonic) / power(x, 100, 8000, 220) }
			if brightness(hard) <= brightness(soft) {
				t.Fatal("hard note should have stronger upper partials")
			}
		})
	}
}

func TestNewInstrumentScore(t *testing.T) {
	for name := range velocityInstruments {
		var s Score
		score := []byte("name: New instrument\nkey: C\nlength: 0.1\nvolume: 4000\ninstrument: " + name + "\nsections:\n  - dynamic: p\n    C1: [c4, c4, c4-e4-g4, z]\n  - dynamic: f\n    C1: [c5, c5, c5-e5-g5, z]\n")
		if _, err := Parse(&s, score, filepath.Join(t.TempDir(), name), 0); err != nil {
			t.Fatal(err)
		}
	}
}

// A key release must reduce energy immediately without changing the held
// portion or introducing a discontinuity at the release boundary.
func TestInstrumentReleaseFollowsHold(t *testing.T) {
	x := drawbarOrgan(220, 1.2, 0.7)
	before := append([]float64(nil), x...)
	hold := 0.8
	dampAfterHold(x, hold)
	for i := 0; i <= sampleCount(hold); i++ {
		if x[i] != before[i] {
			t.Fatal("held sound changed")
		}
	}
	from, to := sampleCount(0.95), sampleCount(1.05)
	if rms(x, from, to) > 0.3*rms(before, from, to) {
		t.Fatal("release fails to damp promptly")
	}
	if x[len(x)-1] != 0 {
		t.Fatal("release does not end silently")
	}
}

// Native velocity synthesis must reach the mix without a second generic
// low-pass. In particular soft organ notes retain their drawbar registration.
func TestNativeVelocityAvoidsDoubleFiltering(t *testing.T) {
	n, err := makeNote("a4", Section{}, Score{Length: 0.3, Volume: 10000, Instrument: "organ", Plain: true})
	if err != nil {
		t.Fatal(err)
	}
	n.velocity = 0.2
	got, err := n.encode()
	if err != nil {
		t.Fatal(err)
	}
	want := drawbarOrgan(440, 0.3, 0.2)
	if len(got) != len(want) {
		t.Fatal("wrong output length")
	}
	for i, v := range want {
		if got[i] != int(10000*v) {
			t.Fatalf("native timbre changed at sample %d", i)
		}
	}
}

package muse

import (
	"math"
	"testing"
)

func TestNextInstrumentCharacters(t *testing.T) {
	// Bowed voices must sustain after their attack instead of becoming plucks.
	for _, name := range []string{"violin", "cello"} {
		x := instruments[name](220, 2)
		if rms(x, ms(1000), ms(1300)) < 0.7*rms(x, ms(200), ms(500)) {
			t.Errorf("%s fails to sustain", name)
		}
	}
	// A pad should swell; a bass should articulate promptly.
	pad, bass := synthPad(220, 2, 0.7), synthBass(220, 2, 0.7)
	early := func(x []float64) float64 { return rms(x, ms(10), ms(50)) / rms(x, ms(350), ms(450)) }
	if early(pad) >= 0.3 || early(bass) <= 0.8 {
		t.Fatal("pad and bass attack envelopes are not distinct")
	}
	// A wooden bar loses energy much sooner than the metal vibraphone bar.
	decay := func(x []float64) float64 { return rms(x, ms(700), ms(800)) / rms(x, ms(10), ms(100)) }
	if decay(marimba(220, 2, 0.7)) >= decay(vibraphone(220, 2, 0.7))*0.5 {
		t.Fatal("marimba sustains too much")
	}
	if decay(uprightBass(110, 2, 0.7)) >= decay(bassVelocity(110, 2, 0.7)) {
		t.Fatal("upright bass should have a shorter plucked body")
	}
	if decay(harp(220, 2, 0.7)) <= decay(marimba(220, 2, 0.7)) {
		t.Fatal("harp should ring beyond marimba")
	}
}

func TestSynthHighNotesStayBandLimited(t *testing.T) {
	// At 8 kHz only the fundamental and second harmonic fit. A third
	// harmonic would fold to 20.1 kHz; verify that bin is far below the root.
	for _, fn := range []velocityInstrument{synthBass, synthLead, synthPad} {
		x := fn(8000, 0.5, 1)
		root := power(x, ms(100), ms(400), 8000)
		folded := power(x, ms(100), ms(400), sampleRate-24000)
		if folded > root*1e-5 {
			t.Fatal("high synth note has excess folded harmonic energy")
		}
	}
}

func TestNextInstrumentsLowRegisterAndDeterminism(t *testing.T) {
	for name, f := range map[string]float64{"cello": 65.406, "upright-bass": 41.203, "synth-bass": 32.703, "harp": 65.406} {
		x, y := instruments[name](f, 1.5), instruments[name](f, 1.5)
		for i := range x {
			if x[i] != y[i] {
				t.Fatalf("%s rendering is not deterministic", name)
			}
		}
		best, cents := 0.0, 0.0
		for c := -15.0; c <= 15; c++ {
			if p := power(x, 0, len(x), f*math.Exp2(c/1200)); p > best {
				best, cents = p, c
			}
		}
		if math.Abs(cents) > 3 {
			t.Errorf("%s is %.0f cents out in the low register", name, cents)
		}
	}
}

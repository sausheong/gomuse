package muse

import (
	"math"
	"strings"
	"testing"
)

// Exercise the same range of dynamics and registers as the exposed benchmark,
// including all registered procedural instruments rather than a fixed subset.
func TestInstrumentQualityMatrix(t *testing.T) {
	for _, name := range InstrumentNames() {
		if name == "voice" || name == "sax" || strings.HasPrefix(name, "scat-") {
			continue
		}
		frequencies := []float64{130.8128, 261.6256, 523.2511}
		switch name {
		case "bass", "upright-bass", "synth-bass":
			frequencies = []float64{41.2034, 110, 195.9977}
		case "trombone", "bassoon":
			frequencies = []float64{65.4064, 195.9977, 523.2511}
		case "french-horn":
			frequencies = []float64{123.4708, 391.9954, 698.4565}
		case "oboe":
			frequencies = []float64{233.0819, 440, 880}
		case "nylon-guitar":
			frequencies = []float64{82.4069, 220, 659.2551}
		case "glockenspiel", "xylophone":
			frequencies = []float64{261.6256, 1046.5023, 2093.0045}
		case "cello":
			frequencies = []float64{65.4064, 195.9977, 523.2511}
		case "violin":
			frequencies = []float64{195.9977, 440, 1318.5102}
		case "flute":
			frequencies = []float64{261.6256, 523.2511, 1046.5023}
		case "trumpet":
			frequencies = []float64{261.6256, 391.9954, 1046.5023}
		case "saxophone":
			frequencies = []float64{123.4708, 293.6648, 880}
		case "clarinet":
			frequencies = []float64{164.8138, 493.8833, 783.9909}
		}
		for _, f := range frequencies {
			for _, v := range []float64{.25, .7, 1} {
				fn, ok := velocityInstruments[name]
				if !ok {
					t.Fatalf("%s lacks native velocity", name)
				}
				x := fn(f, .4, v)
				energy := 0.
				if len(x) != sampleCount(.4) {
					t.Fatalf("%s wrong buffer length", name)
				}
				for _, a := range x {
					if math.IsNaN(a) || math.IsInf(a, 0) || math.Abs(a) > 1.000001 {
						t.Fatalf("%s invalid signal at %gHz velocity %g", name, f, v)
					}
					energy += a * a
				}
				if energy < 1 || math.Abs(x[0]) > 1e-6 || math.Abs(x[len(x)-1]) > 1e-6 {
					t.Fatalf("%s silent or abrupt endpoints at %gHz velocity %g", name, f, v)
				}
			}
		}
	}
}

func TestBrassFallLowersAudiblePitch(t *testing.T) {
	for _, name := range []string{"trumpet", "saxophone"} {
		tune, err := buildTune(gestureScore(name, NoteExpression{}))
		if err != nil {
			t.Fatal(err)
		}
		n := tune.ch1[0]
		n.gesture = &Gesture{Kind: "fall", DurationMS: 400, Semitones: 4}
		n.performance.Vibrato = 0
		var voice phraseVoice
		raw := voice.render(n, 1, false)
		x := make([]float64, len(raw))
		for i, a := range raw {
			x[i] = float64(a)
		}
		estimate := func(start, end float64) float64 {
			best, hz := 0., 0.
			for f := 160.; f <= 240.; f += .5 {
				if p := power(x, sampleCount(start), sampleCount(end), f); p > best {
					best, hz = p, f
				}
			}
			return hz
		}
		early, late := estimate(.2, .35), estimate(.94, .99)
		if math.Abs(early-220) > 2 || late > 185 || late < 170 {
			t.Fatalf("%s fall pitch %.1f -> %.1fHz", name, early, late)
		}
	}
}

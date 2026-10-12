package muse

import "math"

func init() {
	registerVelocityInstrument("synth-bass", synthBass, 0.12)
	registerVelocityInstrument("synth-lead", synthLead, 0.16)
	registerVelocityInstrument("synth-pad", synthPad, 0.8)
}

// These presets use finite Fourier oscillator banks. Every detuned partial
// is checked by additive against maxPartialFreq, avoiding discontinuous
// saw/square waves and their aliased harmonics. Partial envelopes produce
// the opening/closing spectrum without a nonlinear filter.
func synthBass(f, d, v float64) []float64 { return synthNote(f, d, v, 0) }
func synthLead(f, d, v float64) []float64 { return synthNote(f, d, v, 1) }
func synthPad(f, d, v float64) []float64  { return synthNote(f, d, v, 2) }

func synthNote(f, d, velocity float64, kind int) []float64 {
	v := playingVelocity(velocity)
	detune, weights := []float64{0}, []float64{1}
	attack, release := 0.004, 0.04
	if kind == 1 {
		detune, weights = []float64{-3, 0, 3}, []float64{0.2, 0.6, 0.2}
		attack = 0.012
		release = 0.07
	}
	if kind == 2 {
		detune, weights = []float64{-5, 0, 5}, []float64{0.22, 0.56, 0.22}
		attack = 0.3
		release = 0.2
	}
	var ps []partial
	for i, cents := range detune {
		root := f * math.Exp2(cents/1200)
		for h := 1.0; h <= 64 && h*root < maxPartialFreq; h++ {
			cutoff := f * (2 + 18*v*v)
			amp := weights[i] / h / (1 + math.Pow(h*root/cutoff, 2))
			stages := [][2]float64{{0.25, 0}, {0.75, (h - 1) * 3}}
			if kind == 0 {
				amp /= math.Sqrt(h)
				stages = [][2]float64{{0.3, 0}, {0.7, (h - 1) * 8}}
			}
			if kind == 2 {
				amp /= math.Pow(h, 0.25)
				stages = [][2]float64{{1, 0}, {-0.6, 1.8}}
			}
			ps = append(ps, partial{h * root, amp, stages})
		}
	}
	return additive(sampleCount(d), ps, attack, release)
}

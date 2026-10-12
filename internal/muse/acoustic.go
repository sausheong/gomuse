package muse

import "math"

func init() {
	registerVelocityInstrument("upright-bass", uprightBass, 0.18)
	registerVelocityInstrument("harp", harp, 0.9)
	registerVelocityInstrument("marimba", marimba, 0.25)
}

// Upright bass is a pizzicato preset. Body resonances colour the string
// harmonics; a quicker initial decay separates it from electric bass.
func uprightBass(f, d, velocity float64) []float64 {
	v := playingVelocity(velocity)
	var ps []partial
	cutoff := 550 + 1500*v*v
	for h := 1.0; h <= 32; h++ {
		hz := f * h * math.Sqrt(1+0.00008*h*h)
		body := resonanceGain([]formant{{95, 80, 0.7}, {210, 150, 0.45}, {650, 500, 0.2}}, 0.6, hz)
		amp := math.Sin(math.Pi*h*0.18) / h * body / (1 + math.Pow(hz/cutoff, 2))
		rate := loss(hz, 0.9, 1.8, 0.8)
		ps = append(ps, partial{hz, amp, [][2]float64{{0.7, rate * 3}, {0.3, rate}}})
	}
	return additive(sampleCount(d), ps, 0.008-0.004*v, 0.06)
}

// A harp's plucked spectrum has little string stiffness and a long low
// register tail. Upper strings and upper partials lose energy more quickly.
func harp(f, d, velocity float64) []float64 {
	v := playingVelocity(velocity)
	var ps []partial
	cutoff := 1800 + 7000*v*v
	for h := 1.0; h <= 40; h++ {
		hz := h * f * math.Sqrt(1+0.000015*h*h)
		amp := math.Sin(math.Pi*h*0.13) / math.Pow(h, 1.15) / (1 + math.Pow(hz/cutoff, 2))
		rate := loss(hz, 0.25+f/3000, 0.7, 0.3)
		ps = append(ps, partial{hz, amp, [][2]float64{{0.35, rate * 5}, {0.65, rate}}})
	}
	return additive(sampleCount(d), ps, 0.0025, 0.08)
}

// Tuned wooden-bar modes, with a softer strike and shorter sustain than
// vibraphone. Mallet strength controls the fast, inharmonic upper modes.
func marimba(f, d, velocity float64) []float64 {
	v := playingVelocity(velocity)
	ratios := []float64{1, 4, 10, 18.4}
	amplitudes := []float64{1, 0.055 + 0.23*v*v, 0.065 * v * v, 0.025 * v * v}
	rates := []float64{2.2 + f/1100, 9 + f/700, 24, 40}
	ps := make([]partial, 0, len(ratios))
	for i, r := range ratios {
		ps = append(ps, partial{r * f, amplitudes[i], [][2]float64{{1, rates[i]}}})
	}
	return additive(sampleCount(d), ps, 0.005-0.002*v, 0.035)
}

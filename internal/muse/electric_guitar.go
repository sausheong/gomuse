package muse

import "math"

func init() {
	registerVelocityInstrument("electric-guitar", electricGuitar, 0.35)
}

// electricGuitar models a picked string and magnetic pickup, with sustained
// upper partials coloured by a mildly driven amp and a dark speaker cabinet.
// The amp colour is constructed from band-limited partials rather than a
// nonlinear waveshaper, so it cannot generate folded high-frequency harmonics.
func electricGuitar(freq, duration, velocity float64) []float64 {
	v := playingVelocity(velocity)
	var ps []partial
	for n := 1.0; n <= 40; n++ {
		f := n * freq
		pickup := math.Sin(n * math.Pi * 0.17)
		cabinet := 1 / (1 + math.Pow(f/(2400+2100*v), 4))
		amp := pickup / math.Pow(n, 1.35-0.5*v) * cabinet
		ps = append(ps, partial{f, amp, [][2]float64{
			{0.35, 7 + 0.7*n},
			{0.65, 0.65 + 0.10*n},
		}})
	}
	return additive(sampleCount(duration), ps, 0.001, 0.025)
}

package muse

import "math"

// Strings: instruments whose notes are a string set ringing and left to die
// away - plucked (guitar, bass) or struck (piano). Each is a set of fixed
// partials built by additive, each partial decaying at its own rate.

func init() {
	registerVelocityInstrument("guitar", guitarVelocity, 0.8)
	registerVelocityInstrument("piano", pianoVelocity, 0.3)
	registerVelocityInstrument("bass", bassVelocity, 0.15)
}

// guitar is a plucked steel string. Plucking a fifth of the way along the
// string gives overtone n a level of sin(nπ/5)/n (so every 5th overtone is
// missing), the overtones are very slightly sharp, and each dies away at its
// own rate - bright at the pluck, mellow as it rings.
func guitar(freq, duration float64) []float64 { return guitarVelocity(freq, duration, 0.7) }

func guitarVelocity(freq, duration, velocity float64) []float64 {
	v := playingVelocity(velocity)
	const (
		pluckAt = 0.2    // pluck position, as a fraction of the string
		stiff   = 0.0001 // inharmonicity: how sharp the overtones run
	)
	var ps []partial
	for n := 1.0; n <= 24; n++ {
		f := n * freq * math.Sqrt(1+stiff*n*n)
		// Body colours the vibrating string without adding unrelated pitches.
		body := resonanceGain([]formant{{110, 100, 0.35}, {220, 180, 0.25}, {650, 600, 0.15}}, 0.65, f)
		cutoff := 1800 + 6000*v*v
		amp := math.Sin(n*math.Pi*pluckAt) / n * body / (1 + (f/cutoff)*(f/cutoff))
		ps = append(ps, partial{f, amp, [][2]float64{{1, loss(f, 0.5, 2, 2)}}})
	}
	return additive(sampleCount(duration), ps, 0.0015, 0.01)
}

// piano is a hammered string. The hammer strikes about a seventh of the way
// along and its felt softens the highest overtones. Above the bass each note
// has two or three strings tuned a fraction apart, which beat gently against each
// other. The overtones run sharper than a guitar's, more so up the keyboard,
// and each decays in two stages: a quick drop after the strike, then a long,
// quiet tail. The damper takes a moment to stop the strings at the end.
func piano(freq, duration float64) []float64 { return pianoVelocity(freq, duration, 0.7) }

func pianoVelocity(freq, duration, velocity float64) []float64 {
	v := playingVelocity(velocity)
	const (
		strikeAt = 1.0 / 7 // hammer position, as a fraction of the string

	)
	felt := 1200 + 4200*v*v          // harder hammer strikes excite more upper partials
	stiff := 0.00025 * freq / 261.63 // inharmonicity grows up the keyboard
	detune := []float64{0}
	if freq > 100 {
		detune = []float64{-0.8, 0.8}
	}
	if freq > 200 {
		detune = []float64{-1.1, 0, 1.1}
	}
	var ps []partial
	for n := 1.0; n <= 24; n++ {
		f := n * freq * math.Sqrt(1+stiff*n*n)
		amp := math.Sin(n*math.Pi*strikeAt) / n / (1 + math.Pow(f/felt, 2.4))
		amp *= resonanceGain([]formant{{180, 150, .12}, {520, 400, .15}, {1600, 1100, .10}}, .85, f)
		tail := loss(f, 0.3, 1.2, 0.8)
		for _, c := range detune {
			ps = append(ps, partial{
				freq:   f * math.Pow(2, c/1200),
				amp:    amp / float64(len(detune)),
				stages: [][2]float64{{0.7, 4*tail + 1}, {0.3, tail}},
			})
		}
	}
	x := additive(sampleCount(duration), ps, 0.004-0.0025*v, 0.05)
	addStringExcitation(x, freq, v, 0, true)
	finishInstrument(x, duration, .05)
	return x
}

// bass is an electric bass guitar, played with the fingers. Its strings are
// thick, so the overtones run sharper than a guitar's. It's plucked a little
// under a quarter of the way along, over the neck pickup (plucking nearer the
// bridge would make it brighter), and the pickup rounds off everything
// above a velocity-dependent cutoff, so the tone stays solid but opens on harder attacks. The fundamental rings steadily for a
// long time while the upper overtones fade quickly, which is what lets a
// bass line sit under everything else without getting in its way.
func bass(freq, duration float64) []float64 { return bassVelocity(freq, duration, 0.7) }

func bassVelocity(freq, duration, velocity float64) []float64 {
	v := playingVelocity(velocity)
	const (
		pluckAt = 0.22   // pluck position, as a fraction of the string
		stiff   = 0.0004 // inharmonicity: thick strings run sharper

	)
	pickup := 350 + 1000*v*v
	var ps []partial
	for n := 1.0; n <= 20; n++ {
		f := n * freq * math.Sqrt(1+stiff*n*n)
		amp := math.Sin(n*math.Pi*pluckAt) / n / (1 + (f/pickup)*(f/pickup))
		ps = append(ps, partial{f, amp, [][2]float64{
			{0.3 + 0.45*v, loss(f, 1.2, 4, 4)},  // the bright edge of the pluck, gone quickly
			{0.7 - 0.45*v, loss(f, 0.35, 1, 1)}, // the long, steady body of the note
		}})
	}
	return additive(sampleCount(duration), ps, 0.003, 0.04)
}

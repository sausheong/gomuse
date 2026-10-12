package muse

import "math"

// the harmonic describes the additional harmonics added to the fundamental
// frequency. Each one is scaled to peak at 1, so the volume of a score means
// the same thing whichever harmonic it uses.
type harmonic = func(input float64) float64

var harmonics map[string]harmonic

func init() {
	harmonics = make(map[string]harmonic)
	harmonics["first"] = first
	harmonics["second"] = normalize(rawSecond)
	harmonics["third"] = normalize(rawThird)
	harmonics["stringed"] = normalize(rawStringed)
	harmonics["hollow"] = normalize(overtones(hollowLevels))
	harmonics["bright"] = normalize(overtones(brightLevels))
	harmonics["mellow"] = normalize(overtones(mellowLevels))
	harmonics["organ"] = normalize(overtones(organLevels))
	harmonics["nasal"] = normalize(overtones(nasalLevels))
}

func base(input float64) float64 {
	return 2 * math.Pi * input
}

// fundamental only - a pure sine wave
func first(input float64) float64 {
	return math.Sin(base(input))
}

// fundamental plus the 2nd harmonic
func rawSecond(input float64) float64 {
	return math.Sin(base(input)) + math.Sin(base(input)*2)
}

// fundamental plus the 2nd and 3rd harmonics
func rawThird(input float64) float64 {
	return math.Sin(base(input)) + math.Sin(base(input)*2) + math.Sin(base(input)*3)
}

// a plucked string: a strong fundamental with harmonics that fall away.
// Strings have nothing below the fundamental, so there's no sub-harmonic.
func rawStringed(input float64) float64 {
	return 3*math.Sin(base(input)) + 1.5*math.Sin(base(input)*2) + 0.25*math.Sin(base(input)*3) + 0.125*math.Sin(base(input)*4)
}

// The harmonics below are classic waveforms and instrument colours, built
// from the first nine overtones. Stopping at nine keeps the top overtone of
// the highest notes (c7, about 2 kHz) below the 22 kHz Nyquist limit, where
// it would otherwise fold back as a harsh, out-of-tune tone.

// overtones builds a harmonic from the levels of overtones 1, 2, 3, ...;
// a negative level flips that overtone's phase
func overtones(levels []float64) harmonic {
	return func(input float64) float64 {
		x := base(input)
		v := 0.0
		for i, l := range levels {
			if l != 0 {
				v += l * math.Sin(float64(i+1)*x)
			}
		}
		return v
	}
}

var (
	// hollow is a soft square wave: odd overtones only, each at 1/n. The
	// missing even overtones give the hollow, woody sound of a clarinet.
	hollowLevels = []float64{1, 0, 1.0 / 3, 0, 1.0 / 5, 0, 1.0 / 7, 0, 1.0 / 9}
	// bright is a soft sawtooth: every overtone at 1/n, the buzzing sound
	// of brass and bowed strings, and of most analogue synths.
	brightLevels = []float64{1, 1.0 / 2, 1.0 / 3, 1.0 / 4, 1.0 / 5, 1.0 / 6, 1.0 / 7, 1.0 / 8, 1.0 / 9}
	// mellow is a triangle wave: odd overtones falling as 1/n², with every
	// other one flipped, so it's close to a pure tone with a little body,
	// like a flute or an ocarina.
	mellowLevels = []float64{1, 0, -1.0 / 9, 0, 1.0 / 25, 0, -1.0 / 49, 0, 1.0 / 81}
	// organ is a pipe organ registration: the 8' stop (the fundamental)
	// with 4' (the octave), 2 2/3' (the twelfth), 2' and the higher
	// mixture stops drawn progressively softer.
	organLevels = []float64{1, 0.8, 0.6, 0.5, 0, 0.35, 0, 0.25}
	// nasal is strongest at the 3rd and 4th overtones, with a weaker
	// fundamental - the reedy, nasal sound of an oboe or a muted trumpet.
	nasalLevels = []float64{0.5, 0.8, 1, 0.9, 0.6, 0.4, 0.25, 0.15, 0.1}
)

// normalize scales h to peak at 1. Every harmonic here repeats once per
// cycle of the fundamental, so sampling one cycle finds the peak.
func normalize(h harmonic) harmonic {
	m := 0.0
	for i := range 100000 {
		m = math.Max(m, math.Abs(h(float64(i)/100000)))
	}
	return func(input float64) float64 {
		return h(input) / m
	}
}

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

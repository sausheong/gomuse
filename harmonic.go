package main

import "math"

// the harmonic describes the additional harmonics added to the fundamental frequency
type harmonic = func(input float64) float64

var harmonics map[string]harmonic

func init() {
	harmonics = make(map[string]harmonic)
	harmonics["first"] = first
	harmonics["second"] = second
	harmonics["third"] = third
	harmonics["stringed"] = stringed
}

func base(input float64) float64 {
	return 2 * math.Pi * input
}

func first(input float64) float64 {
	return math.Sin(base(input))
}

func second(input float64) float64 {
	return math.Sin(base(input)) + math.Sin(base(input)*2)
}

func third(input float64) float64 {
	return math.Sin(base(input)) + math.Sin(base(input)*2) + math.Sin(base(input)*3)
}

func stringed(input float64) float64 {
	return 3*math.Sin(base(input)) + 0.5*math.Sin(base(input)*0.5) + 1.5*math.Sin(base(input)*2) + 0.25*math.Sin(base(input)*3) + 0.125*math.Sin(base(input)*4)
}

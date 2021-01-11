package main

import (
	"fmt"
	"math"
)

// Envelopes control the shape of the note and how it's played
type envelope = func(input float64, duration float64) float64

// -- .
//     \
//      \
func drop(input float64, duration float64) float64 {
	return math.Cos((math.Pi * input) / (2 * duration))
}

//   . --
//  /
// /
func rise(input float64, duration float64) float64 {
	return math.Sin((math.Pi * input) / (2 * duration))
}

//   . -- .
//  /      \
// /        \
func round(input float64, duration float64) float64 {
	fmt.Println(">", input, "->", math.Cos((math.Pi*input)/duration))
	return math.Sin((math.Pi * input) / duration)
}

func curve(input float64, duration float64) float64 {
	return math.Cos(math.Pi*input) * math.Log(input) / (duration)
}

package main

import (
	"math"
)

// Envelopes control the shape of the note and how it's played
type envelope = func(input float64, duration float64) float64

var envelopes map[string]envelope

func init() {
	envelopes = make(map[string]envelope)
	envelopes["drop"] = drop
	envelopes["rise"] = rise
	envelopes["round"] = round
	envelopes["triangle"] = triangle
	envelopes["tadpole"] = tadpole
	envelopes["flat"] = flat
	envelopes["combi"] = combi
	envelopes["diamond"] = diamond
}

//
// -----
//
func flat(input float64, duration float64) float64 {
	return 1
}

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
	return math.Sin(math.Pi * input / duration)
}

//   /\
//  /  \
// /    \
func triangle(input float64, duration float64) float64 {
	return (2 / math.Pi) * math.Asin(math.Sin(2*math.Pi*input/duration))
}

// tadpole shaped envelope

func tadpole(input float64, duration float64) float64 {
	return math.Sin(math.Pi*input/duration) -
		(0.5 * math.Sin(2*math.Pi*input/duration)) +
		(0.333 * math.Sin(3*math.Pi*input/duration)) -
		(0.25 * math.Sin(4*math.Pi*input/duration))
}

func combi(input float64, duration float64) float64 {
	return math.Sin((math.Pi*input)/(duration))/2 + math.Cos((math.Pi*input)/(2*duration))/3
}

func diamond(input float64, duration float64) float64 {
	return (2/math.Pi)*math.Asin(math.Sin(math.Pi*input/duration))/4 +
		(2/math.Pi)*math.Asin(math.Sin(2*math.Pi*input/duration))/4

}

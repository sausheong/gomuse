package muse

import (
	"math"
)

// Envelopes control the shape of the note and how it's played. Every
// envelope stays within [0, 1], peaks at 1 and is 0 at the end of the note,
// so the volume of a score means the same thing whichever envelope it uses
// and notes never end with a click.
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
	envelopes["drawl"] = drawl
	envelopes["tempered"] = tempered
}

// releaseTime is how long envelopes that would otherwise end at full volume
// (flat, rise) take to fade out, so the waveform isn't cut off mid-cycle.
const releaseTime = 0.01 // seconds

// release is 1 for most of the note and fades linearly to 0 over the last
// releaseTime seconds (or the last quarter of a very short note).
func release(input float64, duration float64) float64 {
	r := math.Min(releaseTime, duration/4)
	return math.Max(0, math.Min(1, (duration-input)/r))
}

// ------.
//
//	\
func flat(input float64, duration float64) float64 {
	return release(input, duration)
}

// -- .
//
//	\
//	 \
func drop(input float64, duration float64) float64 {
	return math.Cos((math.Pi * input) / (2 * duration))
}

//	 . --.
//	/     \
//
// /
func rise(input float64, duration float64) float64 {
	// reach full volume just as the release starts, then fade out
	top := duration - math.Min(releaseTime, duration/4)
	return math.Sin(math.Pi/2*math.Min(1, input/top)) * release(input, duration)
}

//	 . -- .
//	/      \
//
// /        \
func round(input float64, duration float64) float64 {
	return math.Sin(math.Pi * input / duration)
}

//	 /\
//	/  \
//
// /    \
func triangle(input float64, duration float64) float64 {
	return (2 / math.Pi) * math.Asin(math.Sin(math.Pi*input/duration))
}

// tadpole is a big head followed by a rippling tail: a quick attack, then a
// wobbly decay to silence. It's the truncated Fourier series of a sawtooth,
// played backwards so the ramp falls instead of rising.
//
//	/\
//	  \/\
//	     \_
func tadpole(input float64, duration float64) float64 {
	x := math.Pi * (duration - input) / duration
	return (math.Sin(x) - math.Sin(2*x)/2 + math.Sin(3*x)/3 - math.Sin(4*x)/4) / tadpolePeak
}

// tadpolePeak is the maximum of the unscaled tadpole series, so tadpole peaks
// at exactly 1.
var tadpolePeak = peak(func(x float64) float64 {
	x *= math.Pi
	return math.Sin(x) - math.Sin(2*x)/2 + math.Sin(3*x)/3 - math.Sin(4*x)/4
})

// combi mixes round and drop.
func combi(input float64, duration float64) float64 {
	return (math.Sin((math.Pi*input)/(duration))/2 + math.Cos((math.Pi*input)/(2*duration))/3) / combiPeak
}

var combiPeak = peak(func(x float64) float64 {
	return math.Sin(math.Pi*x)/2 + math.Cos(math.Pi*x/2)/3
})

// diamond mixes a triangle with a triangle of twice the rate, giving a tall
// first swell and a smaller second one.
//
//	/\
//	  \/\
func diamond(input float64, duration float64) float64 {
	return rawDiamond(input/duration) / diamondPeak
}

// rawDiamond is the diamond shape over a note of length 1. The faster
// triangle dips below zero in the second half; a negative envelope only
// flips the waveform, so what you hear is its magnitude, which is used here.
func rawDiamond(x float64) float64 {
	return math.Abs((2/math.Pi)*math.Asin(math.Sin(math.Pi*x))/4 +
		(2/math.Pi)*math.Asin(math.Sin(2*math.Pi*x))/4)
}

var diamondPeak = peak(rawDiamond)

// drawl starts loud and decays quickly, then slowly, like a note that
// lingers. The underlying 1/log10 curve never reaches zero, so it's shifted
// and scaled to run from 1 at the start to 0 at the end.
func drawl(input float64, duration float64) float64 {
	start, end := rawDrawl(0), rawDrawl(1)
	return (rawDrawl(input/duration) - end) / (start - end)
}

func rawDrawl(x float64) float64 {
	return 1 / math.Log10(2*math.Pi*x+1.9)
}

// tempered is drop shaped by drawl's curve: a soft, plucked decay.
func tempered(input float64, duration float64) float64 {
	return drop(input, duration) * rawDrawl(input/duration) / rawDrawl(0)
}

// peak returns the maximum of f over [0, 1], sampled finely enough for the
// smooth shapes used here.
func peak(f func(float64) float64) float64 {
	m := 0.0
	for i := range 100001 {
		m = math.Max(m, f(float64(i)/100000))
	}
	return m
}

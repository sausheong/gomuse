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
	envelopes["adsr"] = adsr
	envelopes["pluck"] = pluck
	envelopes["bell"] = bell
	envelopes["bowed"] = bowed
	envelopes["staccato"] = staccato
	envelopes["tremolo"] = tremolo
}

// releaseTime is how long envelopes that would otherwise end at full volume
// (flat, rise) take to fade out, so the waveform isn't cut off mid-cycle.
const releaseTime = 0.01 // seconds

// release is 1 for most of the note and fades linearly to 0 over the last
// releaseTime seconds (or the last quarter of a very short note).
func release(input float64, duration float64) float64 {
	r := math.Min(releaseTime, duration/4)
	return clampFloat((duration-input)/r, 0, 1)
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

// The envelopes above stretch their shape to fit the note, so a long note
// fades as slowly as a short one fades quickly. Real instruments don't work
// like that: a struck bell rings for the same time whatever the written
// length, and a held organ note stays level until it's let go. The
// envelopes below are timed in seconds instead, with each stage limited to
// a share of very short notes so every one still peaks at 1 and ends at 0.

// fadeOut is 1 until the last r seconds of the note, then falls to 0 along
// a raised cosine, which is gentler on the ear than a straight line.
func fadeOut(input, duration, r float64) float64 {
	if left := duration - input; left < r {
		return 0.5 - 0.5*math.Cos(math.Pi*math.Max(0, left)/r)
	}
	return 1
}

// adsr is the classic synthesiser envelope: a quick attack to full volume,
// a short decay to a sustain level that holds for as long as the note is
// held, then a release. It suits keyboards, synths and anything played
// with a steady tone.
//
//	/\
//	  \_______
//	          \.
func adsr(input float64, duration float64) float64 {
	const (
		attack  = 0.01 // seconds
		decay   = 0.1
		sustain = 0.7
		rel     = 0.08
	)
	a := math.Min(attack, duration/4)
	d := math.Min(decay, duration/4)
	var v float64
	switch {
	case input < a:
		v = input / a
	case input < a+d:
		v = 1 - (1-sustain)*(input-a)/d
	default:
		v = sustain
	}
	return v * fadeOut(input, duration, math.Min(rel, duration/4))
}

// struck is a near-instant attack followed by an exponential decay with a
// time constant of tau seconds, whatever the note's length, and a short
// fade at the end so a long-ringing note stops cleanly.
func struck(input, duration, attack, tau, rel float64) float64 {
	a := math.Min(attack, duration/4)
	if input < a {
		return input / a
	}
	return math.Exp(-(input-a)/tau) * fadeOut(input, duration, math.Min(rel, duration/4))
}

// pluck is a plucked string: it speaks at once and dies away exponentially,
// losing about two thirds of its level every 0.35 s. Unlike drop, a long
// note doesn't fade more slowly than a short one - it just rings out.
//
//	|\
//	| \_
//	|   \___
func pluck(input float64, duration float64) float64 {
	return struck(input, duration, 0.002, 0.35, 0.02)
}

// bell is a struck bell, chime or vibraphone: the same instant attack as
// pluck, but a long ring with a time constant of 1.5 s.
//
//	|\
//	|  \___
//	|      \______
func bell(input float64, duration float64) float64 {
	return struck(input, duration, 0.001, 1.5, 0.05)
}

// bowed is a bowed string, a held wind note or a pad: it starts softly as
// the bow bites, settles at 85%, swells slightly towards full volume while
// it's held, and is let go gently.
//
//	   .----^.
//	  /       \
//	_/         \_
func bowed(input float64, duration float64) float64 {
	const settle = 0.85
	a := math.Min(0.12, duration/3)
	r := math.Min(0.12, duration/4)
	var v float64
	if input < a {
		u := input / a
		v = settle * u * u * (3 - 2*u) // smoothstep
	} else {
		// swell from settle to 1, reaching it just as the release starts
		span := math.Max(duration-r-a, 1e-9)
		v = settle + (1-settle)*math.Min(1, (input-a)/span)
	}
	return v * fadeOut(input, duration, r)
}

// staccato plays a note short and detached: it sounds for half the note
// (and never more than 0.3 s), then leaves the rest silent.
//
//	|\
//	| \______
func staccato(input float64, duration float64) float64 {
	sound := math.Min(duration/2, 0.3)
	if input >= sound {
		return 0
	}
	a := math.Min(0.003, sound/4)
	if input < a {
		return input / a
	}
	return fadeOut(input, sound, 0.3*sound)
}

// tremolo is a held note whose loudness pulses about 6 times a second, by
// up to 30%, like a vibraphone's motor, an accordion or a tremolo pedal.
// The pulse is timed to be at full strength when the attack ends, so the
// note still peaks at 1.
//
//	/\/\/\/\/\.
//	          \.
func tremolo(input float64, duration float64) float64 {
	const (
		rate  = 6.0 // Hz
		depth = 0.3
	)
	a := math.Min(0.005, duration/4)
	v := 1.0
	if input < a {
		v = input / a
	} else {
		v = 1 - depth*(1-math.Cos(2*math.Pi*rate*(input-a)))/2
	}
	return v * fadeOut(input, duration, math.Min(0.05, duration/4))
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

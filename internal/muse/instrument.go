package muse

import "math"

// An instrument synthesises a whole note at once, given its frequency and
// duration, instead of combining an envelope with a harmonic. That lets each
// overtone change on its own over the note - in a plucked or struck string
// the high overtones die away much faster than the fundamental, which is
// what makes it sound like a string rather than an organ.
//
// An instrument returns exactly sampleCount(duration) samples, peaking at 1
// and ending at 0, so volume means the same as it does for envelopes.
type instrument = func(freq, duration float64) []float64

// instruments holds every instrument by name. Each file that defines
// instruments registers them here in an init function, most through
// registerVelocityInstrument (see keyboards.go).
var instruments = map[string]instrument{}

// maxPartialFreq keeps overtones well below the Nyquist frequency (half the
// sample rate), where they would otherwise fold back as harsh aliases.
const maxPartialFreq = 0.45 * sampleRate

// a partial is one sine component of a note, whose level is a sum of
// exponentially decaying stages (weight, decay rate per second)
type partial struct {
	freq   float64
	amp    float64
	stages [][2]float64
}

// additive renders the partials into n samples, fades in over attack
// seconds, fades out over the last damp seconds, then scales the result to
// peak at 1. Each partial is a rotating phasor and each decay stage a
// per-sample multiplier, so there's no sin or exp call per sample.
func additive(n int, partials []partial, attack, damp float64) []float64 {
	out := make([]float64, n)
	for _, p := range partials {
		if p.freq >= maxPartialFreq {
			continue
		}
		w := 2 * math.Pi * p.freq / sampleRate
		cw, sw := math.Cos(w), math.Sin(w)
		c, s := 1.0, 0.0 // phasor at phase 0, so every partial starts at 0
		levels := make([]float64, len(p.stages))
		factors := make([]float64, len(p.stages))
		for i, st := range p.stages {
			levels[i] = st[0] * p.amp
			factors[i] = math.Exp(-st[1] / sampleRate)
		}
		for k := range out {
			g := 0.0
			for i := range levels {
				g += levels[i]
				levels[i] *= factors[i]
			}
			out[k] += g * s
			c, s = c*cw-s*sw, s*cw+c*sw
			if k%1024 == 1023 { // keep the phasor on the unit circle
				r := math.Hypot(c, s)
				c, s = c/r, s/r
			}
		}
	}

	// attack and damping ramps, each at most a quarter of the note
	a := math.Min(attack, float64(n)/sampleRate/4) * sampleRate
	d := math.Min(damp, float64(n)/sampleRate/4) * sampleRate
	peak := 0.0
	for k := range out {
		if float64(k) < a {
			out[k] *= float64(k) / a
		}
		if left := float64(n - 1 - k); left < d {
			out[k] *= left / d
		}
		peak = math.Max(peak, math.Abs(out[k]))
	}
	if peak > 0 {
		for k := range out {
			out[k] /= peak
		}
	}
	return out
}

// loss is how fast (per second) a string loses energy at a given frequency:
// a little for every frequency, and much more for high ones.
func loss(hz, base, linear, square float64) float64 {
	k := hz / 1000
	return base + linear*k + square*k*k
}

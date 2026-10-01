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

var instruments map[string]instrument

func init() {
	instruments = make(map[string]instrument)
	instruments["guitar"] = guitar
	instruments["piano"] = piano
}

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

// guitar is a plucked steel string. Plucking a fifth of the way along the
// string gives overtone n a level of sin(nπ/5)/n (so every 5th overtone is
// missing), the overtones are very slightly sharp, and each dies away at its
// own rate - bright at the pluck, mellow as it rings.
func guitar(freq, duration float64) []float64 {
	const (
		pluckAt = 0.2    // pluck position, as a fraction of the string
		stiff   = 0.0001 // inharmonicity: how sharp the overtones run
	)
	var ps []partial
	for n := 1.0; n <= 24; n++ {
		f := n * freq * math.Sqrt(1+stiff*n*n)
		amp := math.Sin(n*math.Pi*pluckAt) / n
		ps = append(ps, partial{f, amp, [][2]float64{{1, loss(f, 0.5, 2, 2)}}})
	}
	return additive(sampleCount(duration), ps, 0.0015, 0.01)
}

// piano is a hammered string. The hammer strikes about a seventh of the way
// along and its felt softens the highest overtones. Above the bass each note
// has two strings tuned a fraction apart, which beat gently against each
// other. The overtones run sharper than a guitar's, more so up the keyboard,
// and each decays in two stages: a quick drop after the strike, then a long,
// quiet tail. The damper takes a moment to stop the strings at the end.
func piano(freq, duration float64) []float64 {
	const (
		strikeAt = 1.0 / 7 // hammer position, as a fraction of the string
		felt     = 3000.0  // Hz; the hammer felt rolls off overtones above this
	)
	stiff := 0.00025 * freq / 261.63 // inharmonicity grows up the keyboard
	detune := []float64{0}
	if freq > 100 {
		detune = []float64{-0.8, 0.8} // cents
	}
	var ps []partial
	for n := 1.0; n <= 24; n++ {
		f := n * freq * math.Sqrt(1+stiff*n*n)
		amp := math.Sin(n*math.Pi*strikeAt) / n / (1 + (f/felt)*(f/felt))
		tail := loss(f, 0.3, 1.2, 0.8)
		for _, c := range detune {
			ps = append(ps, partial{
				freq:   f * math.Pow(2, c/1200),
				amp:    amp / float64(len(detune)),
				stages: [][2]float64{{0.7, 4*tail + 1}, {0.3, tail}},
			})
		}
	}
	return additive(sampleCount(duration), ps, 0.002, 0.05)
}

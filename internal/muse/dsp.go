package muse

import "math"

// Small signal helpers shared by the instruments, the mixer and validation.

// finiteRange reports whether x is a finite number from lo to hi
func finiteRange(x, lo, hi float64) bool {
	return !math.IsNaN(x) && !math.IsInf(x, 0) && x >= lo && x <= hi
}

// clampFloat limits x to the range lo to hi
func clampFloat(x, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, x)) }

// dBGain converts a level in dB to an amplitude factor
func dBGain(db float64) float64 { return math.Pow(10, db/20) }

// ramp is a raised-cosine rise from 0 to 1 over n samples
func ramp(k int, n float64) float64 {
	if n <= 0 || float64(k) >= n {
		return 1
	}
	return 0.5 - 0.5*math.Cos(math.Pi*float64(k)/n)
}

// normalise scales x to peak at 1
func normalise(x []float64) {
	peak := 0.0
	for _, v := range x {
		peak = math.Max(peak, math.Abs(v))
	}
	if peak > 0 {
		for k := range x {
			x[k] /= peak
		}
	}
}

// fadeTail fades the last r seconds of x to silence, so a drum that hasn't
// quite died away by the end of its buffer doesn't click
func fadeTail(x []float64, r float64) {
	m := min(len(x), sampleCount(r))
	for k := 0; k < m; k++ {
		x[len(x)-1-k] *= float64(k) / float64(m)
	}
}

// finishInstrument tapers transients and normalises without adding harmonics.
func finishInstrument(x []float64, duration, release float64) {
	if len(x) == 0 {
		return
	}
	fadeTail(x, math.Min(release, duration/4))
	x[0], x[len(x)-1] = 0, 0
	normalise(x)
}

// dampAfterHold preserves the held note and damps from key/tongue release.
// A smooth exponential reaches exact silence without renormalising the tail.
func dampAfterHold(x []float64, hold float64) {
	start := sampleCount(hold)
	if start >= len(x)-1 {
		return
	}
	count := len(x) - 1 - start
	for i := start; i < len(x); i++ {
		u := float64(i-start) / float64(count)
		x[i] *= (math.Exp(-5*u) - math.Exp(-5)) / (1 - math.Exp(-5))
	}
}

// a formant is one resonance of the vocal tract or an instrument's body
type formant struct {
	freq, width, gain float64
}

// resonanceGain is the body's gain at hz: a sum of resonance peaks
// (Lorentzians, as in formantGain) over a floor
func resonanceGain(rs []formant, floor, hz float64) float64 {
	g := floor
	for _, r := range rs {
		x := (hz - r.freq) / (r.width / 2)
		g += r.gain / math.Sqrt(1+x*x)
	}
	return g
}

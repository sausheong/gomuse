package muse

import (
	"math"
	"math/rand/v2"
)

// velocityInstrument is optional: instruments without it retain their
// legacy rendering path. Velocity shapes timbre; the mixer still controls loudness.
type velocityInstrument func(freq, duration, velocity float64) []float64

var velocityInstruments = map[string]velocityInstrument{}

func init() {
	registerVelocityInstrument("electric-piano", electricPiano, 0.35)
	registerVelocityInstrument("organ", drawbarOrgan, 0.035)
	registerVelocityInstrument("vibraphone", vibraphone, 0.6)
}

func registerVelocityInstrument(name string, fn velocityInstrument, release float64) {
	instruments[name] = func(f, d float64) []float64 { return fn(f, d, 0.7) }
	velocityInstruments[name] = fn
	releases[name] = release
}

func playingVelocity(v float64) float64 {
	if v <= 0 {
		return 0.7
	}
	return math.Max(0.1, math.Min(1, v))
}

// The low tine modes ring while pickup harmonics and the upper strike
// modes decay faster. Register and strike strength set the spectral balance.
func electricPiano(f, d, v float64) []float64 {
	v = playingVelocity(v)
	ps := []partial{}
	for h := 1.0; h <= 12; h++ {
		amp := math.Pow(0.12+0.33*v, h-1)
		if int(h)%2 == 1 {
			amp *= 1.25
		}
		rate := 0.65 + f/2600 + (h-1)*(1.4+f/1600)
		ps = append(ps, partial{h * f, amp, [][2]float64{{0.65, rate}, {0.35, rate * 0.42}}})
	}
	// Tine strike modes soften towards the top of the keyboard.
	for i, ratio := range []float64{7.03, 20.1} {
		amp := (0.035 + 0.22*v*v) / float64(i+1) / (1 + f/1200)
		ps = append(ps, partial{ratio * f, amp, [][2]float64{{1, 9 + float64(i)*12}}})
	}
	return additive(sampleCount(d), ps, 0.004-0.002*v, 0.06)
}

// Fixed drawbar registration, with a short key-contact transient. The
// fundamental remains strongest; no rotary speaker is implied by this preset.
func drawbarOrgan(f, d, v float64) []float64 {
	var ps []partial
	for i, ratio := range []float64{0.5, 1.5, 1, 2, 3, 4, 5, 6, 8} {
		amp := []float64{0.28, 0.12, 1, 0.35, 0.2, 0.12, 0.07, 0.04, 0.025}[i]
		ps = append(ps, partial{ratio * f, amp, [][2]float64{{1, 0}}})
	}
	x := additive(sampleCount(d), ps, 0.003, 0.025)
	// A very short band-shaped contact transient, not a pitched sixth harmonic.
	rng := rand.New(rand.NewPCG(uint64(f*1000), 0x0a6a))
	slow, fast := 0.0, 0.0
	for i := range x {
		t := float64(i) / sampleRate
		noise := rng.Float64()*2 - 1
		fast += 0.45 * (noise - fast)
		slow += 0.08 * (noise - slow)
		x[i] += 0.045 * (fast - slow) * math.Exp(-t/0.006) * math.Min(1, t/0.001)
	}
	finishInstrument(x, d, 0.025)
	return x
}

// Bar modes include the tuned fourth harmonic and inharmonic upper modes.
// A gentle 5 Hz amplitude modulation suggests the resonator shutters.
func vibraphone(f, d, v float64) []float64 {
	v = playingVelocity(v)
	ps := []partial{}
	for i, ratio := range []float64{1, 4, 9.7, 16, 23.1} {
		amp := []float64{1, 0.1 + 0.24*v*v, 0.075 * v * v, 0.03 * v * v, 0.015 * v * v}[i]
		decay := []float64{0.55 + f/5000, 2.1 + f/2000, 6, 12, 20}[i]
		ps = append(ps, partial{ratio * f, amp, [][2]float64{{1, decay}}})
	}
	// The resonator reinforces the fundamental after a small energy build-up.
	ps = append(ps, partial{f, 0.16, [][2]float64{{1, 0.75}, {-1, 18}}})
	x := additive(sampleCount(d), ps, 0.0015, 0.08)
	for i := range x {
		x[i] *= 0.86 + 0.14*math.Cos(2*math.Pi*5*float64(i)/sampleRate)
	}
	normalise(x)
	return x
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

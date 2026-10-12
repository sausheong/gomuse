package muse

import (
	"math"
	"math/rand/v2"
)

func init() {
	registerVelocityInstrument("violin", violin, 0.16)
	registerVelocityInstrument("cello", cello, 0.22)
}

func violin(f, d, v float64) []float64 {
	return bowedNote(f, d, v, []formant{{460, 300, 0.5}, {1100, 800, 0.3}, {2700, 1700, 0.8}}, 0.075, 9, 0x7101)
}
func cello(f, d, v float64) []float64 {
	return bowedNote(f, d, v, []formant{{180, 150, 0.65}, {450, 350, 0.4}, {1600, 1200, 0.35}}, 0.11, 7, 0xce110)
}

// A sustained bowed-string approximation: band-limited harmonics shaped by
// body resonances, gradual bow engagement, small delayed vibrato and quiet
// filtered friction. It is a solo-note model, not a connected bow simulation.
func bowedNote(f, d, velocity float64, body []formant, attack, vibrato float64, seed uint64) []float64 {
	out := make([]float64, sampleCount(d))
	if len(out) == 0 {
		return out
	}
	v := playingVelocity(velocity)
	count := max(1, min(64, int(maxPartialFreq/(f*math.Exp2(vibrato/1200)))))
	levels := make([]float64, count+1)
	for h := 1; h <= count; h++ {
		hz := float64(h) * f
		levels[h] = resonanceGain(body, 0.55, hz) / math.Pow(float64(h), 1.6-0.5*v)
	}
	rng := rand.New(rand.NewPCG(uint64(f*1000), seed))
	phase, noise, slow := 0.0, 0.0, 0.0
	attack = math.Min(attack*(1.2-0.4*v), d/3)
	for i := range out {
		t := float64(i) / sampleRate
		bow := math.Min(1, t/attack)
		bow = bow * bow * (3 - 2*bow)
		onset := math.Min(1, math.Max(0, (t-0.25)/0.35))
		cents := vibrato * onset * math.Sin(2*math.Pi*5.1*(t-0.25))
		phase += 2 * math.Pi * f * math.Exp2(cents/1200) / sampleRate
		s1, c1 := math.Sincos(phase)
		previous, current := 0.0, s1
		sum := levels[1] * current
		// Upper harmonics engage more gradually than the fundamental.
		edge := 0.6 + 0.4*bow
		gain := 1.0
		for h := 2; h <= count; h++ {
			previous, current = current, 2*c1*current-previous
			gain *= edge
			sum += levels[h] * current * gain
		}
		noise += 0.35 * (rng.Float64()*2 - 1 - noise)
		slow += 0.06 * (noise - slow)
		out[i] = bow * (sum*(1+0.008*math.Sin(2*math.Pi*1.3*t)) + 0.025*(noise-slow)*(0.5+v))
	}
	finishInstrument(out, d, 0.08)
	return out
}

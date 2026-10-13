package muse

import (
	"math"
	"math/rand/v2"
)

// Hand drums use a short skin contact and independently decaying membrane
// modes. The modes differ from the pitch-dropping kit toms.
func handDrum(root float64) func(*rand.Rand, int) []float64 {
	return func(rng *rand.Rand, n int) []float64 {
		x := make([]float64, n)
		for i := range x {
			t := float64(i) / sampleRate
			for j, r := range []float64{1, 1.59, 2.14, 2.65} {
				x[i] += math.Sin(2*math.Pi*root*r*t) * math.Exp(-t*(9+float64(j)*11)) / float64(j+1)
			}
			x[i] += .22 * (rng.Float64()*2 - 1) * math.Exp(-t*150)
		}
		return x
	}
}
func shaker(rng *rand.Rand, n int) []float64 {
	x := make([]float64, n)
	low := 0.
	for i := range x {
		t := float64(i) / sampleRate
		noise := rng.Float64()*2 - 1
		low += .18 * (noise - low)
		x[i] = (noise - low) * (1 - math.Exp(-t*650)) * math.Exp(-t*32)
	}
	return x
}
func tambourine(rng *rand.Rand, n int) []float64 {
	x := make([]float64, n)
	for j, f := range []float64{2300, 3170, 4213, 5579, 7133} {
		phase := rng.Float64() * 2 * math.Pi
		for i := range x {
			t := float64(i) / sampleRate
			x[i] += math.Sin(2*math.Pi*f*t+phase) * math.Exp(-t*(9+float64(j)*2)) * .16
		}
	}
	for i := range x {
		t := float64(i) / sampleRate
		x[i] += .4 * (rng.Float64()*2 - 1) * math.Exp(-t*45)
	}
	return x
}
func claves(rng *rand.Rand, n int) []float64 {
	detune := 1 + .003*(rng.Float64()*2-1)
	x := make([]float64, n)
	for i := range x {
		t := float64(i) / sampleRate
		x[i] = math.Sin(2*math.Pi*2500*detune*t)*math.Exp(-t*65) + .35*math.Sin(2*math.Pi*6100*detune*t)*math.Exp(-t*110)
	}
	return x
}
func triangleHit(rng *rand.Rand, n int) []float64 {
	detune := 1 + .002*(rng.Float64()*2-1)
	x := make([]float64, n)
	for j, f := range []float64{3100, 4771, 6389, 8491, 11117} {
		for i := range x {
			t := float64(i) / sampleRate
			x[i] += math.Sin(2*math.Pi*f*detune*t) * math.Exp(-t*(1.6+float64(j)*.65)) / float64(j+1)
		}
	}
	return x
}
func brushTap(rng *rand.Rand, n int) []float64 {
	x := make([]float64, n)
	low := 0.
	for i := range x {
		t := float64(i) / sampleRate
		low += .3 * (rng.Float64()*2 - 1 - low)
		x[i] = low*math.Exp(-t*19) + .08*math.Sin(2*math.Pi*185*t)*math.Exp(-t*30)
	}
	return x
}
func brushSweep(rng *rand.Rand, n int) []float64 {
	x := make([]float64, n)
	low, slow := 0., 0.
	for i := range x {
		t := float64(i) / sampleRate
		low += .32 * (rng.Float64()*2 - 1 - low)
		slow += .025 * (low - slow)
		edge := math.Min(1, float64(i)/(.025*sampleRate)) * math.Min(1, float64(n-1-i)/(.06*sampleRate))
		x[i] = (low - slow) * (.7 + .3*math.Sin(2*math.Pi*2.3*t)) * edge
	}
	return x
}

func drumNoteSamples(n note, name string) []float64 {
	d := drumKit[name]
	if name != "brushsweep" {
		return d.samples[n.hit%drumVariants]
	}
	hold := n.length
	if n.placed && n.hold > 0 {
		hold = n.hold
	}
	x := brushSweep(rand.New(rand.NewPCG(uint64(n.hit), 0xb205)), sampleCount(hold))
	normalise(x)
	return x
}
func drumNoteLength(n note, name string) int {
	if name == "brushsweep" {
		if n.placed && n.hold > 0 {
			return sampleCount(n.hold)
		}
		return sampleCount(n.length)
	}
	return len(drumKit[name].samples[0])
}

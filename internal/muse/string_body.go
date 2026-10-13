package muse

import (
	"math"
	"math/rand/v2"
)

// A short, band-shaped excitation suggests felt impact or finger/string contact.
// Fixed seeds make repeated renders reproducible; bass stroke changes remain subtle.
func addStringExcitation(x []float64, f, v float64, stroke int, piano bool) {
	rng := rand.New(rand.NewPCG(uint64(f*1000), uint64(stroke)+91))
	fast, slow := 0.0, 0.0
	decay, amount, bodyHz := .025, .035*v*v, 95.0
	if piano {
		decay, amount, bodyHz = .007+.006*(1-v), .014+.035*v*v, 420
	}
	for i := range x {
		t := float64(i) / sampleRate
		if t > .25 {
			break
		}
		noise := rng.Float64()*2 - 1
		fast += .32 * (noise - fast)
		slow += .035 * (noise - slow)
		attack := 1 - math.Exp(-t/.0015)
		contact := (fast - slow) * math.Exp(-t/decay)
		body := .45 * math.Sin(2*math.Pi*bodyHz*t) * math.Exp(-t/.035)
		x[i] += amount * attack * (contact + body)
	}
}

// A small driven resonator bank approximates related open-string resonance
// with the damper pedal down. It is bounded, local to a note, and damped by
// the caller on pedal release; this is not a coupled model of all 88 keys.
func addPedalResonance(x []float64, f float64) {
	type resonator struct{ a, r, scale, y1, y2 float64 }
	var bank []resonator
	for _, ratio := range []float64{.5, 1, 1.5, 2, 3} {
		hz := f * ratio
		if hz < 27.5 || hz > maxPartialFreq {
			continue
		}
		r := math.Exp(-1 / (sampleRate * .35))
		bank = append(bank, resonator{a: 2 * r * math.Cos(2*math.Pi*hz/sampleRate), r: r, scale: .025 * math.Sin(2*math.Pi*hz/sampleRate)})
	}
	for i, v := range x {
		for j := range bank {
			b := &bank[j]
			y := b.a*b.y1 - b.r*b.r*b.y2 + (1-b.r)*v
			x[i] += b.scale * y
			b.y2, b.y1 = b.y1, y
		}
	}
}

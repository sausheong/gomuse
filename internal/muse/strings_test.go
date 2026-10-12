package muse

import (
	"math"
	"testing"
)

// the bass plays in tune right down to its lowest string, E1, where the
// shared tuning test doesn't reach
func TestBassInTuneLow(t *testing.T) {
	for _, f := range []float64{41.20, 55, 73.42} { // E1, A1, D2
		x := bass(f, 1)
		best, bestC := 0.0, 0.0
		for c := -30.0; c <= 30; c++ {
			if p := power(x, 0, len(x), f*math.Exp2(c/1200)); p > best {
				best, bestC = p, c
			}
		}
		if math.Abs(bestC) > 5 {
			t.Errorf("bass at %v Hz is %+.0f cents out, want within 5", f, bestC)
		}
	}
}

// the bass is rounder than the guitar: less of its sound is in the upper
// overtones
func TestBassDarkerThanGuitar(t *testing.T) {
	f := 110.0 // A2, in both ranges
	b := brightness(bass(f, 1), 0, ms(300), f)
	g := brightness(guitar(f, 1), 0, ms(300), f)
	if b >= g {
		t.Fatalf("bass overtone ratio %.3g, guitar %.3g - want the bass darker", b, g)
	}
}

// Velocity changes the string excitation before mixing, not just gain.
func TestExistingStringsRespondToStrike(t *testing.T) {
	for _, name := range []string{"piano", "guitar", "bass"} {
		fn := velocityInstruments[name]
		soft, hard := fn(110, 1, 0.2), fn(110, 1, 1)
		if brightness(hard, ms(10), ms(150), 110) <= brightness(soft, ms(10), ms(150), 110) {
			t.Errorf("%s hard strike is not brighter", name)
		}
	}
}

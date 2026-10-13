package muse

import (
	"math"
	"reflect"
	"testing"
)

func TestStringBodyBoundedAndRepeatable(t *testing.T) {
	for _, f := range []float64{41.2, 110, 440, 1760} {
		for _, v := range []float64{.15, .7, 1} {
			for _, fn := range []velocityInstrument{pianoVelocity, uprightBass} {
				x := fn(f, .5, v)
				if !reflect.DeepEqual(x, fn(f, .5, v)) {
					t.Fatal("render not deterministic")
				}
				for _, a := range x {
					if math.IsNaN(a) || math.IsInf(a, 0) || math.Abs(a) > 1.000001 {
						t.Fatal("invalid instrument sample")
					}
				}
				if x[0] != 0 || x[len(x)-1] != 0 {
					t.Fatal("nonzero endpoints")
				}
			}
		}
	}
	a, b := uprightBassStroke(110, .5, .7, 0), uprightBassStroke(110, .5, .7, 1)
	if reflect.DeepEqual(a, b) {
		t.Fatal("repeated bass strokes did not vary")
	}
	var delta, energy float64
	for i := range a {
		delta += (a[i] - b[i]) * (a[i] - b[i])
		energy += a[i] * a[i]
	}
	if delta/energy > .1 {
		t.Fatal("stroke variation is excessive")
	}
}
func TestPedalResonanceAndDamping(t *testing.T) {
	for _, f := range []float64{27.5, 55, 220, 880, 3520} {
		x := pianoVelocity(f, 1, .7)
		before := append([]float64(nil), x...)
		addPedalResonance(x, f)
		if reflect.DeepEqual(x, before) {
			t.Fatal("resonance had no effect")
		}
		for _, v := range x {
			if math.IsNaN(v) || math.IsInf(v, 0) || math.Abs(v) > 1.2 {
				t.Fatal("unbounded resonance")
			}
		}
		dampAfterHold(x, .8)
		if x[len(x)-1] != 0 {
			t.Fatal("pedal release failed to damp")
		}
	}
}

package muse

import (
	"math"
	"testing"
)

func TestScatSignalContract(t *testing.T) {
	for _, syllable := range []string{"doo", "bah", "wah"} {
		for _, punchy := range []bool{false, true} {
			for _, duration := range []float64{0, .003, .2, 1} {
				a := scat(196, duration, syllable, punchy)
				if len(a) != sampleCount(duration) {
					t.Fatal("duration mismatch")
				}
				peak := 0.0
				for _, v := range a {
					if math.IsNaN(v) || math.IsInf(v, 0) || math.Abs(v) > 1.000001 {
						t.Fatal("invalid sample")
					}
					peak = math.Max(peak, math.Abs(v))
				}
				if len(a) > 0 && (a[0] != 0 || a[len(a)-1] != 0 || peak == 0) {
					t.Fatal("invalid envelope or silent voice")
				}
			}
		}
	}
}

func TestScatDeliveryChangesMoreThanGain(t *testing.T) {
	for _, syllable := range []string{"doo", "bah", "wah"} {
		a, b := scat(196, .8, syllable, false), scat(196, .8, syllable, true)
		aa, bb, ab := 0.0, 0.0, 0.0
		for i := range a {
			aa += a[i] * a[i]
			bb += b[i] * b[i]
			ab += a[i] * b[i]
		}
		// A mere volume change has correlation 1 regardless of its level.
		if math.Abs(ab/math.Sqrt(aa*bb)) > .98 {
			t.Fatal("delivery reduced to gain change")
		}
	}
}

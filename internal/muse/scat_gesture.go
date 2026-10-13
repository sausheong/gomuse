package muse

import (
	"fmt"
	"math"
)

// ScatGesture describes a hand-shaped performance of one note. Curves are
// evenly spaced across the note and smooth between points. Empty curves inherit
// the normal renderer. Timing remains explicit in the score's notes and rests.
type ScatGesture struct {
	Pitch   []float64 `yaml:"pitch,omitempty"` // cents relative to the written pitch
	Mouth   []float64 `yaml:"mouth,omitempty"` // closed oo (0) to open ah (1)
	Effort  []float64 `yaml:"effort,omitempty"`
	Air     []float64 `yaml:"air,omitempty"`
	Attack  float64   `yaml:"attack,omitempty"` // seconds; zero inherits
	Release float64   `yaml:"release,omitempty"`
}

func (g ScatGesture) validate() error {
	for _, c := range []struct {
		name   string
		points []float64
		lo, hi float64
	}{
		{"pitch", g.Pitch, -600, 600}, {"mouth", g.Mouth, 0, 1},
		{"effort", g.Effort, 0, 1}, {"air", g.Air, 0, 1},
	} {
		if len(c.points) == 1 || len(c.points) > 64 {
			return fmt.Errorf("gesture %s needs 2 to 64 points or an empty curve", c.name)
		}
		for _, v := range c.points {
			if !isFinite(v) || v < c.lo || v > c.hi {
				return fmt.Errorf("gesture %s must be finite in [%g, %g]", c.name, c.lo, c.hi)
			}
		}
	}
	for _, v := range []float64{g.Attack, g.Release} {
		if !isFinite(v) || v < 0 || v > 1 {
			return fmt.Errorf("gesture attack/release must be finite in [0, 1] seconds")
		}
	}
	return nil
}

func scatCurve(points []float64, position, fallback float64) float64 {
	if len(points) == 0 {
		return fallback
	}
	if len(points) == 1 {
		return points[0]
	}
	x := math.Max(0, math.Min(1, position)) * float64(len(points)-1)
	i := min(int(x), len(points)-2)
	f := x - float64(i)
	f = f * f * (3 - 2*f)
	return points[i] + (points[i+1]-points[i])*f
}

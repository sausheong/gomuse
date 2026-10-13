package muse

import "fmt"

// Ensemble controls intentional placement independently of random humanisation.
// Offsets affect attack only; the shared score clock determines note endings.
type Ensemble struct {
	OffsetsMS     map[string]float64 `yaml:"offsets_ms,omitempty"`
	Variation     map[string]float64 `yaml:"variation,omitempty"`
	ChordSpreadMS map[string]float64 `yaml:"chord_spread_ms,omitempty"`
}

func validateEnsemble(s *Score) error {
	for _, sec := range s.Sections {
		if !finiteRange(sec.Rubato, -.3, .3) {
			return fmt.Errorf("rubato must be finite and -0.3..0.3")
		}
	}
	if s.Ensemble == nil {
		return nil
	}
	for _, entry := range []struct {
		m      map[string]float64
		lo, hi float64
		name   string
	}{{s.Ensemble.OffsetsMS, -40, 40, "offsets_ms"}, {s.Ensemble.Variation, 0, 1, "variation"}, {s.Ensemble.ChordSpreadMS, 0, 120, "chord_spread_ms"}} {
		for ch, v := range entry.m {
			if !mixChannels[ch] || !finiteRange(v, entry.lo, entry.hi) {
				return fmt.Errorf("ensemble %s: invalid channel or amount", entry.name)
			}
		}
	}
	return nil
}

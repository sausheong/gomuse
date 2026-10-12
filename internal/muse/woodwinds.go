package muse

// These presets share the wind oscillator, but use distinct excitation
// spectra. Clarinet even harmonics strengthen above the low register.
func init() {
	for name, fn := range map[string]velocityInstrument{"flute": flute, "clarinet": clarinet} {
		registerVelocityInstrument(name, fn, 0.09)
	}
}

func flute(f, d, v float64) []float64 {
	v = playingVelocity(v)
	upper := max(0.0, min(1.0, (f-500)/1200))
	w := wind{
		resonances: []formant{{800, 1400, 0.5}}, floor: 0.5,
		tilt: 3.6 - 0.9*v - 0.35*upper, bright: 0.25, attack: 0.055 - 0.02*v,
		sustain: 0.85, settle: 0.15, damp: 0.05,
		scoopTime: 0.03, vibDepth: 8, vibRate: 5.2, vibDelay: 0.35,
		air: true, breath: 0.018 + 0.012*v, chiff: 0.025 * v, seed: 0xf107e,
	}
	return windNote(w, f, d)
}

func clarinet(f, d, v float64) []float64 {
	v = playingVelocity(v)
	upper := max(0.0, min(1.0, (f-350)/650))
	w := wind{
		resonances: []formant{{1100, 1600, 0.6}, {2800, 1800, 0.2}}, floor: 0.5,
		tilt: 1.8 - 0.5*v, even: 0.08 + 0.45*upper, bright: 0.16,
		attack: 0.045 - 0.02*v, sustain: 0.85, settle: 0.15, damp: 0.055,
		scoopTime: 0.03, vibDepth: 2, vibRate: 5, vibDelay: 0.5,
		air: true, breath: 0.008, chiff: 0.02 * v, seed: 0xc1a,
	}
	return windNote(w, f, d)
}

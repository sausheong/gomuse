package muse

import "math"

func init() {
	for name, fn := range map[string]velocityInstrument{
		"trombone": trombone, "french-horn": frenchHorn, "oboe": oboe, "bassoon": bassoon,
		"string-ensemble": stringEnsemble, "nylon-guitar": nylonGuitar,
		"glockenspiel": glockenspiel, "xylophone": xylophone, "tubular-bells": tubularBells,
	} {
		registerVelocityInstrument(name, fn, .25)
	}
	releases["string-ensemble"] = .45
	releases["nylon-guitar"] = .6
	releases["glockenspiel"] = 1.2
	releases["tubular-bells"] = 2
}

// Each bore has a distinct resonance envelope and attack. These are
// procedural approximations, sharing the phase-continuous wind oscillator.
func expandedWind(name string, v float64) wind {
	v = playingVelocity(v)
	w := wind{floor: .3, tilt: 1.6 - .65*v, bright: .2, attack: .05, sustain: .8, settle: .18, damp: .08, scoopTime: .04, vibRate: 5, vibDelay: .4, breath: .006, chiff: .018 * v, seed: 0xabc1}
	switch name {
	case "trombone":
		w.resonances = []formant{{600, 700, .8}, {1500, 1200, .5}}
		w.tilt, w.scoop, w.attack = 1.4-.7*v, 12, .05-.02*v
	case "french-horn":
		w.resonances = []formant{{350, 500, 1}, {950, 800, .4}}
		w.tilt, w.attack, w.chiff = 2.4-.7*v, .09-.025*v, .005*v
	case "oboe":
		w.resonances = []formant{{1100, 700, 1}, {2800, 1200, .7}}
		w.tilt, w.air, w.vibDepth, w.seed = 1.5-.5*v, true, 5, 0x0b0e
	case "bassoon":
		w.resonances = []formant{{450, 280, 1}, {1200, 700, .5}}
		w.tilt, w.air, w.attack, w.seed = 1.9-.6*v, true, .065, 0xba5500
	}
	return w
}
func trombone(f, d, v float64) []float64   { return windNote(expandedWind("trombone", v), f, d) }
func frenchHorn(f, d, v float64) []float64 { return windNote(expandedWind("french-horn", v), f, d) }
func oboe(f, d, v float64) []float64       { return windNote(expandedWind("oboe", v), f, d) }
func bassoon(f, d, v float64) []float64    { return windNote(expandedWind("bassoon", v), f, d) }

// Five separately detuned bow voices create beating without a chorus delay.
func stringEnsemble(f, d, v float64) []float64 {
	out := make([]float64, sampleCount(d))
	for j, cents := range []float64{-9, -4, 0, 4, 9} {
		x := bowedNote(f*math.Exp2(cents/1200), d, v, []formant{{420, 360, .6}, {1500, 1300, .45}, {2800, 1600, .35}}, .12+float64(j)*.009, 4+float64(j), uint64(0xe050+j))
		weight := .1
		if j == 2 {
			weight = .6
		}
		for i := range out {
			out[i] += weight * x[i]
		}
	}
	finishInstrument(out, d, .15)
	return out
}

func nylonGuitar(f, d, v float64) []float64 {
	v = playingVelocity(v)
	var ps []partial
	for h := 1.; h <= 40; h++ {
		hz := f * h * math.Sqrt(1+.000006*h*h)
		body := resonanceGain([]formant{{110, 100, .6}, {220, 180, .55}, {650, 600, .25}}, .6, hz)
		amp := math.Sin(math.Pi*h*.19) * body / math.Pow(h, 1.3) / (1 + math.Pow(hz/(1000+3500*v*v), 2))
		rate := loss(hz, .65, 1.1, .45)
		ps = append(ps, partial{hz, amp, [][2]float64{{.35, rate * 4}, {.65, rate}}})
	}
	return additive(sampleCount(d), ps, .007-.003*v, .06)
}

func struckModes(f, d, v float64, ratios, amps, rates []float64, attack float64) []float64 {
	v = playingVelocity(v)
	var ps []partial
	for i, r := range ratios {
		a := amps[i]
		if i > 0 {
			a *= .15 + .85*v*v
		}
		ps = append(ps, partial{f * r, a, [][2]float64{{1, rates[i] + f/12000}}})
	}
	return additive(sampleCount(d), ps, attack, .06)
}
func glockenspiel(f, d, v float64) []float64 {
	return struckModes(f, d, v, []float64{1, 2.76, 5.4, 8.93}, []float64{1, .45, .22, .1}, []float64{.65, 1.8, 4, 8}, .001)
}
func xylophone(f, d, v float64) []float64 {
	return struckModes(f, d, v, []float64{1, 3, 6, 10}, []float64{1, .65, .3, .12}, []float64{4, 9, 18, 30}, .0015)
}
func tubularBells(f, d, v float64) []float64 {
	// Modes around a perceived strike pitch rather than a harmonic string.
	return struckModes(f, d, v, []float64{1, 2, 2.756, 4.07, 5.43, 7.02}, []float64{1, .6, .5, .35, .2, .12}, []float64{.35, .65, .9, 1.4, 2.3, 3.8}, .002)
}

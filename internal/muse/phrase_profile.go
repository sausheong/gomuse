package muse

import "math"

func sustainedInstrument(name string) bool {
	switch name {
	case "trombone", "french-horn", "oboe", "bassoon", "clarinet", "flute", "trumpet", "saxophone", "sax", "violin", "cello", "synth-lead":
		return true
	}
	return false
}
func bowedInstrument(name string) bool { return name == "violin" || name == "cello" }
func windInstrument(name string) bool {
	return name == "trombone" || name == "french-horn" || name == "oboe" || name == "bassoon" || name == "clarinet" || name == "flute" || name == "trumpet" || name == "saxophone" || name == "sax"
}
func pluckedInstrument(name string) bool {
	return name == "nylon-guitar" || name == "guitar" || name == "electric-guitar" || name == "bass" || name == "upright-bass"
}

type phraseProfile struct {
	attack, release, noise, gain float64
	levels                       [33]float64
}

func profileFor(name string, f, v float64) phraseProfile {
	p := phraseProfile{attack: .045 - .018*v, release: .065, noise: .008, gain: .19}
	switch name {
	case "trombone", "french-horn", "oboe", "bassoon":
		w := expandedWind(name, v)
		p.attack, p.release, p.noise, p.gain = w.attack, w.damp, w.breath, .2
	case "violin":
		p.attack, p.release, p.noise = .065, .1, .025
	case "cello":
		p.attack, p.release, p.noise = .09, .14, .022
	case "flute":
		p.attack, p.release, p.noise, p.gain = .055-.018*v, .08, .016, .42
	case "trumpet":
		p.attack, p.release, p.noise, p.gain = .024+.025*(1-v), .07, .006, .16
	case "saxophone", "sax":
		p.attack, p.release, p.noise, p.gain = .04+.025*(1-v), .1, .016, .18
	case "synth-lead":
		p.attack, p.release, p.noise, p.gain = .008, .12, 0, .19
	}
	for h := 1; h < len(p.levels); h++ {
		hz := f * float64(h)
		harmonic := float64(h)
		switch name {
		case "trombone", "french-horn", "oboe", "bassoon":
			w := expandedWind(name, v)
			p.levels[h] = resonanceGain(w.resonances, w.floor, hz) / math.Pow(harmonic, w.tilt)
		case "violin":
			p.levels[h] = resonanceGain([]formant{{460, 300, .5}, {1100, 800, .3}, {2700, 1700, .8}}, .55, hz) / math.Pow(harmonic, 1.6-.5*v)
		case "cello":
			p.levels[h] = resonanceGain([]formant{{180, 150, .65}, {450, 350, .4}, {1600, 1200, .35}}, .55, hz) / math.Pow(harmonic, 1.65-.5*v)
		case "clarinet":
			p.levels[h] = resonanceGain([]formant{{1100, 1600, .6}, {2800, 1800, .2}}, .5, hz) / math.Pow(harmonic, 1.8-.5*v)
			if h%2 == 0 {
				p.levels[h] *= .08 + .45*clarinetRegister(f)
			}
		case "flute":
			p.levels[h] = resonanceGain([]formant{{800, 1400, .5}}, .5, hz) / math.Pow(harmonic, 3.6-.9*v-.35*math.Min(1, math.Max(0, (f-500)/1200)))
		case "trumpet", "saxophone", "sax":
			w := trumpetSpec
			if name != "trumpet" {
				w = saxophoneSpec
			}
			p.levels[h] = resonanceGain(w.resonances, w.floor, hz) / math.Pow(harmonic, w.tilt+.8*(1-v))
			p.levels[h] /= 1 + math.Pow(hz/(1400+4200*v*v), 2)
		case "synth-lead":
			p.levels[h] = 1 / math.Pow(harmonic, 1.2) / (1 + math.Pow(hz/(900+5500*v*v), 2))
		}
	}
	return p
}

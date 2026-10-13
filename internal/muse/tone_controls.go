package muse

import (
	"fmt"
	"math"
	"strings"
)

// ToneControls are channel settings; a section entry replaces its score entry.
// Pointer fields distinguish an explicit zero from an omitted default.
type ToneControls struct {
	Drawbars []float64   `yaml:"drawbars,omitempty"`
	Rotary   *Modulation `yaml:"rotary,omitempty"`
	Tremolo  *Modulation `yaml:"tremolo,omitempty"`
	Synth    *SynthPatch `yaml:"synth,omitempty"`
}
type Modulation struct {
	Rate  float64 `yaml:"rate"`
	Depth float64 `yaml:"depth"`
}
type SynthPatch struct {
	Wave           string   `yaml:"wave,omitempty"`
	Detune         *float64 `yaml:"detune,omitempty"`
	AttackMS       *float64 `yaml:"attack_ms,omitempty"`
	ReleaseMS      *float64 `yaml:"release_ms,omitempty"`
	Cutoff         *float64 `yaml:"cutoff,omitempty"`
	Resonance      *float64 `yaml:"resonance,omitempty"`
	FilterEnvelope *float64 `yaml:"filter_envelope,omitempty"`
	FilterDecayMS  *float64 `yaml:"filter_decay_ms,omitempty"`
}

func validateTone(s *Score) error {
	check := func(m map[string]ToneControls) error {
		for ch, c := range m {
			if !partChannels[ch] {
				return fmt.Errorf("tone: unknown pitched channel %q", ch)
			}
			if c.Drawbars != nil {
				if len(c.Drawbars) != 9 {
					return fmt.Errorf("tone drawbars requires nine levels (0..8)")
				}
				total := 0.
				for _, v := range c.Drawbars {
					if !finiteRange(v, 0, 8) {
						return fmt.Errorf("drawbar level must be 0..8")
					}
					total += v
				}
				if total == 0 {
					return fmt.Errorf("at least one drawbar must be audible")
				}
			}
			for _, m := range []*Modulation{c.Rotary, c.Tremolo} {
				if m != nil && (!finiteRange(m.Rate, .1, 12) || !finiteRange(m.Depth, 0, 1)) {
					return fmt.Errorf("modulation rate must be .1..12 Hz and depth 0..1")
				}
			}
			if p := c.Synth; p != nil {
				if p.Wave != "" && p.Wave != "sine" && p.Wave != "triangle" && p.Wave != "saw" && p.Wave != "square" {
					return fmt.Errorf("synth wave must be sine, triangle, saw or square")
				}
				for _, r := range []struct {
					p      *float64
					lo, hi float64
					name   string
				}{{p.Detune, 0, 30, "detune"}, {p.AttackMS, 0, 2000, "attack_ms"}, {p.ReleaseMS, 0, 3000, "release_ms"}, {p.Cutoff, 40, 18000, "cutoff"}, {p.Resonance, .5, 8, "resonance"}, {p.FilterEnvelope, -4, 4, "filter_envelope"}, {p.FilterDecayMS, 1, 5000, "filter_decay_ms"}} {
					if r.p != nil && !finiteRange(*r.p, r.lo, r.hi) {
						return fmt.Errorf("synth %s must be %g..%g", r.name, r.lo, r.hi)
					}
				}
			}
		}
		return nil
	}
	if err := check(s.Tone); err != nil {
		return err
	}
	for _, sec := range s.Sections {
		if err := check(sec.Tone); err != nil {
			return err
		}
	}
	return nil
}
func toneFor(s Score, sec Section, ch string, n *note) error {
	c, ok := sec.Tone[ch]
	if !ok {
		c, ok = s.Tone[ch]
	}
	if !ok {
		return nil
	}
	if (c.Drawbars != nil || c.Rotary != nil) && n.instrumentName != "organ" {
		return fmt.Errorf("drawbars and rotary require organ")
	}
	if c.Tremolo != nil && n.instrumentName != "electric-piano" {
		return fmt.Errorf("tremolo requires electric-piano")
	}
	if c.Synth != nil && !strings.HasPrefix(n.instrumentName, "synth-") {
		return fmt.Errorf("synth patch requires a synth instrument")
	}
	if c.Synth != nil && n.performance != nil {
		return fmt.Errorf("editable synth patches cannot use sustained performance; omit performance for this channel")
	}
	n.tone = &c
	if c.Synth != nil && c.Synth.ReleaseMS != nil {
		n.release = *c.Synth.ReleaseMS / 1000
	}
	return nil
}
func valueOr(p *float64, d float64) float64 {
	if p != nil {
		return *p
	}
	return d
}

func controlledSamples(n note, f, hold float64) []float64 {
	c := n.tone
	var x []float64
	if c.Synth != nil {
		x = patchNote(f, hold, n.release, n.velocity, n.instrumentName, *c.Synth)
	} else if c.Drawbars != nil {
		var ps []partial
		for i, r := range []float64{.5, 1.5, 1, 2, 3, 4, 5, 6, 8} {
			a := 0.
			if c.Drawbars[i] > 0 {
				a = math.Exp2(c.Drawbars[i] - 8)
			}
			ps = append(ps, partial{f * r, a, [][2]float64{{1, 0}}})
		}
		x = additive(sampleCount(hold+n.release), ps, .003, .025)
	} else {
		x = n.velocityIns(f, hold+n.release, n.velocity)
	}
	if c.Synth == nil {
		dampAfterHold(x, hold)
	}
	if c.Tremolo != nil {
		m := c.Tremolo
		for i := range x {
			t := float64(n.start+i) / sampleRate
			x[i] *= 1 - m.Depth*.5*(1-math.Cos(2*math.Pi*m.Rate*t))
		}
	}
	if c.Rotary != nil {
		// Mono rotor approximation: fractional Doppler delay plus amplitude
		// shading. Absolute score time prevents an LFO restart on each chord.
		m := c.Rotary
		src := append([]float64(nil), x...)
		for i := range x {
			phase := 2 * math.Pi * m.Rate * float64(n.start+i) / sampleRate
			pos := float64(i) - m.Depth*.0015*sampleRate*(1+math.Sin(phase))
			a := int(math.Floor(pos))
			sample := 0.
			if a >= 0 && a+1 < len(src) {
				u := pos - float64(a)
				sample = src[a]*(1-u) + src[a+1]*u
			}
			x[i] = sample * (1 - .35*m.Depth*(1-math.Cos(phase)))
		}
		fadeTail(x, .008)
	}
	if len(x) > 0 {
		x[0] = 0
		x[len(x)-1] = 0
	}
	return x
}

func patchNote(f, hold, release, v float64, name string, p SynthPatch) []float64 {
	d := hold + release
	attack := .008
	detune := 3.
	if name == "synth-pad" {
		attack = .3
		detune = 5
	}
	if name == "synth-bass" {
		attack = .004
		detune = 0
	}
	attack = valueOr(p.AttackMS, attack*1000) / 1000
	detune = valueOr(p.Detune, detune)
	wave := p.Wave
	if wave == "" {
		wave = "saw"
	}
	var ps []partial
	for _, cents := range []float64{-detune, 0, detune} {
		root := f * math.Exp2(cents/1200)
		for h := 1.; h <= 128 && h*root < maxPartialFreq; h++ {
			a := 1 / h
			switch wave {
			case "sine":
				if h != 1 {
					continue
				}
			case "square":
				if int(h)%2 == 0 {
					continue
				}
			case "triangle":
				if int(h)%2 == 0 {
					continue
				}
				a = math.Pow(-1, (h-1)/2) / (h * h)
			}
			ps = append(ps, partial{root * h, a / 3, [][2]float64{{1, 0}}})
		}
	}
	x := additive(sampleCount(d), ps, 0, 0)
	cutoff := valueOr(p.Cutoff, 1200+6000*playingVelocity(v)*playingVelocity(v))
	q := valueOr(p.Resonance, .707)
	env := valueOr(p.FilterEnvelope, 0)
	decay := valueOr(p.FilterDecayMS, 350) / 1000
	// Topology-preserving state-variable low-pass. Cutoff modulation remains
	// stable below Nyquist, and normalization bounds resonance amplification.
	ic1, ic2 := 0., 0.
	for i := range x {
		t := float64(i) / sampleRate
		fc := math.Max(40, math.Min(18000, cutoff*math.Exp2(env*math.Exp(-t/decay))))
		g := math.Tan(math.Pi * fc / sampleRate)
		k := 1 / q
		a1 := 1 / (1 + g*(g+k))
		a2 := g * a1
		a3 := g * a2
		v3 := x[i] - ic2
		v1 := a1*ic1 + a2*v3
		v2 := ic2 + a2*ic1 + a3*v3
		ic1 = 2*v1 - ic1
		ic2 = 2*v2 - ic2
		x[i] = v2
		if attack > 0 {
			x[i] *= math.Min(1, t/attack)
		}
	}
	dampAfterHold(x, hold)
	finishInstrument(x, d, math.Min(.005, release))
	return x
}

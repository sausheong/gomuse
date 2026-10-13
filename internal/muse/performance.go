package muse

import (
	"fmt"
	"math"
	"math/rand/v2"
)

// Performance enables instrument-specific phrasing, pedal and strum controls.
// Section entries replace score entries; an empty entry disables performance.
type Performance struct {
	Preset       string  `yaml:"preset,omitempty"`
	Pedal        string  `yaml:"pedal,omitempty"`        // piano: down or up
	Strum        string  `yaml:"strum,omitempty"`        // guitar: down, up, alternate
	StrumMS      float64 `yaml:"strum_ms,omitempty"`     // total chord spread, 0..120 ms
	Articulation string  `yaml:"articulation,omitempty"` // wind/bow: legato, detached, staccato; guitar: picked, let-ring, palm-mute
	Swell        float64 `yaml:"swell,omitempty"`        // 0..1; amplitude and brightness arch
	Vibrato      float64 `yaml:"vibrato,omitempty"`      // 0..30 cents, delayed onset
}

func performanceFor(s Score, sec Section, ch string) (*Performance, error) {
	p, ok := sec.Performance[ch]
	if !ok {
		p, ok = s.Performance[ch]
	}
	if !ok {
		return nil, nil
	}
	resolved, err := resolvePerformance(s, p)
	return &resolved, err
}

func validatePerformance(s *Score) error {
	check := func(m map[string]Performance) error {
		for ch, p := range m {
			resolved, err := resolvePerformance(*s, p)
			if err != nil {
				return err
			}
			p = resolved
			if !partChannels[ch] {
				return fmt.Errorf("performance: unknown pitched channel %q", ch)
			}
			if p.Articulation != "" && p.Articulation != "legato" && p.Articulation != "detached" && p.Articulation != "staccato" && p.Articulation != "picked" && p.Articulation != "let-ring" && p.Articulation != "palm-mute" {
				return fmt.Errorf("performance: unknown articulation %q", p.Articulation)
			}
			if math.IsNaN(p.Swell) || math.IsInf(p.Swell, 0) || p.Swell < 0 || p.Swell > 1 {
				return fmt.Errorf("performance swell must be 0..1")
			}
			if math.IsNaN(p.Vibrato) || math.IsInf(p.Vibrato, 0) || p.Vibrato < 0 || p.Vibrato > 30 {
				return fmt.Errorf("performance vibrato must be 0..30 cents")
			}
			if p.Pedal != "" && p.Pedal != "up" && p.Pedal != "down" {
				return fmt.Errorf("performance pedal must be up or down")
			}
			if p.Strum != "" && p.Strum != "up" && p.Strum != "down" && p.Strum != "alternate" {
				return fmt.Errorf("performance strum must be up, down or alternate")
			}
			if math.IsNaN(p.StrumMS) || math.IsInf(p.StrumMS, 0) || p.StrumMS < 0 || p.StrumMS > 120 {
				return fmt.Errorf("performance strum_ms must be 0..120")
			}
			if p.StrumMS > 0 && p.Strum == "" {
				return fmt.Errorf("strum_ms requires a strum direction")
			}
			if p.Articulation == "" && (p.Swell != 0 || p.Vibrato != 0) {
				return fmt.Errorf("performance expression requires an articulation")
			}
		}
		return nil
	}
	for name, p := range s.PerformancePresets {
		if name == "" || p.Preset != "" {
			return fmt.Errorf("performance preset names must be nonempty; nested references are unsupported")
		}
		if err := check(map[string]Performance{"C1": p}); err != nil {
			return err
		}
	}
	if err := check(s.Performance); err != nil {
		return err
	}
	for _, sec := range s.Sections {
		if err := check(sec.Performance); err != nil {
			return err
		}
	}
	return nil
}

// phraseVoice retains oscillator phase, smoothed pitch, harmonic amplitudes
// and vibrato time across a slur. Its coefficients are bounded by a fixed
// conservative gain, never independently peak-normalised for each note.
type phraseVoice struct {
	vibratoPhase, vibratoDepth float64
	rng                        *rand.Rand
	noise, slow                float64
	phase, freq, age, level    float64
	harmonics                  [33]float64
	active                     bool
}

func (v *phraseVoice) render(n note, hold float64, join bool) []int {
	p := n.performance
	f := frequency(n.pitch[0] + n.accidental[0])
	if !v.active {
		*v = phraseVoice{freq: f, active: true, rng: rand.New(rand.NewPCG(71, uint64(len(n.instrumentName))))}
	}
	fromFrequency := v.freq
	sounding := hold
	if p.Articulation == "staccato" {
		sounding *= 0.5
	}
	if p.Articulation == "detached" {
		sounding *= 0.85
	}
	profile := profileFor(n.instrumentName, f, playingVelocity(n.velocity))
	release := profile.release
	// A breath reserves silence inside this event, including its release.
	// Cap it at half the performed hold so short events stay playable.
	if n.breath > 0 {
		join = false
		end := math.Max(0, math.Min(sounding, hold-math.Min(n.breath, hold/2)))
		release = math.Min(release, end/3)
		sounding = end - release
	}
	duration := sounding + release
	if join {
		duration = hold
	}
	out := make([]int, sampleCount(duration))
	gain := n.gain
	if gain == 0 {
		gain = 1
	}
	velocity := playingVelocity(n.velocity)
	base, attack := profile.levels, profile.attack
	if n.attackMS != nil {
		attack = *n.attackMS / 1000
	}
	for i := range out {
		t := float64(i) / sampleRate
		progress := math.Min(1, t/math.Max(hold, 1.0/sampleRate))
		arch := math.Pow(math.Sin(math.Pi*progress), 2)
		target := float64(n.vol) * gain * (1 - 0.2*p.Swell + p.Swell*0.2*arch)
		// Smooth dynamics and spectrum through a pitch transition.
		v.level += (target - v.level) * 0.002
		pitchRate := 0.004
		if n.instrumentName == "clarinet" {
			pitchRate = .0025
		}
		if n.gesture != nil && n.gesture.Kind == "portamento" {
			v.freq = glideFrequency(fromFrequency, f, t, n.gesture.seconds(hold))
		} else {
			v.freq += (f - v.freq) * pitchRate
		}
		delay := math.Min(1, math.Max(0, (v.age-0.25)/0.3))
		modulation := p.Vibrato * delay * math.Sin(2*math.Pi*5.1*v.age)
		if n.noteVibrato != nil {
			rate := n.noteVibrato.RateHz
			if rate == 0 {
				rate = 5.1
			}
			v.vibratoDepth += (n.noteVibrato.depthAt(t) - v.vibratoDepth) * 0.003
			modulation = v.vibratoDepth * math.Sin(v.vibratoPhase)
			v.vibratoPhase = math.Mod(v.vibratoPhase+2*math.Pi*rate/sampleRate, 2*math.Pi)
		}
		hz := v.freq * math.Exp2(modulation/1200)
		if n.gesture != nil && n.gesture.Kind == "fall" {
			d := n.gesture.seconds(hold)
			u := math.Max(0, math.Min(1, (t-(sounding-d))/math.Max(d, 1.0/sampleRate)))
			steps := n.gesture.Semitones
			if steps == 0 {
				steps = 2
			}
			hz *= math.Exp2(-steps * u * u / 12)
		}
		v.phase = math.Mod(v.phase+2*math.Pi*hz/sampleRate, 2*math.Pi)
		s1, c1 := math.Sincos(v.phase)
		prev, cur := 0.0, s1
		sum := 0.0
		brightness := n.phraseTone(t)
		for h := 1; h < len(base); h++ {
			targetH := base[h] * (1 - 0.35*p.Swell*(1-arch)*float64(h-1)/float64(h))
			if brightness != 0 {
				targetH *= math.Pow(float64(h), 0.6*brightness)
			}
			v.harmonics[h] += (targetH - v.harmonics[h]) * 0.003
			if float64(h)*hz < maxPartialFreq {
				sum += cur * v.harmonics[h]
			}
			prev, cur = cur, 2*c1*cur-prev
		}
		env := math.Min(1, v.age/attack)
		if !join && t >= sounding {
			u := math.Min(1, (t-sounding)/release)
			env *= 0.5 + 0.5*math.Cos(math.Pi*u)
		}
		v.noise += 0.35 * (v.rng.Float64()*2 - 1 - v.noise)
		v.slow += 0.06 * (v.noise - v.slow)
		friction := profile.noise
		if n.instrumentName == "clarinet" {
			pressure := math.Max(.1, math.Min(1, velocity+n.phraseDB(t)/30+.1*n.phraseTone(t)))
			friction = .004 + .009*pressure*pressure + .006*velocity*math.Exp(-v.age/.025)
			env *= 1 - .12*math.Exp(-v.age/.018)
		}
		if windInstrument(n.instrumentName) && n.instrumentName != "clarinet" {
			friction *= math.Max(.2, math.Min(1.5, 1+n.phraseDB(t)/24+.2*n.phraseTone(t)))
		}
		sum += friction * (v.noise - v.slow) * (0.5 + velocity)
		out[i] = int(sum * env * v.level * profile.gain)
		v.age += 1.0 / sampleRate
	}
	if !join {
		if len(out) > 0 {
			out[len(out)-1] = 0
		}
		v.active = false
	}
	return out
}

func (p Performance) enabled() bool {
	return p.Articulation != "" || p.Pedal != "" || p.Strum != "" || p.StrumMS != 0
}

func validateInstrumentPerformance(n note, p Performance) error {
	switch n.instrumentName {
	case "piano":
		if p.Pedal == "" || p.Articulation != "" || p.Strum != "" || p.Swell != 0 || p.Vibrato != 0 {
			return fmt.Errorf("piano performance accepts pedal only")
		}
	case "nylon-guitar", "guitar", "electric-guitar", "bass", "upright-bass":
		if p.Pedal != "" || p.Swell != 0 || p.Vibrato != 0 {
			return fmt.Errorf("guitar performance accepts articulation and strum only")
		}
		if p.Articulation != "" && p.Articulation != "picked" && p.Articulation != "let-ring" && p.Articulation != "palm-mute" {
			return fmt.Errorf("guitar articulation must be picked, let-ring or palm-mute")
		}
		if (n.instrumentName == "bass" || n.instrumentName == "upright-bass") && (len(n.pitch) > 1 || p.Strum != "") {
			return fmt.Errorf("bass performance requires a single note and no strum")
		}
		if len(n.pitch) > 6 {
			return fmt.Errorf("guitar performance supports at most six pitches per chord")
		}
	case "trombone", "french-horn", "oboe", "bassoon", "violin", "cello", "clarinet", "flute", "trumpet", "saxophone", "sax", "synth-lead":
		if p.Pedal != "" || p.Strum != "" || (p.Articulation != "legato" && p.Articulation != "detached" && p.Articulation != "staccato") {
			return fmt.Errorf("sustained performance requires legato, detached or staccato")
		}
		if len(n.pitch) > 1 {
			return fmt.Errorf("performance requires a monophonic part")
		}
	default:
		return fmt.Errorf("performance does not support %s", n.instrumentName)
	}
	return nil
}

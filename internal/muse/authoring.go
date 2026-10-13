package muse

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// NoteMark travels with its note. Pointers distinguish explicit zero from unset.
type NoteMark struct {
	Accent  *float64      `yaml:"accent,omitempty"`
	Vibrato *VibratoShape `yaml:"vibrato,omitempty"`
	Pedal   string        `yaml:"pedal,omitempty"`
	Gesture *Gesture      `yaml:"gesture,omitempty"`
}
type HeldRule struct {
	MinBeats float64 `yaml:"min_beats"`
	NoteMark `yaml:",inline"`
}

// PlayingStyle is independent of event numbers and can be reused after editing.
type PlayingStyle struct {
	PedalLiftRest float64      `yaml:"pedal_lift_rest,omitempty"`
	Performance   *Performance `yaml:"performance,omitempty"`
	Notes         NoteMark     `yaml:"notes,omitempty"`
	Held          []HeldRule   `yaml:"held,omitempty"`
	Shape         *PhraseShape `yaml:"shape,omitempty"`
	Slur          bool         `yaml:"slur,omitempty"`
	BreathMS      float64      `yaml:"breath_ms,omitempty"`
	AttackMS      float64      `yaml:"attack_ms,omitempty"`
}
type WrittenPhrase struct {
	Notes     string   `yaml:"notes"`
	Unphrased bool     `yaml:"unphrased,omitempty"`
	PeakDB    *float64 `yaml:"peak_db,omitempty"`
	BreathMS  *float64 `yaml:"breath_ms,omitempty"`
	AttackMS  *float64 `yaml:"attack_ms,omitempty"`
}

func (p *WrittenPhrase) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind == yaml.ScalarNode {
		return n.Decode(&p.Notes)
	}
	type plain WrittenPhrase
	return n.Decode((*plain)(p))
}
func number(v float64) *float64 { return &v }
func defaultPlayingStyles() map[string]PlayingStyle {
	return map[string]PlayingStyle{
		"dry-piano": {Performance: &Performance{Pedal: "up"}},
		"plain":     {},
		"lyrical-wind": {Performance: &Performance{Articulation: "legato", Vibrato: 3}, Notes: NoteMark{Accent: number(-.9), Vibrato: &VibratoShape{Depth: 0}}, Held: []HeldRule{
			{MinBeats: 1, NoteMark: NoteMark{Accent: number(.2)}},
			{MinBeats: 1.2, NoteMark: NoteMark{Accent: number(.2), Vibrato: &VibratoShape{Depth: 4, DelayMS: 350, RiseMS: 500, RateHz: 4.8}}},
			{MinBeats: 2, NoteMark: NoteMark{Accent: number(.8), Vibrato: &VibratoShape{Depth: 7, DelayMS: 350, RiseMS: 500, RateHz: 4.8}}},
		}, Shape: &PhraseShape{StartDB: -1.8, PeakDB: .8, EndDB: -2.8, PeakAt: .58, Tone: &ToneShape{Start: -.3, Peak: .2, End: -.4}}, Slur: true, BreathMS: 65, AttackMS: 38},
		"gentle-piano":  {PedalLiftRest: .25, Performance: &Performance{Pedal: "up"}, Notes: NoteMark{Accent: number(-.8), Pedal: "down"}},
		"two-feel-bass": {Notes: NoteMark{Accent: number(-1)}},
	}
}
func mergeMark(a, b NoteMark) NoteMark {
	if b.Accent != nil {
		a.Accent = b.Accent
	}
	if b.Vibrato != nil {
		a.Vibrato = b.Vibrato
	}
	if b.Pedal != "" {
		a.Pedal = b.Pedal
	}
	if b.Gesture != nil {
		a.Gesture = b.Gesture
	}
	return a
}
func copyMap[K comparable, V any](src map[K]V) map[K]V {
	dst := make(map[K]V, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// expandWrittenScore compiles authoring syntax into the established renderer
// model without changing the caller's score or accepting ambiguous mixed sources.
func expandWrittenScore(src *Score) (*Score, error) {
	s := *src
	s.Sections = append([]Section(nil), src.Sections...)
	styles := defaultPlayingStyles()
	for name, style := range s.PlayingStyles {
		styles[name] = style
	}
	for ch, name := range s.Playing {
		if !partChannels[ch] {
			return nil, fmt.Errorf("playing: unknown channel %q", ch)
		}
		if _, ok := styles[name]; !ok {
			return nil, fmt.Errorf("unknown playing style %q", name)
		}
	}
	for name, st := range styles {
		prev := 0.
		for _, r := range st.Held {
			if !finiteRange(r.MinBeats, 0, 1000) || r.MinBeats <= prev {
				return nil, fmt.Errorf("style %s: held thresholds must be positive and increasing", name)
			}
			prev = r.MinBeats
		}
		if !finiteRange(st.PedalLiftRest, 0, 4) {
			return nil, fmt.Errorf("style %s: pedal_lift_rest must be 0..4 beats", name)
		}
		if !finiteRange(st.BreathMS, 0, 500) || !finiteRange(st.AttackMS, 0, 250) {
			return nil, fmt.Errorf("style %s: invalid breath or attack", name)
		}
		if st.Shape != nil && (st.Shape.From != 0 || st.Shape.To != 0) {
			return nil, fmt.Errorf("style shapes use phrase boundaries, not event numbers")
		}
	}
	for si := range s.Sections {
		sec := &s.Sections[si]
		for ch, name := range sec.Playing {
			if !partChannels[ch] {
				return nil, fmt.Errorf("playing: unknown channel %q", ch)
			}
			if _, ok := styles[name]; !ok {
				return nil, fmt.Errorf("unknown playing style %q", name)
			}
		}
		for c, ch := range channelNames[:3] {
			name := s.Playing[ch]
			if override, ok := sec.Playing[ch]; ok {
				name = override
			}
			if name != "" && len(sectionChannel(*sec, c)) > 0 {
				return nil, fmt.Errorf("%s: playing styles require parts instead of legacy note lists", ch)
			}
		}
		sec.Expression = copyMap(sec.Expression)
		sec.Performance = copyMap(sec.Performance)
		if len(sec.Parts) > 0 && (s.Singing != nil || s.Scat != nil || sec.Scat != nil) {
			return nil, fmt.Errorf("written parts currently support instrumental scores only")
		}
		for ch, phrases := range sec.Parts {
			if !partChannels[ch] {
				return nil, fmt.Errorf("parts: unknown pitched channel %q", ch)
			}
			c := 0
			for channelNames[c] != ch {
				c++
			}
			if len(sectionChannel(*sec, c)) > 0 {
				return nil, fmt.Errorf("section %d %s: use parts or note lists, not both", si+1, ch)
			}
			if _, ok := sec.Expression[ch]; ok {
				return nil, fmt.Errorf("section %d %s: parts cannot mix with indexed expression", si+1, ch)
			}
			name := s.Playing[ch]
			if override, ok := sec.Playing[ch]; ok {
				name = override
			}
			style, found := styles[name]
			if name != "" && !found {
				return nil, fmt.Errorf("unknown playing style %q", name)
			}
			if style.Performance != nil {
				existing, ok := sec.Performance[ch]
				if !ok {
					existing, ok = s.Performance[ch]
				}
				if ok && !reflect.DeepEqual(existing, *style.Performance) {
					return nil, fmt.Errorf("%s: playing style conflicts with explicit performance", ch)
				}
				sec.Performance[ch] = *style.Performance
			}
			e := NoteExpression{Accents: map[int]float64{}, Vibrato: map[int]VibratoShape{}, Pedal: map[int]string{}, Gestures: map[int]Gesture{}, Breaths: map[int]float64{}, AttackMS: map[int]float64{}}
			var notes []string
			for _, p := range phrases {
				first, last := 0, 0
				count := len(notes)
				for _, token := range strings.Fields(p.Notes) {
					if token == "|" {
						continue
					}
					pieces := strings.Split(token, "@")
					raw := pieces[0]
					beats := 1.
					pitchText := raw
					if prefix, rest, ok := strings.Cut(raw, ":"); ok {
						var err error
						beats, err = strconv.ParseFloat(prefix, 64)
						if err != nil || !finiteRange(beats, 0, 1e9) || beats <= 0 {
							return nil, fmt.Errorf("invalid written duration %q", prefix)
						}
						pitchText = rest
					}
					notes = append(notes, raw)
					i := len(notes)
					mark := NoteMark{}
					if pitchText == "z" && style.PedalLiftRest > 0 && beats <= style.PedalLiftRest {
						mark.Pedal = "up"
					}
					if pitchText != "z" {
						if first == 0 {
							first = i
						}
						last = i
						mark = style.Notes
						for _, r := range style.Held {
							if beats >= r.MinBeats {
								mark = mergeMark(mark, r.NoteMark)
							}
						}
					}
					for _, label := range pieces[1:] {
						m, ok := s.Marks[label]
						if !ok {
							return nil, fmt.Errorf("unknown note mark %q", label)
						}
						mark = mergeMark(mark, m)
					}
					if mark.Accent != nil {
						e.Accents[i] = *mark.Accent
					}
					if mark.Vibrato != nil {
						e.Vibrato[i] = *mark.Vibrato
					}
					if mark.Pedal != "" {
						e.Pedal[i] = mark.Pedal
					}
					if mark.Gesture != nil {
						e.Gestures[i] = *mark.Gesture
					}
				}
				if count == len(notes) {
					return nil, fmt.Errorf("section %d %s: empty written phrase", si+1, ch)
				}
				if first == 0 || p.Unphrased {
					if p.PeakDB != nil || p.AttackMS != nil || p.BreathMS != nil {
						return nil, fmt.Errorf("phrase overrides require a sounding phrase")
					}
					continue
				}
				if style.Shape != nil {
					shape := *style.Shape
					shape.From, shape.To = first, last
					if p.PeakDB != nil {
						shape.PeakDB = *p.PeakDB
					}
					e.Phrases = append(e.Phrases, shape)
				} else if p.PeakDB != nil {
					return nil, fmt.Errorf("peak_db requires a style shape")
				}
				if style.Slur {
					e.Slurs = append(e.Slurs, SlurGroup{From: first, To: last})
				}
				breath, attack := style.BreathMS, style.AttackMS
				if p.BreathMS != nil {
					breath = *p.BreathMS
				}
				if p.AttackMS != nil {
					attack = *p.AttackMS
				}
				if !finiteRange(breath, 0, 500) || !finiteRange(attack, 0, 250) {
					return nil, fmt.Errorf("invalid written phrase breath or attack")
				}
				if breath > 0 {
					e.Breaths[last] = breath
				}
				if attack > 0 {
					e.AttackMS[first] = attack
				}
			}
			switch ch {
			case "C1":
				sec.C1 = notes
			case "C2":
				sec.C2 = notes
			case "C3":
				sec.C3 = notes
			}
			sec.Expression[ch] = e
		}
	}
	return &s, nil
}

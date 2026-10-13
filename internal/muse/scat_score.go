package muse

import (
	"context"
	"fmt"
	"math"
)

// Scat controls an entirely local procedural vocal channel. Nil numeric
// controls inherit the delivery preset, so zero remains an intentional value.
type Scat struct {
	Channel      string `yaml:"channel"`
	ScatControls `yaml:",inline"`
}
type ScatControls struct {
	Clarity      *float64 `yaml:"clarity,omitempty"`
	Delivery     string   `yaml:"delivery,omitempty"`
	Effort       *float64 `yaml:"effort,omitempty"`
	Breathiness  *float64 `yaml:"breathiness,omitempty"`
	Articulation string   `yaml:"articulation,omitempty"`
	Phrasing     string   `yaml:"phrasing,omitempty"`
	Breaths      *float64 `yaml:"breaths,omitempty"`
}
type ScatSection struct {
	Phonemes     [][]string    `yaml:"phonemes,omitempty"`
	Gestures     []ScatGesture `yaml:"gestures,omitempty"`
	ScatControls `yaml:",inline"`
	Syllables    []string  `yaml:"syllables"`
	Emphasis     []float64 `yaml:"emphasis,omitempty"`
}
type scatSettings struct{ effort, air, crisp, detached, breaths, scoop, vibrato, clarity float64 }

func scatPreset(name string) scatSettings {
	switch name {
	case "playful":
		return scatSettings{.58, .13, .8, 0, .22, 32, 16, 0}
	case "assertive":
		return scatSettings{.88, .05, 1, 0, .32, 22, 12, 0}
	default:
		return scatSettings{.22, .38, .25, 0, .18, 12, 9, 0}
	}
}
func resolveScat(global ScatControls, section ScatControls) scatSettings {
	delivery := global.Delivery
	if section.Delivery != "" {
		delivery = section.Delivery
	}
	s := scatPreset(delivery)
	for _, c := range []ScatControls{global, section} {
		if c.Clarity != nil {
			s.clarity = *c.Clarity
		}
		if c.Effort != nil {
			s.effort = *c.Effort
		}
		if c.Breathiness != nil {
			s.air = *c.Breathiness
		}
		if c.Breaths != nil {
			s.breaths = *c.Breaths
		}
		switch c.Articulation {
		case "gentle":
			s.crisp = .2
		case "crisp":
			s.crisp = 1
		}
		switch c.Phrasing {
		case "connected":
			s.detached = 0
		case "detached":
			s.detached = 1
		}
	}
	return s
}
func validateScatControls(c ScatControls) error {
	if c.Delivery != "" && c.Delivery != "relaxed" && c.Delivery != "playful" && c.Delivery != "assertive" {
		return fmt.Errorf("scat.delivery must be relaxed, playful or assertive")
	}
	if c.Articulation != "" && c.Articulation != "gentle" && c.Articulation != "crisp" {
		return fmt.Errorf("scat.articulation must be gentle or crisp")
	}
	if c.Phrasing != "" && c.Phrasing != "connected" && c.Phrasing != "detached" {
		return fmt.Errorf("scat.phrasing must be connected or detached")
	}
	for _, v := range []*float64{c.Effort, c.Breathiness, c.Breaths, c.Clarity} {
		if v != nil && (!isFinite(*v) || *v < 0 || *v > 1) {
			return fmt.Errorf("scat numeric controls must be finite and between 0 and 1")
		}
	}
	return nil
}

// Reuse the exact monophonic score/tempo alignment rules of singing without
// selecting or executing a neural backend. The copy never mutates the score.
func scatProxy(s *Score) *Score {
	p := *s
	p.Scat = nil
	p.Singing = &Singing{Channel: s.Scat.Channel, Language: "en"}
	p.Sections = append([]Section(nil), s.Sections...)
	for i := range p.Sections {
		if sc := p.Sections[i].Scat; sc != nil {
			p.Sections[i].Lyrics = append([]string(nil), sc.Syllables...)
		}
	}
	return &p
}
func validateScat(s *Score) error {
	if s.Scat == nil {
		for _, sec := range s.Sections {
			if sec.Scat != nil {
				return fmt.Errorf("section scat requires a score scat channel")
			}
		}
		return nil
	}
	if s.Singing != nil {
		return fmt.Errorf("scat and neural singing cannot be combined in one score")
	}
	if err := validateScatControls(s.Scat.ScatControls); err != nil {
		return err
	}
	for i, sec := range s.Sections {
		if sec.Scat == nil {
			return fmt.Errorf("section %d requires scat.syllables including empty strings for rests", i+1)
		}
		c := sec.Scat
		if len(c.Phonemes) > 0 && len(c.Phonemes) != len(c.Syllables) {
			return fmt.Errorf("section %d: phonemes must match syllables", i+1)
		}
		for j, ph := range c.Phonemes {
			if len(ph) > 0 {
				if c.Syllables[j] == "" || c.Syllables[j] == "+" {
					return fmt.Errorf("section %d: rests and extensions cannot carry phonemes", i+1)
				}
				if _, err := parseScatPhones(ph); err != nil {
					return fmt.Errorf("section %d note %d: %w", i+1, j+1, err)
				}
			}
		}
		if len(c.Gestures) != 0 && len(c.Gestures) != len(c.Syllables) {
			return fmt.Errorf("section %d: scat.gestures must match syllables", i+1)
		}
		for _, g := range c.Gestures {
			if err := g.validate(); err != nil {
				return fmt.Errorf("section %d: %w", i+1, err)
			}
		}
		if err := validateScatControls(c.ScatControls); err != nil {
			return fmt.Errorf("section %d: %w", i+1, err)
		}
		if len(c.Emphasis) != 0 && len(c.Emphasis) != len(c.Syllables) {
			return fmt.Errorf("section %d: scat.emphasis must match syllables", i+1)
		}
		for _, v := range c.Emphasis {
			if !isFinite(v) || v < 0 || v > 1 {
				return fmt.Errorf("section %d: emphasis must be finite and between 0 and 1", i+1)
			}
		}
		for j, v := range c.Syllables {
			if len(c.Phonemes) > 0 && len(c.Phonemes[j]) > 0 {
				continue
			}
			_, isWord := scatWords[v]
			if !isWord && v != "" && v != "+" && v != "doo" && v != "bah" && v != "wah" {
				return fmt.Errorf("section %d: unsupported scat syllable %q (use doo, bah, wah, a supported word, + or an empty rest)", i+1, v)
			}
		}
	}
	return validateSinging(scatProxy(s))
}

type scatEvent struct {
	VocalNote
	pronunciation          *scatPronunciation
	gesture                ScatGesture
	settings               scatSettings
	emphasis               float64
	phraseStart, phraseEnd float64
}

func scatEvents(s *Score, t *tune) ([]scatEvent, float64, error) {
	proxy := scatProxy(s)
	req, err := vocalRequest(proxy, t)
	if err != nil {
		return nil, 0, err
	}
	var settings []scatSettings
	var emphases []float64
	var gestures []ScatGesture
	var pronunciations []*scatPronunciation
	for _, sec := range s.Sections {
		for i, syllable := range sec.Scat.Syllables {
			if syllable != "" {
				e := .5
				if len(sec.Scat.Emphasis) > 0 {
					e = sec.Scat.Emphasis[i]
				}
				settings = append(settings, resolveScat(s.Scat.ScatControls, sec.Scat.ScatControls))
				emphases = append(emphases, e)
				g := ScatGesture{}
				if len(sec.Scat.Gestures) > 0 {
					g = sec.Scat.Gestures[i]
				}
				gestures = append(gestures, g)
				var pronunciation *scatPronunciation
				if len(sec.Scat.Phonemes) > 0 && len(sec.Scat.Phonemes[i]) > 0 {
					var err error
					pronunciation, err = parseScatPhones(sec.Scat.Phonemes[i])
					if err != nil {
						return nil, 0, err
					}
				}
				pronunciations = append(pronunciations, pronunciation)
			}
		}
	}
	var events []scatEvent
	for _, p := range req.Phrases {
		last := p.Notes[len(p.Notes)-1]
		for _, n := range p.Notes {
			i := len(events)
			events = append(events, scatEvent{VocalNote: n, pronunciation: pronunciations[i], gesture: gestures[i], settings: settings[i], emphasis: emphases[i], phraseStart: p.Notes[0].Start, phraseEnd: last.Start + last.Duration})
		}
	}
	return events, req.Duration, nil
}
func attachScat(ctx context.Context, s *Score, t *tune) error {
	if !validKey(t.key) {
		return fmt.Errorf("unknown key signature - %s", t.key)
	}
	events, duration, err := scatEvents(s, t)
	if err != nil {
		return err
	}
	x, err := renderScatPhrase(ctx, events, duration)
	if err != nil {
		return err
	}
	samples := make([]int, len(x))
	for i, v := range x {
		samples[i] = int(math.Round(v * 32768))
	}
	if t.external == nil {
		t.external = make(map[int][]int)
	}
	t.external[singingChannel(scatProxy(s))] = samples
	return nil
}

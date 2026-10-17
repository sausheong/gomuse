package muse

import (
	"fmt"
	"math"
)

// NoteExpression addresses one-based events in a section, counting rests.
// Accents are dB offsets; phrases interpolate dB over actual performed time.
type NoteExpression struct {
	Preset   string               `yaml:"preset,omitempty"`
	AttackMS map[int]float64      `yaml:"attack_ms,omitempty"`
	Gestures map[int]Gesture      `yaml:"gestures,omitempty"`
	Pedal    map[int]string       `yaml:"pedal,omitempty"`
	Vibrato  map[int]VibratoShape `yaml:"vibrato,omitempty"`
	Breaths  map[int]float64      `yaml:"breaths,omitempty"`
	Slurs    []SlurGroup          `yaml:"slurs,omitempty"`
	Accents  map[int]float64      `yaml:"accents,omitempty"`
	Phrases  []PhraseShape        `yaml:"phrases,omitempty"`
}
type PhraseShape struct {
	Tone    *ToneShape `yaml:"tone,omitempty"`
	From    int        `yaml:"from"`
	To      int        `yaml:"to"`
	StartDB float64    `yaml:"start_db"`
	PeakDB  float64    `yaml:"peak_db"`
	EndDB   float64    `yaml:"end_db"`
	PeakAt  float64    `yaml:"peak_at,omitempty"` // zero defaults to midpoint
}
type phraseCurve struct {
	shape                   PhraseShape
	start, duration, offset float64
}

// boundedDB reports whether v is a usable dB offset for an accent or phrase
func boundedDB(v float64) bool { return finiteRange(v, -12, 6) }
func validateNoteExpression(s *Score) error {
	for name, e := range s.ExpressionPresets {
		if name == "" || e.Preset != "" {
			return fmt.Errorf("expression preset names must be nonempty; nested references are unsupported")
		}
	}
	for si, sec := range s.Sections {
		for ch, e := range sec.Expression {
			resolved, err := resolveExpression(s, e)
			if err != nil {
				return err
			}
			e = resolved
			c := -1
			for i, name := range channelNames[:3] {
				if ch == name {
					c = i
				}
			}
			if c < 0 {
				return fmt.Errorf("section %d expression: unknown pitched channel %q", si+1, ch)
			}
			ns := sectionChannel(sec, c)
			for i, v := range e.Accents {
				if i < 1 || i > len(ns) || !boundedDB(v) {
					return fmt.Errorf("section %d %s: accent requires event 1..%d and finite dB -12..6", si+1, ch, len(ns))
				}
			}
			for i, pedal := range e.Pedal {
				if i < 1 || i > len(ns) || (pedal != "up" && pedal != "down") {
					return fmt.Errorf("expression %s: pedal requires a valid event and up/down", ch)
				}
				n, err := makeChannelNote(ns[i-1], sec, *s, ch)
				if err != nil {
					return err
				}
				if n.instrumentName != "piano" {
					return fmt.Errorf("expression pedal requires piano")
				}
			}
			last := 0
			for _, p := range e.Phrases {
				if p.From <= last || p.To < p.From || p.To > len(ns) || !boundedDB(p.StartDB) || !boundedDB(p.PeakDB) || !boundedDB(p.EndDB) || !finiteRange(p.PeakAt, 0, 1) || p.PeakAt == 1 {
					return fmt.Errorf("section %d %s: phrases must be ordered, non-overlapping event ranges, dB -12..6, peak_at 0..1 (exclusive of 1)", si+1, ch)
				}
				last = p.To
			}
			if err := validateGestures(s, sec, ch, e, ns); err != nil {
				return err
			}
			if err := validateAdvancedExpression(s, sec, ch, e, ns); err != nil {
				return err
			}
		}
	}
	return nil
}
func (p PhraseShape) db(frac float64) float64 {
	peak := p.PeakAt
	if peak == 0 {
		peak = .5
	}
	frac = clampFloat(frac, 0, 1)
	a, b, u := p.StartDB, p.PeakDB, frac/peak
	if frac > peak {
		a, b, u = p.PeakDB, p.EndDB, (frac-peak)/(1-peak)
	}
	u = u * u * (3 - 2*u) // smooth slopes at the peak and both ends
	return a + (b-a)*u
}
func (n note) phraseDB(age float64) float64 {
	if n.phrase == nil {
		return 0
	}
	p := n.phrase
	return p.shape.db((p.offset + age - p.start) / math.Max(p.duration, 1.0/sampleRate))
}
func applyNoteExpression(s *Score, t *tune) {
	for c, notes := range t.channels() {
		if c >= 3 {
			continue
		}
		starts := noteStarts(notes)
		offset := 0
		for _, sec := range s.Sections {
			e, _ := resolveExpression(s, sec.Expression[channelNames[c]])
			applyAdvancedExpression(notes, offset, len(sectionChannel(sec, c)), e)
			applyGestures(notes, offset, e)
			pedal := ""
			for k := 0; k < len(sectionChannel(sec, c)); k++ {
				if value, ok := e.Pedal[k+1]; ok {
					pedal = value
				}
				if pedal != "" {
					n := &notes[offset+k]
					p := Performance{Pedal: pedal}
					if n.performance != nil {
						p = *n.performance
						p.Pedal = pedal
					}
					n.performance = &p
					n.release = math.Max(n.release, .18)
				}
			}
			for event, db := range e.Accents {
				n := &notes[offset+event-1]
				n.gain = n.effectiveGain() * dBGain(db)
				n.velocity = clampFloat(playingVelocity(n.velocity)+db/velocityRange, minVelocity, 1)
			}
			for _, p := range e.Phrases {
				first, last := offset+p.From-1, offset+p.To-1
				duration := starts[last] + heldTime(notes[last]) - starts[first]
				for i := first; i <= last; i++ {
					notes[i].phrase = &phraseCurve{p, starts[first], duration, starts[i]}
				}
			}
			offset += len(sectionChannel(sec, c))
		}
	}
}

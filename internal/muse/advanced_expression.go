package muse

import (
	"fmt"
	"math"
)

// VibratoShape overrides the section vibrato for one event. Its envelope restarts
// at note onset, while the modulation phase remains continuous through a slur.
type VibratoShape struct {
	Depth   float64 `yaml:"depth"` // cents; zero explicitly disables vibrato
	DelayMS float64 `yaml:"delay_ms,omitempty"`
	RiseMS  float64 `yaml:"rise_ms,omitempty"`
	RateHz  float64 `yaml:"rate_hz,omitempty"` // zero selects 5.1 Hz
}
type SlurGroup struct {
	From int `yaml:"from"`
	To   int `yaml:"to"`
}
type ToneShape struct {
	Start float64 `yaml:"start"`
	Peak  float64 `yaml:"peak"`
	End   float64 `yaml:"end"`
}

func finiteRange(x, lo, hi float64) bool {
	return !math.IsNaN(x) && !math.IsInf(x, 0) && x >= lo && x <= hi
}
func (v VibratoShape) depthAt(age float64) float64 {
	u := math.Max(0, math.Min(1, (age-v.DelayMS/1000)/math.Max(v.RiseMS/1000, 1.0/sampleRate)))
	return v.Depth * u * u * (3 - 2*u)
}
func validateAdvancedExpression(s *Score, sec Section, ch string, e NoteExpression, events []string) error {
	check := func(i int, restOK bool) error {
		if i < 1 || i > len(events) {
			return fmt.Errorf("expression %s: event %d outside 1..%d", ch, i, len(events))
		}
		n, err := makeChannelNote(events[i-1], sec, *s, ch)
		if err != nil {
			return err
		}
		if n.performance == nil || !sustainedInstrument(n.instrumentName) {
			return fmt.Errorf("expression %s: vibrato, breaths, slurs and tone require a supported sustained-instrument performance", ch)
		}
		if !restOK && len(n.pitch) == 0 {
			return fmt.Errorf("expression %s: control on rest at event %d", ch, i)
		}
		return nil
	}
	for i, ms := range e.AttackMS {
		if err := check(i, false); err != nil {
			return err
		}
		if !finiteRange(ms, .5, 250) {
			return fmt.Errorf("attack_ms must be 0.5..250")
		}
	}
	for i, v := range e.Vibrato {
		if err := check(i, false); err != nil {
			return err
		}
		if !finiteRange(v.Depth, 0, 30) || !finiteRange(v.DelayMS, 0, 5000) || !finiteRange(v.RiseMS, 0, 5000) || !(v.RateHz == 0 || finiteRange(v.RateHz, 1, 10)) {
			return fmt.Errorf("expression %s: vibrato depth 0..30 cents, delay/rise 0..5000 ms, rate 1..10 Hz (0 defaults)", ch)
		}
	}
	for i, ms := range e.Breaths {
		if err := check(i, false); err != nil {
			return err
		}
		if !finiteRange(ms, 0, 500) {
			return fmt.Errorf("expression %s: breath must be 0..500 ms", ch)
		}
	}
	if e.Slurs != nil {
		for i := range events {
			if err := check(i+1, true); err != nil {
				return err
			}
		}
	}
	last := 0
	for _, g := range e.Slurs {
		if g.From <= last || g.To < g.From || g.To > len(events) {
			return fmt.Errorf("expression %s: slurs must be ordered non-overlapping event ranges", ch)
		}
		for i := g.From; i <= g.To; i++ {
			if err := check(i, false); err != nil {
				return err
			}
		}
		last = g.To
	}
	for _, p := range e.Phrases {
		if p.Tone == nil {
			continue
		}
		if !finiteRange(p.Tone.Start, -1, 1) || !finiteRange(p.Tone.Peak, -1, 1) || !finiteRange(p.Tone.End, -1, 1) {
			return fmt.Errorf("expression %s: tone values must be -1..1", ch)
		}
		// Range validation is performed by validateNoteExpression first.
		for i := p.From; i <= p.To; i++ {
			if err := check(i, true); err != nil {
				return err
			}
		}
	}
	return nil
}
func applyAdvancedExpression(notes []note, offset, count int, e NoteExpression) {
	for i, ms := range e.AttackMS {
		ms := ms
		notes[offset+i-1].attackMS = &ms
	}
	for i, v := range e.Vibrato {
		v := v
		notes[offset+i-1].noteVibrato = &v
	}
	for i, ms := range e.Breaths {
		notes[offset+i-1].breath = ms / 1000
	}
	if e.Slurs != nil {
		for i := offset; i < offset+count; i++ {
			notes[i].explicitSlurs = true
		}
		for _, g := range e.Slurs {
			for i := g.From; i <= g.To; i++ {
				notes[offset+i-1].slurGroup = offset + g.From
			}
		}
	}
}
func canSlur(a, b note) bool {
	if b.gesture != nil && (b.gesture.Kind == "tongue" || b.gesture.Kind == "bow-change") {
		return false
	}
	if a.gesture != nil && a.gesture.Kind == "fall" {
		return false
	}
	if a.breath > 0 {
		return false
	}
	if !a.explicitSlurs && !b.explicitSlurs {
		return true
	}
	return a.explicitSlurs && b.explicitSlurs && a.slurGroup > 0 && a.slurGroup == b.slurGroup
}
func (n note) phraseTone(age float64) float64 {
	if n.phrase == nil || n.phrase.shape.Tone == nil {
		return 0
	}
	p := n.phrase
	t := p.shape.Tone
	shape := PhraseShape{StartDB: t.Start, PeakDB: t.Peak, EndDB: t.End, PeakAt: p.shape.PeakAt}
	return shape.db((p.offset + age - p.start) / math.Max(p.duration, 1.0/sampleRate))
}

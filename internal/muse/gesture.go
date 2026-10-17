package muse

import (
	"fmt"
	"math"
)

// Gesture describes a deliberate change within an event. Duration is in ms;
// semitones is a positive downward interval for falls, otherwise omitted.
type Gesture struct {
	Kind       string  `yaml:"kind"`
	DurationMS float64 `yaml:"duration_ms,omitempty"`
	Semitones  float64 `yaml:"semitones,omitempty"`
}

func (g *Gesture) transition() bool {
	return g != nil && (g.Kind == "portamento" || g.Kind == "slide" || g.Kind == "hammer-on")
}
func (g *Gesture) seconds(hold float64) float64 {
	ms := g.DurationMS
	if ms == 0 {
		ms = 120
		if g.Kind == "hammer-on" {
			ms = 18
		}
		if g.Kind == "fall" {
			ms = 240
		}
	}
	return math.Min(ms/1000, hold*.6)
}
func glideFrequency(from, to, t, duration float64) float64 {
	if duration <= 0 {
		return to
	}
	u := clampFloat(t/duration, 0, 1)
	u = u * u * (3 - 2*u)
	return from * math.Exp2(math.Log2(to/from)*u)
}
func validateGestures(s *Score, sec Section, ch string, e NoteExpression, events []string) error {
	for i, g := range e.Gestures {
		if i < 1 || i > len(events) || !finiteRange(g.DurationMS, 0, 1000) || !finiteRange(g.Semitones, 0, 12) {
			return fmt.Errorf("gesture: invalid event, duration (0..1000 ms) or fall interval (0..12 semitones)")
		}
		n, err := makeChannelNote(events[i-1], sec, *s, ch)
		if err != nil {
			return err
		}
		if n.performance == nil || len(n.pitch) != 1 {
			return fmt.Errorf("gesture requires a played monophonic performance event")
		}
		valid := false
		switch g.Kind {
		case "tongue":
			valid = windInstrument(n.instrumentName)
		case "fall":
			valid = fallInstrument(n.instrumentName)
		case "bow-change", "portamento":
			valid = bowedInstrument(n.instrumentName)
		case "slide", "mute":
			valid = pluckedInstrument(n.instrumentName)
		case "hammer-on":
			valid = guitarInstrument(n.instrumentName)
		}
		if !valid || (g.Kind != "fall" && g.Semitones != 0) {
			return fmt.Errorf("gesture %q is not supported by %s or has inapplicable semitones", g.Kind, n.instrumentName)
		}
		if !g.transition() && g.Kind != "fall" && g.DurationMS != 0 {
			return fmt.Errorf("duration_ms applies to transitions and falls only")
		}
		if g.transition() {
			if i == 1 {
				return fmt.Errorf("transition needs a preceding event in this section")
			}
			previous, err := makeChannelNote(events[i-2], sec, *s, ch)
			if err != nil {
				return err
			}
			if len(previous.pitch) != 1 || previous.instrumentName != n.instrumentName || previous.performance == nil {
				return fmt.Errorf("transition needs the same instrument's preceding monophonic note")
			}
			if g.Kind == "portamento" {
				if n.performance.Articulation != "legato" || previous.performance.Articulation != "legato" {
					return fmt.Errorf("portamento requires legato")
				}
				if e.Slurs != nil {
					inside := false
					for _, group := range e.Slurs {
						if group.From <= i-1 && group.To >= i {
							inside = true
						}
					}
					if !inside {
						return fmt.Errorf("portamento cannot cross a slur boundary")
					}
				}
			}
			if prior, ok := e.Gestures[i-1]; ok && prior.Kind == "mute" {
				return fmt.Errorf("transition cannot follow a muted note")
			}
			if e.Breaths[i-1] > 0 {
				return fmt.Errorf("transition cannot follow a breath mark")
			}
			if pluckedInstrument(n.instrumentName) && (n.performance.Articulation == "palm-mute" || previous.performance.Articulation == "palm-mute") {
				return fmt.Errorf("transition cannot use palm-mute articulation")
			}
		}
	}
	return nil
}
func applyGestures(notes []note, offset int, e NoteExpression) {
	for i, g := range e.Gestures {
		n := &notes[offset+i-1]
		n.gesture = &g
		if pluckedInstrument(n.instrumentName) {
			n.pluckPhrase = true
			if g.transition() {
				notes[offset+i-2].pluckPhrase = true
			}
		}
	}
}

// Reject timing/control combinations that cannot actually connect before any
// synthesis starts, so Check and rendering agree about transition feasibility.
func validatePreparedGestures(t *tune) error {
	for _, notes := range t.channels() {
		starts := noteStarts(notes)
		for i, n := range notes {
			if n.gesture.transition() {
				if i == 0 {
					return fmt.Errorf("transition has no predecessor")
				}
				previous := notes[i-1]
				start, previousStart := starts[i], starts[i-1]
				if start <= previousStart || math.Abs(start-previousStart-heldTime(previous)) >= .04 {
					return fmt.Errorf("transition needs contiguous performed notes (gap below 40 ms)")
				}
				if n.gesture.Kind == "portamento" && !canSlur(previous, n) {
					return fmt.Errorf("portamento needs a connected slur")
				}
			}
		}
	}
	return nil
}

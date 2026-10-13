package muse

import "fmt"

var builtInPerformancePresets = map[string]Performance{
	"lyrical-wind":   {Articulation: "legato", Vibrato: 3},
	"bowed-legato":   {Articulation: "legato", Vibrato: 7},
	"short-notes":    {Articulation: "staccato"},
	"guitar-picking": {Articulation: "picked", Strum: "alternate", StrumMS: 35},
	"bass-fingers":   {Articulation: "picked"},
	"piano-dry":      {Pedal: "up"},
}

func resolvePerformance(s Score, p Performance) (Performance, error) {
	if p.Preset == "" {
		return p, nil
	}
	name := p.Preset
	p.Preset = ""
	if p != (Performance{}) {
		return p, fmt.Errorf("performance preset references cannot contain overrides; define a separate preset")
	}
	resolved, ok := s.PerformancePresets[name]
	if !ok {
		resolved, ok = builtInPerformancePresets[name]
	}
	if !ok {
		return p, fmt.Errorf("unknown performance preset %q", name)
	}
	if resolved.Preset != "" {
		return p, fmt.Errorf("nested performance presets are not supported")
	}
	return resolved, nil
}
func resolveExpression(s *Score, e NoteExpression) (NoteExpression, error) {
	if e.Preset == "" {
		return e, nil
	}
	if len(e.AttackMS)+len(e.Gestures)+len(e.Pedal)+len(e.Vibrato)+len(e.Breaths)+len(e.Accents)+len(e.Phrases) > 0 || e.Slurs != nil {
		return e, fmt.Errorf("expression preset references cannot contain overrides")
	}
	resolved, ok := s.ExpressionPresets[e.Preset]
	if !ok {
		return e, fmt.Errorf("unknown expression preset %q", e.Preset)
	}
	if resolved.Preset != "" {
		return e, fmt.Errorf("nested expression presets are not supported")
	}
	return resolved, nil
}

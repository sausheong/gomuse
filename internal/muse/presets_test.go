package muse

import (
	"reflect"
	"testing"
)

func TestReusablePresetsEquivalentToInline(t *testing.T) {
	expression := NoteExpression{Accents: map[int]float64{2: 2}, Vibrato: map[int]VibratoShape{3: {Depth: 6, DelayMS: 100}}, Gestures: map[int]Gesture{2: {Kind: "tongue"}}}
	s := gestureScore("flute", expression)
	inline, err := buildTune(s)
	if err != nil {
		t.Fatal(err)
	}
	s.PerformancePresets = map[string]Performance{"solo": {Articulation: "legato"}}
	s.ExpressionPresets = map[string]NoteExpression{"phrase": expression}
	s.Performance = map[string]Performance{"C1": {Preset: "solo"}}
	s.Sections[0].Expression = map[string]NoteExpression{"C1": {Preset: "phrase"}}
	named, err := buildTune(s)
	if err != nil {
		t.Fatal(err)
	}
	a, _, _ := encodeChannel(inline.ch1)
	b, _, _ := encodeChannel(named.ch1)
	if !reflect.DeepEqual(a, b) {
		t.Fatal("preset changed rendering")
	}
	if s.Performance["C1"].Preset != "solo" || s.Sections[0].Expression["C1"].Preset != "phrase" {
		t.Fatal("preset expansion mutated score")
	}
	s.Performance["C1"] = Performance{Preset: "missing"}
	if Check(s) == nil {
		t.Fatal("accepted missing preset")
	}
	s.Performance["C1"] = Performance{Preset: "lyrical-wind", Vibrato: 2}
	if Check(s) == nil {
		t.Fatal("accepted ambiguous override")
	}
}

package muse

import (
	"gopkg.in/yaml.v3"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestMistyWrittenPerformanceMatchesReference(t *testing.T) {
	read := func(path string) *Score {
		t.Helper()
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var s Score
		if err = yaml.Unmarshal(b, &s); err != nil {
			t.Fatal(err)
		}
		return &s
	}
	old := read("../../examples/quality-programme/authoring-reference/expanded.yaml")
	written := read("../../examples/quality-programme/authoring-reference/compact.yaml")
	if len(written.Sections[0].Parts) == 0 {
		t.Fatal("compact fixture is not in the editable format")
	}
	expanded, err := expandWrittenScore(written)
	if err != nil {
		t.Fatal(err)
	}
	if len(old.Sections) != len(expanded.Sections) {
		t.Fatal("section count changed")
	}
	for i, a := range old.Sections {
		b := expanded.Sections[i]
		for c, ch := range channelNames[:3] {
			if !reflect.DeepEqual(sectionChannel(a, c), sectionChannel(b, c)) {
				t.Fatalf("section %d %s music changed", i+1, ch)
			}
			// YAML canonicalisation disregards irrelevant nil/empty maps.
			x, _ := yaml.Marshal(a.Expression[ch])
			y, _ := yaml.Marshal(b.Expression[ch])
			if string(x) != string(y) {
				t.Errorf("section %d %s expression differs\nold: %s\nnew: %s", i+1, ch, x, y)
			}
			xperf, err := performanceFor(*old, a, ch)
			if err != nil {
				t.Fatal(err)
			}
			yperf, err := performanceFor(*expanded, b, ch)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(xperf, yperf) {
				t.Errorf("section %d %s performance differs", i+1, ch)
			}
		}
	}
	if err := Check(written); err != nil {
		t.Fatal(err)
	}
}

func TestWrittenMarksSurviveInsertionAndDoNotMutate(t *testing.T) {
	var s Score
	err := yaml.Unmarshal([]byte(`key: C
length: 0.4
instrument: clarinet
volume: 12000
playing: {C1: lyrical-wind}
marks: {lean: {accent: 1.5}}
sections:
- parts:
    C1:
    - notes: c4 d4@lean 2:e4
`), &s)
	if err != nil {
		t.Fatal(err)
	}
	before, _ := yaml.Marshal(s)
	a, err := expandWrittenScore(&s)
	if err != nil {
		t.Fatal(err)
	}
	if a.Sections[0].Expression["C1"].Accents[2] != 1.5 {
		t.Fatal("mark missing")
	}
	if _, err := buildTune(&s); err != nil {
		t.Fatal(err)
	}
	after, _ := yaml.Marshal(s)
	if string(before) != string(after) {
		t.Fatal("compiler mutated authoring source")
	}
	s.Sections[0].Parts["C1"][0].Notes = "c4 b3 d4@lean 2:e4"
	b, err := expandWrittenScore(&s)
	if err != nil {
		t.Fatal(err)
	}
	e := b.Sections[0].Expression["C1"]
	if e.Accents[3] != 1.5 || e.Phrases[0].To != 4 || e.Slurs[0].To != 4 || e.Breaths[4] != 65 {
		t.Fatal("inserting a note broke expression anchors")
	}
}
func TestWrittenScoreRejectsAmbiguity(t *testing.T) {
	for _, doc := range []string{
		`playing: {C1: typo}`,
		`sections: [{parts: {C4: [c4]}}]`,
		`sections: [{C1: [c4], parts: {C1: [d4]}}]`,
		`sections: [{expression: {C1: {}}, parts: {C1: [d4]}}]`,
		`sections: [{parts: {C1: [c4@typo]}}]`,
		`sections: [{parts: {C1: [""]}}]`,
		`sections: [{parts: {C1: [{notes: c4, peak_db: 2}]}}]`,
		`sections: [{playing: {C1: typo}}]`,
	} {
		var s Score
		if err := yaml.Unmarshal([]byte(doc), &s); err != nil {
			t.Fatal(err)
		}
		if _, err := expandWrittenScore(&s); err == nil {
			t.Errorf("accepted %s", doc)
		}
	}
	// A scalar musical line is shorthand for notes, and bars are visual separators.
	var s Score
	if err := yaml.Unmarshal([]byte(`sections: [{parts: {C1: ["c4 d4 | e4 f4"]}}]`), &s); err != nil {
		t.Fatal(err)
	}
	e, err := expandWrittenScore(&s)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(e.Sections[0].C1, " ") != "c4 d4 e4 f4" {
		t.Fatal("bar separators became events")
	}
}

func TestCustomWrittenStyleAndExplicitZero(t *testing.T) {
	var s Score
	err := yaml.Unmarshal([]byte(`key: C
length: 0.4
instrument: clarinet
volume: 12000
playing: {C1: soft}
playing_styles:
  soft:
    performance: {articulation: legato}
    notes: {accent: -2}
    held:
    - {min_beats: 2, accent: 1}
    shape: {start_db: -2, peak_db: 0, end_db: -3}
    slur: true
    breath_ms: 80
    attack_ms: 30
marks:
  neutral: {accent: 0}
sections:
- parts:
    C1:
    - {notes: "c4 2:d4@neutral", peak_db: 0, breath_ms: 0, attack_ms: 0}
`), &s)
	if err != nil {
		t.Fatal(err)
	}
	e, err := expandWrittenScore(&s)
	if err != nil {
		t.Fatal(err)
	}
	expr := e.Sections[0].Expression["C1"]
	if expr.Accents[1] != -2 || expr.Accents[2] != 0 || len(expr.Breaths) != 0 || len(expr.AttackMS) != 0 {
		t.Fatal("custom defaults or zero overrides lost")
	}
	if err := Check(&s); err != nil {
		t.Fatal(err)
	}
	s.Sections[0].C1 = []string{"c4"}
	s.Sections[0].Parts = nil
	if _, err := expandWrittenScore(&s); err == nil {
		t.Fatal("silently ignored playing style on legacy notes")
	}
}

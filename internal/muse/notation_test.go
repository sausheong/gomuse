package muse

import (
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const notationFixture = `version: 2
name: Named score
key: C
tempo: 120
meter: 4/4
volume: 1000
tracks:
  tune: {instrument: flute, playing: soft, level: 0.8}
  piano: {instrument: piano}
  kit: {instrument: drums}
styles:
  soft:
    extends: lyrical-wind
    breath_ms: 80
patterns:
  melody: "c4 d4 e4 f4 |"
  beat: "kick hat snare hat |"
sections:
  verse:
    bars: 1
    dynamic: mp
    parts:
      tune: {use: melody}
      kit: {use: beat}
  chorus:
    extends: verse
    dynamic: mf
    parts:
      piano: "4:c3-e3-g3 |"
form: [verse, {use: chorus, repeat: 2}]
`

func TestNotationExpansionAndInheritance(t *testing.T) {
	var s Score
	if err := yaml.Unmarshal([]byte(notationFixture), &s); err != nil {
		t.Fatal(err)
	}
	if len(s.Sections) != 3 || s.Length != .5 || s.Instruments["C1"] != "flute" || s.Instruments["C2"] != "piano" {
		t.Fatalf("wrong compilation: %+v", s)
	}
	if s.Sections[1].Dynamic != "mf" || s.Sections[0].Parts["C2"][0].Notes != "4:z" {
		t.Fatal("inheritance or silence failed")
	}
	st := s.PlayingStyles["soft"]
	if st.BreathMS != 80 || st.Shape == nil || st.Performance.Articulation != "legato" {
		t.Fatal("style inheritance failed")
	}
	expanded, err := ExpandScore([]byte(notationFixture))
	if err != nil {
		t.Fatal(err)
	}
	var got Score
	if err = yaml.Unmarshal(expanded, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Sections[0].Parts) != 0 || !reflect.DeepEqual(got.Sections[1].C1, []string{"c4", "d4", "e4", "f4"}) {
		t.Fatal("not fully expanded")
	}
	if err = Check(&got); err != nil {
		t.Fatal(err)
	}
	// Repeated references must not alias their compiled expression or part slices.
	s.Sections[1].Parts["C1"][0].Notes = "4:g4"
	if s.Sections[2].Parts["C1"][0].Notes == "4:g4" {
		t.Fatal("repeats share mutable part slices")
	}
}
func TestNotationErrors(t *testing.T) {
	tests := []struct{ name, old, new, want string }{
		{"version", "version: 2", "version: 3", "unsupported score version"},
		{"tempo", "tempo: 120", "tempo: 0", "tempo"},
		{"meter", "meter: 4/4", "meter: 4/3", "meter"},
		{"typo", "tempo: 120", "temop: 120", "field temop"},
		{"track typo", "instrument: flute", "instrument: flute, levle: 1", "field levle"},
		{"bar", "c4 d4 e4 f4 |", "c4 d4 e4 |", "section verse, track tune: bar 1"},
		{"duration", "c4 d4 e4 f4 |", "3:c4", "expected 4"},
		{"pattern", "use: melody", "use: missing", "unknown pattern"},
		{"repeat", "repeat: 2", "repeat: 0", "repeat must"},
		{"cycle", "extends: verse", "extends: chorus", "cyclic"},
		{"style cycle", "extends: lyrical-wind", "extends: soft", "cyclic"},
		{"phrase typo", `melody: "c4 d4 e4 f4 |"`, `melody: {notes: "c4 d4 e4 f4 |", breth_ms: 2}`, "unknown phrase field"},
		{"pattern cycle", `melody: "c4 d4 e4 f4 |"`, `melody: {use: melody}`, "cyclic"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := strings.ReplaceAll(notationFixture, tt.old, tt.new)
			var s Score
			err := yaml.Unmarshal([]byte(doc), &s)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("got %v, want %q", err, tt.want)
			}
		})
	}
}
func TestNotationPatternSequenceAndPickup(t *testing.T) {
	doc := `version: 2
key: C
tempo: 90
tracks:
  lead: {instrument: piano}
patterns:
  cell: "0.5:c4 0.5:d4"
  pair: [{use: cell, repeat: 2}]
sections:
  pickup:
    beats: 2
    parts: {lead: {use: pair}}
form: [pickup]
`
	out, err := ExpandScore([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	var s Score
	if err = yaml.Unmarshal(out, &s); err != nil {
		t.Fatal(err)
	}
	if len(s.Sections[0].C1) != 4 {
		t.Fatal("nested pattern did not expand")
	}
}
func TestNotationExpansionBound(t *testing.T) {
	doc := strings.Replace(notationFixture, `melody: "c4 d4 e4 f4 |"`, `melody: {use: big, repeat: 1024}
  big: {use: cell, repeat: 1024}
  cell: "c4 d4 e4 f4 |"`, 1)
	var s Score
	err := yaml.Unmarshal([]byte(doc), &s)
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("expected expansion bound, got %v", err)
	}
}

func TestNotationSequenceAndSilentOverride(t *testing.T) {
	doc := strings.Replace(notationFixture, "tune: {use: melody}", "tune: {sequence: [melody]}", 1)
	doc = strings.Replace(doc, "piano: \"4:c3-e3-g3 |\"", "piano: \"4:c3-e3-g3 |\"\n      kit: off", 1)
	var s Score
	if err := yaml.Unmarshal([]byte(doc), &s); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(s.Sections[1].Drums, []string{"4:z"}) {
		t.Fatal("off did not silence inherited drums")
	}
	if err := Check(&s); err != nil {
		t.Fatal(err)
	}
}

package muse

import (
	"context"
	"gopkg.in/yaml.v3"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestScatGestureValidation(t *testing.T) {
	for _, g := range []ScatGesture{{Pitch: []float64{1}}, {Pitch: []float64{0, 601}}, {Air: []float64{0, math.NaN()}}, {Mouth: []float64{-1, 1}}, {Effort: make([]float64, 65)}, {Attack: -1}, {Release: math.Inf(1)}} {
		if g.validate() == nil {
			t.Errorf("accepted invalid gesture: %+v", g)
		}
	}
	if got := scatCurve([]float64{-100, 100}, .5, 99); got != 0 {
		t.Fatalf("midpoint=%g", got)
	}
	for _, pos := range []float64{-1, 2} {
		got := scatCurve([]float64{0, 1}, pos, 99)
		if got != math.Max(0, math.Min(1, pos)) {
			t.Fatal("curve extrapolated")
		}
	}
	var s Score
	if err := yaml.Unmarshal([]byte(scatScoreFixture), &s); err != nil {
		t.Fatal(err)
	}
	s.Sections[0].Scat.Gestures = []ScatGesture{{}}
	if validateScat(&s) == nil {
		t.Fatal("accepted misaligned gestures")
	}
}

func TestScatAuthoredPitchIsAudible(t *testing.T) {
	e := scatEvent{VocalNote: VocalNote{0, 1, 57, "doo", .4}, settings: scatPreset("relaxed"), emphasis: .5, phraseEnd: 1, gesture: ScatGesture{Pitch: []float64{300, 300}}}
	x, err := renderScatPhrase(context.Background(), []scatEvent{e}, 1)
	if err != nil {
		t.Fatal(err)
	}
	base := 220.0
	if power(x, ms(300), ms(800), base*math.Exp2(300.0/1200)) < 20*power(x, ms(300), ms(800), base) {
		t.Fatal("authored bend did not move the audible fundamental")
	}
}

func TestScatHandShapedExamples(t *testing.T) {
	for _, name := range []string{"tentative", "exuberant"} {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile("../../examples/singing/procedural/performance/" + name + ".yaml")
			if err != nil {
				t.Fatal(err)
			}
			var s Score
			if _, err = Parse(&s, data, filepath.Join(t.TempDir(), name), 0); err != nil {
				t.Fatal(err)
			}
			tune, err := buildTune(&s)
			if err != nil {
				t.Fatal(err)
			}
			events, duration, err := scatEvents(&s, &tune)
			if err != nil {
				t.Fatal(err)
			}
			if len(events) != 8 || math.Abs(duration-6) > .001 {
				t.Fatalf("events=%d duration=%g", len(events), duration)
			}
			for _, e := range events {
				if len(e.gesture.Pitch) == 0 {
					t.Fatal("lost authored gesture")
				}
			}
		})
	}
}

package muse

import (
	"math"
	"reflect"
	"testing"
)

func gestureScore(ins string, e NoteExpression) *Score {
	s := expressionScore()
	s.Instrument = ins
	s.Length = .4
	s.Sections[0].C1 = []string{"a3", "c4", "e4", "a3"}
	mode := "legato"
	if pluckedInstrument(ins) {
		mode = "picked"
	}
	s.Performance = map[string]Performance{"C1": {Articulation: mode}}
	s.Sections[0].Expression = map[string]NoteExpression{"C1": e}
	return s
}
func TestSustainedFamiliesShareExpression(t *testing.T) {
	var results = map[string][]int{}
	for _, ins := range []string{"trombone", "french-horn", "oboe", "bassoon", "clarinet", "violin", "cello", "flute", "trumpet", "saxophone", "synth-lead"} {
		s := gestureScore(ins, NoteExpression{Vibrato: map[int]VibratoShape{3: {Depth: 8, DelayMS: 80, RiseMS: 150}}, Phrases: []PhraseShape{{From: 1, To: 4, StartDB: -2, PeakDB: 1, EndDB: -3, Tone: &ToneShape{Start: -.3, Peak: .3, End: -.2}}}})
		tune, err := buildTune(s)
		if err != nil {
			t.Fatalf("%s: %v", ins, err)
		}
		x, _, err := encodeChannel(tune.ch1)
		if err != nil {
			t.Fatal(err)
		}
		peak := 0
		for _, v := range x {
			peak = max(peak, int(math.Abs(float64(v))))
		}
		if peak == 0 || peak > 30000 {
			t.Fatalf("%s bad peak %d", ins, peak)
		}
		if channelSamples(tune.ch1) < len(x) {
			t.Fatal("release not budgeted")
		}
		results[ins] = x
	}
	if reflect.DeepEqual(results["trumpet"], results["flute"]) || reflect.DeepEqual(results["violin"], results["cello"]) {
		t.Fatal("families share an identical voice")
	}
}
func TestGestureAudioAndContinuity(t *testing.T) {
	for _, tc := range []struct{ ins, kind string }{{"trombone", "fall"}, {"french-horn", "fall"}, {"oboe", "tongue"}, {"bassoon", "tongue"}, {"nylon-guitar", "slide"}, {"nylon-guitar", "hammer-on"}, {"trumpet", "tongue"}, {"saxophone", "fall"}, {"cello", "bow-change"}, {"violin", "portamento"}, {"guitar", "hammer-on"}, {"electric-guitar", "slide"}, {"bass", "slide"}, {"upright-bass", "mute"}} {
		plain, err := buildTune(gestureScore(tc.ins, NoteExpression{}))
		if err != nil {
			t.Fatal(err)
		}
		shaped, err := buildTune(gestureScore(tc.ins, NoteExpression{Gestures: map[int]Gesture{2: {Kind: tc.kind}}}))
		if err != nil {
			t.Fatalf("%s: %v", tc.kind, err)
		}
		a, an, err := encodeChannel(plain.ch1)
		if err != nil {
			t.Fatal(err)
		}
		b, bn, err := encodeChannel(shaped.ch1)
		if err != nil {
			t.Fatalf("%s: %v", tc.kind, err)
		}
		if an != bn || reflect.DeepEqual(a, b) {
			t.Fatalf("%s failed to change sound while retaining timeline", tc.kind)
		}
		if channelSamples(shaped.ch1) < len(b) {
			t.Fatal("gesture exceeds render budget")
		}
	}
	if glideFrequency(220, 440, 0, .2) != 220 || glideFrequency(220, 440, .2, .2) != 440 || math.Abs(glideFrequency(220, 440, .1, .2)-math.Sqrt(220*440)) > 1e-9 {
		t.Fatal("glide not smooth in pitch space")
	}
	// A connected slide must retain oscillator phase rather than replay a new attack.
	tune, _ := buildTune(gestureScore("guitar", NoteExpression{Gestures: map[int]Gesture{2: {Kind: "slide", DurationMS: 150}}}))
	var v pluckedVoice
	v.render(tune.ch1[0], .4, true)
	phase, age := v.phase[1], v.age
	x := v.render(tune.ch1[1], .4, true)
	if phase == 0 || v.age <= age || x[0] == 0 {
		t.Fatal("slide restarted the string")
	}
}
func TestGestureAndAttackValidation(t *testing.T) {
	for _, tc := range []struct {
		ins string
		e   NoteExpression
	}{
		{"piano", NoteExpression{Gestures: map[int]Gesture{2: {Kind: "slide"}}}},
		{"guitar", NoteExpression{Gestures: map[int]Gesture{1: {Kind: "hammer-on"}}}},
		{"guitar", NoteExpression{Gestures: map[int]Gesture{2: {Kind: "slide", DurationMS: math.NaN()}}}},
		{"trumpet", NoteExpression{Gestures: map[int]Gesture{2: {Kind: "fall", Semitones: 13}}}},
		{"cello", NoteExpression{Slurs: []SlurGroup{{1, 1}, {2, 4}}, Gestures: map[int]Gesture{2: {Kind: "portamento"}}}},
		{"flute", NoteExpression{AttackMS: map[int]float64{1: 0}}},
	} {
		if Check(gestureScore(tc.ins, tc.e)) == nil {
			t.Fatalf("accepted %+v", tc)
		}
	}
}
func TestExplicitAttackChangesOnset(t *testing.T) {
	fast, _ := buildTune(gestureScore("flute", NoteExpression{AttackMS: map[int]float64{1: 5}}))
	slow, _ := buildTune(gestureScore("flute", NoteExpression{AttackMS: map[int]float64{1: 100}}))
	a, _, _ := encodeChannel(fast.ch1)
	b, _, _ := encodeChannel(slow.ch1)
	energy := func(x []int) float64 {
		v := 0.
		for _, n := range x[sampleCount(.005):sampleCount(.035)] {
			v += float64(n) * float64(n)
		}
		return v
	}
	if energy(a) < energy(b)*2 {
		t.Fatal("explicit attack did not shape onset")
	}
}

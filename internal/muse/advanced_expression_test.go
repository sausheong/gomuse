package muse

import (
	"math"
	"reflect"
	"testing"
)

func advancedScore(e NoteExpression) *Score {
	s := expressionScore()
	s.Instrument = "clarinet"
	s.Length = .5
	s.Performance = map[string]Performance{"C1": {Articulation: "legato"}}
	s.Sections[0].Expression = map[string]NoteExpression{"C1": e}
	return s
}
func TestSelectiveVibratoDelayAndDisable(t *testing.T) {
	base, err := buildTune(advancedScore(NoteExpression{}))
	if err != nil {
		t.Fatal(err)
	}
	n := base.ch1[0]
	var steady, shaped phraseVoice
	a := steady.render(n, 1, false)
	n.noteVibrato = &VibratoShape{Depth: 15, DelayMS: 250, RiseMS: 300, RateHz: 5}
	b := shaped.render(n, 1, false)
	if !reflect.DeepEqual(a[:sampleCount(.25)], b[:sampleCount(.25)]) {
		t.Fatal("vibrato started before delay")
	}
	if reflect.DeepEqual(a[sampleCount(.6):sampleCount(.9)], b[sampleCount(.6):sampleCount(.9)]) {
		t.Fatal("vibrato failed to develop")
	}
	if n.noteVibrato.depthAt(.25) != 0 || math.Abs(n.noteVibrato.depthAt(.55)-15) > 1e-9 {
		t.Fatal("wrong vibrato envelope")
	}
	n.performance = &Performance{Articulation: "legato", Vibrato: 20}
	n.noteVibrato = &VibratoShape{}
	var disabled phraseVoice
	if !reflect.DeepEqual(a, disabled.render(n, 1, false)) {
		t.Fatal("zero note vibrato failed to override inherited vibrato")
	}
}
func TestBreathsAndSlurGroups(t *testing.T) {
	s := advancedScore(NoteExpression{Breaths: map[int]float64{2: 120}, Slurs: []SlurGroup{{1, 2}, {3, 4}}})
	got, err := buildTune(s)
	if err != nil {
		t.Fatal(err)
	}
	if !canSlur(got.ch1[0], got.ch1[1]) || canSlur(got.ch1[1], got.ch1[2]) || !canSlur(got.ch1[2], got.ch1[3]) {
		t.Fatal("slur boundaries incorrect")
	}
	a, nominal, err := encodeChannel(got.ch1)
	if err != nil {
		t.Fatal(err)
	}
	if nominal != sampleCount(2) {
		t.Fatal("breath shifted the written timeline")
	}
	for _, x := range a[sampleCount(.9):sampleCount(1)] {
		if x != 0 {
			t.Fatal("breath did not create silence before next attack")
		}
	}
	if got.ch1[2].length != .5 {
		t.Fatal("following event duration changed")
	}
	var voice phraseVoice
	n := got.ch1[1]
	n.breath = .5
	tiny := voice.render(n, .02, false)
	if len(tiny) > sampleCount(.01)+1 || len(tiny) == 0 {
		t.Fatal("short-note breath cap incorrect")
	}
	// Groups alone must interrupt a formerly connected pair, even without a rest.
	s = advancedScore(NoteExpression{Slurs: []SlurGroup{{1, 2}, {3, 4}}})
	got, _ = buildTune(s)
	if canSlur(got.ch1[1], got.ch1[2]) {
		t.Fatal("explicit group boundary connected")
	}
	legacy, _ := buildTune(advancedScore(NoteExpression{}))
	if !canSlur(legacy.ch1[1], legacy.ch1[2]) {
		t.Fatal("legacy connection changed")
	}
}
func TestContinuousToneChangesSpectrum(t *testing.T) {
	s := advancedScore(NoteExpression{Phrases: []PhraseShape{{From: 1, To: 4, Tone: &ToneShape{Start: -.7, Peak: .7, End: -.7}}}})
	tune, err := buildTune(s)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(tune.ch1[0].phraseTone(.5)-tune.ch1[1].phraseTone(0)) > 1e-9 {
		t.Fatal("tone reset at note boundary")
	}
	n := tune.ch1[0]
	render := func(tone float64) []float64 {
		n.phrase = &phraseCurve{shape: PhraseShape{Tone: &ToneShape{Start: tone, Peak: tone, End: tone}}, duration: 1}
		var v phraseVoice
		ints := v.render(n, 1, false)
		x := make([]float64, len(ints))
		for i, a := range ints {
			x[i] = float64(a)
		}
		return x
	}
	dark, bright := render(-.7), render(.7)
	f := frequency(n.pitch[0] + n.accidental[0])
	ratio := func(x []float64) float64 { return power(x, ms(200), ms(700), 3*f) / power(x, ms(200), ms(700), f) }
	if ratio(bright) <= ratio(dark)*2 {
		t.Fatal("tone changed gain without sufficient spectral change")
	}
}
func TestAdvancedExpressionRejectsInvalidControls(t *testing.T) {
	cases := []NoteExpression{
		{Vibrato: map[int]VibratoShape{0: {Depth: 3}}},
		{Vibrato: map[int]VibratoShape{1: {Depth: math.NaN()}}},
		{Vibrato: map[int]VibratoShape{1: {RateHz: 11}}},
		{Vibrato: map[int]VibratoShape{1: {DelayMS: -1}}},
		{Breaths: map[int]float64{1: math.Inf(1)}},
		{Slurs: []SlurGroup{{1, 3}, {3, 4}}},
		{Slurs: []SlurGroup{{0, 1}}},
		{Phrases: []PhraseShape{{From: 1, To: 4, Tone: &ToneShape{Peak: 2}}}},
	}
	for _, e := range cases {
		if Check(advancedScore(e)) == nil {
			t.Fatalf("accepted %+v", e)
		}
	}
	e := NoteExpression{Breaths: map[int]float64{1: 100}}
	s := advancedScore(e)
	s.Sections[0].C1[0] = "z"
	if Check(s) == nil {
		t.Fatal("accepted breath on rest")
	}
	s = advancedScore(e)
	s.Instrument = "piano"
	s.Performance = nil
	if Check(s) == nil {
		t.Fatal("accepted unsupported instrument")
	}
	s = advancedScore(NoteExpression{Slurs: []SlurGroup{{1, 4}}})
	s.Sections[0].C1[1] = "z"
	if Check(s) == nil {
		t.Fatal("accepted slur across rest")
	}
}

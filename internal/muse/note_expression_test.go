package muse

import (
	"math"
	"reflect"
	"testing"
)

func expressionScore() *Score {
	return &Score{Key: "C", Instrument: "piano", Length: .2, Volume: 10000, Plain: true, Sections: []Section{{C1: []string{"c4", "d4", "e4", "f4"}}}}
}
func TestNoteAccentsAndContinuousPhrase(t *testing.T) {
	s := expressionScore()
	baseline, err := buildTune(s)
	if err != nil {
		t.Fatal(err)
	}
	s.Sections[0].Expression = map[string]NoteExpression{"C1": {Accents: map[int]float64{2: 3, 3: -3}, Phrases: []PhraseShape{{From: 1, To: 4, StartDB: -4, PeakDB: 0, EndDB: -6}}}}
	got, err := buildTune(s)
	if err != nil {
		t.Fatal(err)
	}
	if got.ch1[1].gain <= baseline.ch1[1].gain || got.ch1[1].velocity <= playingVelocity(baseline.ch1[1].velocity) {
		t.Fatal("accent did not increase gain and touch")
	}
	if got.ch1[2].velocity >= playingVelocity(baseline.ch1[2].velocity) {
		t.Fatal("soft accent did not soften touch")
	}
	for i := 0; i < 3; i++ {
		if math.Abs(got.ch1[i].phraseDB(.2)-got.ch1[i+1].phraseDB(0)) > 1e-9 {
			t.Fatal("phrase restarted at note boundary")
		}
	}
	if got.ch1[0].phraseDB(0) != -4 || math.Abs(got.ch1[2].phraseDB(0)) > 1e-9 || got.ch1[3].phraseDB(.2) != -6 {
		t.Fatal("wrong phrase endpoints or peak")
	}
	if got.ch1[3].phraseDB(1) != -6 {
		t.Fatal("release should retain final expression")
	}
	for i := range got.ch1 {
		if got.ch1[i].length != baseline.ch1[i].length {
			t.Fatal("expression changed rhythm")
		}
	}
	// An explicit empty entry must preserve legacy output exactly.
	s.Sections[0].Expression = map[string]NoteExpression{"C1": {}}
	empty, _ := buildTune(s)
	a, _, _ := encodeChannel(baseline.ch1)
	b, _, _ := encodeChannel(empty.ch1)
	if !reflect.DeepEqual(a, b) {
		t.Fatal("empty expression changed output")
	}
}
func TestNoteExpressionValidation(t *testing.T) {
	for _, e := range []NoteExpression{
		{Accents: map[int]float64{0: 1}}, {Accents: map[int]float64{5: 1}}, {Accents: map[int]float64{1: math.NaN()}},
		{Phrases: []PhraseShape{{From: 1, To: 5}}}, {Phrases: []PhraseShape{{From: 2, To: 1}}},
		{Phrases: []PhraseShape{{From: 1, To: 2}, {From: 2, To: 4}}}, {Phrases: []PhraseShape{{From: 1, To: 4, PeakAt: 1}}},
		{Phrases: []PhraseShape{{From: 1, To: 4, PeakDB: math.Inf(1)}}},
	} {
		s := expressionScore()
		s.Sections[0].Expression = map[string]NoteExpression{"C1": e}
		if Check(s) == nil {
			t.Fatalf("accepted %+v", e)
		}
	}
	s := expressionScore()
	s.Sections[0].Expression = map[string]NoteExpression{"drums": {}}
	if Check(s) == nil {
		t.Fatal("accepted unsupported channel")
	}
}
func TestPhraseFollowsPerformedTiming(t *testing.T) {
	s := expressionScore()
	s.Plain = false
	zero := 0.0
	s.Humanize = &zero
	s.Sections[0].Ritardando = .5
	s.Sections[0].Fermata = 1
	s.Sections[0].Expression = map[string]NoteExpression{"C1": {Phrases: []PhraseShape{{From: 1, To: 4, StartDB: -3, EndDB: -5}}}}
	tune, err := buildTune(s)
	if err != nil {
		t.Fatal(err)
	}
	first, last := tune.ch1[0], tune.ch1[3]
	if first.phrase.duration <= .8 {
		t.Fatal("phrase ignored tempo stretch")
	}
	if math.Abs(last.phraseDB(last.hold)+5) > 1e-9 {
		t.Fatal("phrase did not end with performed hold")
	}
}

func TestPhraseGainReachesAudio(t *testing.T) {
	s := expressionScore()
	base, _ := buildTune(s)
	s.Sections[0].Expression = map[string]NoteExpression{"C1": {Phrases: []PhraseShape{{From: 1, To: 4, StartDB: -6, PeakDB: -6, EndDB: -6}}}}
	shaped, _ := buildTune(s)
	a, _, err := encodeChannel(base.ch1)
	if err != nil {
		t.Fatal(err)
	}
	b, _, err := encodeChannel(shaped.ch1)
	if err != nil {
		t.Fatal(err)
	}
	if len(a) != len(b) {
		t.Fatal("phrase changed duration")
	}
	for i := range a {
		if math.Abs(float64(b[i])-float64(a[i])*dBGain(-6)) > 1 {
			t.Fatal("phrase gain did not reach rendered audio")
		}
	}
}

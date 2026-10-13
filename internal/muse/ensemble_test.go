package muse

import (
	"math"
	"testing"
)

func TestEnsembleSharedClockAndOffsets(t *testing.T) {
	s := expressionScore()
	s.Plain = false
	z := 0.
	s.Humanize = &z
	s.Sections[0].C2 = append([]string(nil), s.Sections[0].C1...)
	s.Sections[0].Rubato = .2
	s.Ensemble = &Ensemble{OffsetsMS: map[string]float64{"C2": 12}}
	tune, err := buildTune(s)
	if err != nil {
		t.Fatal(err)
	}
	for i, a := range tune.ch1 {
		b := tune.ch2[i]
		if math.Abs(float64(b.start-a.start)/sampleRate-.012) > 1.0/sampleRate {
			t.Fatal("intentional offset wrong")
		}
		_, ae := secs(a)
		_, be := secs(b)
		if math.Abs(ae-be) > 1e-9 {
			t.Fatal("parts lost shared ending grid")
		}
	}
	again, _ := buildTune(s)
	for i, n := range tune.ch1 {
		m := again.ch1[i]
		if n.start != m.start || n.hold != m.hold || n.spread != m.spread {
			t.Fatal("timing not repeatable")
		}
	}
	tm := buildTempoMap(s, &tune)
	if math.Abs(tm[0].at(tm[0].written, true)-tm[0].duration) > 1e-9 {
		t.Fatal("rubato drifted section boundary")
	}
	prev := -1.
	for u := 0.; u <= tm[0].written; u += .001 {
		now := tm[0].at(u, false)
		if now <= prev {
			t.Fatal("nonmonotonic rubato")
		}
		prev = now
	}
}
func TestEnsembleChordPlacementAndValidation(t *testing.T) {
	s := expressionScore()
	s.Plain = false
	z := 0.
	s.Humanize = &z
	s.Sections[0].C1 = []string{"c4-e4-g4", "d4-f4-a4"}
	s.Ensemble = &Ensemble{ChordSpreadMS: map[string]float64{"C1": 40}}
	tune, err := buildTune(s)
	if err != nil {
		t.Fatal(err)
	}
	if tune.ch1[0].spread != .02 {
		t.Fatal("wrong chord spread")
	}
	s.Ensemble.OffsetsMS = map[string]float64{"C1": math.NaN()}
	if Check(s) == nil {
		t.Fatal("accepted nan")
	}
	s.Ensemble = nil
	s.Sections[0].Rubato = .4
	if Check(s) == nil {
		t.Fatal("accepted excessive rubato")
	}
}

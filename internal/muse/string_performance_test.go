package muse

import (
	"math"
	"testing"
)

func stringNote(t *testing.T, ins, event string, p Performance) note {
	t.Helper()
	n, err := makeChannelNote(event, Section{}, Score{Instrument: ins, Length: 0.25, Volume: 10000, Plain: true, Performance: map[string]Performance{"C1": p}}, "C1")
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func TestPianoPedalLiftsOnRest(t *testing.T) {
	down := Performance{Pedal: "down"}
	up := Performance{Pedal: "up"}
	notes := []note{stringNote(t, "piano", "c4", down), stringNote(t, "piano", "z", down), stringNote(t, "piano", "z", up), stringNote(t, "piano", "z", up)}
	prepareStringPerformance(notes)
	if math.Abs(notes[0].sustainHold-0.5) > 1e-9 {
		t.Fatalf("pedal lift at %g", notes[0].sustainHold)
	}
	x, nominal, err := encodeChannel(notes)
	if err != nil {
		t.Fatal(err)
	}
	if nominal != sampleCount(1) {
		t.Fatal("pedal changed written duration")
	}
	energy := 0
	for _, a := range x[sampleCount(.3):sampleCount(.4)] {
		energy += a * a
	}
	if energy == 0 {
		t.Fatal("pedal did not sustain through rest")
	}
	for _, a := range x[sampleCount(.65):sampleCount(.9)] {
		if a != 0 {
			t.Fatal("pedal release failed to silence")
		}
	}
	if channelSamples(notes) < len(x) {
		t.Fatal("pedal exceeds budget")
	}
}

func TestGuitarMuteStrumAndRestrike(t *testing.T) {
	pick := stringNote(t, "guitar", "a3", Performance{Articulation: "picked"})
	mute := stringNote(t, "guitar", "a3", Performance{Articulation: "palm-mute"})
	if len(renderStringPerformance(mute, 0)) >= len(renderStringPerformance(pick, 0)) {
		t.Fatal("palm mute should shorten decay")
	}
	chord := stringNote(t, "guitar", "a3-a4", Performance{Strum: "alternate", StrumMS: 100})
	down, up := renderStringPerformance(chord, 0), renderStringPerformance(chord, 1)
	strength := func(x []int, f float64) float64 {
		v := make([]float64, len(x))
		for i, a := range x {
			v[i] = float64(a)
		}
		return power(v, ms(5), ms(40), f)
	}
	if strength(down, 220) <= strength(up, 220) {
		t.Fatal("down strum should start with low note")
	}
	if strength(up, 440) <= strength(down, 440) {
		t.Fatal("up strum should start with high note")
	}
	notes := []note{stringNote(t, "guitar", "a3", Performance{Articulation: "let-ring"}), stringNote(t, "guitar", "b3", Performance{Articulation: "let-ring"}), stringNote(t, "guitar", "a3", Performance{Articulation: "let-ring"})}
	prepareStringPerformance(notes)
	pitch := notes[0].pitch[0] + notes[0].accidental[0]
	if notes[0].stopPitches[pitch] != 0.5 {
		t.Fatal("repeated pitch does not damp the old string")
	}
	for _, n := range notes {
		if channelSamples([]note{n}) < len(renderStringPerformance(n, 0)) {
			t.Fatal("string budget too small")
		}
	}
}

func TestStringPerformanceValidation(t *testing.T) {
	for _, tc := range []struct {
		ins string
		p   Performance
	}{
		{"piano", Performance{Articulation: "legato"}},
		{"violin", Performance{Articulation: "picked"}},
		{"guitar", Performance{Pedal: "down"}},
		{"guitar", Performance{StrumMS: 20}},
		{"piano", Performance{Pedal: "half"}},
		{"guitar", Performance{Strum: "down", StrumMS: math.NaN()}},
	} {
		s := Score{Instrument: tc.ins, Length: .2, Volume: 1000, Performance: map[string]Performance{"C1": tc.p}}
		if validatePerformance(&s) == nil {
			if _, err := makeChannelNote("a4", Section{}, s, "C1"); err == nil {
				t.Fatalf("accepted invalid %+v", tc)
			}
		}
	}
}

func TestEventPedalClearsHarmony(t *testing.T) {
	s := expressionScore()
	s.Sections[0].C1 = []string{"c4", "z", "d4", "z"}
	s.Sections[0].Expression = map[string]NoteExpression{"C1": {Pedal: map[int]string{1: "down", 2: "up", 3: "down", 4: "up"}}}
	tune, err := buildTune(s)
	if err != nil {
		t.Fatal(err)
	}
	if tune.ch1[0].sustainHold != .2 || math.Abs(tune.ch1[2].sustainHold-.2) > 1e-9 {
		t.Fatal("pedal failed to clear on rest")
	}
	for _, n := range tune.ch1 {
		if n.performance == nil || n.release < .18 {
			t.Fatal("pedal renderer not budgeted")
		}
	}
	for _, pedal := range []map[int]string{{0: "up"}, {1: "half"}, {5: "down"}} {
		s.Sections[0].Expression = map[string]NoteExpression{"C1": {Pedal: pedal}}
		if Check(s) == nil {
			t.Fatal("accepted invalid pedal")
		}
	}
	s.Instrument = "guitar"
	s.Sections[0].Expression = map[string]NoteExpression{"C1": {Pedal: map[int]string{1: "down"}}}
	if Check(s) == nil {
		t.Fatal("accepted guitar pedal")
	}
}

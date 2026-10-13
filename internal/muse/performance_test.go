package muse

import (
	"gopkg.in/yaml.v3"
	"math"
	"reflect"
	"testing"
)

func TestPerformanceValidation(t *testing.T) {
	for _, bad := range []string{
		"{C1: {articulation: slurred}}", "{drums: {articulation: legato}}", "{C1: {articulation: legato, swell: 2}}", "{C1: {articulation: legato, vibrato: .nan}}", "{C1: {swell: 0.3}}",
	} {
		var s Score
		if err := yaml.Unmarshal([]byte("performance: "+bad), &s); err != nil {
			t.Fatal(err)
		}
		if validatePerformance(&s) == nil {
			t.Errorf("accepted %s", bad)
		}
	}
	for _, tc := range []struct{ ins, note string }{{"piano", "c4"}, {"clarinet", "c4-e4"}} {
		s := Score{Instrument: tc.ins, Length: 0.2, Volume: 1000, Performance: map[string]Performance{"C1": {Articulation: "legato"}}}
		if _, err := makeChannelNote(tc.note, Section{}, s, "C1"); err == nil {
			t.Fatal("unsupported performance accepted")
		}
	}
}

func TestPerformanceLegacyAndDisable(t *testing.T) {
	s := Score{Instrument: "clarinet", Length: 0.2, Volume: 1000}
	a, _ := makeChannelNote("a4", Section{}, s, "C1")
	s.Performance = map[string]Performance{"C1": {Articulation: "legato"}}
	b, _ := makeChannelNote("a4", Section{Performance: map[string]Performance{"C1": {}}}, s, "C1")
	x, _, err := encodeChannel([]note{a})
	if err != nil {
		t.Fatal(err)
	}
	y, _, err := encodeChannel([]note{b})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(x, y) {
		t.Fatal("disabled performance changed legacy audio")
	}
}

func TestPhraseVoiceContinuityAndRelease(t *testing.T) {
	s := Score{Instrument: "clarinet", Length: 0.3, Volume: 12000, Performance: map[string]Performance{"C1": {Articulation: "legato"}}}
	n, _ := makeChannelNote("a4", Section{}, s, "C1")
	var v phraseVoice
	first := v.render(n, 0.3, true)
	phase, age := v.phase, v.age
	second := v.render(n, 0.3, false)
	if age < 0.29 || phase == 0 {
		t.Fatal("voice did not retain state")
	}
	if math.Abs(float64(first[len(first)-1]-second[0])) > 1000 {
		t.Fatal("discontinuous joined boundary")
	}
	if v.active || second[len(second)-1] != 0 {
		t.Fatal("note-off did not end voice")
	}
	// Repeated legato notes should match one held note when expression is flat.
	var whole phraseVoice
	sustained := whole.render(n, 0.6, false)
	joined := append(first, second...)
	if len(joined) != len(sustained) {
		t.Fatal("joined duration changed")
	}
	for i := range joined {
		if math.Abs(float64(joined[i]-sustained[i])) > 1 {
			t.Fatalf("phase reset at sample %d", i)
		}
	}
}

func TestPerformanceTimingAndRests(t *testing.T) {
	s := Score{Instrument: "violin", Length: 0.25, Volume: 8000, Performance: map[string]Performance{"C1": {Articulation: "legato", Swell: 0.4, Vibrato: 9}}}
	var notes []note
	for _, pitch := range []string{"a4", "b4", "z", "a4"} {
		n, err := makeChannelNote(pitch, Section{}, s, "C1")
		if err != nil {
			t.Fatal(err)
		}
		notes = append(notes, n)
	}
	x, nominal, err := encodeChannel(notes)
	if err != nil {
		t.Fatal(err)
	}
	if nominal != sampleCount(1) {
		t.Fatal("performance changed the beat grid")
	}
	for _, v := range x[sampleCount(0.65):sampleCount(0.75)] {
		if v != 0 {
			t.Fatal("voice did not stop during rest")
		}
	}
	y, _, _ := encodeChannel(notes)
	if !reflect.DeepEqual(x, y) {
		t.Fatal("non-deterministic render")
	}
	notes[0].performance = &Performance{Articulation: "staccato"}
	z, _, _ := encodeChannel(notes[:1])
	if len(z) >= sampleCount(0.25+0.1) {
		t.Fatal("staccato did not shorten sound")
	}
}

func TestPerformanceSwellAndVibrato(t *testing.T) {
	s := Score{Instrument: "violin", Length: 2, Volume: 12000, Performance: map[string]Performance{"C1": {Articulation: "legato", Swell: 1, Vibrato: 12}}}
	n, _ := makeChannelNote("a4", Section{}, s, "C1")
	var v phraseVoice
	ints := v.render(n, 2, false)
	x := make([]float64, len(ints))
	for i, a := range ints {
		x[i] = float64(a)
	}
	if rms(x, ms(900), ms(1100)) <= rms(x, ms(1650), ms(1850))*1.1 {
		t.Fatal("swell does not develop within the note")
	}
	if swing := pitchSwing(x, ms(700), ms(1800), 440); swing < 5 || swing > 18 {
		t.Fatalf("unexpected vibrato depth %.1f", swing)
	}
}

func TestPerformancePlainSampleBudgetIncludesRelease(t *testing.T) {
	s := Score{Instrument: "violin", Length: 0.2, Volume: 1000, Plain: true, Performance: map[string]Performance{"C1": {Articulation: "legato"}}}
	n, err := makeChannelNote("a4", Section{}, s, "C1")
	if err != nil {
		t.Fatal(err)
	}
	x, _, err := encodeChannel([]note{n})
	if err != nil {
		t.Fatal(err)
	}
	if channelSamples([]note{n}) < len(x) {
		t.Fatal("sample budget misses phrase release")
	}
}

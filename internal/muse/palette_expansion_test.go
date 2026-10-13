package muse

import (
	"math"
	"path/filepath"
	"strings"
	"testing"
)

func TestExpandedPaletteDistinct(t *testing.T) {
	for _, pair := range [][2]string{{"trombone", "trumpet"}, {"french-horn", "trombone"}, {"oboe", "clarinet"}, {"bassoon", "oboe"}, {"nylon-guitar", "guitar"}, {"string-ensemble", "violin"}, {"glockenspiel", "vibraphone"}, {"xylophone", "marimba"}, {"tubular-bells", "glockenspiel"}} {
		a, b := instruments[pair[0]](220, .5), instruments[pair[1]](220, .5)
		delta := 0.
		for i := range a {
			delta += math.Abs(a[i] - b[i])
		}
		if delta/float64(len(a)) < .01 {
			t.Fatalf("%v indistinguishable", pair)
		}
	}
}
func TestExpandedPercussion(t *testing.T) {
	for _, name := range []string{"shaker", "tambourine", "conga", "bongo", "claves", "triangle", "brushtap", "brushsweep"} {
		for _, dur := range []float64{.02, .4, 1.3} {
			n, err := makeDrumNote(name, Section{}, Score{Length: dur, Volume: 3000})
			if err != nil {
				t.Fatal(err)
			}
			x := n.kitData(3000)
			if len(x) != drumLength(n) {
				t.Fatalf("%s length mismatch", name)
			}
			energy := 0.
			for _, a := range x {
				energy += float64(a * a)
			}
			if energy == 0 || x[0] != 0 || x[len(x)-1] != 0 {
				t.Fatalf("%s silent or abrupt", name)
			}
			if name == "brushsweep" && len(x) != sampleCount(dur) {
				t.Fatal("sweep ignores written duration")
			}
		}
	}
	n, err := makeDrumNote("kick-brushtap-shaker", Section{}, Score{Length: .3})
	if err != nil || len(n.kit) != 3 {
		t.Fatalf("compound hit: %v", err)
	}
	n = note{length: 1, hold: .7, placed: true, kit: []string{"brushsweep"}}
	if len(n.kitData(1000)) != sampleCount(.7) {
		t.Fatal("sweep ignores performed time")
	}
}
func TestNewWindAndNylonPerformance(t *testing.T) {
	for _, name := range []string{"trombone", "french-horn", "oboe", "bassoon", "nylon-guitar"} {
		articulation := "legato"
		if name == "nylon-guitar" {
			articulation = "let-ring"
		}
		s := Score{Key: "C", Length: .1, Volume: 1000, Instrument: name, Performance: map[string]Performance{"C1": {Articulation: articulation}}, Sections: []Section{{C1: []string{"c4", "d4", "e4", "z"}}}}
		tune, err := buildTune(&s)
		if err != nil {
			t.Fatal(err)
		}
		x, err := tune.encode()
		if err != nil {
			t.Fatal(err)
		}
		peak := 0
		for _, a := range x {
			peak = max(peak, a)
		}
		if peak < 10 {
			t.Fatalf("%s silent", name)
		}
	}
}
func TestToneValidationAndRouting(t *testing.T) {
	for _, bad := range []string{
		"tone: {C1: {drawbars: [8, 2]}}", "tone: {C1: {tremolo: {rate: 0, depth: 1}}}",
		"tone: {C1: {rotary: {rate: 4, depth: 2}}}", "tone: {drums: {}}",
		"tone: {C1: {synth: {wave: noise}}}", "tone: {C1: {synth: {cutoff: .nan}}}",
		"tone: {C1: {synth: {resonance: 100}}}", "tone: {C1: {drawbars: [0,0,0,0,0,0,0,0,0]}}",
	} {
		var s Score
		_, err := Parse(&s, []byte("key: C\nlength: 0.1\nvolume: 1000\ninstrument: synth-lead\n"+bad+"\nsections: [{C1: [c4]}]"), filepath.Join(t.TempDir(), "bad"), 0)
		if err == nil {
			t.Fatalf("accepted %s", bad)
		}
	}
	var s Score
	yaml := `key: C
length: 0.1
volume: 1000
instrument: organ
tone: {C1: {drawbars: [0,0,8,0,0,0,0,0,0]}}
sections:
 - C1: [c4]
 - tone: {C1: {}}
   C1: [c4]
`
	if _, err := Parse(&s, []byte(yaml), filepath.Join(t.TempDir(), "routing"), 0); err != nil {
		t.Fatal(err)
	}
	tune, err := buildTune(&s)
	if err != nil {
		t.Fatal(err)
	}
	if tune.ch1[0].tone.Drawbars == nil || tune.ch1[1].tone.Drawbars != nil {
		t.Fatal("section replacement broken")
	}
	for _, name := range []string{"piano", "voice"} {
		s.Instrument = name
		if err := Check(&s); err == nil {
			t.Fatalf("accepted organ controls on %s", name)
		}
	}
}
func TestToneAudioControls(t *testing.T) {
	ptr := func(v float64) *float64 { return &v }
	n := note{instrumentName: "organ", velocityIns: drawbarOrgan, release: .1, tone: &ToneControls{Drawbars: []float64{0, 0, 8, 0, 0, 0, 0, 0, 0}}}
	x := controlledSamples(n, 220, 1)
	if power(x, 1000, 20000, 220) < 100*power(x, 1000, 20000, 440) {
		t.Fatal("drawbars did not select fundamental")
	}
	base := append([]float64(nil), x...)
	n.tone.Rotary = &Modulation{Rate: 5, Depth: .8}
	x = controlledSamples(n, 220, 1)
	delta := 0.
	for i := range x {
		delta += math.Abs(x[i] - base[i])
	}
	if delta < 100 {
		t.Fatal("rotary inaudible")
	}
	n = note{instrumentName: "electric-piano", velocityIns: electricPiano, release: .1, tone: &ToneControls{Tremolo: &Modulation{Rate: 5, Depth: 1}}}
	x = controlledSamples(n, 220, 1)
	base = electricPiano(220, 1.1, 0)
	dampAfterHold(base, 1)
	if rms(x, 4000, 4800) > .2*rms(base, 4000, 4800) {
		t.Fatal("tremolo lacks expected trough")
	}
	for _, wave := range []string{"sine", "triangle", "square", "saw"} {
		for _, q := range []float64{.5, 8} {
			p := SynthPatch{Wave: wave, Resonance: ptr(q), Cutoff: ptr(17000), FilterEnvelope: ptr(4), AttackMS: ptr(40)}
			x = patchNote(440, .2, .3, .8, "synth-lead", p)
			if len(x) != sampleCount(.5) {
				t.Fatal("patch length")
			}
			for _, a := range x {
				if !finiteRange(a, -1.000001, 1.000001) {
					t.Fatal("unstable filter")
				}
			}
			if x[0] != 0 || x[len(x)-1] != 0 {
				t.Fatal("patch endpoints")
			}
		}
	}
	low := patchNote(220, .5, .1, .7, "synth-lead", SynthPatch{Wave: "saw", Cutoff: ptr(300)})
	high := patchNote(220, .5, .1, .7, "synth-lead", SynthPatch{Wave: "saw", Cutoff: ptr(8000)})
	bright := func(x []float64) float64 { return power(x, 2000, 16000, 1760) / power(x, 2000, 16000, 220) }
	if bright(high) < 10*bright(low) {
		t.Fatal("filter cutoff ineffective")
	}
}
func TestToneReleaseBudget(t *testing.T) {
	s := Score{Key: "C", Length: .1, Volume: 1000, Instrument: "synth-pad", Tone: map[string]ToneControls{"C1": {Synth: &SynthPatch{ReleaseMS: func() *float64 { v := 3000.; return &v }()}}}, Sections: []Section{{C1: []string{"c4"}}}}
	tune, err := buildTune(&s)
	if err != nil {
		t.Fatal(err)
	}
	if channelSamples(tune.ch1) < sampleCount(3.1) {
		t.Fatal("release absent from sample budget")
	}
	s.Performance = map[string]Performance{"C1": {Articulation: "legato"}}
	s.Instrument = "synth-lead"
	if err = Check(&s); err == nil || !strings.Contains(err.Error(), "patches") {
		t.Fatalf("silently ignored patch: %v", err)
	}
}

func TestSynthPatchMusicalControls(t *testing.T) {
	ptr := func(v float64) *float64 { return &v }
	render := func(p SynthPatch) []float64 { return patchNote(220, 1, .3, .7, "synth-lead", p) }
	ratio := func(x []float64, h float64, from, to float64) float64 {
		return power(x, sampleCount(from), sampleCount(to), 220*h) / power(x, sampleCount(from), sampleCount(to), 220)
	}
	common := SynthPatch{Detune: ptr(0), Cutoff: ptr(12000)}
	saw := render(common)
	common.Wave = "square"
	square := render(common)
	common.Wave = "sine"
	sine := render(common)
	if ratio(saw, 2, .2, .5) < 100*ratio(square, 2, .2, .5) || ratio(square, 3, .2, .5) < 100*ratio(sine, 3, .2, .5) {
		t.Fatal("waveforms do not have expected harmonic structure")
	}
	common.Wave = "saw"
	common.Cutoff = ptr(880)
	common.Resonance = ptr(.5)
	flat := render(common)
	common.Resonance = ptr(6)
	res := render(common)
	if ratio(res, 4, .2, .5) < 4*ratio(flat, 4, .2, .5) {
		t.Fatal("resonance does not emphasise cutoff")
	}
	common.Resonance = ptr(.707)
	common.Cutoff = ptr(400)
	common.FilterEnvelope = ptr(4)
	common.FilterDecayMS = ptr(120)
	sweep := render(common)
	if ratio(sweep, 8, .03, .09) < 5*ratio(sweep, 8, .65, .8) {
		t.Fatal("positive filter envelope does not close")
	}
	common.FilterDecayMS = ptr(1500)
	slow := render(common)
	if ratio(slow, 8, .65, .8) < 3*ratio(sweep, 8, .65, .8) {
		t.Fatal("filter decay time ineffective")
	}
	common = SynthPatch{Wave: "sine", Detune: ptr(0), Cutoff: ptr(8000), AttackMS: ptr(500)}
	slowAttack := render(common)
	common.AttackMS = ptr(0)
	fastAttack := render(common)
	if rms(slowAttack, 100, 2000) > .2*rms(fastAttack, 100, 2000) {
		t.Fatal("attack control ineffective")
	}
	common.Detune = ptr(20)
	detuned := render(common)
	delta := 0.
	for i := range detuned {
		delta += math.Abs(detuned[i] - fastAttack[i])
	}
	if delta < 100 {
		t.Fatal("detune ineffective")
	}
}

func TestGuitarFamilyAlternatingStrums(t *testing.T) {
	for _, name := range []string{"guitar", "nylon-guitar", "electric-guitar"} {
		n := stringNote(t, name, "a3-a4", Performance{Strum: "alternate", StrumMS: 100})
		rest := note{length: .5}
		got, _, err := encodeChannel([]note{n, rest, n})
		if err != nil {
			t.Fatal(err)
		}
		first := renderStringPerformance(n, 0)
		second := renderStringPerformance(n, 1)
		at := sampleCount(n.length + rest.length)
		expected := make([]int, len(got))
		for i, v := range first {
			expected[i] += v
		}
		for i, v := range second {
			expected[at+i] += v
		}
		for i, v := range got {
			if v != expected[i] {
				t.Fatalf("%s does not alternate across a rest at %d", name, i)
			}
		}
	}
}

package muse

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestScatWordOnsetsDoNotStretch(t *testing.T) {
	for name, word := range scatWords {
		// By 200 ms all supported onsets must have given way to the vowel,
		// independent of whether the note lasts half a second or four seconds.
		for _, duration := range []float64{.5, 4} {
			f := scatWordFrame(word, .2, duration)
			if f.voiced != 1 || f.frication != 0 || f.nasal != 0 || f.burst != 0 {
				t.Errorf("%s/%g has stretched onset: %+v", name, duration, f)
			}
		}
	}
	f := scatWordFrame(scatWords["stay"], .035, 2)
	if f.voiced != 0 || f.frication <= 0 {
		t.Fatal("stay must begin with unvoiced s")
	}
	f = scatWordFrame(scatWords["stay"], .13, 2)
	if f.voiced != 0 || f.frication != 0 {
		t.Fatal("missing t closure")
	}
}

func TestScatWordDiphthongWaitsForEnd(t *testing.T) {
	w := scatWords["my"]
	middle := scatWordFrame(w, 2, 4)
	end := scatWordFrame(w, 4, 4)
	if middle.formants != w.vowel || end.formants != w.end {
		t.Fatalf("vowel was not sustained: %+v %+v", middle, end)
	}
}

func TestScatWordExtensionContinuous(t *testing.T) {
	for _, clarity := range []float64{0, 1} {
		e := scatEvent{VocalNote: VocalNote{0, 1, 55, "stay", .4}, settings: scatPreset("relaxed"), emphasis: .5, phraseEnd: 1}
		e.settings.clarity = clarity
		a, err := renderScatPhrase(context.Background(), []scatEvent{e}, 1)
		if err != nil {
			t.Fatal(err)
		}
		e.Duration = .5
		extension := e
		extension.Start = .5
		extension.Lyric = "+"
		b, err := renderScatPhrase(context.Background(), []scatEvent{e, extension}, 1)
		if err != nil {
			t.Fatal(err)
		}
		for i := range a {
			if math.Abs(a[i]-b[i]) > 1e-12 {
				t.Fatalf("extension restarted word at sample %d", i)
			}
		}
	}
}

func TestScatWordScores(t *testing.T) {
	for _, name := range []string{"vocabulary", "phrase", "check_one", "check_two", "check_three", "first_words", "clarity/clear"} {
		data, err := os.ReadFile("../../examples/singing/procedural/words/" + name + ".yaml")
		if err != nil {
			t.Fatal(err)
		}
		if _, err = Parse(&Score{}, data, filepath.Join(t.TempDir(), filepath.Base(name)), 0); err != nil {
			t.Fatal(err)
		}
	}
}

func TestScatClarityValidationAndInheritance(t *testing.T) {
	for _, v := range []float64{-1, 1.01, math.NaN(), math.Inf(1)} {
		if validateScatControls(ScatControls{Clarity: &v}) == nil {
			t.Errorf("accepted clarity %g", v)
		}
	}
	one, zero := 1.0, 0.0
	if got := resolveScat(ScatControls{Clarity: &one}, ScatControls{}).clarity; got != 1 {
		t.Fatal("lost global clarity")
	}
	if got := resolveScat(ScatControls{Clarity: &one}, ScatControls{Clarity: &zero}).clarity; got != 0 {
		t.Fatal("explicit zero did not override")
	}
}

func TestScatStayHasCompactOffglide(t *testing.T) {
	for _, duration := range []float64{.7, 2, 4} {
		w := scatWords["stay"]
		if f := scatWordFrame(w, duration-.1, duration); f.formants != w.vowel {
			t.Fatal("stay starts its offglide too early")
		}
		if f := scatWordFrame(w, duration, duration); f.formants != w.end {
			t.Fatal("stay does not complete its offglide")
		}
	}
}

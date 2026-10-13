package muse

import (
	"context"
	"gopkg.in/yaml.v3"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestScatPhonemeValidation(t *testing.T) {
	for _, ph := range [][]string{{}, {"K"}, {"NOPE", "AH"}, {"AH", "IY"}, {"S", "T", "R", "M", "N", "AH"}} {
		if _, err := parseScatPhones(ph); err == nil {
			t.Fatalf("accepted %v", ph)
		}
	}
	for _, ph := range [][]string{{"B", "R", "AY", "T"}, {"F", "IY", "L"}, {"V", "AE", "N"}, {"D", "ER"}, {"K", "AE", "R"}, {"N", "IH", "NG"}} {
		if _, err := parseScatPhones(ph); err != nil {
			t.Fatal(err)
		}
	}
	var s Score
	if err := yaml.Unmarshal([]byte(scatScoreFixture), &s); err != nil {
		t.Fatal(err)
	}
	s.Sections[0].Scat.Phonemes = [][]string{{"HH", "EY"}}
	if validateScat(&s) == nil {
		t.Fatal("accepted misaligned phonemes")
	}
	s.Sections[0].Scat.Phonemes = [][]string{{"HH", "EY"}, {"IY"}, nil, nil}
	if validateScat(&s) == nil {
		t.Fatal("extension with repeated phonemes accepted")
	}
}
func TestScatPhonemeVowelTakesLongNote(t *testing.T) {
	p, err := parseScatPhones([]string{"B", "R", "AY", "T"})
	if err != nil {
		t.Fatal(err)
	}
	for _, duration := range []float64{.8, 4} {
		f := p.frame(duration/2, duration)
		if f.voiced != 1 || f.burst != 0 || f.frication != 0 {
			t.Fatal("consonant stretched into vowel")
		}
		f = p.frame(duration-.01, duration)
		if f.burst <= 0 {
			t.Fatal("final t release missing")
		}
	}
}
func TestScatPhonemeTieDoesNotRepeatCoda(t *testing.T) {
	p, _ := parseScatPhones([]string{"L", "AY", "T"})
	e := scatEvent{VocalNote: VocalNote{0, 1, 55, "light", .4}, pronunciation: p, settings: scatPreset("playful"), emphasis: .5, phraseEnd: 1}
	e.settings.clarity = 1
	a, err := renderScatPhrase(context.Background(), []scatEvent{e}, 1)
	if err != nil {
		t.Fatal(err)
	}
	e.Duration = .5
	end := e
	end.Start = .5
	end.Lyric = "+"
	end.pronunciation = nil
	b, err := renderScatPhrase(context.Background(), []scatEvent{e, end}, 1)
	if err != nil {
		t.Fatal(err)
	}
	for i := range a {
		if math.Abs(a[i]-b[i]) > 1e-12 {
			t.Fatalf("phonemes restarted at %d", i)
		}
	}
}
func TestScatPhonemeDemo(t *testing.T) {
	b, err := os.ReadFile("../../examples/singing/procedural/phonemes/come_alive.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var s Score
	if _, err = Parse(&s, b, filepath.Join(t.TempDir(), "demo"), 0); err != nil {
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
	if len(events) != 27 || math.Abs(duration-18) > 1e-6 {
		t.Fatalf("events %d duration %g", len(events), duration)
	}
	for _, e := range events {
		if e.pronunciation == nil {
			t.Fatal("lost pronunciation")
		}
	}
}

// Keep the familiar onsets anchored to the intelligible word prototype.
func TestScatPhonemeFamiliarOnsets(t *testing.T) {
	for _, tc := range []struct {
		phone, onset string
		span         float64
	}{
		{"HH", "h", .15}, {"M", "m", .085}, {"N", "n", .085}, {"Y", "y", .16}, {"W", "w", .16},
	} {
		vowel := scatVowels["IY"].start
		for i := 0; i < 100; i++ {
			u := float64(i) / 100
			got := scatConsonantFrame(tc.phone, u, vowel, false)
			want := scatWordFrame(scatWord{onset: tc.onset, vowel: vowel, end: vowel}, u*tc.span, 1)
			if math.Abs(got.voiced-want.voiced) > 1e-12 || math.Abs(got.aspiration-want.aspiration) > 1e-12 || math.Abs(got.nasal-want.nasal) > 1e-12 {
				t.Fatalf("%s onset differs at %g", tc.phone, u)
			}
			for j := range got.formants {
				if math.Abs(got.formants[j]-want.formants[j]) > 1e-9 {
					t.Fatalf("%s formant differs at %g", tc.phone, u)
				}
			}
		}
	}
}

func TestScatPhonemeCodaEntersFromVowel(t *testing.T) {
	vowel := scatVowels["AY"].end
	for _, p := range []string{"P", "B", "T", "D", "K", "G", "S", "F", "V", "M", "N", "NG", "L", "R"} {
		f := scatConsonantFrame(p, 0, vowel, true)
		if f.voiced != 1 || f.frication != 0 || f.burst != 0 {
			t.Fatalf("%s cuts vowel abruptly: %+v", p, f)
		}
		if f.formants != vowel {
			t.Fatalf("%s jumps formants", p)
		}
	}
	// A final D closes and releases; Z sustains turbulent, voiced airflow.
	d := scatConsonantFrame("D", .5, vowel, true)
	z := scatConsonantFrame("Z", .5, vowel, true)
	if d.voiced >= .1 || d.frication != 0 || z.voiced < .4 || z.frication <= 0 {
		t.Fatal("stop/fricative contrast lost")
	}
	if scatConsonantFrame("D", .73, vowel, true).burst <= 0 {
		t.Fatal("final D release absent")
	}
}

func TestFinalZRetainsFrictionWithoutStretchingOnset(t *testing.T) {
	p, _ := parseScatPhones([]string{"R", "AY", "Z"})
	for _, duration := range []float64{.7, 3} {
		f := p.frame(duration-.06, duration)
		if f.frication < .7 || f.voiced < .4 || f.burst != 0 {
			t.Fatalf("final Z lost sustained voiced friction: %+v", f)
		}
	}
	if phoneSpan([]string{"Z"}) != .085 {
		t.Fatal("changed onset timing")
	}
}

func TestFinalZOverlapsVowel(t *testing.T) {
	p, _ := parseScatPhones([]string{"R", "AY", "Z"})
	duration := .7
	boundary := duration - math.Min(codaSpan(p.coda), duration*.25)
	a, b := p.frame(boundary-1e-7, duration), p.frame(boundary, duration)
	if math.Abs(a.frication-b.frication) > 1e-5 || math.Abs(a.voiced-b.voiced) > 1e-5 {
		t.Fatal("discontinuous vowel to Z overlap")
	}
	early := p.frame(boundary-.04, duration)
	if early.frication <= .1 || early.voiced != 1 {
		t.Fatal("Z does not overlap voiced vowel")
	}
}

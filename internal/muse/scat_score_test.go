package muse

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const scatScoreFixture = `name: Phrase voice
key: D
length: 0.25
instrument: piano
volume: 12000
humanize: 0
reverb: 0
scat: {channel: C1, delivery: playful}
sections:
 - C1: [f3, g3, a3, z]
   C2: [4:d3]
   scat:
     syllables: [doo, +, wah, ""]
     emphasis: [0.2, 0.4, 0.9, 0]
 - C1: [a3, b3, a3, z]
   C2: [4:d3]
   ritardando: 0.4
   scat: {delivery: assertive, syllables: [bah, doo, +, ""]}
`

func loadScatTest(t *testing.T, doc string) (*Score, tune) {
	t.Helper()
	s := &Score{}
	if err := yaml.Unmarshal([]byte(doc), s); err != nil {
		t.Fatal(err)
	}
	tune, err := buildTune(s)
	if err != nil {
		t.Fatal(err)
	}
	return s, tune
}
func TestScatPhraseScoreTimeline(t *testing.T) {
	s, tune := loadScatTest(t, scatScoreFixture)
	events, duration, err := scatEvents(s, &tune)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 6 || events[0].MIDI != 54 || events[3].Start != 1 || math.Abs(duration-2.2) > 1e-9 {
		t.Fatalf("incorrect timeline %+v %v", events, duration)
	}
	if events[0].settings.effort >= events[3].settings.effort || events[1].Lyric != "+" {
		t.Fatal("lost delivery or extension")
	}
	before := s.Sections[0].Scat.Syllables[0]
	proxy := scatProxy(s)
	proxy.Sections[0].Lyrics[0] = "changed"
	if s.Sections[0].Scat.Syllables[0] != before {
		t.Fatal("proxy mutated original")
	}
	out := filepath.Join(t.TempDir(), "voice")
	if _, err = Parse(&Score{}, []byte(scatScoreFixture), out, 0); err != nil {
		t.Fatal(err)
	}
	if st, err := os.Stat(out + ".wav"); err != nil || st.Size() < 44 {
		t.Fatal("no rendered WAV", err)
	}
}
func TestScatValidation(t *testing.T) {
	for _, doc := range []string{
		strings.Replace(scatScoreFixture, "channel: C1", "channel: drums", 1),
		strings.Replace(scatScoreFixture, "delivery: playful", "delivery: angry", 1),
		strings.Replace(scatScoreFixture, "delivery: playful", "effort: .nan", 1),
		strings.Replace(scatScoreFixture, "delivery: playful", "breathiness: 2", 1),
		strings.Replace(scatScoreFixture, "delivery: playful", "breaths: -1", 1),
		strings.Replace(scatScoreFixture, "delivery: playful", "articulation: broken", 1),
		strings.Replace(scatScoreFixture, "delivery: playful", "phrasing: broken", 1),
		strings.Replace(scatScoreFixture, "[doo, +, wah, \"\"]", "[doo, wah]", 1),
		strings.Replace(scatScoreFixture, "[doo, +, wah, \"\"]", "[+, +, wah, \"\"]", 1),
		strings.Replace(scatScoreFixture, "[doo, +, wah, \"\"]", "[doo, +, unknown, \"\"]", 1),
		strings.Replace(scatScoreFixture, "[doo, +, wah, \"\"]", "[doo, +, wah, bah]", 1),
		strings.Replace(scatScoreFixture, "[0.2, 0.4, 0.9, 0]", "[1]", 1),
		strings.Replace(scatScoreFixture, "[0.2, 0.4, 0.9, 0]", "[1, .inf, 0, 0]", 1),
		strings.Replace(scatScoreFixture, "[f3, g3, a3, z]", "[f3-a3, g3, a3, z]", 1),
		strings.Replace(scatScoreFixture, "C2: [4:d3]", "C2: [3:d3]", 1),
		strings.Replace(scatScoreFixture, "scat: {channel: C1, delivery: playful}", "", 1),
		strings.Replace(scatScoreFixture, "scat: {channel: C1, delivery: playful}", "scat: {channel: C1}\nsinging: {channel: C1, language: en}", 1),
	} {
		var s Score
		if err := yaml.Unmarshal([]byte(doc), &s); err != nil {
			t.Fatal(err)
		}
		if err := Check(&s); err == nil {
			t.Fatalf("invalid score accepted: %s", doc)
		}
	}
}
func TestScatExtensionPreservesContinuousState(t *testing.T) {
	settings := scatPreset("playful")
	a := []scatEvent{{VocalNote: VocalNote{0, 1, 55, "wah", .4}, settings: settings, emphasis: .5, phraseStart: 0, phraseEnd: 1}}
	b := []scatEvent{a[0], a[0]}
	b[0].Duration = .5
	b[1].Start = .5
	b[1].Duration = .5
	b[1].Lyric = "+"
	x, err := renderScatPhrase(context.Background(), a, 1)
	if err != nil {
		t.Fatal(err)
	}
	y, err := renderScatPhrase(context.Background(), b, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(x, y) {
		t.Fatal("same-pitch extension restarted or changed phrase")
	}
}
func TestScatPhraseControlsAndMute(t *testing.T) {
	s, tune := loadScatTest(t, scatScoreFixture)
	events, duration, _ := scatEvents(s, &tune)
	x, err := renderScatPhrase(context.Background(), events, duration)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range x {
		if !finiteRange(v, -1, 1) {
			t.Fatal("bad signal")
		}
	}
	for i := range events {
		events[i].Gain = 0
	}
	muted, _ := renderScatPhrase(context.Background(), events, duration)
	for _, v := range muted {
		if v != 0 {
			t.Fatal("mute did not mute")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := renderScatPhrase(ctx, events, duration); err == nil {
		t.Fatal("ignored cancellation")
	}
	zero := 0.0
	settings := resolveScat(ScatControls{Delivery: "assertive", Effort: &zero, Breathiness: &zero, Breaths: &zero}, ScatControls{})
	if settings.effort != 0 || settings.air != 0 || settings.breaths != 0 {
		t.Fatal("zero controls replaced by defaults")
	}
}
func TestScatPhraseDeliveriesNotGainOnly(t *testing.T) {
	var previous []float64
	for _, name := range []string{"relaxed", "playful", "assertive"} {
		events := []scatEvent{{VocalNote: VocalNote{0, .8, 55, "wah", .4}, settings: scatPreset(name), emphasis: .8, phraseStart: 0, phraseEnd: .8}}
		x, err := renderScatPhrase(context.Background(), events, 1)
		if err != nil {
			t.Fatal(err)
		}
		if previous != nil {
			aa, bb, ab := 0.0, 0.0, 0.0
			for i := range x {
				aa += x[i] * x[i]
				bb += previous[i] * previous[i]
				ab += x[i] * previous[i]
			}
			if math.Abs(ab/math.Sqrt(aa*bb)) > .98 {
				t.Fatal("preset only changed gain")
			}
		}
		previous = x
	}
}

func TestScatDetachedAndBreathing(t *testing.T) {
	settings := scatPreset("relaxed")
	settings.breaths = 0
	note := scatEvent{VocalNote: VocalNote{0, .8, 55, "wah", .4}, settings: settings, emphasis: .5, phraseStart: 0, phraseEnd: .8}
	connected, _ := renderScatPhrase(context.Background(), []scatEvent{note}, 1.2)
	note.settings.detached = 1
	detached, _ := renderScatPhrase(context.Background(), []scatEvent{note}, 1.2)
	if rms(detached, ms(700), ms(740)) >= rms(connected, ms(700), ms(740))*.2 {
		t.Fatal("detached note did not release early")
	}
	note.settings.detached = 0
	note.settings.breaths = 1
	breathing, _ := renderScatPhrase(context.Background(), []scatEvent{note}, 1.2)
	if rms(breathing, ms(850), ms(960)) <= rms(connected, ms(850), ms(960))*2 {
		t.Fatal("breath not audible in rest")
	}
}
func TestScatRejectsBeforeWritingAndHonoursCap(t *testing.T) {
	path := filepath.Join(t.TempDir(), "voice")
	if err := os.WriteFile(path+".wav", []byte("keep existing"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Parse(&Score{}, []byte(scatScoreFixture), path, 10); err == nil {
		t.Fatal("ignored sample cap")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ParseWithSinger(ctx, &Score{}, []byte(scatScoreFixture), path, 0, nil); err == nil {
		t.Fatal("cancelled render succeeded")
	}
	b, _ := os.ReadFile(path + ".wav")
	if string(b) != "keep existing" {
		t.Fatal("failed render overwrote file")
	}
}

func TestScatEffortChangesHarmonicBalance(t *testing.T) {
	settings := scatPreset("relaxed")
	settings.air = 0
	settings.breaths = 0
	settings.scoop = 0
	settings.vibrato = 0
	event := scatEvent{VocalNote: VocalNote{0, 1, 55, "doo", .4}, settings: settings, emphasis: .5, phraseStart: 0, phraseEnd: 1}
	event.settings.effort = 0
	soft, _ := renderScatPhrase(context.Background(), []scatEvent{event}, 1)
	event.settings.effort = 1
	strong, _ := renderScatPhrase(context.Background(), []scatEvent{event}, 1)
	f := 440 * math.Exp2((55.0-69)/12)
	a, b := brightness(soft, ms(300), ms(700), f), brightness(strong, ms(300), ms(700), f)
	if b <= a*1.2 {
		t.Fatalf("effort did not brighten source: soft %g strong %g", a, b)
	}
}
func TestScatExampleScores(t *testing.T) {
	files, err := filepath.Glob("../../examples/singing/procedural/*.yaml")
	if err != nil || len(files) != 4 {
		t.Fatalf("missing examples: %v", err)
	}
	for _, path := range files {
		t.Run(filepath.Base(path), func(t *testing.T) {
			doc, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := Parse(&Score{}, doc, filepath.Join(t.TempDir(), "out"), 0); err != nil {
				t.Fatal(err)
			}
		})
	}
}

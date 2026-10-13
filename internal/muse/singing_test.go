package muse

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type recordingSinger struct {
	request VocalRequest
	called  bool
	bad     bool
}

func (f *recordingSinger) Render(_ context.Context, r VocalRequest) ([]float64, error) {
	f.called = true
	f.request = r
	x := make([]float64, sampleCount(r.Duration))
	for i := range x {
		x[i] = 0.25
	}
	if f.bad {
		x[0] = math.NaN()
	}
	return x, nil
}

const singingTestScore = `name: Test vocal
key: D
length: 0.25
instrument: piano
volume: 10000
humanize: 0
reverb: 0
singing: {channel: C1, language: en}
sections:
 - C1: [f4, g4, a4, z]
   C2: [4:d3]
   lyrics: [stay, with, me, ""]
 - ritardando: 0.4
   C1: [a4, b4, a4, z]
   C2: [4:d3]
   lyrics: [near, +, me, ""]
`

func TestSingingUsesSharedTempoAndKey(t *testing.T) {
	f := &recordingSinger{}
	var s Score
	_, err := ParseWithSinger(context.Background(), &s, []byte(singingTestScore), filepath.Join(t.TempDir(), "song"), 0, f)
	if err != nil {
		t.Fatal(err)
	}
	r := f.request
	if len(r.Phrases) != 2 {
		t.Fatalf("phrases: %+v", r)
	}
	if r.Phrases[0].Notes[0].MIDI != 66 {
		t.Fatalf("F in D major should be MIDI 66: %+v", r.Phrases[0])
	}
	if math.Abs(r.Phrases[1].Notes[0].Start-1) > 1e-9 || math.Abs(r.Duration-2.2) > 1e-9 {
		t.Fatalf("tempo map: %+v", r)
	}
	if r.Phrases[1].Notes[1].Lyric != "+" {
		t.Fatal("lost melisma")
	}
}
func TestSingingRequiresExplicitBackend(t *testing.T) {
	_, err := Parse(&Score{}, []byte(singingTestScore), filepath.Join(t.TempDir(), "song"), 0)
	if err == nil || !strings.Contains(err.Error(), "-singer") {
		t.Fatalf("got %v", err)
	}
}
func TestSingingRejectsBadLyricsBeforeBackend(t *testing.T) {
	for _, doc := range []string{
		strings.Replace(singingTestScore, "[stay, with, me, \"\"]", "[stay]", 1),
		strings.Replace(singingTestScore, "[stay, with, me, \"\"]", "[+, with, me, \"\"]", 1),
		strings.Replace(singingTestScore, "[f4, g4, a4, z]", "[f4-a4, g4, a4, z]", 1),
		strings.Replace(singingTestScore, "[stay, with, me, \"\"]", "[stay, with, me, rest]", 1),
	} {
		f := &recordingSinger{}
		_, err := ParseWithSinger(context.Background(), &Score{}, []byte(doc), filepath.Join(t.TempDir(), "song"), 0, f)
		if err == nil || f.called {
			t.Fatalf("bad score reached backend: %v", err)
		}
	}
}
func TestSingingZeroLevelMutes(t *testing.T) {
	f := &recordingSinger{}
	var s Score
	doc := strings.Replace(singingTestScore, "volume: 10000", "volume: 10000\nlevels: {C1: 0}", 1)
	_, err := ParseWithSinger(context.Background(), &s, []byte(doc), filepath.Join(t.TempDir(), "song"), 0, f)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range f.request.Phrases {
		for _, n := range p.Notes {
			if n.Gain != 0 {
				t.Fatalf("gain %v", n.Gain)
			}
		}
	}
}
func TestSingingInvalidOutputDoesNotOverwrite(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "song")
	os.WriteFile(out+".wav", []byte("original"), 0600)
	_, err := ParseWithSinger(context.Background(), &Score{}, []byte(singingTestScore), out, 0, &recordingSinger{bad: true})
	if err == nil {
		t.Fatal("accepted NaN")
	}
	b, _ := os.ReadFile(out + ".wav")
	if string(b) != "original" {
		t.Fatal("overwrote old audio")
	}
}

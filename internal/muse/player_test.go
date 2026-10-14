package muse

import (
	"gopkg.in/yaml.v3"
	"os"
	"testing"
)

func TestPlayerStreamMatchesOffline(t *testing.T) {
	for _, tc := range []struct{ name, extra, ins, notes string }{
		{"dry", "plain: true", "piano", "[c4, d4, 2:e4, z]"},
		{"room", "reverb: 0.2", "nylon-guitar", "[c4-e4-g4, 2:d4-f4-a4, e4, z]"},
		{"small", "reverb: 0.25\nroom: {preset: small, sends: {C1: 0.4}}\nmix: {C1: {low_cut_hz: 80, high_cut_hz: 7000, tilt_db: 2, gain_db: -2}}", "piano", "[c4, d4, 2:e4, z]"},
		{"legato", "reverb: 0\nperformance: {C1: {articulation: legato, vibrato: 5}}", "clarinet", "[c4, d4, 2:e4, z]"},
		{"strum", "reverb: 0\nperformance: {C1: {articulation: let-ring, strum: alternate}}", "nylon-guitar", "[c4-e4-g4, d4-f4-a4, 2:e4-g4-b4, z]"},
		{"rotary", "reverb: 0\ntone: {C1: {rotary: {rate: 5, depth: 0.7}}}", "organ", "[c4, d4, 2:e4, z]"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			doc := []byte("name: parity\nkey: D\nlength: 0.08\nvolume: 1000\nhumanize: 0\ninstrument: " + tc.ins + "\n" + tc.extra + "\nsections: [{C1: " + tc.notes + "}]\n")
			_, want, err := RenderPCM(doc)
			if err != nil {
				t.Fatal(err)
			}
			for _, block := range []int{127, 4096, 44100} {
				stream, err := NewPCMStream(doc)
				if err != nil {
					t.Fatal(err)
				}
				var got []int
				for {
					pcm, done, err := stream.Next(block)
					if err != nil {
						t.Fatal(err)
					}
					got = append(got, pcm...)
					if done {
						break
					}
				}
				if len(got) < len(want) {
					t.Fatalf("block %d short: %d < %d", block, len(got), len(want))
				}
				for i, v := range want {
					if got[i] != v {
						t.Fatalf("block %d sample %d: %d != %d", block, i, got[i], v)
					}
				}
				for _, v := range got[len(want):] {
					if v != 0 {
						t.Fatal("nonzero extra tail")
					}
				}
			}
		})
	}
}
func TestPlayerProgressiveAndLimits(t *testing.T) {
	doc := []byte("key: C\nlength: 1\nvolume: 1000\ninstrument: piano\nsections: [{C1: [c4,d4,e4,f4,g4,a4,b4,c5]}]")
	p, err := NewPCMStream(doc)
	if err != nil {
		t.Fatal(err)
	}
	x, done, err := p.Next(4096)
	if err != nil || done || len(x) != 8192 {
		t.Fatalf("first chunk: %d %v %v", len(x), done, err)
	}
	if p.channels[0].cursor.index >= len(p.channels[0].cursor.notes) {
		t.Fatal("rendered whole song before first chunk")
	}
	if _, _, err = p.Next(0); err == nil {
		t.Fatal("zero block accepted")
	}
	for _, bad := range []string{
		"key: C\nlength: 601\ninstrument: piano\nsections: [{C1: [c4]}]",
		"key: C\nlength: 1\ninstrument: piano\nsections: [{C1: [c4], C2: [c4,d4]}]",
		"key: C\nsections: []", "key: C\nsinging: {channel: C1}",
	} {
		if _, err := NewPCMStream([]byte(bad)); err == nil {
			t.Fatal("invalid score accepted")
		}
	}
}
func TestPlayerBufferedUsesExportRenderer(t *testing.T) {
	doc := []byte("key: C\nlength: 0.1\ninstrument: organ\nvolume: 6000\nmastering: {rms_dbfs: -20, peak_dbfs: -1}\nsections: [{C1: [c4-e4-g4,z]}]")
	info, got, err := RenderPCM(doc)
	if err != nil {
		t.Fatal(err)
	}
	var s Score
	if err = yaml.Unmarshal(doc, &s); err != nil {
		t.Fatal(err)
	}
	tu, err := buildTune(&s)
	if err != nil {
		t.Fatal(err)
	}
	want, err := tu.encode()
	if err != nil {
		t.Fatal(err)
	}
	if !info.Mastering || len(got) != len(want) {
		t.Fatal("buffered metadata")
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatal("export mismatch")
		}
	}
}
func TestPlayerExpansionScores(t *testing.T) {
	for _, name := range []string{"instrument_palette_3", "tone_controls_demo"} {
		doc, err := os.ReadFile("../../scores/" + name + ".yaml")
		if err != nil {
			t.Fatal(err)
		}
		_, want, err := RenderPCM(doc)
		if err != nil {
			t.Fatal(err)
		}
		p, err := NewPCMStream(doc)
		if err != nil {
			t.Fatal(err)
		}
		at := 0
		for {
			chunk, done, err := p.Next(4096)
			if err != nil {
				t.Fatal(err)
			}
			for _, v := range chunk {
				if at < len(want) {
					if v != want[at] {
						t.Fatalf("%s sample %d: %d != %d", name, at, v, want[at])
					}
				} else if v != 0 {
					t.Fatal("extra nonzero tail")
				}
				at++
			}
			if done {
				break
			}
		}
		if at < len(want) {
			t.Fatal("short stream")
		}
	}
}

func TestPlayerAcceptsLongSongs(t *testing.T) {
	doc := []byte("key: C\nlength: 1\nplain: true\ninstrument: piano\nsections: [{C1: [301:z,c4]}]")
	p, err := NewPCMStream(doc)
	if err != nil {
		t.Fatal(err)
	}
	if p.Info.Duration <= 300 {
		t.Fatal("long song duration")
	}
}

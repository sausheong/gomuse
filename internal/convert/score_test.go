package convert

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sausheong/gomuse/internal/muse"
	"gopkg.in/yaml.v3"
)

const page = `name: After All
key: D
length: 1.0
sections:
  # bar 1 - G
  - C1: [2:b4, 2:z]
    C2: [4:b3-d4]
    C3: [g2, g2, g2, g2]
  # bar 2
  - C1: [0.333:a4, 0.333:b4, 0.334:c5, 3:d5]
    C2: [4:a3-c4-e4]
    C3: [4:a2]`

func TestExtractYAML(t *testing.T) {
	reply := "Here it is:\n```yaml\nname: x\nsections:\n  - C1: [c4]\n```\nDone."
	if got, want := ExtractYAML(reply), "name: x\nsections:\n  - C1: [c4]"; got != want {
		t.Fatalf("ExtractYAML = %q, want %q", got, want)
	}
	if got := ExtractYAML("  name: y \n"); got != "name: y" {
		t.Fatalf("ExtractYAML without a fence = %q", got)
	}
}

func TestSplitPage(t *testing.T) {
	p, err := SplitPage(page)
	if err != nil {
		t.Fatal(err)
	}
	if p.Header != (Header{Name: "After All", Key: "D", Length: 1}) {
		t.Fatalf("header = %+v", p.Header)
	}
	if !strings.HasPrefix(p.Sections, "  # bar 1 - G\n  - C1:") {
		t.Fatalf("sections text lost its comments or indent: %q", p.Sections)
	}
	if _, err := SplitPage("name: x\n"); err == nil {
		t.Fatal("expected an error for a page without sections")
	}
	if _, err := SplitPage("name: x\nsections: [{C1: [c4]}]"); err == nil {
		t.Fatal("expected an error for a flow-style sections list")
	}
}

func TestRenderIsAValidScore(t *testing.T) {
	p, _ := SplitPage(page)
	doc := Render("After All\nconverted", p.Header, Sound{Instrument: "piano", Volume: 3000}, p.Sections, p.Sections)
	var s muse.Score
	if err := yaml.Unmarshal([]byte(doc), &s); err != nil {
		t.Fatalf("rendered score does not parse: %v\n%s", err, doc)
	}
	if len(s.Sections) != 4 || s.Instrument != "piano" || s.Volume != 3000 || s.Length != 1 {
		t.Fatalf("rendered score = %+v", s)
	}
	if !strings.HasPrefix(doc, "# After All\n# converted\n") {
		t.Fatalf("missing comment header:\n%s", doc)
	}
	if probs := Problems(doc); len(probs) != 0 {
		t.Fatalf("Problems = %v", probs)
	}

	// and muse renders it, C3 included
	out := filepath.Join(t.TempDir(), "x")
	if _, err := muse.Parse(&s, []byte(doc), out, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(out + ".wav"); err != nil {
		t.Fatal(err)
	}
}

func TestRenderEnvelopeAndQuotedName(t *testing.T) {
	doc := Render("", Header{Name: "Tune: #1", Key: "C", Length: 0.5}, Sound{Envelope: "drop", Harmonic: "stringed", Volume: 1}, "  - C1: [c4]")
	var s muse.Score
	if err := yaml.Unmarshal([]byte(doc), &s); err != nil {
		t.Fatal(err)
	}
	if s.Name != "Tune: #1" || s.Envelope != "drop" || s.Harmonic != "stringed" || s.Instrument != "" {
		t.Fatalf("got %+v", s)
	}
}

func TestProblems(t *testing.T) {
	head := "name: x\nkey: C\nlength: 1.0\ninstrument: piano\nvolume: 1000\nsections:\n"
	cases := []struct {
		name, sections, want string
	}{
		{"uneven bar", "  - C1: [c4, d4]\n    C2: [c3]\n", "different lengths (C1 = 2 beats, C2 = 1 beats)"},
		{"missing channel", "  - C1: [c4]\n    C3: [c3]\n  - C1: [c4]\n", "section 2: C3 is empty"},
		{"bad note", "  - C1: [h4]\n", "[C1]"},
		{"bad length", "  - C1: [x:c4]\n    C2: [c4]\n", "bad length"},
		{"no sections", "", "no sections"},
		{"bad yaml", "  - C1: [c4\n", "does not parse"},
	}
	for _, c := range cases {
		probs := Problems(head + c.sections)
		if !strings.Contains(strings.Join(probs, "\n"), c.want) {
			t.Errorf("%s: Problems = %q, want one containing %q", c.name, probs, c.want)
		}
	}
	if probs := Problems(head + "  - C1: [0.333:c4, 0.333:d4, 0.333:e4]\n    C2: [c3]\n"); len(probs) != 0 {
		t.Errorf("triplets within tolerance should pass, got %v", probs)
	}
}

func TestAutoVolume(t *testing.T) {
	head := "name: x\nkey: C\nlength: 1.0\ninstrument: piano\nvolume: 1\nsections:\n"
	// widest C2 chord is 3 (balanced to 2.4), widest C3 chord is 2
	// (balanced to 1.4): 30000 / 3.8
	doc := head + "  - C1: [c4, 2:z]\n    C2: [3:c4-e4-g4]\n    C3: [3:c2-c3]\n"
	if got := AutoVolume(doc); got != 7800 {
		t.Fatalf("AutoVolume = %d, want 7800", got)
	}
	if got := AutoVolume(head + "  - C1: [c4]\n"); got != 8000 {
		t.Fatalf("AutoVolume for a single line = %d, want the 8000 cap", got)
	}
}

func TestLoadEnv(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	os.WriteFile(path, []byte("# comment\n\nMUSE_T_A=\"quoted\"\nexport MUSE_T_B='single'\nMUSE_T_C=kept\n"), 0o600)
	t.Setenv("MUSE_T_C", "from env")
	os.Unsetenv("MUSE_T_A")
	os.Unsetenv("MUSE_T_B")
	t.Cleanup(func() { os.Unsetenv("MUSE_T_A"); os.Unsetenv("MUSE_T_B") })
	if err := LoadEnv(path); err != nil {
		t.Fatal(err)
	}
	for k, want := range map[string]string{"MUSE_T_A": "quoted", "MUSE_T_B": "single", "MUSE_T_C": "from env"} {
		if got := os.Getenv(k); got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
	if err := LoadEnv(filepath.Join(t.TempDir(), "missing")); err != nil {
		t.Errorf("a missing .env should not be an error, got %v", err)
	}
}

func TestNextContext(t *testing.T) {
	p, _ := SplitPage(page)
	pc := nextContext(p.Header, []string{p.Sections})
	if pc.bars != 2 {
		t.Fatalf("bars = %d, want 2", pc.bars)
	}
	if strings.Join(pc.channels, ",") != "C1,C2,C3" {
		t.Fatalf("channels = %v", pc.channels)
	}
	if !strings.HasPrefix(pc.tail, "  # bar 1 - G") {
		t.Fatalf("tail = %q", pc.tail)
	}
	req := pageRequest("p2.png", 2, 3, pc)
	for _, want := range []string{"numbering them from 3", "C1, C2, C3", "key: D"} {
		if !strings.Contains(req, want) {
			t.Errorf("page request missing %q:\n%s", want, req)
		}
	}
}

func TestRenderMelodyInstrument(t *testing.T) {
	h := Header{Name: "Song", Key: "C", Length: 0.5}
	three := "  - C1: [c4]\n    C2: [e4]\n    C3: [c3]"
	two := "  - C1: [c4]\n    C2: [e4]"
	snd := Sound{Instrument: "piano", Melody: "voice", Volume: 3000}

	doc := Render("", h, snd, three)
	if !strings.Contains(doc, "instruments: {C1: voice}\n") {
		t.Fatalf("three staves should name the melody instrument:\n%s", doc)
	}
	if probs := Problems(doc); len(probs) != 0 {
		t.Fatalf("rendered score has problems: %v", probs)
	}
	if doc := Render("", h, snd, two); strings.Contains(doc, "instruments:") {
		t.Errorf("two staves shouldn't set a melody instrument:\n%s", doc)
	}
	snd.Melody = ""
	if doc := Render("", h, snd, three); strings.Contains(doc, "instruments:") {
		t.Errorf("empty Melody shouldn't set one:\n%s", doc)
	}
}

func TestAutoVolumeWithMelody(t *testing.T) {
	h := Header{Name: "Song", Key: "C", Length: 0.5}
	sec := "  - C1: [c4]\n    C2: [e4-g4]\n    C3: [c3-g3]"
	plain := AutoVolume(Render("", h, Sound{Instrument: "piano", Volume: 1}, sec))
	voiced := AutoVolume(Render("", h, Sound{Instrument: "piano", Melody: "voice", Volume: 1}, sec))
	if plain != voiced || voiced <= 0 {
		t.Errorf("volume %d with melody vs %d without, want the same", voiced, plain)
	}
}

func TestPromptAndScoreAcceptExpressionKeys(t *testing.T) {
	for _, key := range []string{"dynamic:", "hairpin:", "ritardando:", "fermata:"} {
		if !strings.Contains(systemPrompt, key) {
			t.Errorf("system prompt doesn't mention %q", key)
		}
	}
	doc := "key: C\nlength: 0.5\ninstrument: piano\nsections:\n  - dynamic: p\n    hairpin: cresc\n    ritardando: 0.2\n    fermata: 1.5\n    C1: [c4, d4]\n"
	if p := Problems(doc); len(p) != 0 {
		t.Errorf("Problems rejected expression keys: %v", p)
	}
}

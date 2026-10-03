package muse

import (
	"math"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func tuneOf(t *testing.T, score string) (*Score, tune) {
	t.Helper()
	var s Score
	if err := yaml.Unmarshal([]byte(score), &s); err != nil {
		t.Fatal(err)
	}
	tu, err := buildTune(&s)
	if err != nil {
		t.Fatal(err)
	}
	return &s, tu
}

const pianoScore = `
key: C
length: 0.5
instrument: piano
volume: 10000
sections:
  - C1: [c4, 0.5:e4, 0.5:g4, 2:c5]
    C2: [4:c3]
    C3: [4:c2]
`

func TestRingOverOverlapsTheNextNote(t *testing.T) {
	_, tu := tuneOf(t, pianoScore)
	data, nominal, err := encodeChannel(tu.ch1)
	if err != nil {
		t.Fatal(err)
	}
	// timing stretches the last section (a final ritardando), so the
	// played length is at least the written 2 s
	if nominal < sampleCount(2.0) {
		t.Fatalf("nominal length = %d, want at least %d", nominal, sampleCount(2.0))
	}
	if tail := len(data) - nominal; tail < sampleCount(releases["piano"])/2 {
		t.Fatalf("channel rings %d samples past its end, want about %d", tail, sampleCount(releases["piano"]))
	}

	// the first note alone, played as written, ends in silence at 0.5 s;
	// with its tail it is still sounding there
	first := tu.ch1[0]
	alone, _ := first.encode()
	at := sampleCount(first.length)
	if math.Abs(float64(alone[at])) < 100 {
		t.Fatalf("first note is %d at its written end, want it still ringing", alone[at])
	}
}

func TestPlainPlaysAsWritten(t *testing.T) {
	s, tu := tuneOf(t, strings.Replace(pianoScore, "volume: 10000", "volume: 10000\nplain: true", 1))
	for _, ch := range [][]note{tu.ch1, tu.ch2, tu.ch3} {
		for _, n := range ch {
			if n.release != 0 || n.gain != 1 {
				t.Fatalf("plain note has release %v and gain %v, want 0 and 1", n.release, n.gain)
			}
		}
	}
	if reverbAmount(s) != 0 || tu.reverb != 0 {
		t.Fatal("plain score should have no reverb")
	}
	data, nominal, _ := encodeChannel(tu.ch1)
	if len(data) != nominal {
		t.Fatalf("plain channel is %d samples, want exactly %d", len(data), nominal)
	}
}

func TestEnvelopesGetNoTail(t *testing.T) {
	_, tu := tuneOf(t, "key: C\nlength: 0.5\nenvelope: drop\nharmonic: first\nvolume: 1000\nsections:\n  - C1: [c4, d4]\n")
	data, nominal, _ := encodeChannel(tu.ch1)
	if len(data) != nominal {
		t.Fatalf("envelope channel is %d samples, want %d", len(data), nominal)
	}
}

func TestAccentsAndBalance(t *testing.T) {
	_, tu := tuneOf(t, pianoScore)
	c1 := tu.ch1 // c4 (downbeat), e4 (beat 1), g4 (off-beat), c5 (beat 2)
	for i, want := range []float64{downbeat, onBeat, offBeat, onBeat} {
		if g := c1[i].gain; g > want+1e-9 || g < want*(1-jitter)-1e-9 {
			t.Errorf("C1 note %d gain = %v, want %v less up to %v%%", i, g, want, jitter*100)
		}
	}
	if g := tu.ch2[0].gain; g > channelGains[1] || g < channelGains[1]*(1-jitter) {
		t.Errorf("C2 gain = %v, want about %v", g, channelGains[1])
	}
	if g := tu.ch3[0].gain; g > channelGains[2] || g < channelGains[2]*(1-jitter) {
		t.Errorf("C3 gain = %v, want about %v", g, channelGains[2])
	}

	// a single line isn't balanced down
	_, solo := tuneOf(t, "key: C\nlength: 0.5\ninstrument: piano\nvolume: 1000\nsections:\n  - C1: [c4]\n")
	if g := solo.ch1[0].gain; g < downbeat*(1-jitter) {
		t.Errorf("solo gain = %v, want about %v", g, downbeat)
	}
}

func TestExpressionIsRepeatable(t *testing.T) {
	_, a := tuneOf(t, pianoScore)
	_, b := tuneOf(t, pianoScore)
	for i := range a.ch1 {
		if a.ch1[i].gain != b.ch1[i].gain {
			t.Fatal("the same score should get the same gains every time")
		}
	}
}

func TestReverbAddsATail(t *testing.T) {
	l := make([]int, 1000)
	r := make([]int, 1000)
	l[0], r[0] = 20000, 20000
	ol, or := reverb(l, r, 0.5)
	if len(ol) <= len(l) || len(ol) != len(or) {
		t.Fatalf("reverb output is %d/%d samples, want longer than %d", len(ol), len(or), len(l))
	}
	if ol[0] != 20000 {
		t.Fatalf("the dry sound should pass through unchanged, got %d", ol[0])
	}
	echo := 0
	for _, v := range ol[len(l):] {
		echo = max(echo, abs(v))
	}
	if echo == 0 {
		t.Fatal("no echo after the input ended")
	}
	diff := false
	for k := range ol {
		if ol[k] != or[k] {
			diff = true
			break
		}
	}
	if !diff {
		t.Fatal("left and right tails should differ, for width")
	}

	if dl, _ := reverb(l, r, 0); len(dl) != len(l) {
		t.Fatal("no reverb should leave the length alone")
	}
}

func TestReverbAmount(t *testing.T) {
	half, over := 0.5, 3.0
	cases := []struct {
		s    Score
		want float64
	}{
		{Score{}, defaultReverb},
		{Score{Reverb: &half}, 0.5},
		{Score{Reverb: &over}, 1},
		{Score{Reverb: &half, Plain: true}, 0},
	}
	for _, c := range cases {
		if got := reverbAmount(&c.s); got != c.want {
			t.Errorf("reverbAmount(%+v) = %v, want %v", c.s, got, c.want)
		}
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// the length cap must count performed time, not just written time, or a
// ritardando or fermata could render far more audio than the cap allows
func TestChannelSamplesCountsPerformedTime(t *testing.T) {
	score := "key: C\nlength: 1.0\ninstrument: piano\nvolume: 1000\nsections:\n  - C1: [4:c4]\n    fermata: 4\n"
	_, tu := tuneOf(t, score)
	if got, written := channelSamples(tu.ch1), sampleCount(4+releases["piano"]); got < written+sampleCount(3.5) {
		t.Fatalf("channelSamples = %d, want the 4-beat fermata counted on top of %d", got, written)
	}
}

func TestLevelsReplaceTheBalance(t *testing.T) {
	score := strings.Replace(pianoScore, "volume: 10000", "volume: 10000\nlevels: {C2: 0.5, C3: 0.25}", 1)
	_, tu := tuneOf(t, score)
	if g := tu.ch2[0].gain; g > 0.5 || g < 0.5*(1-jitter) {
		t.Errorf("C2 gain = %v, want about 0.5", g)
	}
	if g := tu.ch3[0].gain; g > 0.25 || g < 0.25*(1-jitter) {
		t.Errorf("C3 gain = %v, want about 0.25", g)
	}
	if g := tu.ch1[0].gain; g < downbeat*(1-jitter) {
		t.Errorf("C1 gain = %v, want the default balance of about 1", g)
	}

	// a plain score still honours the levels it asks for, and nothing else
	_, plain := tuneOf(t, strings.Replace(score, "volume: 10000", "volume: 10000\nplain: true", 1))
	if plain.ch2[0].gain != 0.5 || plain.ch1[0].gain != 1 {
		t.Errorf("plain gains C1 %v C2 %v, want 1 and exactly 0.5", plain.ch1[0].gain, plain.ch2[0].gain)
	}
}

func TestLevelsValidation(t *testing.T) {
	for _, bad := range []string{"{C4: 0.5}", "{C2: 1.5}", "{C2: -0.1}"} {
		var s Score
		if err := yaml.Unmarshal([]byte(strings.Replace(pianoScore, "volume: 10000", "volume: 10000\nlevels: "+bad, 1)), &s); err != nil {
			t.Fatal(err)
		}
		if err := Check(&s); err == nil || !strings.Contains(err.Error(), "levels") {
			t.Errorf("levels %s: Check = %v, want a levels error", bad, err)
		}
	}
}

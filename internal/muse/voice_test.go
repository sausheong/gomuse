package muse

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// pitchTrack follows the fundamental of x by demodulating it at f (which
// should divide the sample rate into whole samples, so a one period moving
// average removes every overtone) and differentiating the phase. It returns
// the pitch in cents relative to f, one value per sample (0 at the edges).
func pitchTrack(x []float64, f float64) []float64 {
	period := int(math.Round(sampleRate / f))
	w := 2 * math.Pi * f / sampleRate
	re := make([]float64, len(x)+1)
	im := make([]float64, len(x)+1)
	for k, v := range x { // running sums for the moving average
		re[k+1] = re[k] + v*math.Cos(w*float64(k))
		im[k+1] = im[k] - v*math.Sin(w*float64(k))
	}
	n := len(x) - period
	phase := make([]float64, n)
	for k := 0; k < n; k++ {
		phase[k] = math.Atan2(im[k+period]-im[k], re[k+period]-re[k])
		if k > 0 {
			for phase[k]-phase[k-1] > math.Pi {
				phase[k] -= 2 * math.Pi
			}
			for phase[k]-phase[k-1] < -math.Pi {
				phase[k] += 2 * math.Pi
			}
		}
	}
	const lag = 200
	cents := make([]float64, len(x))
	for k := lag; k < n-lag; k++ {
		hz := (phase[k+lag] - phase[k-lag]) / (2 * lag) * sampleRate / (2 * math.Pi)
		cents[k+period/2] = 1200 * math.Log2(1+hz/f)
	}
	return cents
}

func span(x []float64) float64 {
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, v := range x {
		lo, hi = math.Min(lo, v), math.Max(hi, v)
	}
	return hi - lo
}

// Long notes get a vibrato near 5.5 Hz, a few tens of cents deep, and the
// first 0.2 s is steady.
func TestVoiceVibrato(t *testing.T) {
	const f = 220.5
	x := voice(f, 2)
	c := pitchTrack(x, f)
	at := func(s float64) int { return int(s * sampleRate) }

	if early := span(c[at(0.08):at(0.19)]); early > 12 {
		t.Errorf("pitch moves %.1f cents in the first 0.2 s, want it steady", early)
	}
	late := c[at(0.8):at(1.6)]
	if depth := span(late) / 2; depth < 25 || depth > 70 {
		t.Errorf("vibrato depth %.1f cents, want about 45", depth)
	}
	// count upward zero crossings of the pitch about its mean
	mean := 0.0
	for _, v := range late {
		mean += v / float64(len(late))
	}
	cross, below := 0, false
	for _, v := range late { // with hysteresis, to ignore tracking noise
		if v < mean-15 {
			below = true
		} else if v > mean+15 && below {
			below = false
			cross++
		}
	}
	if rate := float64(cross) / (float64(len(late)) / sampleRate); rate < 4.8 || rate > 6.2 {
		t.Errorf("vibrato rate %.2f Hz, want about 5.5", rate)
	}
}

// The vowel lifts overtones near the formants well above the plain -12 dB
// per octave slope of the source.
func TestVoiceFormants(t *testing.T) {
	f := 130.8
	x := voice(f, 1)
	from, to := int(0.1*sampleRate), int(0.2*sampleRate) // before vibrato
	h := func(n float64) float64 { return power(x, from, to, n*f) }
	// 5th overtone ~654 Hz is on F1; the 3rd (392 Hz) is below it. A flat
	// source would have h5/h3 = (3/5)^4 = 0.13.
	if r := h(5) / h(3); r < 0.4 {
		t.Errorf("overtone 5 / overtone 3 = %.2f, want the first formant to lift it well above 0.13", r)
	}
	// the 8th (1046 Hz) is on F2, the 14th (1831 Hz) between F2 and F3
	if r := h(8) / h(14); r < 20 {
		t.Errorf("overtone 8 / overtone 14 = %.1f, want a clear drop above the formants", r)
	}
}

// formantGain peaks at each formant, and the first one follows a high pitch.
func TestFormantGain(t *testing.T) {
	if formantGain(vowelAh, 100, 650) <= formantGain(vowelAh, 100, 400) {
		t.Error("no peak at the first formant")
	}
	if formantGain(vowelAh, 100, 1080) <= formantGain(vowelAh, 100, 1500) {
		t.Error("no peak at the second formant")
	}
	if formantGain(vowelAh, 880, 880) <= formantGain(vowelAh, 880, 650) {
		t.Error("first formant should follow a high pitch")
	}
}

// A sustained voice is about as loud as a piano note of the same peak.
func TestVoiceLoudnessComparableToPiano(t *testing.T) {
	for _, f := range []float64{130.81, 261.63, 523.25} {
		v, p := voice(f, 1), piano(f, 1)
		ratio := rms(v, 0, len(v)) / rms(p, 0, len(p))
		if ratio < 0.6 || ratio > 3 {
			t.Errorf("at %v Hz the voice's RMS is %.2f x the piano's, want 0.6 to 3", f, ratio)
		}
	}
}

func TestVoiceIsDeterministicAndRegistered(t *testing.T) {
	a, b := voice(300, 0.7), voice(300, 0.7)
	for i := range a {
		if a[i] != b[i] {
			t.Fatal("voice is not deterministic")
		}
	}
	if _, ok := instruments["voice"]; !ok {
		t.Fatal("voice not registered")
	}
	if releases["voice"] < 0.1 || releases["voice"] > 0.2 {
		t.Errorf("voice release %v, want 0.12-0.15", releases["voice"])
	}
}

func TestPansDefaults(t *testing.T) {
	note1 := []note{{}}
	three := &tune{ch1: note1, ch2: note1, ch3: note1}
	two := &tune{ch1: note1, ch2: note1}
	one := &tune{ch1: note1}
	eq := func(got, want []float64) bool {
		if len(got) != len(want) {
			return false
		}
		for i := range got {
			if got[i] != want[i] {
				return false
			}
		}
		return true
	}
	if got := pans(&Score{}, three); !eq(got, []float64{0, 0.35, -0.35}) {
		t.Errorf("three parts: %v", got)
	}
	if got := pans(&Score{}, two); !eq(got, []float64{0.3, -0.3, 0}) {
		t.Errorf("two parts: %v", got)
	}
	if got := pans(&Score{}, one); !eq(got, []float64{0, 0, 0}) {
		t.Errorf("one part: %v", got)
	}
}

func TestPansOverrideAndPlain(t *testing.T) {
	note1 := []note{{}}
	three := &tune{ch1: note1, ch2: note1, ch3: note1}
	got := pans(&Score{Pan: map[string]float64{"C1": -0.5, "C3": 1}}, three)
	if len(got) != 3 || got[0] != -0.5 || got[1] != 0.35 || got[2] != 1 {
		t.Errorf("override: %v", got)
	}
	if got := pans(&Score{Plain: true, Pan: map[string]float64{"C1": 1}}, three); got != nil {
		t.Errorf("plain should give nil, got %v", got)
	}
}

func TestValidateParts(t *testing.T) {
	bad := []struct {
		name string
		s    Score
		want string
	}{
		{"channel", Score{Instruments: map[string]string{"C4": "voice"}}, "C4"},
		{"instrument", Score{Instruments: map[string]string{"C1": "kazoo"}}, "kazoo"},
		{"section channel", Score{Sections: []Section{{Instruments: map[string]string{"D1": "piano"}}}}, "D1"},
		{"section instrument", Score{Sections: []Section{{Instruments: map[string]string{"C2": "nope"}}}}, "nope"},
		{"pan channel", Score{Pan: map[string]float64{"L": 0}}, "unknown channel"},
		{"pan high", Score{Pan: map[string]float64{"C1": 1.5}}, "-1"},
		{"pan low", Score{Pan: map[string]float64{"C2": -2}}, "-1"},
		{"pan nan", Score{Pan: map[string]float64{"C2": math.NaN()}}, "-1"},
	}
	for _, c := range bad {
		err := validateParts(&c.s)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: got %v, want an error mentioning %q", c.name, err, c.want)
		}
	}
	ok := Score{
		Instruments: map[string]string{"C1": "voice", "C3": "guitar"},
		Pan:         map[string]float64{"C1": -1, "C2": 1, "C3": 0},
		Sections:    []Section{{Instruments: map[string]string{"C2": "piano"}}},
	}
	if err := validateParts(&ok); err != nil {
		t.Errorf("valid score rejected: %v", err)
	}
}

func TestScoreWithVoice(t *testing.T) {
	score := []byte(`
name: Voice
key: C
length: 0.25
instrument: piano
volume: 8000
instruments: {C1: voice}
pan: {C1: 0.1}
sections:
  - C1: [c4, 2:e4, g4]
    C2: [e4-g4, 2:c4-e4, z]
    C3: [c3, 2:g2, z]
`)
	var s Score
	out := filepath.Join(t.TempDir(), "voice")
	if _, err := Parse(&s, score, out, 0); err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if _, err := os.Stat(out + ".wav"); err != nil {
		t.Fatalf("no wav written: %v", err)
	}
	bad := []byte(strings.Replace(string(score), "{C1: voice}", "{C1: kazoo}", 1))
	if _, err := Parse(&Score{}, bad, out, 0); err == nil {
		t.Fatal("expected an error for an unknown instrument")
	}
}

// TestVoiceVeryHighPitch checks the contract holds even above the range
// where overtones fit: a single partial is still produced.
func TestVoiceVeryHighPitch(t *testing.T) {
	out := voice(12000, 0.5)
	if len(out) != sampleCount(0.5) {
		t.Fatalf("got %d samples", len(out))
	}
	peak := 0.0
	for _, v := range out {
		peak = math.Max(peak, math.Abs(v))
	}
	if math.Abs(peak-1) > 1e-9 {
		t.Errorf("peak %v, want 1", peak)
	}
}

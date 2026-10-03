package muse

import (
	"math"
	"os"
	"path/filepath"
	"testing"
)

// power returns the strength of frequency f in x[from:to], via the Goertzel
// algorithm (a single bin of a Fourier transform).
func power(x []float64, from, to int, f float64) float64 {
	w := 2 * math.Pi * f / sampleRate
	coeff := 2 * math.Cos(w)
	var s1, s2 float64
	for _, v := range x[from:to] {
		s1, s2 = v+coeff*s1-s2, s1
	}
	return s1*s1 + s2*s2 - coeff*s1*s2
}

// rms is the root mean square level of x[from:to]
func rms(x []float64, from, to int) float64 {
	sum := 0.0
	for _, v := range x[from:to] {
		sum += v * v
	}
	return math.Sqrt(sum / float64(to-from))
}

var testPitches = []float64{82.41, 196, 261.63, 440, 987.77} // guitar low E up to B5

// Every instrument returns exactly the note's sample count, peaks at 1,
// starts and ends silently, for short and long notes alike.
func TestInstrumentsAreNormalised(t *testing.T) {
	for name, ins := range instruments {
		for _, f := range testPitches {
			for _, d := range []float64{0.05, 0.5, 2} {
				x := ins(f, d)
				if len(x) != sampleCount(d) {
					t.Fatalf("%s(%v Hz, %vs) gave %d samples, want %d", name, f, d, len(x), sampleCount(d))
				}
				hi := 0.0
				for _, v := range x {
					hi = math.Max(hi, math.Abs(v))
				}
				if math.Abs(hi-1) > 1e-9 {
					t.Errorf("%s(%v Hz, %vs) peaks at %v, want 1", name, f, d, hi)
				}
				if math.Abs(x[0]) > 1e-9 || math.Abs(x[len(x)-1]) > 1e-9 {
					t.Errorf("%s(%v Hz, %vs) starts at %v and ends at %v, want 0", name, f, d, x[0], x[len(x)-1])
				}
			}
		}
	}
}

// The strongest frequency near the note must be the note itself, within a
// few cents - the piano's detuned strings and the overtones' stretch must
// not pull the pitch.
func TestInstrumentsPlayInTune(t *testing.T) {
	for name, ins := range instruments {
		for _, f := range testPitches {
			x := ins(f, 1)
			best, bestCents := 0.0, 0.0
			for cents := -50.0; cents <= 50; cents++ {
				if p := power(x, 0, len(x), f*math.Pow(2, cents/1200)); p > best {
					best, bestCents = p, cents
				}
			}
			if math.Abs(bestCents) > 3 {
				t.Errorf("%s at %v Hz is %v cents out of tune", name, f, bestCents)
			}
		}
	}
}

// Plucked and struck strings (guitar and piano) get quieter and mellower as they ring: the
// overtones fade faster than the fundamental.
func TestInstrumentsDecayAndMellow(t *testing.T) {
	// only the plucked and struck strings; a voice sustains
	for _, name := range []string{"guitar", "piano"} {
		ins := instruments[name]
		f := 196.0
		x := ins(f, 2)
		early, late := 0, sampleRate // first 50 ms vs 50 ms starting at 1 s
		win := sampleRate / 20
		if rms(x, late, late+win) >= rms(x, early, early+win) {
			t.Errorf("%s doesn't decay", name)
		}
		bright := func(from int) float64 {
			return power(x, from, from+win, 4*f) / power(x, from, from+win, f)
		}
		if bright(late) >= bright(early)/4 {
			t.Errorf("%s: 4th overtone/fundamental goes from %v to %v, want it to fade much faster", name, bright(early), bright(late))
		}
	}
}

// Plucking a fifth of the way along the string silences every 5th overtone.
func TestGuitarPluckPositionRemovesFifthOvertone(t *testing.T) {
	f := 110.0
	x := guitar(f, 1)
	win := sampleRate / 10
	p4 := power(x, 0, win, 4*f*math.Sqrt(1+0.0001*16))
	p5 := power(x, 0, win, 5*f*math.Sqrt(1+0.0001*25))
	if p5 > p4/100 {
		t.Fatalf("5th overtone power %v, want far below the 4th's %v", p5, p4)
	}
}

// Higher piano notes die away faster than lower ones.
func TestPianoHighNotesDecayFaster(t *testing.T) {
	sustain := func(f float64) float64 {
		x := piano(f, 2)
		win := sampleRate / 20
		return rms(x, sampleRate, sampleRate+win) / rms(x, 0, win)
	}
	if low, high := sustain(130.81), sustain(1046.5); high >= low {
		t.Fatalf("level after 1 s: C3 %v, C6 %v - want the high note to have faded more", low, high)
	}
}

func TestScoreWithInstrument(t *testing.T) {
	score := []byte(`
name: Instruments
key: G
length: 0.25
instrument: guitar
volume: 8000
sections:
  - C1: [g3, b3, d4-g4]
    C2: [g2, 2:z]
  - instrument: piano
    C1: [f4, 2:e4-g4]
    C2: [3:c3]
`)
	var s Score
	out := filepath.Join(t.TempDir(), "ins")
	if _, err := Parse(&s, score, out, 0); err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if _, err := os.Stat(out + ".wav"); err != nil {
		t.Fatalf("no wav written: %v", err)
	}
}

func TestMakeNoteInstrument(t *testing.T) {
	// an instrument doesn't need an envelope or harmonic
	n, err := makeNote("c4", Section{}, Score{Length: 1, Volume: 1, Instrument: "piano"})
	if err != nil {
		t.Fatalf("makeNote with instrument returned error: %v", err)
	}
	if n.ins == nil || n.env != nil || n.har != nil {
		t.Fatal("note should use the instrument and no envelope or harmonic")
	}
	// a section's instrument overrides the score's
	if _, err := makeNote("c4", Section{Instrument: "nope"}, Score{Length: 1, Volume: 1, Instrument: "piano"}); err == nil {
		t.Fatal("expected an error for an unknown instrument")
	}
}

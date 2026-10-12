package muse

import (
	"math"
	"testing"
)

const shapeTol = 1e-9

// sample returns n+1 evenly spaced values of e over a note of the given
// duration, from its start to its end.
func sample(e envelope, duration float64, n int) []float64 {
	vs := make([]float64, n+1)
	for i := range vs {
		vs[i] = e(duration*float64(i)/float64(n), duration)
	}
	return vs
}

// Every envelope must stay within [0, 1], peak at 1 and end at 0, for short
// and long notes alike, so volumes are comparable and notes never click.
func TestEnvelopesAreNormalised(t *testing.T) {
	for name, e := range envelopes {
		for _, d := range []float64{0.02, 0.25, 1, 4} {
			vs := sample(e, d, 4000)
			lo, hi := math.Inf(1), math.Inf(-1)
			for _, v := range vs {
				lo, hi = math.Min(lo, v), math.Max(hi, v)
			}
			if lo < -shapeTol {
				t.Errorf("%s(d=%v) dips to %v, want >= 0", name, d, lo)
			}
			if hi > 1+shapeTol || hi < 0.999 {
				t.Errorf("%s(d=%v) peaks at %v, want 1", name, d, hi)
			}
			if end := e(d, d); math.Abs(end) > 1e-6 {
				t.Errorf("%s(d=%v) ends at %v, want 0", name, d, end)
			}
		}
	}
}

func TestTriangleIsASingleLinearPeak(t *testing.T) {
	cases := map[float64]float64{0: 0, 0.25: 0.5, 0.5: 1, 0.75: 0.5, 1: 0}
	for x, want := range cases {
		if got := triangle(x, 1); math.Abs(got-want) > 1e-9 {
			t.Errorf("triangle(%v) = %v, want %v", x, got, want)
		}
	}
}

// the tadpole's head (its peak) comes early and its tail decays to silence
func TestTadpoleHeadComesFirst(t *testing.T) {
	vs := sample(tadpole, 1, 1000)
	peakAt := 0
	for i, v := range vs {
		if v > vs[peakAt] {
			peakAt = i
		}
	}
	if peakAt > 300 {
		t.Fatalf("tadpole peaks at %d%% of the note, want within the first 30%%", peakAt/10)
	}
	if vs[900] > 0.5 {
		t.Fatalf("tadpole is still at %v at 90%% of the note, want a decaying tail", vs[900])
	}
}

func TestDecayingEnvelopesStartLoudAndFall(t *testing.T) {
	for _, name := range []string{"drop", "drawl", "tempered"} {
		vs := sample(envelopes[name], 1, 1000)
		if math.Abs(vs[0]-1) > shapeTol {
			t.Errorf("%s starts at %v, want 1", name, vs[0])
		}
		for i := 1; i < len(vs); i++ {
			if vs[i] > vs[i-1]+shapeTol {
				t.Errorf("%s rises at sample %d (%v -> %v), want it to only fall", name, i, vs[i-1], vs[i])
				break
			}
		}
	}
}

// drawl decays fast then slowly: by the middle of the note it has lost more
// than half its level, unlike a straight line
func TestDrawlDecaysFastThenSlowly(t *testing.T) {
	if mid := drawl(0.5, 1); mid > 0.4 {
		t.Fatalf("drawl(0.5) = %v, want a fast early decay below 0.4", mid)
	}
}

// flat and rise hold their level until a short release at the very end
func TestFlatAndRiseReleaseOnlyAtTheEnd(t *testing.T) {
	d := 1.0
	if v := flat(d-releaseTime-0.001, d); v != 1 {
		t.Errorf("flat before the release = %v, want 1", v)
	}
	if v := flat(d-releaseTime/2, d); math.Abs(v-0.5) > 1e-9 {
		t.Errorf("flat halfway through the release = %v, want 0.5", v)
	}
	if v := rise(d-releaseTime-0.001, d); v < 0.99 {
		t.Errorf("rise just before the release = %v, want close to 1", v)
	}
}

func TestHarmonicsAreNormalised(t *testing.T) {
	for name, h := range harmonics {
		hi := 0.0
		for i := range 100000 {
			hi = math.Max(hi, math.Abs(h(float64(i)/100000)))
		}
		if hi > 1+1e-6 || hi < 0.999 {
			t.Errorf("%s peaks at %v, want 1", name, hi)
		}
		if v := h(0); math.Abs(v) > shapeTol {
			t.Errorf("%s(0) = %v, want 0 so notes start silently", name, v)
		}
	}
}

// Every harmonic must repeat once per cycle of the fundamental. A
// sub-harmonic (like the 0.5x term stringed used to have) would make the
// waveform repeat only every 2 cycles and add a tone an octave too low.
func TestHarmonicsHaveNoSubHarmonics(t *testing.T) {
	for name, h := range harmonics {
		for i := range 1000 {
			x := float64(i) / 1000
			if d := math.Abs(h(x) - h(x+1)); d > 1e-9 {
				t.Errorf("%s(%v) != %s(%v+1) (diff %v): waveform doesn't repeat each cycle", name, x, name, x, d)
				break
			}
		}
	}
}

// normalising only rescales: the relative strength of the overtones is
// unchanged, e.g. stringed's fundamental stays twice its 2nd harmonic
func TestStringedOvertoneBalance(t *testing.T) {
	h := harmonics["stringed"]
	// project onto sin(2πx) and sin(4πx) over one cycle
	var a1, a2 float64
	n := 100000
	for i := range n {
		x := float64(i) / float64(n)
		a1 += h(x) * math.Sin(2*math.Pi*x)
		a2 += h(x) * math.Sin(4*math.Pi*x)
	}
	if ratio := a1 / a2; math.Abs(ratio-2) > 1e-6 {
		t.Fatalf("stringed fundamental / 2nd harmonic = %v, want 2", ratio)
	}
}

// -- envelopes timed in seconds ---------------------------------------

// adsr holds its sustain level through the middle of a long note
func TestADSRHoldsTheSustain(t *testing.T) {
	if v := adsr(0.01, 2); math.Abs(v-1) > 1e-9 {
		t.Errorf("adsr at the end of the attack = %v, want 1", v)
	}
	for _, at := range []float64{0.3, 1, 1.8} {
		if v := adsr(at, 2); math.Abs(v-0.7) > 1e-9 {
			t.Errorf("adsr(%v s) = %v, want the 0.7 sustain", at, v)
		}
	}
}

// pluck and bell decay at their own rate whatever the note length, and the
// bell rings far longer
func TestStruckEnvelopesDecayByTheClock(t *testing.T) {
	for _, name := range []string{"pluck", "bell"} {
		e := envelopes[name]
		if short, long := e(0.5, 1), e(0.5, 4); math.Abs(short-long) > 1e-9 {
			t.Errorf("%s at 0.5 s is %v in a 1 s note but %v in a 4 s note, want the same", name, short, long)
		}
		vs := sample(e, 2, 2000)
		for i := 10; i < len(vs); i++ { // past the attack it only falls
			if vs[i] > vs[i-1]+shapeTol {
				t.Errorf("%s rises at sample %d", name, i)
				break
			}
		}
	}
	if p, b := pluck(1, 3), bell(1, 3); b < 4*p {
		t.Errorf("after 1 s pluck is at %v and bell at %v, want the bell ringing much longer", p, b)
	}
}

// bowed starts softly and swells to full volume just before it's let go
func TestBowedSwells(t *testing.T) {
	d := 2.0
	if v := bowed(0.03, d); v > 0.5 {
		t.Errorf("bowed after 30 ms = %v, want a soft start", v)
	}
	if mid, late := bowed(0.6, d), bowed(1.85, d); late <= mid {
		t.Errorf("bowed is %v at 0.6 s and %v at 1.85 s, want it to swell while held", mid, late)
	}
}

// staccato sounds briefly and leaves the rest of the note silent
func TestStaccatoIsShort(t *testing.T) {
	for _, d := range []float64{0.4, 2} {
		sound := math.Min(d/2, 0.3)
		for _, at := range []float64{sound, sound + 0.01, d * 0.9} {
			if v := staccato(at, d); v != 0 {
				t.Errorf("staccato(%v s) in a %v s note = %v, want silence after %v s", at, d, v, sound)
			}
		}
	}
}

// tremolo pulses about 6 times a second
func TestTremoloPulses(t *testing.T) {
	vs := sample(tremolo, 2, 20000)
	dips := 0
	for i := 1; i+1 < len(vs)*9/10; i++ {
		if vs[i] < vs[i-1] && vs[i] < vs[i+1] {
			dips++
		}
	}
	if dips < 10 || dips > 13 {
		t.Fatalf("tremolo dips %d times in 2 s, want about 12 (6 Hz)", dips)
	}
	if lo := tremolo(0.005+1.0/12, 2); math.Abs(lo-0.7) > 1e-6 {
		t.Fatalf("tremolo at the bottom of a pulse = %v, want 0.7", lo)
	}
}

// -- waveforms ----------------------------------------------------------

// overtone is the strength of overtone n in harmonic h over one cycle
func overtone(h harmonic, n int) float64 {
	const steps = 20000
	var s, c float64
	for i := range steps {
		x := float64(i) / steps
		s += h(x) * math.Sin(2*math.Pi*float64(n)*x)
		c += h(x) * math.Cos(2*math.Pi*float64(n)*x)
	}
	return 2 * math.Hypot(s, c) / steps
}

func TestWaveformColours(t *testing.T) {
	hollow, bright, mellow := harmonics["hollow"], harmonics["bright"], harmonics["mellow"]
	for n := 2; n <= 8; n += 2 {
		if v := overtone(hollow, n); v > 1e-6 {
			t.Errorf("hollow has even overtone %d at %v, want none", n, v)
		}
	}
	if overtone(bright, 2) < 0.4*overtone(bright, 1) {
		t.Error("bright should keep its even overtones (2nd at half the fundamental)")
	}
	// mellow's overtones fall far faster than hollow's: 1/n² against 1/n
	if m, h := overtone(mellow, 3)/overtone(mellow, 1), overtone(hollow, 3)/overtone(hollow, 1); m > h/2 {
		t.Errorf("mellow's 3rd overtone is %.3f of its fundamental, hollow's %.3f - want mellow much purer", m, h)
	}
	nasal := harmonics["nasal"]
	if overtone(nasal, 3) <= overtone(nasal, 1) {
		t.Error("nasal's 3rd overtone should be stronger than its fundamental")
	}
	organ := harmonics["organ"]
	if overtone(organ, 5) > 1e-6 || overtone(organ, 2) < 0.5*overtone(organ, 1) {
		t.Error("organ should have a strong octave (4') and no 5th overtone")
	}
}

// no waveform goes above the 9th overtone, so the highest notes don't alias
func TestWaveformsStayBandLimited(t *testing.T) {
	for name, h := range harmonics {
		for n := 10; n <= 16; n++ {
			if v := overtone(h, n); v > 1e-6 {
				t.Errorf("%s has overtone %d at %v, want nothing above the 9th", name, n, v)
			}
		}
	}
}

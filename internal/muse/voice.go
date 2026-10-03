package muse

import (
	"fmt"
	"math"
	"math/rand/v2"
	"sort"
)

// Voice: a singing instrument for melodies, per-channel instrument checks,
// and the stereo placement of the channels.

func init() {
	instruments["voice"] = voice
	releases["voice"] = 0.13
}

// A sung vowel is a buzz made by the vocal folds, shaped by the throat and
// mouth. The buzz is harmonic with a spectrum falling about 12 dB an octave
// (each overtone n has a level of 1/n²); the mouth is a set of resonances,
// the formants, that lift whichever overtones lie near them. The formants
// are what say "ah" rather than "ee"; they stay put when the pitch changes.
const (
	vibratoRate  = 5.5  // Hz
	vibratoDepth = 45.0 // cents either side of the pitch
	vibratoDelay = 0.2  // seconds before the vibrato starts...
	vibratoFade  = 0.3  // ...and seconds it takes to reach full depth
	voiceAttack  = 0.06 // seconds of fade-in
	voiceDamp    = 0.12 // seconds of fade-out
	// A note is normalised to peak at 1, and an unchanging tone with that
	// peak carries far more energy than a piano note, which dies away. So
	// the voice peaks as it opens (the onset emphasis) and settles to a
	// fraction of that, keeping its loudness near a piano's.
	voiceSustain = 0.4  // level it settles to
	voiceSettle  = 0.4  // seconds it takes to settle
	breathLevel  = 0.03 // onset breath noise, relative to the voice
	maxOvertones = 40
)

// a formant is one resonance of the vocal tract
type formant struct {
	freq, width, gain float64
}

// vowelAh is a warm open "ah"
var vowelAh = []formant{{650, 90, 1}, {1080, 100, 0.7}, {2650, 120, 0.35}}

// formantGain is the vocal tract's gain at hz: each formant is a resonance
// peak (a Lorentzian: full gain at its centre, half power one half
// bandwidth away), plus a small floor so no overtone is entirely silent.
// A soprano can't let the pitch rise far above the first formant without
// losing power, so singers move it up to follow the note; so does this.
func formantGain(vowel []formant, f0, hz float64) float64 {
	g := 0.02
	for i, f := range vowel {
		centre := f.freq
		if i == 0 {
			centre = math.Max(centre, math.Min(0.95*f0, 1100))
		}
		x := (hz - centre) / (f.width / 2)
		g += f.gain / math.Sqrt(1+x*x)
	}
	return g
}

// ramp is a raised-cosine rise from 0 to 1 over n samples
func ramp(k int, n float64) float64 {
	if n <= 0 || float64(k) >= n {
		return 1
	}
	return 0.5 - 0.5*math.Cos(math.Pi*float64(k)/n)
}

// voice is a sung "ah". A new note starts softly, the pitch holds steady for
// a moment and then a vibrato grows in, so short notes stay plain and long
// ones bloom. The pitch also wanders a couple of cents very slowly and the
// level shimmers a little, as no human holds a note perfectly still, and a
// breath of noise at the start stands in for the air before the tone.
//
// The pitch moves, so overtones can't each be a fixed-rate phasor as in
// additive. Instead the fundamental's phase is accumulated and the
// overtones' sines and cosines come from it by Chebyshev's recurrence
// (sin (n+1)x = 2 cos x sin nx - sin (n-1)x), which costs one multiply
// and subtraction per overtone and sample. Each overtone starts at its own
// phase so they don't line up into a sharp spike, which would leave the
// note quiet for its peak.
func voice(freq, duration float64) []float64 {
	n := sampleCount(duration)
	out := make([]float64, n)
	if n == 0 {
		return out
	}

	// leave headroom for the vibrato and drift, but always keep the
	// fundamental, so even a very high note is a tone and not silence
	count := max(1, min(maxOvertones, int(maxPartialFreq/(freq*math.Pow(2, 60.0/1200)))))
	a := make([]float64, count+1) // a[h]*sin + b[h]*cos is overtone h
	b := make([]float64, count+1)
	for h := 1; h <= count; h++ {
		hz := float64(h) * freq
		amp := formantGain(vowelAh, freq, hz) / float64(h*h)
		theta := 0.5 * math.Pi * float64(h*h) / float64(count)
		a[h], b[h] = amp*math.Cos(theta), amp*math.Sin(theta)
	}

	rng := rand.New(rand.NewPCG(uint64(freq*1000), 0x5a17))
	phase := 0.0
	breath := 0.0
	for k := range out {
		t := float64(k) / sampleRate

		// pitch in cents: delayed vibrato plus a slow drift
		cents := 2*math.Sin(2*math.Pi*0.31*t+1) + 1.2*math.Sin(2*math.Pi*0.77*t+2.5)
		vib := 0.0
		if t > vibratoDelay {
			u := math.Min(1, (t-vibratoDelay)/vibratoFade)
			vib = u * u * (3 - 2*u) * math.Sin(2*math.Pi*vibratoRate*(t-vibratoDelay))
			cents += vibratoDepth * vib
		}
		phase += 2 * math.Pi * freq * math.Exp2(cents/1200) / sampleRate

		s1, c1 := math.Sincos(phase)
		sPrev, s := 0.0, s1
		cPrev, c := 1.0, c1
		sum := a[1]*s + b[1]*c
		for h := 2; h <= count; h++ {
			sPrev, s = s, 2*c1*s-sPrev
			cPrev, c = c, 2*c1*c-cPrev
			sum += a[h]*s + b[h]*c
		}

		// a steadier level than the pitch: slight shimmer, and the
		// loudness follows the vibrato a little, as it does in a throat
		sum *= 1 + 0.03*math.Sin(2*math.Pi*4.1*t+0.7) + 0.06*vib

		// a sung note leans into its start and then settles
		sum *= voiceSustain + (1-voiceSustain)*math.Exp(-(t-voiceAttack)/voiceSettle)

		breath = 0.9*breath + (rng.Float64()*2-1)*0.1
		sum += breathLevel * breath * 3 * math.Exp(-t/0.08)

		out[k] = sum
	}

	// the first sample of the tone would otherwise be the sum of the
	// overtones' phase offsets, so fade in from silence and out again
	att := math.Min(voiceAttack, float64(n)/sampleRate/4) * sampleRate
	rel := math.Min(voiceDamp, float64(n)/sampleRate/4) * sampleRate
	peak := 0.0
	for k := range out {
		out[k] *= ramp(k, att) * ramp(n-1-k, rel)
		peak = math.Max(peak, math.Abs(out[k]))
	}
	if peak > 0 {
		for k := range out {
			out[k] /= peak
		}
	}
	out[0], out[n-1] = 0, 0
	return out
}

// Default stereo positions, from -1 (left) to 1 (right), as heard from the
// audience: the melody sits in the middle; with a piano underneath, the
// right hand (C2) leans right and the left hand (C3) left, as on the
// keyboard; with only two parts they sit either side of the middle.
const (
	panSpread = 0.35
	panPair   = 0.3
)

// pans returns the stereo position of each channel, or nil for the plain
// layout (C1 left, C2 right, C3 centre).
func pans(s *Score, t *tune) []float64 {
	if s.Plain {
		return nil
	}
	p := []float64{0, 0, 0}
	switch {
	case len(t.ch3) > 0:
		p = []float64{0, panSpread, -panSpread}
	case len(t.ch2) > 0:
		p = []float64{panPair, -panPair, 0}
	}
	for i, c := range []string{"C1", "C2", "C3"} {
		if v, ok := s.Pan[c]; ok {
			p[i] = v
		}
	}
	return p
}

// partChannels are the channels a score can set an instrument or pan for
var partChannels = map[string]bool{"C1": true, "C2": true, "C3": true}

// validateParts reports invalid values in the fields this file handles:
// the instruments, pans and levels per channel.
func validateParts(s *Score) error {
	check := func(where string, m map[string]string) error {
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if !partChannels[k] {
				return fmt.Errorf("%s: unknown channel %q - use C1, C2 or C3 ", where, k)
			}
			if _, ok := instruments[m[k]]; !ok {
				return fmt.Errorf("%s: instrument doesn't exist - %s ", where, m[k])
			}
		}
		return nil
	}
	if err := check("instruments", s.Instruments); err != nil {
		return err
	}
	for i, sec := range s.Sections {
		if err := check(fmt.Sprintf("section %d instruments", i+1), sec.Instruments); err != nil {
			return err
		}
	}
	keys := make([]string, 0, len(s.Pan))
	for k := range s.Pan {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if !partChannels[k] {
			return fmt.Errorf("pan: unknown channel %q - use C1, C2 or C3 ", k)
		}
		if v := s.Pan[k]; math.IsNaN(v) || v < -1 || v > 1 {
			return fmt.Errorf("pan: %s is %v - use a value from -1 (left) to 1 (right) ", k, v)
		}
	}
	keys = keys[:0]
	for k := range s.Levels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if !partChannels[k] {
			return fmt.Errorf("levels: unknown channel %q - use C1, C2 or C3 ", k)
		}
		if v := s.Levels[k]; math.IsNaN(v) || v < 0 || v > 1 {
			return fmt.Errorf("levels: %s is %v - use a value from 0 to 1 ", k, v)
		}
	}
	return nil
}

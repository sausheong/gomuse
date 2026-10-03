package muse

import (
	"math"
	"math/rand/v2"
)

// Winds: a trumpet and a saxophone. Both are a steady harmonic tone, like
// the voice, but what makes a horn sound like a horn is how that tone
// changes with loudness. Blowing harder doesn't just make the note louder,
// it makes it brighter: the upper overtones grow much faster than the low
// ones. So as a note starts, the low overtones speak first and the sound
// opens up as it swells - the "blat" of a trumpet, the edge of a sax.

func init() {
	instruments["trumpet"] = trumpet
	instruments["saxophone"] = saxophone
	instruments["sax"] = saxophone
	releases["trumpet"] = 0.08
	releases["saxophone"] = 0.12
	releases["sax"] = 0.12
}

// a wind describes one instrument for windNote
type wind struct {
	resonances []formant // body resonances: the bell, the bore, the mouth
	tilt       float64   // overtone n starts at 1/n^tilt
	floor      float64   // resonance gain for overtones away from any peak
	bright     float64   // how much faster upper overtones grow with breath
	attack     float64   // seconds to full level
	overshoot  float64   // extra level at the end of the attack, settling away
	sustain    float64   // level the note settles to (keeps its loudness near a piano's)
	settle     float64   // seconds it takes to settle
	damp       float64   // seconds of fade-out
	scoop      float64   // cents below the pitch a note starts at...
	scoopTime  float64   // ...and seconds it takes to reach the pitch
	vibDepth   float64   // cents either side of the pitch
	vibRate    float64   // Hz
	vibDelay   float64   // seconds before the vibrato starts
	breath     float64   // breath noise through the note, relative to the tone
	chiff      float64   // extra noise at the onset (the tongue releasing the air)
	seed       uint64
}

// The trumpet's bell lets the upper overtones out, so its spectrum is
// strong up to 1-2 kHz and it is the brightest of the two. It's tongued
// firmly, starts a little under the note as the lips find the pitch, and is
// mostly played straight, with only a touch of vibrato on long notes.
var trumpetSpec = wind{
	resonances: []formant{{1200, 1600, 1}, {2600, 1400, 0.45}},
	tilt:       0.55,
	floor:      0.15,
	bright:     0.3,
	attack:     0.035,
	overshoot:  0.25,
	sustain:    0.5,
	settle:     0.25,
	damp:       0.06,
	scoop:      30,
	scoopTime:  0.05,
	vibDepth:   12,
	vibRate:    5.8,
	vibDelay:   0.35,
	breath:     0.008,
	chiff:      0.05,
	seed:       0x7a3b,
}

// The saxophone's conical bore keeps all the overtones, like a voice, but
// its reed and the player's mouth give it a darker, nasal colour with a
// strong band around 600 Hz and another near 1.7 kHz. It starts more
// softly, slides up into the note more than a trumpet does, sings with a
// clear vibrato, and the breath is heard all through the note.
var saxophoneSpec = wind{
	resonances: []formant{{600, 500, 1}, {1700, 900, 0.5}, {3200, 1200, 0.15}},
	tilt:       0.9,
	floor:      0.08,
	bright:     0.25,
	attack:     0.06,
	overshoot:  0.15,
	sustain:    0.45,
	settle:     0.35,
	damp:       0.1,
	scoop:      50,
	scoopTime:  0.08,
	vibDepth:   25,
	vibRate:    5.0,
	vibDelay:   0.25,
	breath:     0.035,
	chiff:      0.03,
	seed:       0x5a0e,
}

func trumpet(freq, duration float64) []float64   { return windNote(trumpetSpec, freq, duration) }
func saxophone(freq, duration float64) []float64 { return windNote(saxophoneSpec, freq, duration) }

// resonanceGain is the body's gain at hz: a sum of resonance peaks
// (Lorentzians, as in formantGain) over a floor
func resonanceGain(rs []formant, floor, hz float64) float64 {
	g := floor
	for _, r := range rs {
		x := (hz - r.freq) / (r.width / 2)
		g += r.gain / math.Sqrt(1+x*x)
	}
	return g
}

// windNote synthesises one note of a wind instrument. Overtone h is played
// at a level of A[h] × e × r^(h-1), where e is the note's level and
// r = blow^bright, blow being how hard the player is blowing (0 to 1): when
// blow is small, r is small too and the upper overtones all but vanish; at
// full breath r is 1 and the full spectrum sounds. The
// overtones' sines come from the fundamental's phase by Chebyshev's
// recurrence, as in voice, so the pitch can move freely.
func windNote(w wind, freq, duration float64) []float64 {
	n := sampleCount(duration)
	out := make([]float64, n)
	if n == 0 {
		return out
	}

	// leave headroom for the scoop and vibrato, but always keep the
	// fundamental
	count := max(1, min(maxOvertones, int(maxPartialFreq/(freq*math.Exp2(w.vibDepth/1200)))))
	a := make([]float64, count+1)
	b := make([]float64, count+1)
	for h := 1; h <= count; h++ {
		amp := resonanceGain(w.resonances, w.floor, float64(h)*freq) / math.Pow(float64(h), w.tilt)
		theta := 0.5 * math.Pi * float64(h*h) / float64(count) // spread the phases
		a[h], b[h] = amp*math.Cos(theta), amp*math.Sin(theta)
	}

	rng := rand.New(rand.NewPCG(uint64(freq*1000), w.seed))
	phase, noise, dark := 0.0, 0.0, 0.0
	att := math.Min(w.attack, duration/3)
	for k := range out {
		t := float64(k) / sampleRate

		// blow is how hard the player is blowing: a quick rise to full. The
		// level follows it, with a little overshoot that settles away; the
		// settling only keeps the loudness near a piano's, so it doesn't
		// darken the tone
		blow := 1.0
		if t < att {
			u := t / att
			blow = u * u * (3 - 2*u)
		}
		e := blow * (w.sustain + (1-w.sustain+w.overshoot)*math.Exp(-math.Max(0, t-att)/w.settle)) / (1 + w.overshoot)

		// pitch: a scoop up into the note, a delayed vibrato, a slow drift
		cents := -w.scoop*math.Exp(-t/w.scoopTime) + 1.5*math.Sin(2*math.Pi*0.43*t+1.3)
		vib := 0.0
		if t > w.vibDelay {
			u := math.Min(1, (t-w.vibDelay)/0.3)
			vib = u * u * (3 - 2*u) * math.Sin(2*math.Pi*w.vibRate*(t-w.vibDelay))
			cents += w.vibDepth * vib
		}
		phase += 2 * math.Pi * freq * math.Exp2(cents/1200) / sampleRate

		// harder means brighter: overtone h grows as blow^(1 + bright(h-1))
		r := math.Pow(math.Max(blow, 1e-6), w.bright)
		s1, c1 := math.Sincos(phase)
		sPrev, s := 0.0, s1
		cPrev, c := 1.0, c1
		g := 1.0
		sum := a[1]*s + b[1]*c
		for h := 2; h <= count; h++ {
			sPrev, s = s, 2*c1*s-sPrev
			cPrev, c = c, 2*c1*c-cPrev
			g *= r
			sum += g * (a[h]*s + b[h]*c)
		}
		sum *= e * (1 + 0.04*vib)

		// breath: noise darkened by a simple low-pass, strongest at the
		// onset where the tongue lets the air go
		noise = 0.6*noise + 0.4*(rng.Float64()*2-1)
		dark = 0.85*dark + 0.15*noise
		sum += (w.breath*e + w.chiff*math.Exp(-t/0.03)) * dark * 4

		out[k] = sum
	}

	// fade out at the end, then scale to peak at 1
	rel := math.Min(w.damp, duration/4) * sampleRate
	peak := 0.0
	for k := range out {
		out[k] *= ramp(n-1-k, rel)
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

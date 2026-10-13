package muse

import "math"

// Scat is a small source/filter synthesiser, not a text-to-speech model.
// Syllable and delivery are selected as instruments so existing per-section
// routing, mixing and score validation also apply to this experimental voice.
func init() {
	for _, syllable := range []string{"doo", "bah", "wah"} {
		for _, style := range []string{"relaxed", "punchy"} {
			name := "scat-" + syllable + "-" + style
			instruments[name] = func(freq, duration float64) []float64 {
				return scat(freq, duration, syllable, style == "punchy")
			}
			releases[name] = 0.06
		}
	}
}

// A parallel band-pass resonator models one vocal-tract resonance. Keeping
// filter state while moving its centre lets wah open continuously into ah.
type scatResonator struct{ b0, b2, a1, a2, x1, x2, y1, y2 float64 }

func (r *scatResonator) tune(hz, bandwidth float64) {
	w := 2 * math.Pi * hz / sampleRate
	alpha := math.Sin(w) / (2 * hz / bandwidth)
	r.b0 = alpha / (1 + alpha)
	r.b2 = -r.b0
	r.a1 = -2 * math.Cos(w) / (1 + alpha)
	r.a2 = (1 - alpha) / (1 + alpha)
}
func (r *scatResonator) tick(x float64) float64 {
	y := r.b0*x + r.b2*r.x2 - r.a1*r.y1 - r.a2*r.y2
	r.x2, r.x1 = r.x1, x
	r.y2, r.y1 = r.y1, y
	return y
}

func scat(freq, duration float64, syllable string, punchy bool) []float64 {
	n := sampleCount(duration)
	out := make([]float64, n)
	if n == 0 || freq <= 0 || math.IsNaN(freq) || math.IsInf(freq, 0) || freq >= maxPartialFreq {
		return out
	}
	effort, tilt, air := 0.0, 2.0, 0.045
	if punchy {
		effort, tilt, air = 1, 1.15, 0.012
	}
	closed := [3]float64{300, 750, 2200}
	opened := [3]float64{730, 1150, 2600}
	if syllable == "doo" {
		opened = closed
	}
	filters := [3]scatResonator{}
	weights := [3]float64{1, .7, .35}
	count := min(80, int(maxPartialFreq/(freq*1.08)))
	amps := make([]float64, count+1)
	for h := 1; h <= count; h++ {
		amps[h] = 1 / math.Pow(float64(h), tilt)
	}
	phase, noise, peak := 0.0, uint32(0x6d2b79f5), 0.0
	attack := math.Min(.07-.045*effort, duration*.18)
	release := math.Min(.09, duration*.25)
	for k := range out {
		t := float64(k) / sampleRate
		// Pressure, mouth opening and spectral excitation change together;
		// these delivery differences survive matching the output loudness.
		opening := math.Min(1, t/(.075-.035*effort))
		if syllable == "wah" {
			opening = math.Min(1, t/(.24-.08*effort))
		}
		opening = opening * opening * (3 - 2*opening)
		if k%32 == 0 {
			for j := range filters {
				centre := closed[j] + opening*(opened[j]-closed[j])
				centre *= 1 + .06*effort
				filters[j].tune(centre, 80+float64(j)*45+effort*25)
			}
		}
		vib := math.Min(1, math.Max(0, (t-.20)/.25))
		cents := (-12-20*effort)*math.Exp(-t/.065) + (12+10*effort)*math.Min(1, 300/freq)*vib*math.Sin(2*math.Pi*5.1*t)
		phase += 2 * math.Pi * freq * math.Exp2(cents/1200) / sampleRate
		if phase > 2*math.Pi {
			phase -= 2 * math.Pi
		}
		// Band-limited harmonic excitation with style-dependent glottal tilt.
		s, c := math.Sin(phase), math.Cos(phase)
		prev, current, source := 0.0, s, 0.0
		for h := 1; h <= count; h++ {
			source += amps[h] * current
			prev, current = current, 2*c*current-prev
		}
		noise ^= noise << 13
		noise ^= noise >> 17
		noise ^= noise << 5
		white := float64(noise)/2147483648 - 1
		source += air * white
		value := .045 * source
		for j := range filters {
			value += weights[j] * filters[j].tick(source)
		}
		// D and B releases have different brief noise bursts and closures.
		if syllable != "wah" {
			burstStart := .010
			burstFreq := 2500.0
			if syllable == "bah" {
				burstFreq = 700
			}
			bt := t - burstStart
			if bt >= 0 && bt < .018 {
				value += (.12 + .12*effort) * white * math.Sin(2*math.Pi*burstFreq*t) * math.Exp(-bt/.004)
			}
		}
		env := ramp(k, attack*sampleRate) * ramp(n-1-k, release*sampleRate)
		pressure := .78 + .12*math.Sin(math.Pi*float64(k)/float64(n))
		if punchy {
			pressure = .62 + .5*math.Exp(-t/.12)
		}
		out[k] = value * env * pressure
		peak = math.Max(peak, math.Abs(out[k]))
	}
	if peak > 0 {
		for k := range out {
			out[k] /= peak
		}
	}
	return out
}

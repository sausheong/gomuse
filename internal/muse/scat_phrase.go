package muse

import (
	"context"
	"math"
)

// renderScatPhrase keeps one oscillator and one vocal tract for the entire
// channel. Notes steer this state rather than creating independent waveforms.
func renderScatPhrase(ctx context.Context, events []scatEvent, duration float64) ([]float64, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	out := make([]float64, sampleCount(duration))
	if len(events) == 0 {
		return out, nil
	}
	phase := 0.0
	noise := uint32(0x6d2b79f5)
	filters := [3]scatResonator{}
	centres := [3]float64{300, 750, 2200}
	weights := [3]float64{1, .7, .35}
	amps := [81]float64{}
	idx, lastIndex := 0, -1
	syllable := "doo"
	wordStart := 0.0
	var pronunciation *scatPronunciation
	phoneNoise := scatResonator{}
	phoneNoise.tune(6000, 3500)
	wordEnds := scatWordSpans(events)
	fricative := scatResonator{}
	fricative.tune(6000, 3500)
	nasalFilter := scatResonator{}
	nasalFilter.tune(250, 100)
	effort, air, frequency, gain := 0.0, 0.0, 440*math.Exp2((float64(events[0].MIDI)-69)/12), 0.0
	peak, maxGain := 0.0, 0.0
	for _, e := range events {
		maxGain = math.Max(maxGain, e.Gain)
	}
	for k := range out {
		if k%4096 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		t := float64(k) / sampleRate
		for idx+1 < len(events) && t >= events[idx+1].Start {
			idx++
		}
		e := events[idx]
		local := t - e.Start
		progress := local / math.Max(.001, e.Duration)
		active := local >= 0 && local < e.Duration
		if active && idx != lastIndex {
			if e.Lyric != "+" {
				syllable = e.Lyric
				wordStart = e.Start
				pronunciation = e.pronunciation
			}
			lastIndex = idx
		}
		phraseT := math.Max(0, t-e.phraseStart)
		phraseLength := e.phraseEnd - e.phraseStart
		arc := math.Sin(math.Pi * math.Min(1, phraseT/math.Max(.001, phraseLength)))
		targetEffort := clampFloat(e.settings.effort*(.72+.28*arc)+.24*(e.emphasis-.5), 0, 1)
		targetEffort = scatCurve(e.gesture.Effort, progress, targetEffort)
		effort += (targetEffort - effort) * .002
		air += (scatCurve(e.gesture.Air, progress, e.settings.air) - air) * .002
		gain += (e.Gain - gain) * .004
		targetHz := 440 * math.Exp2((float64(e.MIDI)-69)/12)
		// Portamento is logarithmic and deliberately short; it carries the voice
		// through a change of note without restarting the vocal-fold phase.
		glide := .018 + .027*(1-effort)
		if e.settings.detached > 0 {
			glide = .008
		}
		frequency *= math.Exp(math.Log(targetHz/frequency) / (glide * sampleRate))
		wt := math.Max(0, t-wordStart)
		openTime := .065 - .03*e.settings.crisp
		if syllable == "wah" {
			openTime = .25 - .13*effort
		}
		opening := math.Min(1, wt/math.Max(.008, openTime))
		opening = opening * opening * (3 - 2*opening)
		closed := [3]float64{300, 750, 2200}
		opened := [3]float64{730, 1150, 2600}
		if syllable == "doo" {
			opened = closed
		}
		if len(e.gesture.Mouth) > 0 {
			opened = [3]float64{730, 1150, 2600}
			opening = scatCurve(e.gesture.Mouth, progress, opening)
		}
		word, isWord := scatWords[syllable]
		isWord = isWord || pronunciation != nil
		phone := scatPhoneFrame{}
		if isWord {
			if pronunciation != nil {
				phone = pronunciation.frame(wt, wordEnds[idx]-wordStart)
			} else {
				phone = scatWordFrame(word, wt, wordEnds[idx]-wordStart)
			}
		}
		if k%32 == 0 {
			if pronunciation != nil {
				hz, width := phone.noiseHz, phone.noiseWidth
				if hz == 0 {
					hz, width = 6000, 3500
				}
				phoneNoise.tune(hz, width)
			}
			for j := range filters {
				target := (closed[j] + opening*(opened[j]-closed[j])) * (.98 + .09*effort)
				if isWord {
					target = phone.formants[j] * (.99 + .025*effort)
					// Keep vowel identity; mouth gestures add modest colour instead of replacing it.
					if len(e.gesture.Mouth) > 0 {
						target *= .97 + .06*opening
					}
				}
				tracking := .06
				if isWord {
					tracking += .10 * e.settings.clarity
				}
				centres[j] += (target - centres[j]) * tracking
				filters[j].tune(centres[j], 85+float64(j)*45+30*effort)
			}
			tilt := 2.35 - 1.1*effort
			if isWord {
				tilt = 1.35 - .4*effort
			}
			for h := 1; h < len(amps); h++ {
				amps[h] = 1 / math.Pow(float64(h), tilt)
			}
		}
		vibFade := math.Min(1, math.Max(0, (wt-.24)/.25))
		cents := -e.settings.scoop*math.Exp(-wt/.055) + e.settings.vibrato*vibFade*math.Min(1, 300/frequency)*math.Sin(2*math.Pi*5.15*t)
		// Authored pitch curves replace the generic scoop/vibrato.
		cents = scatCurve(e.gesture.Pitch, progress, cents)
		hz := frequency * math.Exp2(cents/1200)
		phase += 2 * math.Pi * hz / sampleRate
		phase = math.Mod(phase, 2*math.Pi)
		si, co := math.Sin(phase), math.Cos(phase)
		prev, cur, source := 0.0, si, 0.0
		count := min(80, int(maxPartialFreq/hz))
		for h := 1; h <= count; h++ {
			source += amps[h] * cur
			prev, cur = cur, 2*co*cur-prev
		}
		noise ^= noise << 13
		noise ^= noise >> 17
		noise ^= noise << 5
		white := float64(noise)/2147483648 - 1
		gate := 0.0
		if active {
			// Connected vowels retain pressure. Stops close briefly before opening;
			// extensions neither repeat the consonant nor restart the vowel envelope.
			gate = 1
			if e.gesture.Attack > 0 {
				gate *= math.Min(1, math.Max(0, local/e.gesture.Attack))
			}
			if e.gesture.Release > 0 {
				gate *= math.Min(1, math.Max(0, (e.Duration-local)/e.gesture.Release))
			}
			if !isWord && e.Lyric != "+" && syllable != "wah" {
				closure := .008 + .012*e.settings.crisp
				gate *= .18 + .82*math.Min(1, math.Max(0, (local-closure)/(.025-.015*e.settings.crisp)))
			}
			attack := .035
			if isWord {
				attack = .008
			}
			gate *= ramp(int(phraseT*sampleRate), attack*sampleRate)
			// A short pressure release separates words, never notes within a tied vowel.
			if isWord && e.settings.clarity > 0 {
				releaseTime := .04
				if pronunciation != nil && len(pronunciation.coda) > 0 {
					releaseTime = .008
				}
				wordRelease := math.Min(1, math.Max(0, (wordEnds[idx]-t)/releaseTime))
				gate *= 1 - e.settings.clarity*(1-wordRelease)
			}
			left := e.phraseEnd - t
			releaseTime := .065
			if pronunciation != nil && len(pronunciation.coda) > 0 {
				releaseTime = .008
			}
			gate *= ramp(int(math.Max(0, left)*sampleRate), releaseTime*sampleRate)
			if e.settings.detached > 0 {
				gate *= ramp(int(math.Max(0, e.Duration*.82-local)*sampleRate), math.Min(.035, e.Duration*.12)*sampleRate)
			}
		}
		breath := 0.0
		// An inhalation occupies an actual written rest, never a vowel or a
		// following consonant. Use at least 120 ms of available breathing space.
		if !active && t >= e.phraseEnd && (idx+1 == len(events) || events[idx+1].Start-e.phraseEnd >= .12) {
			gapEnd := duration
			if idx+1 < len(events) {
				gapEnd = events[idx+1].Start
			}
			span := math.Min(.22, math.Max(0, gapEnd-e.phraseEnd-.025))
			u := (t - e.phraseEnd) / math.Max(.001, span)
			if u >= 0 && u < 1 {
				breath = e.settings.breaths * .09 * math.Sin(math.Pi*u)
			}
		}
		excitation := (source+(.008+.13*air)*white)*gate + breath*white
		if isWord {
			excitation = (source*phone.voiced*(1-.65*phone.nasal)+(.025*air+phone.aspiration)*white)*gate + breath*white
		}
		value := .035 * excitation
		for j := range filters {
			weight := weights[j]
			if isWord {
				weight *= math.Pow(centres[j]/500, .85)
				// Expose vowel-identifying upper resonances without changing their centres.
				weight *= 1 + e.settings.clarity*float64(j)*.18
			}
			value += weight * filters[j].tick(excitation)
		}
		fric := fricative.tick(white)
		// Voiced turbulence shares the oscillator's pulse instead of being an
		// independent hiss layered over a vowel. RMS-normalise the modulation.
		pulse := phone.fricationPulse
		noiseDrive := white * (1 + pulse*si) / math.Sqrt(1+.5*pulse*pulse)
		customNoise := phoneNoise.tick(noiseDrive)
		if pronunciation != nil {
			fric = customNoise
		}
		nasal := nasalFilter.tick(source)
		if isWord {
			value += gate * (fric*(phone.frication+phone.burst)*(1+.6*e.settings.clarity) + nasal*phone.nasal*(.7-.2*e.settings.clarity))
		}
		if active && !isWord && e.Lyric != "+" && syllable != "wah" {
			bt := local - (.008 + .012*e.settings.crisp)
			if bt >= 0 && bt < .015 {
				centre := 2400.0
				if syllable == "bah" {
					centre = 650
				}
				value += (.025 + .11*e.settings.crisp) * white * math.Sin(2*math.Pi*centre*t) * math.Exp(-bt/.0035)
			}
		}
		stress := .78 + .44*e.emphasis
		pressure := (.7 + .25*arc) * stress
		out[k] = value * pressure * gain
		peak = math.Max(peak, math.Abs(out[k]))
	}
	// One normalisation per channel preserves phrase dynamics and all relative
	// emphasis. Score volume still determines absolute gain, including mute.
	if peak > 0 {
		scale := math.Min(.95, maxGain) / peak
		for k := range out {
			out[k] *= scale
		}
	}
	fade := min(len(out)/2, sampleCount(.006))
	for k := 0; k < fade; k++ {
		out[k] *= ramp(k, float64(fade))
		out[len(out)-1-k] *= ramp(k, float64(fade))
	}
	return out, nil
}

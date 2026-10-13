package muse

import (
	"fmt"
	"math"
)

// ARPABET-like symbols describe one sung syllable. The vowel receives the
// remaining note duration; consonants retain bounded articulation times.
type scatVowel struct {
	start, end [3]float64
	glide      float64
}

var scatVowels = map[string]scatVowel{
	"AA": {[3]float64{750, 1200, 2500}, [3]float64{750, 1200, 2500}, 0},
	"AE": {[3]float64{660, 1700, 2500}, [3]float64{660, 1700, 2500}, 0},
	"AH": {[3]float64{620, 1200, 2500}, [3]float64{620, 1200, 2500}, 0},
	"AO": {[3]float64{550, 850, 2400}, [3]float64{550, 850, 2400}, 0},
	"EH": {[3]float64{530, 1850, 2500}, [3]float64{530, 1850, 2500}, 0},
	"ER": {[3]float64{450, 1350, 1700}, [3]float64{450, 1350, 1700}, 0},
	"IH": {[3]float64{400, 2000, 2700}, [3]float64{400, 2000, 2700}, 0},
	"IY": {[3]float64{280, 2300, 3000}, [3]float64{280, 2300, 3000}, 0},
	"UH": {[3]float64{440, 1000, 2300}, [3]float64{440, 1000, 2300}, 0},
	"UW": {[3]float64{320, 750, 2300}, [3]float64{320, 750, 2300}, 0},
	"EY": {[3]float64{530, 1850, 2500}, [3]float64{330, 2200, 2900}, .08},
	"AY": {[3]float64{750, 1200, 2500}, [3]float64{330, 2200, 2900}, .12},
	"OW": {[3]float64{500, 900, 2400}, [3]float64{320, 700, 2300}, .12},
	"AW": {[3]float64{750, 1200, 2500}, [3]float64{350, 850, 2400}, .12},
	"OY": {[3]float64{550, 850, 2400}, [3]float64{330, 2200, 2900}, .12},
}
var scatConsonantTimes = map[string]float64{
	"HH": .15, "M": .085, "N": .085, "NG": .085, "Y": .16, "W": .16,
	"S": .12, "Z": .085, "F": .12, "V": .085, "L": .09, "R": .09,
	"P": .065, "B": .06, "T": .065, "D": .06, "K": .075, "G": .07,
}

type scatPronunciation struct {
	onset, coda []string
	vowel       scatVowel
}

func parseScatPhones(phones []string) (*scatPronunciation, error) {
	if len(phones) == 0 || len(phones) > 9 {
		return nil, fmt.Errorf("phonemes need 1 to 9 symbols per syllable")
	}
	p := &scatPronunciation{}
	vowelAt := -1
	for i, ph := range phones {
		if v, ok := scatVowels[ph]; ok {
			if vowelAt >= 0 {
				return nil, fmt.Errorf("phonemes need exactly one vowel per note; split multisyllable words across notes")
			}
			vowelAt = i
			p.vowel = v
		} else if _, ok := scatConsonantTimes[ph]; !ok {
			return nil, fmt.Errorf("unsupported phoneme %q", ph)
		}
	}
	if vowelAt < 0 {
		return nil, fmt.Errorf("phonemes require a vowel")
	}
	p.onset = append([]string(nil), phones[:vowelAt]...)
	p.coda = append([]string(nil), phones[vowelAt+1:]...)
	if len(p.onset) > 4 || len(p.coda) > 4 {
		return nil, fmt.Errorf("at most four consonants before or after a vowel")
	}
	return p, nil
}
func phoneSpan(phones []string) float64 {
	v := 0.0
	for _, p := range phones {
		v += scatConsonantTimes[p]
	}
	return v
}

// Final Z needs sustained buzzing after the vowel, not a stop-like flick.
func codaSpan(phones []string) float64 {
	total := phoneSpan(phones)
	for _, p := range phones {
		if p == "Z" {
			total += .075
		}
	}
	return total
}
func (p *scatPronunciation) frame(elapsed, duration float64) scatPhoneFrame {
	on, off := math.Min(phoneSpan(p.onset), duration*.3), math.Min(codaSpan(p.coda), duration*.25)
	if elapsed < on && on > 0 {
		return scatClusterFrame(p.onset, elapsed/on, p.vowel.start, false)
	}
	if elapsed >= duration-off && off > 0 {
		return scatClusterFrame(p.coda, (elapsed-duration+off)/off, p.vowel.end, true)
	}
	f := scatWordFrame(scatWord{vowel: p.vowel.start, end: p.vowel.end, maxOffglide: p.vowel.glide}, math.Max(0, elapsed-on), math.Max(.001, duration-on-off))
	if len(p.coda) > 0 && p.coda[0] == "Z" {
		// Begin constriction during the vowel offglide, before the coda slot.
		// Voicing continues so the turbulence is heard as part of this syllable.
		overlap := math.Min(.08, math.Max(0, (duration-on-off)*.3))
		if overlap > 0 {
			blend := scatSmooth((elapsed - (duration - off - overlap)) / overlap)
			f.frication = .5 * blend
			f.fricationPulse = .9 * blend
			f.noiseHz, f.noiseWidth = 4500, 3000
		}
	}
	return f
}
func scatClusterFrame(phones []string, u float64, vowel [3]float64, coda bool) scatPhoneFrame {
	total := phoneSpan(phones)
	if coda {
		total = codaSpan(phones)
	}
	at := math.Max(0, math.Min(1, u)) * total
	for i, p := range phones {
		span := scatConsonantTimes[p]
		if coda && p == "Z" {
			span += .075
		}
		if at < span || i == len(phones)-1 {
			target := vowel
			if !coda && i+1 < len(phones) {
				next := phones[i+1]
				if next == "R" || next == "L" || next == "Y" || next == "W" {
					target = scatConsonantFrame(next, 0, vowel, false).formants
				}
			}
			return scatConsonantFrame(p, math.Min(1, at/span), target, coda)
		}
		at -= span
	}
	return scatPhoneFrame{formants: vowel, voiced: 1}
}
func scatConsonantFrame(p string, u float64, vowel [3]float64, coda bool) scatPhoneFrame {
	f := scatPhoneFrame{formants: vowel, noiseHz: 6000, noiseWidth: 3500}
	transition := scatSmooth((u - .45) / .55)
	if coda {
		transition = 1 - scatSmooth(u/.55)
	}
	switch p {
	case "P", "B", "T", "D", "K", "G":
		f.noiseHz = 3500
		f.noiseWidth = 2200
		if p == "P" || p == "B" {
			f.noiseHz = 750
			f.noiseWidth = 1200
		}
		if p == "K" || p == "G" {
			f.noiseHz = 2200
			f.noiseWidth = 1800
		}
		release := .55
		if coda {
			release = .72
		}
		if u >= release {
			f.burst = 1.2 * math.Exp(-(u-release)*28)
		}
		voiced := p == "B" || p == "D" || p == "G"
		if voiced {
			f.voiced = .08
		}
		if !coda {
			closure := [3]float64{220, 1800, 2600}
			if p == "P" || p == "B" {
				closure = [3]float64{200, 800, 2100}
			}
			if p == "K" || p == "G" {
				closure = [3]float64{250, 2000, 2600}
			}
			blend := scatSmooth((u - release) / .35)
			for j := range closure {
				f.formants[j] = closure[j] + blend*(vowel[j]-closure[j])
			}
			if !voiced && u >= release {
				f.aspiration = .2 * (1 - blend)
			}
			f.voiced += (1 - f.voiced) * scatSmooth((u-.72)/.28)
		} else {
			// Close the tract continuously from the vowel before the release.
			// Retain a little closure voicing for B/D/G without a discontinuity.
			closure := [3]float64{220, 1800, 2600}
			if p == "P" || p == "B" {
				closure = [3]float64{200, 800, 2100}
			}
			if p == "K" || p == "G" {
				closure = [3]float64{250, 2000, 2600}
			}
			blend := scatSmooth(u / .3)
			for j := range closure {
				f.formants[j] = vowel[j] + blend*(closure[j]-vowel[j])
			}
			f.voiced += (1 - f.voiced) * (1 - scatSmooth(u/.3))
		}
	case "S", "Z", "F", "V":
		if p == "F" || p == "V" {
			f.noiseHz = 2500
			f.noiseWidth = 4500
		}
		// Sustained turbulent airflow, with short ramps rather than a burst.
		f.frication = .65 * scatSmooth(u/.12) * (1 - scatSmooth((u-.72)/.28))
		if p == "F" {
			f.frication *= 1.5
		}
		if p == "Z" || p == "V" {
			f.voiced = .45
			f.frication *= .65
		}
		if coda && p == "Z" {
			// Keep a voiced buzz, with enough turbulence to distinguish it from D.
			f.frication = (.5 + .35*scatSmooth(u/.12)) * (1 - scatSmooth((u-.9)/.1))
			f.voiced = .7
			f.fricationPulse = .9
			f.noiseHz, f.noiseWidth = 4500, 3000
		}
		if !coda {
			f.voiced += (1 - f.voiced) * scatSmooth((u-.72)/.28)
		} else {
			// Enter final frication from the held vowel without a hard voice cut.
			f.voiced += (1 - f.voiced) * (1 - scatSmooth(u/.25))
		}
	case "HH":
		if coda {
			f.voiced = 1 - scatSmooth(u)
			f.aspiration = .5 * u
		} else {
			f.voiced = scatSmooth(u)
			f.aspiration = .5 * (1 - u)
		}
	case "M", "N", "NG", "Y", "W", "L", "R":
		start := [3]float64{250, 1100, 2100}
		if !coda {
			if p == "Y" || p == "W" {
				transition = scatSmooth(u)
			}
			if p == "M" || p == "N" || p == "NG" {
				transition = scatSmooth((u - .5) / .5)
			}
		}
		f.voiced = 1
		if p == "M" || p == "N" || p == "NG" {
			f.voiced = .55 + .45*transition
		}
		switch p {
		case "M":
			f.nasal = 1 - transition
		case "N":
			start = [3]float64{280, 1650, 2600}
			f.nasal = 1 - transition
		case "NG":
			start = [3]float64{280, 1900, 2500}
			f.nasal = 1 - transition
		case "Y":
			start = [3]float64{260, 2450, 3100}
		case "W":
			start = [3]float64{250, 600, 2200}
		case "L":
			start = [3]float64{360, 1050, 2700}
		case "R":
			start = [3]float64{350, 1150, 1550}
		}
		for j := range start {
			f.formants[j] = start[j] + transition*(vowel[j]-start[j])
		}
	}
	return f
}

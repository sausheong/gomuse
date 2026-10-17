package muse

import "math"

// Small, explicit pronunciation lexicon. These are synthesis targets, not
// recordings or a general English spelling-to-sound system.
type scatWord struct {
	onset       string
	vowel, end  [3]float64
	maxOffglide float64 // optional cap in seconds for a compact diphthong ending
}

var scatWords = map[string]scatWord{
	"hey":  {"h", [3]float64{530, 1850, 2500}, [3]float64{330, 2200, 2900}, .08},
	"oh":   {"", [3]float64{500, 900, 2400}, [3]float64{320, 700, 2300}, 0},
	"my":   {"m", [3]float64{750, 1200, 2500}, [3]float64{330, 2200, 2900}, 0},
	"you":  {"y", [3]float64{320, 750, 2300}, [3]float64{320, 750, 2300}, 0},
	"stay": {"st", [3]float64{530, 1850, 2500}, [3]float64{330, 2200, 2900}, .08},
	"me":   {"m", [3]float64{280, 2300, 3000}, [3]float64{280, 2300, 3000}, 0},
	"we":   {"w", [3]float64{280, 2300, 3000}, [3]float64{280, 2300, 3000}, 0},
	"see":  {"s", [3]float64{280, 2300, 3000}, [3]float64{280, 2300, 3000}, 0},
	"say":  {"s", [3]float64{530, 1850, 2500}, [3]float64{330, 2200, 2900}, .08},
	"no":   {"n", [3]float64{500, 900, 2400}, [3]float64{320, 700, 2300}, 0},
	"so":   {"s", [3]float64{500, 900, 2400}, [3]float64{320, 700, 2300}, 0},
	"hi":   {"h", [3]float64{750, 1200, 2500}, [3]float64{330, 2200, 2900}, 0},
}

type scatPhoneFrame struct {
	formants                             [3]float64
	voiced, aspiration, frication, burst float64
	nasal                                float64
	noiseHz, noiseWidth                  float64
	fricationPulse                       float64 // Glottal synchronisation of voiced turbulence.
}

func scatSmooth(x float64) float64 {
	x = clampFloat(x, 0, 1)
	return x * x * (3 - 2*x)
}

// scatWordFrame reserves a bounded onset, holds the vowel nucleus, then moves
// through a diphthong near the end. Lengthening a word never lengthens its s/t.
func scatWordFrame(word scatWord, elapsed, duration float64) scatPhoneFrame {
	onset := 0.0
	switch word.onset {
	case "h":
		onset = .15
	case "m", "n":
		onset = .085
	case "y", "w":
		onset = .16
	case "s":
		onset = .12
	case "st":
		onset = .18
	}
	onset = math.Min(onset, duration*.32)
	f := scatPhoneFrame{formants: word.vowel, voiced: 1}
	endSpan := math.Min(.23, math.Max(.06, (duration-onset)*.28))
	if word.maxOffglide > 0 {
		endSpan = math.Min(endSpan, word.maxOffglide)
	}
	dip := scatSmooth((elapsed - (duration - endSpan)) / endSpan)
	for j := range f.formants {
		f.formants[j] += dip * (word.end[j] - word.vowel[j])
	}
	if onset == 0 || elapsed >= onset {
		return f
	}
	u := math.Max(0, elapsed/onset)
	switch word.onset {
	case "h":
		f.voiced = scatSmooth(u)
		f.aspiration = .5 * (1 - u)
	case "m", "n":
		blend := scatSmooth((u - .5) / .5)
		nasal := [3]float64{250, 1100, 2100}
		if word.onset == "n" {
			nasal = [3]float64{280, 1650, 2600}
		}
		for j := range f.formants {
			f.formants[j] = nasal[j] + blend*(word.vowel[j]-nasal[j])
		}
		f.nasal = 1 - blend
		f.voiced = .55 + .45*blend
	case "y", "w":
		start := [3]float64{260, 2450, 3100}
		if word.onset == "w" {
			start = [3]float64{250, 600, 2200}
		}
		blend := scatSmooth(u)
		for j := range f.formants {
			f.formants[j] = start[j] + blend*(word.vowel[j]-start[j])
		}
	case "s":
		f.voiced = scatSmooth((u - .85) / .15)
		f.frication = .7 * math.Sin(math.Pi*math.Min(1, u/.95))
	case "st":
		f.voiced = 0
		if u < .62 {
			f.frication = .7 * math.Sin(math.Pi*u/.62)
		}
		// A silent closure separates the fricative from the alveolar release.
		if u >= .82 {
			f.burst = .25 * math.Exp(-(u-.82)*45)
			f.aspiration = .18 * (1 - scatSmooth((u-.82)/.18))
		}
		f.voiced = scatSmooth((u - .9) / .1)
	}
	return f
}

// Determine the whole syllable span once, including any tied vowel notes.
// Planning uses the end of the last +, so the diphthong is not completed early.
func scatWordSpans(events []scatEvent) []float64 {
	ends := make([]float64, len(events))
	for i := 0; i < len(events); {
		j := i + 1
		for j < len(events) && events[j].Lyric == "+" {
			j++
		}
		end := events[j-1].Start + events[j-1].Duration
		for k := i; k < j; k++ {
			ends[k] = end
		}
		i = j
	}
	return ends
}

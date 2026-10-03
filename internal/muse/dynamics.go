package muse

import (
	"fmt"
	"math"
)

// Dynamics: how loud each note is played, from the score's dynamic
// markings and hairpins. A softer note is also darker, as on a real piano,
// guitar or voice, so the level sets both gain and tone.

// dynamicLevels maps each marking to its level in dB below fff. Steps are
// 4 dB apart: about what a listener hears as one step in force, and wide
// enough that ppp and fff are clearly different without ppp vanishing.
var dynamicLevels = map[string]float64{
	"ppp": -28,
	"pp":  -24,
	"p":   -20,
	"mp":  -16,
	"mf":  -12,
	"f":   -8,
	"ff":  -4,
	"fff": 0,
}

const (
	// dynamicStep is the size of one step between markings, in dB; a
	// hairpin with nowhere marked to go moves by one step
	dynamicStep = 4.0
	minLevel    = -28.0 // ppp
	maxLevel    = 0.0   // fff
	// defaultLevel is mf: the level before the first marking
	defaultLevel = -12.0
	// velocityRange is the dB span over which velocity falls from 1 (fff)
	// to 0, so mf is 0.6
	velocityRange = 30.0
	minVelocity   = 0.1

	// phrase shaping: at most this much of a note's gain is taken away
	// for being low in its phrase, or short beside the phrase's longest
	pitchShape  = 0.06
	lengthShape = 0.04
)

// dBGain converts a level in dB to an amplitude factor
func dBGain(db float64) float64 { return math.Pow(10, db/20) }

// velocityOf is how hard a note at the given level is played, 0.1 to 1
func velocityOf(db float64) float64 {
	return math.Max(minVelocity, math.Min(1, 1+db/velocityRange))
}

// sectionLevels works out the level at the start and end of every section.
// A marking sets the level until the next one; before the first it is mf.
// A hairpin runs from the section's level to the next section's marking,
// or one step up or down when the next section has none, and the level it
// reaches carries on from there. ok is false when the score has no
// dynamics at all.
func sectionLevels(s *Score) (start, end []float64, ok bool) {
	for _, sec := range s.Sections {
		if sec.Dynamic != "" || sec.Hairpin != "" {
			ok = true
		}
	}
	if !ok {
		return nil, nil, false
	}
	start = make([]float64, len(s.Sections))
	end = make([]float64, len(s.Sections))
	cur := defaultLevel
	for i, sec := range s.Sections {
		if sec.Dynamic != "" {
			cur = dynamicLevels[sec.Dynamic]
		}
		start[i] = cur
		end[i] = cur
		if sec.Hairpin == "" {
			continue
		}
		step := dynamicStep
		if sec.Hairpin == "dim" {
			step = -dynamicStep
		}
		target := math.Max(minLevel, math.Min(maxLevel, cur+step))
		if i+1 < len(s.Sections) && s.Sections[i+1].Dynamic != "" {
			target = dynamicLevels[s.Sections[i+1].Dynamic]
		}
		end[i] = target
		cur = target
	}
	return start, end, true
}

// applyDynamics sets each note's velocity and scales its gain from the
// sections' Dynamic and Hairpin markings. A score with no markings keeps
// its gains and velocities (it isn't made quieter, and its melody isn't
// reshaped: the accents and balance from expression.go stand as they are).
// In a marked score the melody also gets a little phrase shaping, at most
// 10% off a note's gain. Plain scores are left alone.
func applyDynamics(s *Score, t *tune) {
	if s.Plain {
		return
	}
	start, end, marked := sectionLevels(s)
	if !marked {
		return
	}
	shapePhrases(s, t.ch1)
	chs := [3][]note{t.ch1, t.ch2, t.ch3}
	for c := range chs {
		i := 0
		for si, sec := range s.Sections {
			ns := sectionChannel(sec, c)
			total := 0.0
			for k := range ns {
				total += chs[c][i+k].length
			}
			at := 0.0
			for range ns {
				n := &chs[c][i]
				frac := 0.0
				if total > 0 {
					frac = at / total
				}
				db := start[si] + (end[si]-start[si])*frac
				g := n.gain
				if g == 0 {
					g = 1
				}
				n.gain = math.Min(1, g*dBGain(db))
				n.velocity = velocityOf(db)
				at += n.length
				i++
			}
		}
	}
}

// highest is the top sounding pitch of a note, in semitones
func highest(n note) int {
	top := math.MinInt
	for k, p := range n.pitch {
		top = max(top, p+n.accidental[k])
	}
	return top
}

// shapePhrases gives the melody a little phrase shape: within a phrase
// (notes between rests, ending after any note of two beats or more) the
// notes near the top are a touch fuller than those near the bottom, and
// the longest notes a touch fuller than the shortest, as a singer leans on
// the peak and the held notes. It only ever lowers gain, by at most 10%.
func shapePhrases(s *Score, notes []note) {
	var phrase []int
	flush := func() {
		if len(phrase) > 1 {
			shapePhrase(notes, phrase)
		}
		phrase = phrase[:0]
	}
	i := 0
	for _, sec := range s.Sections {
		beat := sec.Length
		if beat == 0 {
			beat = s.Length
		}
		for range sec.C1 {
			n := notes[i]
			switch {
			case len(n.pitch) == 0:
				flush()
			default:
				phrase = append(phrase, i)
				if beat > 0 && n.length >= 2*beat-1e-9 {
					flush()
				}
			}
			i++
		}
	}
	flush()
}

func shapePhrase(notes []note, idx []int) {
	lo, hi := math.MaxInt, math.MinInt
	longest := 0.0
	for _, i := range idx {
		p := highest(notes[i])
		lo, hi = min(lo, p), max(hi, p)
		longest = math.Max(longest, notes[i].length)
	}
	for _, i := range idx {
		n := &notes[i]
		rel, lenRel := 1.0, 1.0
		if hi > lo {
			rel = float64(highest(*n)-lo) / float64(hi-lo)
		}
		if longest > 0 {
			lenRel = n.length / longest
		}
		m := 1 - pitchShape*(1-rel) - lengthShape*(1-lenRel)
		g := n.gain
		if g == 0 {
			g = 1
		}
		n.gain = math.Min(1, g*m)
	}
}

// tone shapes an instrument note's samples for how hard it was played:
// a softer note is darker as well as quieter. Two one-pole low-pass
// filters in series, with a cutoff that rises with velocity from about
// twice the note's pitch (pianissimo) to forty times (fortissimo, where
// it's inaudible), take the top off. The filter has no negative lobes, so
// the peak can't grow, and the result ends at silence. It returns samples
// unchanged when velocity is unset (0) or the cutoff is out of the way.
func tone(n note, freq float64, samples []float64) []float64 {
	if n.velocity <= 0 || len(samples) == 0 {
		return samples
	}
	v := math.Min(1, n.velocity)
	fc := freq * (2 + 38*v*v)
	if fc >= 0.4*sampleRate {
		return samples
	}
	a := 1 - math.Exp(-2*math.Pi*fc/sampleRate)
	out := make([]float64, len(samples))
	y1, y2 := 0.0, 0.0
	for k, x := range samples {
		y1 += a * (x - y1)
		y2 += a * (y1 - y2)
		out[k] = y2
	}
	out[len(out)-1] = 0
	return out
}

// validateDynamics reports invalid values in the fields this file handles.
func validateDynamics(s *Score) error {
	for i, sec := range s.Sections {
		if sec.Dynamic != "" {
			if _, ok := dynamicLevels[sec.Dynamic]; !ok {
				return fmt.Errorf("section %d: unknown dynamic %q (use ppp, pp, p, mp, mf, f, ff or fff)", i+1, sec.Dynamic)
			}
		}
		switch sec.Hairpin {
		case "", "cresc", "dim":
		default:
			return fmt.Errorf("section %d: unknown hairpin %q (use cresc or dim)", i+1, sec.Hairpin)
		}
	}
	return nil
}

package muse

import (
	"fmt"
	"math"
	"math/rand/v2"
)

// Timing: small departures from machine-exact time. A metronome is never
// what a player follows: phrases broaden at their ends, a fermata waits,
// swung eighths lean on the beat, and no two hands land on exactly the
// same instant. Everything here is deterministic.

const (
	// finalRitardando is the gentle slowing applied to the last section of
	// a score that doesn't ask for its own: the last bar's notes are
	// stretched by up to 20% by the final note
	finalRitardando = 0.2
	// jitterSD is the standard deviation of a note's start jitter at
	// humanize 1, and jitterMax the most it is ever moved, in seconds
	jitterSD  = 0.006
	jitterMax = 0.020
	// melodyLead is how far, at humanize 1, the top line runs ahead of
	// the beat; a singer or right hand tends to lead the accompaniment
	melodyLead = 0.003
	// minHold is the shortest a note is ever held, in seconds
	minHold = 0.01
	// defaultHumanize applies when a score doesn't set one
	defaultHumanize = 0.5
)

// humanizeAmount is the effective humanize setting of a score
func humanizeAmount(s *Score) float64 {
	if s.Humanize == nil {
		return defaultHumanize
	}
	return math.Max(0, math.Min(1, *s.Humanize))
}

// tempoSection describes one section for the tempo map.
type tempoSection struct {
	written  float64 // written length in seconds
	perf     float64 // performed start, in seconds
	rit      float64 // ritardando: the local duration factor ends at 1+rit
	fermata  float64 // extra hold at the end, in seconds
	beat     float64 // written length of a beat, in seconds
	swing    float64 // swing amount, 0 to 1
	duration float64 // performed length including the fermata
}

// tempoMap maps written time to performed time. It is shared by every
// channel so that the same written beat is the same moment everywhere.
type tempoMap []tempoSection

// buildTempoMap lays the sections end to end. A section's local duration
// factor rises linearly from 1 to 1+r over the section, so written time u
// within it takes u + r*u^2/(2D) seconds, D being the written length. The
// next section starts back in tempo.
func buildTempoMap(s *Score, t *tune) tempoMap {
	chs := [3][]note{t.ch1, t.ch2, t.ch3}
	tm := make(tempoMap, len(s.Sections))
	idx := [3]int{}
	at := 0.0
	for k, sec := range s.Sections {
		d := 0.0
		for c := range chs {
			sum := 0.0
			for range sectionChannel(sec, c) {
				if idx[c] < len(chs[c]) {
					sum += chs[c][idx[c]].length
				}
				idx[c]++
			}
			d = math.Max(d, sum)
		}
		beat := sec.Length
		if beat == 0 {
			beat = s.Length
		}
		if beat <= 0 {
			beat = 0.25
		}
		rit := sec.Ritardando
		if rit == 0 && k == len(s.Sections)-1 {
			rit = finalRitardando
		}
		ts := tempoSection{
			written: d, perf: at, rit: rit, beat: beat, swing: s.Swing,
			fermata: sec.Fermata * beat,
		}
		ts.duration = d*(1+rit/2) + ts.fermata
		at += ts.duration
		tm[k] = ts
	}
	return tm
}

// swung moves a written position within a section onto the swung grid:
// anything halfway through a beat is delayed by swing/6 of a beat, which
// at full swing makes the beat a triplet (2:1). The section end is never
// moved, so sections keep their length.
func (ts tempoSection) swung(u float64) float64 {
	if ts.swing == 0 || u >= ts.written-1e-6 {
		return u
	}
	frac := u/ts.beat - math.Floor(u/ts.beat)
	if math.Abs(frac-0.5) < 1e-3 {
		return u + ts.swing*ts.beat/6
	}
	return u
}

// at is the performed time of written position u within the section,
// where end says u is the end of a note (and so is held by a fermata).
func (ts tempoSection) at(u float64, end bool) float64 {
	atEnd := u >= ts.written-1e-6
	u = ts.swung(u)
	if atEnd {
		u = ts.written
	}
	p := ts.perf + u
	if ts.written > 0 {
		p += ts.rit * u * u / (2 * ts.written)
	}
	if end && atEnd {
		p += ts.fermata
	}
	return p
}

// applyTiming places every note in time (note.placed, start, hold) and
// sets chord spread, from Humanize, Swing and the sections' Ritardando and
// Fermata. All channels stay in step: the same written beat maps to the
// same moment in every channel, apart from small per-note jitter that
// never accumulates, because each note is jittered around its own grid
// position and ends on the grid.
func applyTiming(s *Score, t *tune) {
	if s.Plain {
		return
	}
	tm := buildTempoMap(s, t)
	h := humanizeAmount(s)
	chs := [3][]note{t.ch1, t.ch2, t.ch3}
	for c := range chs {
		rnd := rand.New(rand.NewPCG(uint64(c)+101, 2024))
		i := 0
		for k, sec := range s.Sections {
			ts := tm[k]
			u := 0.0
			ins := channelInstrument(s, sec, c)
			for range sectionChannel(sec, c) {
				if i >= len(chs[c]) {
					break
				}
				n := &chs[c][i]
				i++
				startAt := ts.at(u, false)
				endAt := ts.at(u+n.length, true)
				u += n.length
				if len(n.pitch) > 0 {
					startAt += jitterFor(rnd, h, c)
					startAt = math.Max(0, startAt)
				}
				n.placed = true
				n.start = sampleCount(startAt)
				// the note ends on the grid whatever its start did
				n.hold = math.Max(minHold, endAt-float64(n.start)/sampleRate)
				if len(n.pitch) > 1 {
					n.spread = chordSpread(rnd, h, ins, n.hold, len(n.pitch))
				}
			}
		}
	}
}

// jitterFor draws one note's start offset in seconds: roughly gaussian,
// a few milliseconds wide, clamped, with the melody leading slightly.
// It always consumes the same random numbers, so changing humanize only
// scales the result.
func jitterFor(rnd *rand.Rand, h float64, channel int) float64 {
	j := rnd.NormFloat64() * jitterSD * h
	if channel == 0 {
		j -= melodyLead * h
	}
	return math.Max(-jitterMax, math.Min(jitterMax, j))
}

// chordSpread is the delay between the pitches of a rolled chord: a
// pianist's hand rolls a few milliseconds, a guitarist's strum is slower.
// It is scaled so the default humanize gives the stated range, and kept
// short enough that the roll fits well inside the note.
func chordSpread(rnd *rand.Rand, h float64, ins string, hold float64, pitches int) float64 {
	lo, hi := 0.004, 0.008
	if ins == "guitar" {
		lo, hi = 0.012, 0.020
	}
	sp := (lo + (hi-lo)*rnd.Float64()) * 2 * h
	return math.Min(sp, hold/(2*float64(pitches)))
}

// channelInstrument names the instrument a channel plays in a section,
// following the same precedence as makeChannelNote.
func channelInstrument(s *Score, sec Section, c int) string {
	ch := [3]string{"C1", "C2", "C3"}[c]
	ins := sec.Instrument
	if v, ok := sec.Instruments[ch]; ok {
		ins = v
	} else if v, ok := s.Instruments[ch]; ok && sec.Instrument == "" {
		ins = v
	}
	if ins == "" {
		ins = s.Instrument
	}
	return ins
}

// maxFermataSeconds bounds the total time all fermatas may add to a score.
// The sample cap in Parse counts written note lengths only, so without this
// bound fermatas could stretch the performed audio far past the cap.
const maxFermataSeconds = 60.0

// validateTiming reports invalid values in the fields this file handles.
// A plain score ignores them all, so it is never rejected for them.
func validateTiming(s *Score) error {
	if s.Plain {
		return nil
	}
	if s.Swing < 0 || s.Swing > 1 {
		return fmt.Errorf("swing must be between 0 and 1, got %v", s.Swing)
	}
	if s.Humanize != nil && (*s.Humanize < 0 || *s.Humanize > 1) {
		return fmt.Errorf("humanize must be between 0 and 1, got %v", *s.Humanize)
	}
	held := 0.0
	for i, sec := range s.Sections {
		beat := sec.Length
		if beat <= 0 {
			beat = s.Length
		}
		if beat <= 0 {
			beat = 0.25
		}
		held += sec.Fermata * beat
		if sec.Ritardando <= -0.5 || sec.Ritardando > 1 {
			return fmt.Errorf("section %d: ritardando must be above -0.5 and at most 1, got %v", i+1, sec.Ritardando)
		}
		if sec.Fermata < 0 || sec.Fermata > 8 {
			return fmt.Errorf("section %d: fermata must be between 0 and 8 beats, got %v", i+1, sec.Fermata)
		}
	}
	if held > maxFermataSeconds {
		return fmt.Errorf("fermatas add %.0f seconds in total, at most %.0f are allowed", held, maxFermataSeconds)
	}
	return nil
}

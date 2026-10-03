package muse

import (
	"math"
	"math/rand/v2"
)

// Expression: the small departures from the written score that make a
// performance sound played rather than printed. A score can turn all of
// it off with "plain: true".

// releases is how long each instrument's notes keep sounding after their
// written length, in seconds. A pianist's fingers overlap a little from
// one key to the next before the dampers fall, and a guitarist's strings
// ring on under the next note; the tail is added on top of whatever
// follows, so consecutive notes blend instead of being cut off.
// Envelopes are shapes that end at silence, so they get no tail.
var releases = map[string]float64{
	"guitar": 0.8,
	"piano":  0.3,
}

// channelGains balances the parts when there is more than one: C1 usually
// carries the melody, C2 the inner part or right hand, C3 the bass. A
// score's Levels replace these.
var channelGains = [3]float64{1, 0.8, 0.7}

const (
	// accents: the first note of a section (usually a bar) is the
	// strongest, notes on a beat are next, and notes between beats are
	// lightest
	downbeat = 1.0
	onBeat   = 0.92
	offBeat  = 0.85
	// jitter is the most a note's loudness is randomly lowered by, so that
	// repeated notes aren't identical
	jitter = 0.05
	// defaultReverb is the reverb amount when a score doesn't set one
	defaultReverb = 0.25
)

// expressive applies part balance, accents and loudness jitter to every
// note of a tune. Notes start at the section's beat offsets, so the
// accents follow the beats of each section. The jitter comes from a fixed
// seed, so a score always renders the same way.
func expressive(s *Score, t *tune) {
	chs := [3][]note{t.ch1, t.ch2, t.ch3}
	if s.Plain {
		// a plain score is played as written, but a level the score asks
		// for is part of what's written
		for c, name := range []string{"C1", "C2", "C3"} {
			if lv, ok := s.Levels[name]; ok {
				for i := range chs[c] {
					chs[c][i].gain = lv
				}
			}
		}
		return
	}
	used := 0
	for _, ch := range chs {
		if len(ch) > 0 {
			used++
		}
	}
	rnd := rand.New(rand.NewPCG(1, 2))
	for c := range chs {
		balance := 1.0
		if used > 1 {
			balance = channelGains[c]
		}
		if lv, ok := s.Levels[[]string{"C1", "C2", "C3"}[c]]; ok {
			balance = lv
		}
		i := 0
		for _, sec := range s.Sections {
			beat := sec.Length
			if beat == 0 {
				beat = s.Length
			}
			at := 0.0 // position in the section, in beats
			for range sectionChannel(sec, c) {
				n := &chs[c][i]
				accent := offBeat
				switch {
				case at < 1e-6:
					accent = downbeat
				case math.Abs(at-math.Round(at)) < 1e-3:
					accent = onBeat
				}
				n.gain = balance * accent * (1 - jitter*rnd.Float64())
				at += n.length / beat
				i++
			}
		}
	}
}

func sectionChannel(sec Section, c int) []string {
	return [3][]string{sec.C1, sec.C2, sec.C3}[c]
}

// reverbAmount is how much reverb a score asks for, from 0 (none) to 1
func reverbAmount(s *Score) float64 {
	switch {
	case s.Plain:
		return 0
	case s.Reverb == nil:
		return defaultReverb
	}
	return math.Max(0, math.Min(1, *s.Reverb))
}

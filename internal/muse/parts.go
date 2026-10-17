package muse

import (
	"fmt"
	"maps"
	"slices"
)

// Parts: the settings a score makes per channel - its instrument, its place
// in the stereo field and its level - and checking them.

// Default stereo positions, from -1 (left) to 1 (right), as heard from the
// audience: the melody sits in the middle; with a piano underneath, the
// right hand (C2) leans right and the left hand (C3) left, as on the
// keyboard; with only two parts they sit either side of the middle.
const (
	panSpread = 0.35
	panPair   = 0.3
)

// pans returns the stereo position of each channel (in the order of
// channelNames), or nil for the plain layout (C1 left, C2 right, C3 and the
// drums centre).
func pans(s *Score, t *tune) []float64 {
	if s.Plain {
		return nil
	}
	// the drums sit in the centre, as a kit does in most mixes
	p := []float64{0, 0, 0, 0}
	switch {
	case len(t.ch3) > 0:
		p = []float64{0, panSpread, -panSpread, 0}
	case len(t.ch2) > 0:
		p = []float64{panPair, -panPair, 0, 0}
	}
	for i, c := range channelNames {
		if v, ok := s.Pan[c]; ok {
			p[i] = v
		}
	}
	return p
}

// partChannels are the channels a score can set an instrument for. The
// drums are always the drum kit, but can be panned and levelled like any
// other channel, so mixChannels adds them.
var (
	partChannels = map[string]bool{"C1": true, "C2": true, "C3": true}
	mixChannels  = map[string]bool{"C1": true, "C2": true, "C3": true, "drums": true}
)

// validateParts reports invalid values in the fields this file handles:
// the instruments, pans and levels per channel.
func validateParts(s *Score) error {
	check := func(where string, m map[string]string) error {
		for _, k := range slices.Sorted(maps.Keys(m)) {
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
	for _, k := range slices.Sorted(maps.Keys(s.Pan)) {
		if !mixChannels[k] {
			return fmt.Errorf("pan: unknown channel %q - use C1, C2, C3 or drums ", k)
		}
		if v := s.Pan[k]; !finiteRange(v, -1, 1) {
			return fmt.Errorf("pan: %s is %v - use a value from -1 (left) to 1 (right) ", k, v)
		}
	}
	for _, k := range slices.Sorted(maps.Keys(s.Levels)) {
		if !mixChannels[k] {
			return fmt.Errorf("levels: unknown channel %q - use C1, C2, C3 or drums ", k)
		}
		if v := s.Levels[k]; !finiteRange(v, 0, 1) {
			return fmt.Errorf("levels: %s is %v - use a value from 0 to 1 ", k, v)
		}
	}
	return nil
}

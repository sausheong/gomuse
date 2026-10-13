package muse

import "fmt"

// RoomSettings selects the shared send bus. Omitted sends default to one;
// explicit zero leaves that instrument dry without changing its dry level.
type RoomSettings struct {
	Preset string             `yaml:"preset"`
	Sends  map[string]float64 `yaml:"sends,omitempty"`
}

func validateRoom(s *Score) error {
	if s.Room == nil {
		return nil
	}
	if s.Room.Preset != "small" {
		return fmt.Errorf("room preset must be small")
	}
	for ch, v := range s.Room.Sends {
		if !mixChannels[ch] || !finiteRange(v, 0, 1) {
			return fmt.Errorf("room sends require C1/C2/C3/drums and finite amounts 0..1")
		}
	}
	return nil
}
func smallRoom(l, r []int, cs [channelCount][]int, settings *RoomSettings, amount float64) ([]int, []int) {
	if amount <= 0 || len(l) == 0 {
		return l, r
	}
	sends := [channelCount]float64{1, 1, 1, 1}
	active := false
	for c, ch := range channelNames {
		if v, ok := settings.Sends[ch]; ok {
			sends[c] = v
		}
		if sends[c] > 0 && len(cs[c]) > 0 {
			active = true
		}
	}
	if !active {
		return l, r
	}
	left, right := newRoom(0), newRoom(stereoSpread)
	for _, room := range []*room{left, right} {
		for i := range room.combs {
			c := &room.combs[i]
			c.buf = make([]float64, len(c.buf)*2/3)
			c.feedback = .67
			c.damping = .48
		}
	}
	// Early reflections establish a compact room before its diffuse tail.
	delay := make([]float64, sampleCount(.06))
	at := 0
	low := 0.0
	n := len(l) + sampleCount(1.5)
	ol, or := make([]int, n), make([]int, n)
	last := len(l) - 1
	for k := 0; k < n; k++ {
		in := 0.0
		for c, x := range cs {
			if k < len(x) {
				in += float64(x[k]) * sends[c]
			}
		}
		low += .018 * (in - low)
		in -= low // keep low bass energy out of the room bus
		delay[at] = in
		tap := func(seconds float64) float64 { return delay[(at-sampleCount(seconds)+len(delay))%len(delay)] }
		earlyL := .13*tap(.013) + .07*tap(.031)
		earlyR := .12*tap(.019) + .08*tap(.043)
		diffuse := tap(.011) * roomInput
		dl, dr := 0, 0
		if k < len(l) {
			dl, dr = l[k], r[k]
		}
		ol[k] = dl + int(amount*(earlyL+2*left.process(diffuse)))
		or[k] = dr + int(amount*(earlyR+2*right.process(diffuse)))
		if ol[k] != 0 || or[k] != 0 {
			last = k
		}
		at = (at + 1) % len(delay)
	}
	end := max(len(l), last+1)
	return ol[:end], or[:end]
}

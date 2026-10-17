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

// roomSends is each channel's send level to the room: 1 unless the score
// sets it
func roomSends(settings *RoomSettings) [channelCount]float64 {
	sends := [channelCount]float64{1, 1, 1, 1}
	for c, ch := range channelNames {
		if v, ok := settings.Sends[ch]; ok {
			sends[c] = v
		}
	}
	return sends
}

// smallRoomTail is how long the small room rings past the dry signal
const smallRoomTail = 1.5

func smallRoom(l, r []int, cs [channelCount][]int, settings *RoomSettings, amount float64) ([]int, []int) {
	if amount <= 0 || len(l) == 0 {
		return l, r
	}
	sends := roomSends(settings)
	active := false
	for c := range cs {
		active = active || (sends[c] > 0 && len(cs[c]) > 0)
	}
	if !active {
		return l, r
	}
	bus := newSmallRoomBus()
	n := len(l) + sampleCount(smallRoomTail)
	ol, or := make([]int, n), make([]int, n)
	last := len(l) - 1
	for k := 0; k < n; k++ {
		in := 0.0
		for c, x := range cs {
			if k < len(x) {
				in += float64(x[k]) * sends[c]
			}
		}
		wl, wr := bus.process(in, amount)
		if k < len(l) {
			ol[k], or[k] = l[k]+wl, r[k]+wr
		} else {
			ol[k], or[k] = wl, wr
		}
		if ol[k] != 0 || or[k] != 0 {
			last = k
		}
	}
	end := max(len(l), last+1)
	return ol[:end], or[:end]
}

// smallRoomBus is the shared send bus of the small room: early reflections
// establish a compact room before a shorter, darker Freeverb tail.
type smallRoomBus struct {
	left, right *room
	delay       []float64 // recent input, for the early reflections
	at          int       // write position in delay
	low         float64   // low-pass state, to keep bass out of the room
}

func newSmallRoomBus() *smallRoomBus {
	b := &smallRoomBus{left: newRoom(0), right: newRoom(stereoSpread), delay: make([]float64, sampleCount(.06))}
	for _, room := range []*room{b.left, b.right} {
		for i := range room.combs {
			c := &room.combs[i]
			c.buf = make([]float64, len(c.buf)*2/3)
			c.feedback = .67
			c.damping = .48
		}
	}
	return b
}

// process feeds one sample of the summed sends into the room and returns
// what the room adds to the left and right sides
func (b *smallRoomBus) process(in, amount float64) (int, int) {
	b.low += .018 * (in - b.low)
	in -= b.low // keep low bass energy out of the room bus
	b.delay[b.at] = in
	tap := func(seconds float64) float64 { return b.delay[(b.at-sampleCount(seconds)+len(b.delay))%len(b.delay)] }
	earlyL := .13*tap(.013) + .07*tap(.031)
	earlyR := .12*tap(.019) + .08*tap(.043)
	diffuse := tap(.011) * roomInput
	b.at = (b.at + 1) % len(b.delay)
	return int(amount * (earlyL + 2*b.left.process(diffuse))), int(amount * (earlyR + 2*b.right.process(diffuse)))
}

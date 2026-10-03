package muse

// A room reverb, after Jezar's public domain Freeverb: eight parallel
// feedback comb filters, each with a low-pass in its loop so the echoes
// darken as they fade, then four allpass filters in series to smear the
// echoes into a smooth tail. The right channel's delays are a little longer
// than the left's, which makes the room sound wide.

const (
	roomFeedback = 0.84 // how long the tail is
	roomDamp     = 0.3  // how quickly high frequencies die in the tail
	roomInput    = 0.015
	stereoSpread = 23
	// reverbTail is how long the output is extended so the tail can fade,
	// in seconds; trailing silence is trimmed afterwards
	reverbTail = 3.0
)

// delays in samples, tuned for 44.1 kHz
var (
	combDelays    = []int{1116, 1188, 1277, 1356, 1422, 1491, 1557, 1617}
	allpassDelays = []int{556, 441, 341, 225}
)

type comb struct {
	buf    []float64
	i      int
	filter float64
}

func (c *comb) process(in float64) float64 {
	out := c.buf[c.i]
	c.filter = out*(1-roomDamp) + c.filter*roomDamp
	c.buf[c.i] = in + c.filter*roomFeedback
	if c.i++; c.i == len(c.buf) {
		c.i = 0
	}
	return out
}

type allpass struct {
	buf []float64
	i   int
}

func (a *allpass) process(in float64) float64 {
	b := a.buf[a.i]
	a.buf[a.i] = in + b*0.5
	if a.i++; a.i == len(a.buf) {
		a.i = 0
	}
	return b - in
}

// room is one side of the reverb
type room struct {
	combs     []comb
	allpasses []allpass
}

func newRoom(spread int) *room {
	r := &room{}
	for _, d := range combDelays {
		r.combs = append(r.combs, comb{buf: make([]float64, d+spread)})
	}
	for _, d := range allpassDelays {
		r.allpasses = append(r.allpasses, allpass{buf: make([]float64, d+spread)})
	}
	return r
}

func (r *room) process(in float64) (out float64) {
	for i := range r.combs {
		out += r.combs[i].process(in)
	}
	for i := range r.allpasses {
		out = r.allpasses[i].process(out)
	}
	return out
}

// reverb adds a room to a stereo pair of channels. amount is the level of
// the reverberated sound, from 0 (dry) to 1; the dry sound is kept as it
// is. The result is longer than the input by the length of the tail.
func reverb(l, r []int, amount float64) ([]int, []int) {
	if amount <= 0 || len(l) == 0 {
		return l, r
	}
	n := len(l) + sampleCount(reverbTail)
	left, right := newRoom(0), newRoom(stereoSpread)
	wet := amount * 3 // Freeverb's wet scale
	ol, or := make([]int, n), make([]int, n)
	last := 0
	for k := range n {
		var dl, dr float64
		if k < len(l) {
			dl, dr = float64(l[k]), float64(r[k])
		}
		in := (dl + dr) * roomInput
		ol[k] = int(dl + wet*left.process(in))
		or[k] = int(dr + wet*right.process(in))
		if ol[k] != 0 || or[k] != 0 {
			last = k
		}
	}
	end := max(len(l), last+1)
	return ol[:end], or[:end]
}

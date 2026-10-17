package muse

import (
	"context"
	"fmt"
	"math"

	"gopkg.in/yaml.v3"
)

// PlayerInfo describes browser playback. Progressive playback preserves score
// dynamics but leaves whole-song RMS mastering to buffered playback/export.
type PlayerInfo struct {
	Name       string  `json:"name"`
	SampleRate int     `json:"sampleRate"`
	Duration   float64 `json:"duration"`
	Mastering  bool    `json:"mastering"`
	Scat       bool    `json:"scat"`
}

const playerMaxFrames = sampleRate * 600

func playerTune(doc []byte) (Score, tune, error) {
	var s Score
	if len(doc) > 2<<20 {
		return s, tune{}, fmt.Errorf("score exceeds 2 MB")
	}
	if err := yaml.Unmarshal(doc, &s); err != nil {
		return s, tune{}, err
	}
	if s.Singing != nil {
		return s, tune{}, fmt.Errorf("neural singing needs the local CLI singer; use WAV export for this score")
	}
	t, err := buildTune(&s)
	if err != nil {
		return s, t, err
	}
	if !validKey(t.key) {
		return s, t, fmt.Errorf("unknown key signature: %s", t.key)
	}
	hasNotes := false
	for _, ch := range t.channels() {
		hasNotes = hasNotes || len(ch) > 0
		if channelSamples(ch) > sampleRate*7200 || channelNominal(ch) > playerMaxFrames || channelWritten(ch) > playerMaxFrames {
			return s, t, fmt.Errorf("browser playback supports up to ten minutes and two hours of synthesized voices per channel")
		}
	}
	if !hasNotes {
		return s, t, fmt.Errorf("score contains no notes")
	}
	if s.Scat != nil {
		if err = attachScat(context.Background(), &s, &t); err != nil {
			return s, t, err
		}
	}
	return s, t, nil
}

// RenderPCM renders in memory, with exactly the same mastering and effects as
// WAV export. It does not access the filesystem or execute a singer backend.
func RenderPCM(doc []byte) (PlayerInfo, []int, error) {
	s, t, err := playerTune(doc)
	if err != nil {
		return PlayerInfo{}, nil, err
	}
	pcm, err := t.encode()
	return PlayerInfo{s.Name, sampleRate, float64(len(pcm)/2) / sampleRate, s.Mastering != nil, s.Scat != nil}, pcm, err
}

// pendingEvent is a synthesized note still sounding, at sample at
type pendingEvent struct {
	at   int
	data []int
}

// streamChannel renders one channel incrementally: notes are synthesized
// only shortly before they're needed and kept until they've finished.
type streamChannel struct {
	cursor   channelCursor
	pending  []pendingEvent
	end      int   // the channel's length in samples, including tails
	external []int // a pre-rendered vocal, in place of the notes
	filter   *channelFilter
}

// schedule synthesizes every note that starts before end, plus a little
// lookahead, since timing humanisation can put a following event just ahead
// of its neighbour.
func (c *streamChannel) schedule(end int) error {
	if c.external != nil {
		return nil
	}
	for c.cursor.index < len(c.cursor.notes) {
		n := c.cursor.notes[c.cursor.index]
		pos := c.cursor.at
		if n.placed {
			pos = max(0, n.start)
		}
		if pos >= end+sampleCount(.1) {
			break
		}
		at, data, err := c.cursor.next()
		if err != nil {
			return err
		}
		c.end = max(c.end, at+len(data))
		if len(n.pitch) > 0 || len(n.kit) > 0 {
			c.pending = append(c.pending, pendingEvent{at, data})
		}
	}
	return nil
}

// sample is the channel's mixed and filtered output at sample at
func (c *streamChannel) sample(at int) int {
	x := 0
	if at < len(c.external) {
		x = c.external[at]
	}
	for _, e := range c.pending {
		if i := at - e.at; i >= 0 && i < len(e.data) {
			x += e.data[i]
		}
	}
	if at >= c.end {
		return 0
	}
	if c.filter == nil {
		return x
	}
	return c.filter.process(x, at, c.end)
}

// drop forgets the events that have finished sounding before end
func (c *streamChannel) drop(end int) {
	keep := c.pending[:0]
	for _, e := range c.pending {
		if e.at+len(e.data) > end {
			keep = append(keep, e)
		}
	}
	clear(c.pending[len(keep):])
	c.pending = keep
}

// PCMStream synthesizes only events needed for the next block. Voice state,
// overlapping release tails, channel filters and the room persist between calls.
// Each event is still synthesized as a unit, so an unusually long chord may
// take longer than a block to generate. Call from a worker, never an audio thread.
type PCMStream struct {
	Info     PlayerInfo
	channels [channelCount]streamChannel
	tune     tune
	// frame is the next sample to render, nominal the written length,
	// tail how long the reverb rings on, and final the total length once
	// every note has been synthesized (-1 until then)
	frame, nominal, tail, final int
	gains                       [channelCount][2]float64 // left and right
	sends                       [channelCount]float64
	left, right                 *room         // the default reverb
	small                       *smallRoomBus // the small room, in its place
}

func NewPCMStream(doc []byte) (*PCMStream, error) {
	s, t, err := playerTune(doc)
	if err != nil {
		return nil, err
	}
	p := &PCMStream{tune: t, final: -1, Info: PlayerInfo{Name: s.Name, SampleRate: sampleRate, Mastering: s.Mastering != nil, Scat: s.Scat != nil}}
	t.prepareForRender()
	sends := [channelCount]float64{1, 1, 1, 1}
	if t.room != nil {
		sends = roomSends(t.room)
	}
	ref := -1
	roomActive := false
	for i, ch := range t.channels() {
		nominal := channelNominal(ch)
		if len(ch) > 0 {
			if ref >= 0 && max(nominal-ref, ref-nominal) >= tolerance {
				return nil, fmt.Errorf("channel lengths are different")
			}
			ref = nominal
		}
		p.nominal = max(p.nominal, nominal)
		c := streamChannel{cursor: channelCursor{notes: ch}, end: max(nominal, channelWritten(ch)), filter: newChannelFilter(t.mix[channelNames[i]])}
		if ext := t.external[i]; ext != nil {
			c.external = ext
			c.end = max(nominal, len(ext))
			c.cursor.index = len(ch)
		}
		p.channels[i] = c
		p.gains[i] = channelGainsLR(t, i)
		p.sends[i] = sends[i]
		roomActive = roomActive || (len(ch) > 0 && sends[i] > 0)
	}
	switch {
	case t.reverb <= 0:
	case t.room == nil:
		p.left, p.right = newRoom(0), newRoom(stereoSpread)
		p.tail = sampleCount(reverbTail)
	case roomActive:
		p.small = newSmallRoomBus()
		p.tail = sampleCount(smallRoomTail)
	}
	p.Info.Duration = float64(p.nominal) / sampleRate
	return p, nil
}

// channelGainsLR is how much of channel i goes to the left and right sides:
// the equal-power pan when the tune is panned, otherwise the plain layout
// (C1 left, C2 right, each on both sides if the other is empty, and the
// rest centred)
func channelGainsLR(t tune, i int) [2]float64 {
	if t.pan != nil {
		a := (t.pan[i] + 1) * math.Pi / 4
		return [2]float64{math.Cos(a) * math.Sqrt2, math.Sin(a) * math.Sqrt2}
	}
	switch {
	case i == 0 && len(t.ch2) == 0, i == 1 && len(t.ch1) == 0:
		return [2]float64{1, 1}
	case i == 0:
		return [2]float64{1, 0}
	case i == 1:
		return [2]float64{0, 1}
	}
	return [2]float64{1, 1}
}

// Next returns interleaved signed 16-bit PCM in an int slice. The final block
// may be short. Complete tail time is retained (including terminal silence).
func (p *PCMStream) Next(frames int) ([]int, bool, error) {
	if frames < 1 || frames > sampleRate {
		return nil, false, fmt.Errorf("block size must be 1..44100 frames")
	}
	if p.final >= 0 && p.frame >= p.final {
		return nil, true, nil
	}
	end := p.frame + frames
	done := true
	dryEnd := p.nominal
	for i := range p.channels {
		c := &p.channels[i]
		if err := c.schedule(end); err != nil {
			return nil, false, err
		}
		done = done && c.cursor.index == len(c.cursor.notes)
		dryEnd = max(dryEnd, c.end)
	}
	if done {
		p.final = dryEnd + p.tail
		p.Info.Duration = float64(p.final) / sampleRate
		end = min(end, p.final)
	}
	out := make([]int, 0, (end-p.frame)*2)
	for at := p.frame; at < end; at++ {
		l, r, in := 0, 0, 0.
		for i := range p.channels {
			x := p.channels[i].sample(at)
			l += int(float64(x) * p.gains[i][0])
			r += int(float64(x) * p.gains[i][1])
			in += float64(x) * p.sends[i]
		}
		switch {
		case p.left != nil:
			in = float64(l+r) * roomInput
			l, r = int(float64(l)+p.tune.reverb*3*p.left.process(in)), int(float64(r)+p.tune.reverb*3*p.right.process(in))
		case p.small != nil:
			wl, wr := p.small.process(in, p.tune.reverb)
			l, r = l+wl, r+wr
		}
		out = append(out, clamp(l), clamp(r))
	}
	p.frame = end
	for i := range p.channels {
		p.channels[i].drop(end)
	}
	return out, p.final >= 0 && p.frame >= p.final, nil
}

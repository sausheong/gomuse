package muse

import (
	"context"
	"fmt"
	"gopkg.in/yaml.v3"
	"math"
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
		written := 0
		for _, n := range ch {
			written += sampleCount(n.length)
		}
		if channelSamples(ch) > sampleRate*7200 || channelNominal(ch) > playerMaxFrames || written > playerMaxFrames {
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

type pendingEvent struct {
	at   int
	data []int
}
type streamChannel struct {
	cursor        channelCursor
	pending       []pendingEvent
	end           int
	external      []int
	hp, lp, split float64
}

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
		// Timing humanisation can put a following event just ahead of its neighbour.
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
func (c *streamChannel) sample(at int, m ChannelMix) int {
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
	if m == (ChannelMix{}) {
		return x
	}
	y := float64(x)
	if m.LowCutHz > 0 {
		c.hp += (1 - math.Exp(-2*math.Pi*m.LowCutHz/sampleRate)) * (y - c.hp)
		y -= c.hp
	}
	if m.HighCutHz > 0 {
		c.lp += (1 - math.Exp(-2*math.Pi*m.HighCutHz/sampleRate)) * (y - c.lp)
		y = c.lp
	}
	if m.TiltDB != 0 {
		c.split += (1 - math.Exp(-2*math.Pi*1000/sampleRate)) * (y - c.split)
		y = c.split*dBGain(-m.TiltDB/2) + (y-c.split)*dBGain(m.TiltDB/2)
	}
	if m.LowCutHz > 0 || m.HighCutHz > 0 || m.TiltDB != 0 {
		y *= math.Min(1, float64(c.end-1-at)/math.Min(float64(sampleCount(.01)), float64(max(1, c.end-1))))
	}
	return int(math.Round(y * dBGain(m.GainDB)))
}

// PCMStream synthesizes only events needed for the next block. Voice state,
// overlapping release tails, channel filters and the room persist between calls.
// Each event is still synthesized as a unit, so an unusually long chord may
// take longer than a block to generate. Call from a worker, never an audio thread.
type PCMStream struct {
	Info                        PlayerInfo
	channels                    [channelCount]streamChannel
	tune                        tune
	frame, nominal, tail, final int
	left, right                 *room
	delay                       []float64
	delayAt                     int
	low                         float64
	gains                       [channelCount][2]float64
	sends                       [channelCount]float64
}

func NewPCMStream(doc []byte) (*PCMStream, error) {
	s, t, err := playerTune(doc)
	if err != nil {
		return nil, err
	}
	p := &PCMStream{tune: t, final: -1, Info: PlayerInfo{Name: s.Name, SampleRate: sampleRate, Mastering: s.Mastering != nil, Scat: s.Scat != nil}}
	acc := 0
	if inKey(sharpKeys, t.key) {
		acc = 1
	} else if inKey(flatKeys, t.key) {
		acc = -1
	}
	ref := -1
	for i, ch := range t.channels() {
		for _, n := range ch {
			for j, v := range n.pitch {
				if !n.explicit[j] && inNote(tuneKey[t.key], v) {
					n.accidental[j] += acc
				}
			}
		}
		prepareStringPerformance(ch)
		nominal := channelNominal(ch)
		if len(ch) > 0 {
			if ref >= 0 && absInt(nominal-ref) >= tolerance {
				return nil, fmt.Errorf("channel lengths are different")
			}
			ref = nominal
		}
		p.nominal = max(p.nominal, nominal)
		written := 0
		for _, n := range ch {
			written += sampleCount(n.length)
		}
		p.channels[i] = streamChannel{cursor: channelCursor{notes: ch}, end: max(nominal, written), external: t.external[i]}
		if t.external[i] != nil {
			p.channels[i].end = max(nominal, len(t.external[i]))
			p.channels[i].cursor.index = len(ch)
		}
		if t.pan != nil {
			a := (t.pan[i] + 1) * math.Pi / 4
			p.gains[i] = [2]float64{math.Cos(a) * math.Sqrt2, math.Sin(a) * math.Sqrt2}
		} else {
			switch i {
			case 0:
				p.gains[i] = [2]float64{1, 0}
				if len(t.ch2) == 0 {
					p.gains[i][1] = 1
				}
			case 1:
				p.gains[i] = [2]float64{0, 1}
				if len(t.ch1) == 0 {
					p.gains[i][0] = 1
				}
			default:
				p.gains[i] = [2]float64{1, 1}
			}
		}
		p.sends[i] = 1
		if t.room != nil {
			if v, ok := t.room.Sends[channelNames[i]]; ok {
				p.sends[i] = v
			}
		}
	}
	if t.reverb > 0 {
		p.left, p.right = newRoom(0), newRoom(stereoSpread)
		p.tail = sampleCount(reverbTail)
		if t.room != nil {
			p.tail = sampleCount(1.5)
			p.delay = make([]float64, sampleCount(.06))
			for _, r := range []*room{p.left, p.right} {
				for j := range r.combs {
					c := &r.combs[j]
					c.buf = make([]float64, len(c.buf)*2/3)
					c.feedback = .67
					c.damping = .48
				}
			}
			active := false
			for i, ch := range t.channels() {
				active = active || (len(ch) > 0 && p.sends[i] > 0)
			}
			if !active {
				p.tail = 0
				p.left = nil
				p.right = nil
			}
		}
	}
	p.Info.Duration = float64(p.nominal) / sampleRate
	return p, nil
}
func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
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
		dl, dr, in := 0, 0, 0.
		for i := range p.channels {
			x := p.channels[i].sample(at, p.tune.mix[channelNames[i]])
			dl += int(float64(x) * p.gains[i][0])
			dr += int(float64(x) * p.gains[i][1])
			in += float64(x) * p.sends[i]
		}
		l, r := dl, dr
		if p.left != nil {
			if p.delay == nil {
				in = float64(dl+dr) * roomInput
				l = int(float64(dl) + p.tune.reverb*3*p.left.process(in))
				r = int(float64(dr) + p.tune.reverb*3*p.right.process(in))
			} else {
				p.low += .018 * (in - p.low)
				in -= p.low
				p.delay[p.delayAt] = in
				tap := func(t float64) float64 { return p.delay[(p.delayAt-sampleCount(t)+len(p.delay))%len(p.delay)] }
				earlyL, earlyR := .13*tap(.013)+.07*tap(.031), .12*tap(.019)+.08*tap(.043)
				diffuse := tap(.011) * roomInput
				l += int(p.tune.reverb * (earlyL + 2*p.left.process(diffuse)))
				r += int(p.tune.reverb * (earlyR + 2*p.right.process(diffuse)))
				p.delayAt = (p.delayAt + 1) % len(p.delay)
			}
		}
		out = append(out, clamp(l), clamp(r))
	}
	p.frame = end
	for i := range p.channels {
		c := &p.channels[i]
		keep := c.pending[:0]
		for _, e := range c.pending {
			if e.at+len(e.data) > end {
				keep = append(keep, e)
			}
		}
		clear(c.pending[len(keep):])
		c.pending = keep
	}
	return out, p.final >= 0 && p.frame >= p.final, nil
}

package muse

import "math"

// Pedal/let-ring sustain ends at the next control or instrument change,
// including controls on rests. Each released note is capped at eight seconds
// of extra natural decay. Re-striking a guitar pitch damps its old voice.
func prepareStringPerformance(notes []note) {
	starts := noteStarts(notes)
	boundary := 0.0
	if len(notes) > 0 {
		boundary = starts[len(notes)-1] + heldTime(notes[len(notes)-1])
	}
	// the sustain of the last note runs to the end of the channel
	starts = append(starts, boundary)
	mode := ""
	nextPitch := map[int]float64{}
	for i := len(notes) - 1; i >= 0; i-- {
		n := &notes[i]
		current := ""
		if n.performance != nil {
			if n.instrumentName == "piano" && n.performance.Pedal == "down" {
				current = "pedal"
			}
			if pluckedInstrument(n.instrumentName) && n.performance.Articulation == "let-ring" {
				current = "ring"
			}
		}
		if n.gesture != nil && n.gesture.Kind == "mute" {
			current = ""
		}
		if current != mode {
			boundary = starts[i+1]
			nextPitch = map[int]float64{}
		}
		mode = current
		if current == "" {
			continue
		}
		n.sustainHold = clampFloat(boundary-starts[i], heldTime(*n), heldTime(*n)+8)
		if current == "ring" {
			n.stopPitches = map[int]float64{}
			for j, p := range n.pitch {
				pitch := p + n.accidental[j]
				if stop, ok := nextPitch[pitch]; ok {
					n.stopPitches[pitch] = stop - starts[i]
				}
				nextPitch[pitch] = starts[i]
			}
		}
	}
}

func renderStringPerformance(n note, stroke int) []int {
	p := n.performance
	hold := heldTime(n)
	ring := math.Max(hold, n.sustainHold)
	release := 0.12
	if pluckedInstrument(n.instrumentName) {
		release = 0.18
	}
	if pluckedInstrument(n.instrumentName) && p.Articulation == "palm-mute" {
		ring = math.Min(hold, 0.12)
		release = 0.045
	}
	order := pitchOrder(n)
	spread := 0.0
	if pluckedInstrument(n.instrumentName) && len(n.pitch) > 1 && p.Strum != "" {
		total := p.StrumMS / 1000
		if total == 0 {
			total = 0.04
		}
		total = math.Min(total, hold/2)
		spread = total / float64(len(n.pitch)-1)
		if p.Strum == "up" || (p.Strum == "alternate" && stroke%2 == 1) {
			for i := range order {
				order[i] = len(order) - 1 - order[i]
			}
		}
	}
	gain := n.effectiveGain()
	out := make([]int, sampleCount(ring+release+spread*float64(len(n.pitch)-1)))
	for j, pitch := range n.pitch {
		pitch += n.accidental[j]
		lead := sampleCount(float64(order[j]) * spread)
		length := ring
		if stop, ok := n.stopPitches[pitch]; ok {
			length = math.Min(length, math.Max(0, stop-float64(lead)/sampleRate))
		}
		f := frequency(pitch)
		v := playingVelocity(n.velocity)
		if pluckedInstrument(n.instrumentName) && p.Articulation == "palm-mute" {
			v *= 0.65
		}
		samples := n.velocityIns(f, length+release, v)
		if n.instrumentName == "piano" && p.Pedal == "down" {
			addPedalResonance(samples, f)
		}
		dampAfterHold(samples, length)
		if pluckedInstrument(n.instrumentName) && p.Articulation == "palm-mute" {
			for k := range samples {
				samples[k] *= math.Exp(-float64(k) / sampleRate * 12)
			}
		}
		for k, x := range samples {
			if lead+k < len(out) {
				out[lead+k] += int(x * float64(n.vol) * gain)
			}
		}
	}
	return out
}

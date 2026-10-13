package muse

import "math"

// pluckedVoice keeps the ringing string's partial phases and decay through a
// slide or hammer-on. New picked notes excite a new string; a hammer-on adds
// a small impulse without repeating the pick transient.
type pluckedVoice struct {
	phase, energy [33]float64
	freq, age     float64
	active        bool
}

func pluckPartials(name string, f, v float64) (levels, rates [33]float64, stiff float64) {
	position, cutoff := .2, 1800+6000*v*v
	stiff = .0001
	switch name {
	case "nylon-guitar":
		position, cutoff, stiff = .19, 1000+3500*v*v, .000006
	case "electric-guitar":
		position, cutoff, stiff = .17, 2400+2100*v, 0
	case "bass":
		position, cutoff, stiff = .22, 350+1000*v*v, .0004
	case "upright-bass":
		position, cutoff, stiff = .18, 550+1500*v*v, .00008
	}
	total := 0.0
	for h := 1; h < len(levels); h++ {
		hz := f * float64(h) * math.Sqrt(1+stiff*float64(h*h))
		body := 1.0
		if name == "guitar" || name == "nylon-guitar" {
			body = resonanceGain([]formant{{110, 100, .35}, {220, 180, .25}, {650, 600, .15}}, .65, hz)
		}
		if name == "upright-bass" {
			body = resonanceGain([]formant{{95, 80, .7}, {210, 150, .45}, {650, 500, .2}}, .6, hz)
		}
		levels[h] = math.Sin(math.Pi*float64(h)*position) / float64(h) * body / (1 + math.Pow(hz/cutoff, 2))
		rates[h] = loss(hz, .5, 2, 2)
		switch name {
		case "nylon-guitar":
			rates[h] = loss(hz, .65, 1.1, .45)
			levels[h] /= math.Pow(float64(h), .3)
		case "bass":
			rates[h] = loss(hz, .35, 1, 1)
		case "upright-bass":
			rates[h] = loss(hz, .9, 1.8, .8) * 1.6
		case "electric-guitar":
			rates[h] = .65 + .10*float64(h)
		}
		total += math.Abs(levels[h])
	}
	if total > 0 {
		for h := 1; h < len(levels); h++ {
			levels[h] /= total
		}
	}
	return
}
func (v *pluckedVoice) render(n note, hold float64, join bool) []int {
	f := frequency(n.pitch[0] + n.accidental[0])
	velocity := playingVelocity(n.velocity)
	levels, rates, stiff := pluckPartials(n.instrumentName, f, velocity)
	connected := v.active && n.gesture.transition()
	from := f
	if connected {
		from = v.freq
		if n.gesture.Kind == "hammer-on" {
			for h := 1; h < len(levels); h++ {
				v.energy[h] += .12 * levels[h]
			}
		}
	} else {
		*v = pluckedVoice{freq: f, energy: levels, active: true}
	}
	sounding, release := hold, .1
	muted := n.gesture != nil && n.gesture.Kind == "mute"
	if n.performance.Articulation == "palm-mute" {
		muted = true
	}
	if muted {
		sounding = math.Min(hold, .12)
		release = .035
		join = false
	}
	duration := sounding + release
	if join {
		duration = hold
	}
	out := make([]int, sampleCount(duration))
	gain := n.gain
	if gain == 0 {
		gain = 1
	}
	for i := range out {
		t := float64(i) / sampleRate
		if connected {
			v.freq = glideFrequency(from, f, t, n.gesture.seconds(hold))
		}
		sum := 0.0
		for h := 1; h < len(levels); h++ {
			hz := v.freq * float64(h) * math.Sqrt(1+stiff*float64(h*h))
			v.phase[h] = math.Mod(v.phase[h]+2*math.Pi*hz/sampleRate, 2*math.Pi)
			if hz < maxPartialFreq {
				sum += v.energy[h] * math.Sin(v.phase[h])
			}
			rate := rates[h]
			if muted {
				rate += 14
			}
			v.energy[h] *= math.Exp(-rate / sampleRate)
		}
		env := math.Min(1, v.age/.002)
		if !join && t >= sounding {
			env *= .5 + .5*math.Cos(math.Pi*math.Min(1, (t-sounding)/release))
		}
		out[i] = int(sum * env * float64(n.vol) * gain)
		v.age += 1.0 / sampleRate
	}
	if !join {
		v.active = false
		if len(out) > 0 {
			out[len(out)-1] = 0
		}
	}
	return out
}

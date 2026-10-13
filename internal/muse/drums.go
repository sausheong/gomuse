package muse

import (
	"fmt"
	"math"
	"math/rand/v2"
	"sort"
	"strconv"
	"strings"
)

// Drums: a drum kit, played in a section's drums part. A drum note names
// the drums struck together instead of pitches, e.g. "kick-hat" or
// "0.5:snare", and z is a rest, as in the other channels.
//
// Every drum is synthesised, not sampled: a kick is a sine dropping
// quickly in pitch, a snare a short tuned body plus bright noise for the
// wires, cymbals noise and clashing metallic tones. Each rings for its own
// natural length, whatever the written note length, so a crash still washes
// on after a short note. Every drum is rendered once, in a few slightly
// different variants that the hits cycle through, so a run of hi-hats
// doesn't sound like the same recording repeated.

// a drum is one piece of the kit
type drum struct {
	level   float64 // loudness within the kit, so it's balanced as a drummer would play it
	length  float64 // seconds it rings for
	render  func(rng *rand.Rand, n int) []float64
	samples [drumVariants][]float64 // rendered, each peaking at 1
}

const drumVariants = 4

var drumKit = map[string]*drum{
	"shaker":     {level: .35, length: .25, render: shaker},
	"tambourine": {level: .4, length: .7, render: tambourine},
	"conga":      {level: .7, length: .65, render: handDrum(190)},
	"bongo":      {level: .6, length: .4, render: handDrum(340)},
	"claves":     {level: .45, length: .15, render: claves},
	"triangle":   {level: .3, length: 2.5, render: triangleHit},
	"brushtap":   {level: .6, length: .4, render: brushTap},
	"brushsweep": {level: .5, length: 1, render: brushSweep},
	"kick":       {level: 1, length: 0.6, render: kick},
	"snare":      {level: 0.85, length: 0.35, render: snare},
	"hat":        {level: 0.4, length: 0.15, render: hat(0.03)},
	"openhat":    {level: 0.45, length: 0.8, render: hat(0.3)},
	"crash":      {level: 0.5, length: 3, render: crash},
	"ride":       {level: 0.4, length: 2.5, render: ride},
	"tom1":       {level: 0.8, length: 0.7, render: tom(200)},
	"tom2":       {level: 0.8, length: 0.7, render: tom(150)},
	"tom3":       {level: 0.8, length: 0.8, render: tom(105)},
	"clap":       {level: 0.7, length: 0.35, render: clap},
	"rim":        {level: 0.6, length: 0.08, render: rim},
	"cowbell":    {level: 0.5, length: 0.5, render: cowbell},
}

// drumAliases are the short names a drum part can use
var drumAliases = map[string]string{
	"k": "kick", "bd": "kick",
	"s": "snare", "sd": "snare",
	"h": "hat", "hh": "hat",
	"o": "openhat", "oh": "openhat",
	"c": "crash", "cy": "crash",
	"r": "ride", "rd": "ride",
	"cp": "clap", "rs": "rim", "cb": "cowbell",
}

func init() {
	for name, d := range drumKit {
		n := sampleCount(d.length)
		for v := range drumVariants {
			rng := rand.New(rand.NewPCG(uint64(len(name))*7919+uint64(v), 0xd2))
			x := d.render(rng, n)
			// A sub-millisecond contact onset removes arbitrary sample jumps.
			for i := 0; i < min(len(x), sampleCount(0.0003)); i++ {
				x[i] *= float64(i) / float64(sampleCount(0.0003))
			}
			normalise(x)
			fadeTail(x, 0.01)
			d.samples[v] = x
		}
	}
}

// makeDrumNote makes a drum note from a drums entry such as "kick-hat" or
// "0.5:snare"; z is a rest
func makeDrumNote(noteString string, section Section, score Score) (n note, err error) {
	length := section.Length
	if length == 0 {
		length = score.Length
	}
	vol := section.Volume
	if vol == 0 {
		vol = score.Volume
	}
	n = note{length: length, vol: vol, gain: 1}

	l, hits := 1.0, noteString
	if prefix, rest, ok := strings.Cut(noteString, ":"); ok {
		if l, err = strconv.ParseFloat(prefix, 64); err != nil {
			return n, fmt.Errorf("cannot parse drum note duration - %s > %v", prefix, err)
		}
		hits = rest
	}
	n.length = l * length
	if math.IsNaN(n.length) || math.IsInf(n.length, 0) || n.length <= 0 {
		return n, fmt.Errorf("invalid drum note duration - %v ", n.length)
	}
	if hits == "z" {
		return n, nil
	}
	names := strings.Split(hits, "-")
	if len(names) > maxChordPitches {
		return n, fmt.Errorf("too many drums struck at once - %d, max is %d ", len(names), maxChordPitches)
	}
	for _, h := range names {
		name := strings.ToLower(strings.TrimSpace(h))
		if a, ok := drumAliases[name]; ok {
			name = a
		}
		if _, ok := drumKit[name]; !ok {
			return n, fmt.Errorf("drum doesn't exist - %s (use one of %s) ", h, drumNames())
		}
		n.kit = append(n.kit, name)
	}
	return n, nil
}

// drumNames lists the kit, for error messages
func drumNames() string {
	names := make([]string, 0, len(drumKit))
	for name := range drumKit {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

// kitData renders a drum note: every drum struck, each at its level in the
// kit, added together. The variant follows the note's position so that
// repeated hits differ slightly.
func (n note) kitData(vol int) []int {
	longest := 0
	for _, name := range n.kit {
		longest = max(longest, drumNoteLength(n, name))
	}
	data := make([]int, longest)
	for _, name := range n.kit {
		d := drumKit[name]
		x := drumNoteSamples(n, name)
		// Softer strikes excite fewer upper modes. Filter each piece separately
		// so simultaneous kick/cymbal hits cannot colour one another. Unset
		// velocity retains the original kit, including plain-score playback.
		velocity := n.velocity
		state := 0.0
		cutoff := 2500 + 14500*velocity*velocity
		if name == "kick" || strings.HasPrefix(name, "tom") {
			cutoff = 700 + 10000*velocity*velocity
		}
		alpha := 1 - math.Exp(-2*math.Pi*cutoff/sampleRate)
		for k, v := range x {
			if velocity > 0 {
				state += alpha * (v - state)
				v = state * math.Min(1, float64(len(x)-1-k)/float64(sampleCount(.002)))
			}
			data[k] += int(float64(vol) * d.level * v)
		}
	}
	return data
}

// drumLength is how many samples a drum note's longest drum rings for
func drumLength(n note) int {
	longest := 0
	for _, name := range n.kit {
		longest = max(longest, drumNoteLength(n, name))
	}
	return longest
}

// -- the drums ---------------------------------------------------------

// kick is a sine that starts high and drops within a few tens of
// milliseconds to a low thump, plus a click from the beater
func kick(rng *rand.Rand, n int) []float64 {
	x := make([]float64, n)
	phase := 0.0
	tune := 0.99 + 0.02*rng.Float64()
	for k := range x {
		t := float64(k) / sampleRate
		f := tune * (48 + 100*math.Exp(-t/0.035))
		phase += 2 * math.Pi * f / sampleRate
		x[k] = (math.Sin(phase)+0.16*math.Sin(1.59*phase)*math.Exp(-t/0.07))*math.Exp(-t/0.28) + 0.2*(rng.Float64()*2-1)*math.Exp(-t/0.003)
	}
	return x
}

// snare is a short tuned body (the drum head) plus a burst of bright noise
// (the wires buzzing against the bottom head)
func snare(rng *rand.Rand, n int) []float64 {
	x := make([]float64, n)
	hp := highpass(1500)
	shell := bandpass(950, 1.3)
	for k := range x {
		t := float64(k) / sampleRate
		body := 0.6*math.Sin(2*math.Pi*190*t)*math.Exp(-t/0.05) + 0.3*math.Sin(2*math.Pi*330*t)*math.Exp(-t/0.04)
		wires := hp(rng.Float64()*2-1) * math.Exp(-t/0.13)
		x[k] = body + wires + 0.2*shell(rng.Float64()*2-1)*math.Exp(-t/0.045)
	}
	return x
}

// hat is a hi-hat: very bright noise, choked after tau seconds (closed) or
// left to ring (open)
func hat(tau float64) func(rng *rand.Rand, n int) []float64 {
	return func(rng *rand.Rand, n int) []float64 {
		x := make([]float64, n)
		hp1, hp2 := highpass(7000), highpass(7000)
		metal := cymbalModes(rng, n, tau, []float64{4100, 5380, 6720, 8190, 10330, 12710})
		for k := range x {
			t := float64(k) / sampleRate
			x[k] = 0.7*hp2(hp1(rng.Float64()*2-1))*math.Exp(-t/tau) + 0.22*metal[k]
		}
		return x
	}
}

// crash is a crash cymbal: bright noise with a long wash
func crash(rng *rand.Rand, n int) []float64 {
	x := make([]float64, n)
	hp1, hp2 := highpass(4000), highpass(4000)
	metal := cymbalModes(rng, n, 0.8, []float64{730, 1130, 1770, 2410, 3170, 4090, 5270, 6830, 8770, 11210, 14330})
	for k := range x {
		t := float64(k) / sampleRate
		x[k] = (0.75*hp2(hp1(rng.Float64()*2-1))*math.Exp(-t/1.1) + 0.32*metal[k]) * math.Min(1, t/0.002)
	}
	return x
}

// ride is a ride cymbal: a cluster of clashing metallic tones (a cymbal's
// overtones don't line up into a pitch) over a soft wash of noise
func ride(rng *rand.Rand, n int) []float64 {
	x := make([]float64, n)
	tones := []float64{1730, 2290, 2950, 3540, 4210, 5330, 6720, 8310}
	phases := make([]float64, len(tones))
	for i := range phases {
		phases[i] = rng.Float64() * 2 * math.Pi
	}
	hp := highpass(5000)
	for k := range x {
		t := float64(k) / sampleRate
		ring := 0.0
		for i, f := range tones {
			ring += math.Sin(2*math.Pi*f*t + phases[i])
		}
		x[k] = ring/float64(len(tones))*math.Exp(-t/1.2) + 0.5*hp(rng.Float64()*2-1)*math.Exp(-t/0.6)
	}
	return x
}

// tom is a tom-tom tuned to base Hz: a sine that bends down onto its pitch
// as the head settles after the stick
func tom(base float64) func(rng *rand.Rand, n int) []float64 {
	return func(rng *rand.Rand, n int) []float64 {
		x := make([]float64, n)
		phase := 0.0
		for k := range x {
			t := float64(k) / sampleRate
			phase += 2 * math.Pi * base * (1 + 0.35*math.Exp(-t/0.06)) / sampleRate
			x[k] = (math.Sin(phase)+0.28*math.Sin(1.59*phase)*math.Exp(-t/0.14)+0.12*math.Sin(2.14*phase)*math.Exp(-t/0.08))*math.Exp(-t/0.35) + 0.1*(rng.Float64()*2-1)*math.Exp(-t/0.01)
		}
		return x
	}
}

// clap is a hand clap: a few hands landing a hair apart (three quick bursts
// of noise), then a short tail, all in the middle of the range
func clap(rng *rand.Rand, n int) []float64 {
	x := make([]float64, n)
	bp := bandpass(1200, 1.2)
	for k := range x {
		t := float64(k) / sampleRate
		env := 0.0
		for _, at := range []float64{0, 0.011, 0.022} {
			if t >= at {
				env = math.Max(env, math.Exp(-(t-at)/0.004))
			}
		}
		if t >= 0.03 {
			env = math.Max(env, 0.6*math.Exp(-(t-0.03)/0.12))
		}
		x[k] = bp(rng.Float64()*2-1) * env
	}
	return x
}

// rim is a side stick: the stick laid across the head clicking on the rim
func rim(rng *rand.Rand, n int) []float64 {
	x := make([]float64, n)
	bp := bandpass(2500, 2)
	for k := range x {
		t := float64(k) / sampleRate
		x[k] = math.Sin(2*math.Pi*1700*t)*math.Exp(-t/0.008) + 0.5*bp(rng.Float64()*2-1)*math.Exp(-t/0.006)
	}
	return x
}

// cowbell is two clashing square-ish tones, band-passed to its hollow clank
func cowbell(rng *rand.Rand, n int) []float64 {
	x := make([]float64, n)
	bp := bandpass(900, 1.5)
	for k := range x {
		t := float64(k) / sampleRate
		tone := 0.0
		for _, f := range []float64{545, 815} {
			for h := 1.0; h <= 9 && h*f < maxPartialFreq; h += 2 {
				tone += math.Sin(2*math.Pi*f*h*t) / (h * h)
			}
		}
		x[k] = bp(tone) * math.Exp(-t/0.12)
	}
	return x
}

// -- small filters -----------------------------------------------------

// highpass is a one-pole high-pass filter: it lets through what's above
// cutoff Hz and thins out what's below
func highpass(cutoff float64) func(float64) float64 {
	rc := 1 / (2 * math.Pi * cutoff)
	a := rc / (rc + 1/sampleRate)
	var y, prev float64
	return func(x float64) float64 {
		y = a * (y + x - prev)
		prev = x
		return y
	}
}

// bandpass is a two-pole band-pass filter (an RBJ biquad) centred on
// centre Hz, with quality q (higher is narrower)
func bandpass(centre, q float64) func(float64) float64 {
	w := 2 * math.Pi * centre / sampleRate
	alpha := math.Sin(w) / (2 * q)
	a0 := 1 + alpha
	b0, b2 := alpha/a0, -alpha/a0
	a1, a2 := -2*math.Cos(w)/a0, (1-alpha)/a0
	var x1, x2, y1, y2 float64
	return func(x float64) float64 {
		y := b0*x + b2*x2 - a1*y1 - a2*y2
		x2, x1 = x1, x
		y2, y1 = y1, y
		return y
	}
}

// normalise scales x to peak at 1
func normalise(x []float64) {
	peak := 0.0
	for _, v := range x {
		peak = math.Max(peak, math.Abs(v))
	}
	if peak > 0 {
		for k := range x {
			x[k] /= peak
		}
	}
}

// fadeTail fades the last r seconds of x to silence, so a drum that hasn't
// quite died away by the end of its buffer doesn't click
func fadeTail(x []float64, r float64) {
	m := min(len(x), sampleCount(r))
	for k := 0; k < m; k++ {
		x[len(x)-1-k] *= float64(k) / float64(m)
	}
}

// Damped, inharmonic modes give cymbals a metal body beneath their noise.
// Higher modes lose energy faster; tiny seeded offsets vary each strike.
func cymbalModes(rng *rand.Rand, n int, tau float64, frequencies []float64) []float64 {
	ps := make([]partial, 0, len(frequencies))
	for i, f := range frequencies {
		ps = append(ps, partial{f * (0.995 + 0.01*rng.Float64()), 1 / float64(1+i/3), [][2]float64{{1, (1 + f/12000) / tau}}})
	}
	return additive(n, ps, 0.0003, 0.01)
}

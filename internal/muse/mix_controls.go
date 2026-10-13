package muse

import (
	"fmt"
	"math"
	"strings"
	"sync"
)

// ChannelMix applies to the continuous channel signal before dry panning and
// room sends. Values never change note placement or independently normalise notes.
type ChannelMix struct {
	GainDB    float64 `yaml:"gain_db,omitempty"`
	LowCutHz  float64 `yaml:"low_cut_hz,omitempty"`
	HighCutHz float64 `yaml:"high_cut_hz,omitempty"`
	TiltDB    float64 `yaml:"tilt_db,omitempty"`
}

func validateMix(s *Score) error {
	for ch, m := range s.Mix {
		if !mixChannels[ch] || !finiteRange(m.GainDB, -12, 12) || !finiteRange(m.LowCutHz, 0, 1000) || !finiteRange(m.HighCutHz, 0, 18000) || !finiteRange(m.TiltDB, -6, 6) {
			return fmt.Errorf("invalid mix channel or gain/filter/tilt range")
		}
		if m.LowCutHz > 0 && m.HighCutHz > 0 && m.HighCutHz <= m.LowCutHz {
			return fmt.Errorf("high_cut_hz must exceed low_cut_hz")
		}
	}
	return nil
}
func processChannelMix(x []int, m ChannelMix) {
	if m == (ChannelMix{}) {
		return
	}
	hp, lp, split := 0.0, 0.0, 0.0
	ah := 1 - math.Exp(-2*math.Pi*m.LowCutHz/sampleRate)
	al := 1 - math.Exp(-2*math.Pi*m.HighCutHz/sampleRate)
	as := 1 - math.Exp(-2*math.Pi*1000/sampleRate)
	gain := dBGain(m.GainDB)
	lowGain, highGain := dBGain(-m.TiltDB/2), dBGain(m.TiltDB/2)
	for i, n := range x {
		y := float64(n)
		if m.LowCutHz > 0 {
			hp += ah * (y - hp)
			y -= hp
		}
		if m.HighCutHz > 0 {
			lp += al * (y - lp)
			y = lp
		}
		if m.TiltDB != 0 {
			split += as * (y - split)
			y = split*lowGain + (y-split)*highGain
		}
		// Remove residual filter state smoothly at the existing channel ending.
		if m.LowCutHz > 0 || m.HighCutHz > 0 || m.TiltDB != 0 {
			y *= math.Min(1, float64(len(x)-1-i)/math.Min(float64(sampleCount(.01)), float64(max(1, len(x)-1))))
		}
		x[i] = int(math.Round(y * gain))
	}
}

var calibrationCache sync.Map

func instrumentCalibration(n note) float64 {
	if n.instrumentName == "voice" || strings.HasPrefix(n.instrumentName, "scat-") || n.instrumentName == "" {
		return 1
	}
	mode := "legacy"
	if n.performance != nil && sustainedInstrument(n.instrumentName) {
		mode = "sustain"
	}
	if n.pluckPhrase {
		mode = "pluck"
	}
	key := n.instrumentName + ":" + mode
	if v, ok := calibrationCache.Load(key); ok {
		return v.(float64)
	}
	f := 261.625565
	switch n.instrumentName {
	case "bass", "upright-bass", "synth-bass":
		f = 110
	case "flute", "violin":
		f = 440
	case "cello":
		f = 196
	}
	var samples []float64
	if mode == "legacy" {
		if fn, ok := velocityInstruments[n.instrumentName]; ok {
			samples = fn(f, .6, .7)
		} else {
			return 1
		}
	} else {
		// Use one representative moderate note for each renderer, never the current
		// phrase's audio. The same bounded correction then applies to every note.
		pitchNumber := int(math.Round(12*math.Log2(f/440))) + pitch["a4"]
		ref := note{instrumentName: n.instrumentName, pitch: []int{pitchNumber}, accidental: []int{0}, vol: 100000, velocity: .7, gain: 1, performance: &Performance{Articulation: "legato"}}
		var ints []int
		if mode == "sustain" {
			var v phraseVoice
			ints = v.render(ref, .6, false)
		} else {
			ref.performance.Articulation = "picked"
			var v pluckedVoice
			ints = v.render(ref, .6, false)
		}
		samples = make([]float64, len(ints))
		for i, v := range ints {
			samples[i] = float64(v) / 100000
		}
	}
	sum := 0.
	for _, x := range samples[:min(len(samples), sampleCount(.6))] {
		sum += x * x
	}
	rms := math.Sqrt(sum / float64(min(len(samples), sampleCount(.6))))
	gain := 1.
	if rms > 0 {
		gain = math.Max(.5, math.Min(2, .18/rms))
	}
	calibrationCache.Store(key, gain)
	return gain
}
func applyCalibration(s *Score, t *tune) {
	for c, notes := range t.channels() {
		for i := range notes {
			n := &notes[i]
			if level, ok := s.Levels[channelNames[c]]; ok && level == 0 {
				n.vol = 0
			}
			if s.Calibration && c != drumChannel {
				g := n.gain
				if g == 0 {
					g = 1
				}
				n.gain = g * instrumentCalibration(*n)
			}
		}
	}
}

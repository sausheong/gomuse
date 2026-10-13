package muse

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type stereoStem struct{ left, right []int }
type renderCapture struct {
	stems             map[string]stereoStem
	dryLeft, dryRight []int
	gain              float64
}

func (c *renderCapture) dry(cs [channelCount][]int, pan []float64, l, r []int) {
	c.stems = map[string]stereoStem{}
	c.dryLeft, c.dryRight = l, r
	for i, x := range cs {
		if len(x) == 0 {
			continue
		}
		sl, sr := make([]int, len(l)), make([]int, len(r))
		if pan != nil {
			isolated := [channelCount][]int{}
			isolated[i] = x
			sl, sr, _ = mixPanned(isolated, pan)
		} else {
			for k, v := range x {
				switch i {
				case 0:
					sl[k] = v
					if len(cs[1]) == 0 {
						sr[k] = v
					}
				case 1:
					sr[k] = v
				default:
					sl[k], sr[k] = v, v
				}
			}
		}
		c.stems[channelNames[i]] = stereoStem{sl, sr}
	}
}
func (c *renderCapture) finish(l, r []int, gain float64) float64 {
	wl, wr := append([]int(nil), l...), append([]int(nil), r...)
	for i := range c.dryLeft {
		wl[i] -= c.dryLeft[i]
		wr[i] -= c.dryRight[i]
	}
	c.stems["room"] = stereoStem{wl, wr}
	peak := 0.0
	for _, stem := range c.stems {
		for _, x := range [][]int{stem.left, stem.right} {
			for _, v := range x {
				peak = math.Max(peak, math.Abs(float64(v)))
			}
		}
	}
	if peak > 0 {
		gain = math.Min(gain, 32767*dBGain(-.1)/peak)
	}
	for name, stem := range c.stems {
		stem.left = append(stem.left, make([]int, len(l)-len(stem.left))...)
		stem.right = append(stem.right, make([]int, len(r)-len(stem.right))...)
		scaleStereo(stem.left, stem.right, gain)
		c.stems[name] = stem
	}
	c.gain = gain
	return gain
}

// RenderBundle renders a procedural score once, capturing aligned dry panned
// stems and the shared room return. One gain applies to the master and all stems.
// Existing output directories are never overwritten. Vocal backends are excluded.
func RenderBundle(doc []byte, directory string) error {
	if _, err := os.Stat(directory); !os.IsNotExist(err) {
		return fmt.Errorf("output directory must not already exist")
	}
	var s Score
	if err := yaml.Unmarshal(doc, &s); err != nil {
		return err
	}
	if s.Singing != nil || s.Scat != nil {
		return fmt.Errorf("render bundles currently support instrumental scores only")
	}
	t, err := buildTune(&s)
	if err != nil {
		return err
	}
	capture := &renderCapture{}
	t.capture = capture
	audio, err := t.encode()
	if err != nil {
		return err
	}
	if err = os.Mkdir(directory, 0755); err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(directory, "score.yaml"), doc, 0644); err != nil {
		return err
	}
	if err = writeWAV(filepath.Join(directory, "master"), audio); err != nil {
		return err
	}
	for name, stem := range capture.stems {
		if err = writeWAV(filepath.Join(directory, name), interleave(stem.left, stem.right)); err != nil {
			return err
		}
	}
	info := map[string]any{"name": s.Name, "sample_rate": sampleRate, "frames": len(audio) / 2, "gain": capture.gain, "stems": "dry panned C1/C2/C3/drums plus shared room; all tracks use the same gain", "rounding": "summed stems may differ from master by a few 16-bit units due to rounding"}
	encoded, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(directory, "manifest.json"), append(encoded, '\n'), 0644)
}

package muse

import (
	"fmt"
	"math"
	"os"

	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
)

const (
	sampleRate     = 44100
	bitDepth       = 16
	numChannels    = 2
	wavAudioFormat = 1

	// C1 and C2 are independent melodic lines: even in a well-formed score
	// their notes don't always add up to exactly the same total duration
	// (different rhythms, fractional note lengths that don't sum evenly),
	// and each note's length is now rounded to the nearest whole sample
	// (see sampleCount) rather than drifting through float addition. Across
	// a whole tune that leaves a gap of at most a few hundred samples in
	// the real scores shipped in scores/*.yaml (well under a tenth of a
	// second), which this tolerance absorbs by cropping the longer channel.
	// Anything bigger than that is a genuine mismatch and is still reported
	// as an error below.
	tolerance = 1500

	minSample = -32768
	maxSample = 32767
)

// write data to WAV file
func writeWAV(name string, data []int) (err error) {
	out, err := os.Create(name + ".wav")
	if err != nil {
		return fmt.Errorf("couldn't create wav file - %w", err)
	}
	defer out.Close()

	enc := wav.NewEncoder(out, sampleRate, bitDepth, numChannels, wavAudioFormat)
	buf := &audio.IntBuffer{
		Format: &audio.Format{
			NumChannels: numChannels,
			SampleRate:  sampleRate,
		},
		SourceBitDepth: bitDepth,
		Data:           data,
	}
	if err = enc.Write(buf); err != nil {
		return fmt.Errorf("couldn't write to encoder - %w", err)
	}
	if err = enc.Close(); err != nil {
		return fmt.Errorf("couldn't close encoder - %w", err)
	}
	return nil
}

// The channels are mixed with mix or mixPanned, and every sample is
// hard-clamped to the 16-bit signed range once, by interleave, after the
// channels have been fully summed - so chords or high volumes clip instead
// of wrapping around.

// mix lines up the channels and adds them into a left and right side: C1
// on the left, C2 on the right (or C1 again if there is no C2), and C3 on
// both.
func mix(c1, c2, c3 []int) (l, r []int, err error) {
	// if there is only 1 channel, duplicate the other one
	if len(c2) == 0 {
		c2 = c1
	}
	// if the channel lengths are within a tolerance, crop the longer
	// array so that both arrays are the same
	d1 := len(c1) - len(c2)
	d2 := len(c2) - len(c1)
	if d1 > 0 && d1 < tolerance {
		c1 = c1[:len(c2)]
	}
	if d2 > 0 && d2 < tolerance {
		c2 = c2[:len(c1)]
	}

	if len(c1) != len(c2) {
		err = fmt.Errorf("channel lengths are different - C1: %d C2: %d", len(c1), len(c2))
		return
	}

	// the centre channel gets the same tolerance against C1
	if len(c3) > 0 {
		d := len(c3) - len(c1)
		if d > 0 && d < tolerance {
			c3 = c3[:len(c1)]
		}
		if -d > 0 && -d < tolerance {
			c3 = append(c3, make([]int, -d)...)
		}
		if len(c3) != len(c1) {
			err = fmt.Errorf("channel lengths are different - C1: %d C3: %d", len(c1), len(c3))
			return
		}
	}

	l = make([]int, len(c1))
	r = make([]int, len(c1))
	for i := range c1 {
		l[i], r[i] = c1[i], c2[i]
		if len(c3) > 0 {
			l[i] += c3[i]
			r[i] += c3[i]
		}
	}
	return
}

// mixPanned adds the channels into a left and right side, placing each
// with an equal-power pan law: pan -1 is hard left, 0 centre, 1 hard
// right. The channels must already be the same length (or empty). If C2 is
// empty and C3 is too, C1 is played centred, as a single line.
func mixPanned(cs [channelCount][]int, pan []float64) (l, r []int, err error) {
	n := 0
	for _, c := range cs {
		n = max(n, len(c))
	}
	for i, c := range cs {
		if len(c) > 0 && len(c) != n {
			return nil, nil, fmt.Errorf("channel lengths are different - %s: %d, want %d", channelNames[i], len(c), n)
		}
	}
	l, r = make([]int, n), make([]int, n)
	for i, c := range cs {
		if len(c) == 0 {
			continue
		}
		p := 0.0
		if i < len(pan) {
			p = clampFloat(pan[i], -1, 1)
		}
		// equal power: both sides at 1/sqrt2 in the centre, scaled up by
		// sqrt2 so a centred channel keeps its level on each side
		a := (p + 1) * math.Pi / 4
		gl, gr := math.Cos(a)*math.Sqrt2, math.Sin(a)*math.Sqrt2
		for k, v := range c {
			l[k] += int(float64(v) * gl)
			r[k] += int(float64(v) * gr)
		}
	}
	return l, r, nil
}

// interleave clamps the two sides and interleaves them into stereo samples
func interleave(l, r []int) []int {
	data := make([]int, 0, len(l)*2)
	for i := range l {
		data = append(data, clamp(l[i]), clamp(r[i]))
	}
	return data
}

// clamp hard-limits a sample to the 16-bit signed range
func clamp(x int) int {
	if x > maxSample {
		return maxSample
	}
	if x < minSample {
		return minSample
	}
	return x
}

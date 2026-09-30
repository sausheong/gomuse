package muse

import (
	"fmt"
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
	defer out.Close()
	if err != nil {
		return fmt.Errorf("couldn't create wav file - %w", err)
	}

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

// make stereo channels for the WAV file. Every sample is hard-clamped to
// the 16-bit signed range here, once, after the two channels have been
// fully summed - this is the only place clipping is applied, so chords or
// high volumes can no longer wrap around instead of clipping.
func stereo(c1, c2 []int) (data []int, err error) {
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

	data = make([]int, 0, len(c1)*2)
	for i := range c1 {
		data = append(data, clamp(c1[i]), clamp(c2[i]))
	}
	return
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

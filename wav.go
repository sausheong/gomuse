package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
)

const (
	SampleRate     = 44100
	BitDepth       = 16
	NumChannels    = 2
	WavAudioFormat = 1
)

// write data to WAV file
func write(name string, data []int) (err error) {
	out, err := os.Create(name + ".wav")
	defer out.Close()
	if err != nil {
		fmt.Printf("couldn't create wav file - %v", err)
		return
	}
	enc := wav.NewEncoder(out, SampleRate, BitDepth, NumChannels, WavAudioFormat)
	buf := &audio.IntBuffer{
		Format: &audio.Format{
			NumChannels: NumChannels,
			SampleRate:  SampleRate,
		},
		SourceBitDepth: BitDepth,
		Data:           data,
	}
	if err = enc.Write(buf); err != nil {
		fmt.Printf("couldn't write to encoder - %v", err)
		return
	}
	if err = enc.Close(); err != nil {
		fmt.Printf("couldn't close encoder - %v", err)
		return
	}
	return
}

// make stereo channels for WAV file
func stereo(c1, c2 []int) (data []int, err error) {
	if len(c1) == len(c2) {
		for i := range c1 {
			data = append(data, c1[i], c2[i])
		}
	} else {
		fmt.Println(len(c1), len(c2))
		err = errors.New("Channel lengths are different")
	}
	return
}

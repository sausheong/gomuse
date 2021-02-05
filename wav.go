package main

import (
	"errors"
	"log"
	"os"

	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
)

const (
	sampleRate     = 44100
	bitDepth       = 16
	numChannels    = 2
	wavAudioFormat = 1
	tolerance      = 1500
	maxTuneLength  = 6000000
)

// write data to WAV file
func write(name string, data []int) (err error) {
	out, err := os.Create(name + ".wav")
	defer out.Close()
	if err != nil {
		log.Printf("couldn't create wav file - %v", err)
		return
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
		log.Printf("couldn't write to encoder - %v", err)
		return
	}
	if err = enc.Close(); err != nil {
		log.Printf("couldn't close encoder - %v", err)
		return
	}
	return
}

// make stereo channels for WAV file
func stereo(c1, c2 []int) (data []int, err error) {
	// sometimes if the tunes are long, they run the risk of being killed
	// if the server doesn't have enough memory. In this case, don't process
	// it in the web app. This should normally be ok for the command line tool
	if *serverFlag && len(c1) > maxTuneLength {
		err = errors.New("Tune too long, use the command line tool instead")
		return
	}
	// if there is only 1 channel, duplicate the other one
	if len(c2) == 0 {
		c2 = c1
	}
	// if the channels lengths are within a tolerance, crop the longer
	// array so that both arrays are the same
	d1 := len(c1) - len(c2)
	d2 := len(c2) - len(c1)
	if d1 > 0 && d1 < tolerance {
		c1 = c1[:len(c2)]
	}
	if d2 > 0 && d2 < tolerance {
		c2 = c2[:len(c1)]
	}

	if len(c1) == len(c2) {
		for i := range c1 {
			data = append(data, c1[i], c2[i])
		}
	} else {
		// if the channel lengths are too different, can't process
		log.Println("C1:", len(c1), "C2:", len(c2))
		err = errors.New("Channel lengths are different")
	}
	return
}

package main

import (
	"errors"
	"fmt"
	"math"
	"os"

	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
)

const (
	C              = -9
	D              = -7
	E              = -5
	F              = -4
	G              = -2
	A              = 0
	B              = 2
	c              = 3
	d              = 5
	e              = 7
	f              = 8
	g              = 10
	a              = 12
	b              = 14
	SampleRate     = 44100
	BitDepth       = 16
	NumChannels    = 2
	WavAudioFormat = 1
)

func main() {

	// cnote := note(p(C), 1, drop)
	anote := note(p(A), 1, curve)
	// bnote := note(p(B), 1, drop)

	// cmaj, err := concat(note(p(C), 0.5, drop), note(p(E), 0.5, drop), note(p(G), 0.5, drop))

	// if err != nil {
	// 	panic("Cannot create chord")
	// }

	// melody, err := chain(anote, bnote, cnote)
	// chords, err := chain(note(p(C), 0.5, drop), cmaj, note(p(C), 0.5, drop), cmaj, note(p(C), 0.5, drop), cmaj)
	data, _ := stereo(anote, anote)

	write("test", data)
}

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

func note(frequency float64, duration float64, env envelope) (data []int) {
	for i := 0.0; i < duration; i = i + (1.0 / float64(SampleRate)) {
		x := int(10000 * env(i, duration) * math.Sin(2*math.Pi*frequency*i))
		data = append(data, x)
	}
	return
}

func rest(duration float64) (data []int) {
	for i := 0.0; i < duration; i = i + (1.0 / float64(SampleRate)) {
		data = append(data, 0)
	}
	return
}

// chain notes together to create music!
func chain(notes ...[]int) (data []int, err error) {
	for _, note := range notes {
		data = append(data, note...)
	}
	return
}

// concatenate notes together to make chords
func concat(notes ...[]int) (data []int, err error) {
	// make sure all the notes are the same length
	l := len(notes[0])
	for _, note := range notes {
		if len(note) != l {
			err = errors.New("length of notes are not the same")
			return
		}
	}
	// add up all the notes
	for i := 0; i < l; i++ {
		d := 0
		for _, note := range notes {
			d += note[i]
		}
		data = append(data, d)
	}

	return
}

func stereo(c1, c2 []int) (data []int, err error) {
	if len(c1) == len(c2) {
		for i := range c1 {
			data = append(data, c1[i], c2[i])
		}
	} else {
		err = errors.New("Channel lengths are different")
	}
	return
}

// returns the pitch of the note
func p(step int) float64 {
	return 440.0 * (math.Pow(2, (float64(step) / 12.0)))
}

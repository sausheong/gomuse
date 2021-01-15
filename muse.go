package main

import (
	"errors"
	"math"
)

const (
	c2, d2, e2, f2, g2, a2, b2 = -33, -31, -29, -28, -26, -24, -22
	c3, d3, e3, f3, g3, a3, b3 = -21, -19, -17, -16, -14, -12, -10
	c4, d4, e4, f4, g4, a4, b4 = -9, -7, -5, -4, -2, 0, 2
	c5, d5, e5, f5, g5, a5, b5 = 3, 5, 6, 8, 10, 12, 14
	c6, d6, e6, f6, g6, a6, b6 = 15, 17, 19, 20, 22, 24, 26
)

var tuneKey map[string][]int
var sharpKeys []string
var flatKeys []string

func init() {
	tuneKey = make(map[string][]int)
	initializeTuneKeys()
}

func initializeTuneKeys() {
	tuneKey["C"] = []int{}
	sharpKeys = []string{"G", "D", "A", "E", "B", "F#", "C#"}
	sharpNotes := []int{f2, c2, g2, d2, a2, e2, b2}
	for i, key := range sharpKeys {
		k := []int{}
		for j := 0; j < i+1; j++ {
			for l := 1; l < 6; l++ {
				k = append(k, sharpNotes[j]+(12*l))
			}
		}
		tuneKey[key] = k
	}

	flatKeys = []string{"F", "Bb", "Eb", "Ab", "Db", "Gb", "Cb"}
	flatNotes := []int{b2, e2, a2, d2, g2, c2, f2}
	for i, key := range flatKeys {
		k := []int{}
		for j := 0; j < i+1; j++ {
			for l := 1; l < 6; l++ {
				k = append(k, flatNotes[j]+(12*l))
			}
		}
		tuneKey[key] = k
	}
}

func inKey(key []string, note string) bool {
	for _, item := range key {
		if item == note {
			return true
		}
	}
	return false
}

func inNote(notes []int, note int) bool {
	for _, item := range notes {
		if item == note {
			return true
		}
	}
	return false
}

// Tune represents a piece of music
type Tune struct {
	Key        string
	NoteLength float64
	Channel1   []Note
	Channel2   []Note
}

// Note represents a musical note
type Note struct {
	Pitch      []int // if it's a chord, there will be more than 1 pitch
	Accidental []int // 1 for sharp, -1 for flat, 0 for everything else
	Length     float64
	Envelope   envelope
}

// Encode converts the tune to []int data to be used to create a WAV file
func (t Tune) Encode() (data []int, err error) {
	// apply key
	accidental := 0
	if inKey(sharpKeys, t.Key) {
		accidental = 1
	} else if inKey(flatKeys, t.Key) {
		accidental = -1
	}

	channels := [][]Note{t.Channel1, t.Channel2}
	for _, channel := range channels {
		for _, note := range channel {
			for i, pitch := range note.Pitch {
				if inNote(tuneKey[t.Key], pitch) {
					note.Accidental[i] += accidental
				}
			}
		}
	}
	var channel1, channel2, n []int
	for _, note := range t.Channel1 {
		n, err = note.encode()
		if err != nil {
			return
		}
		channel1 = append(channel1, n...)
	}
	for _, note := range t.Channel2 {
		n, err = note.encode()
		if err != nil {
			return
		}
		channel2 = append(channel2, n...)
	}

	data, err = stereo(channel1, channel2)
	return
}

// encode the note
func (n Note) encode() (data []int, err error) {
	if len(n.Pitch) != len(n.Accidental) {
		err = errors.New("length of pitches and accidentals not the same")
		return
	}

	// this is a rest
	if len(n.Pitch) == 0 && n.Length != 0 {
		data = rest(n.Length)
		return
	}

	// encode into []int
	var notes [][]int
	for i := 0; i < len(n.Pitch); i++ {
		pitch := p(n.Pitch[i] + n.Accidental[i])
		notes = append(notes, note(pitch, n.Length, n.Envelope))
	}
	data, err = concat(notes...)
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

// returns the pitch of the note
func p(step int) float64 {
	return 440.0 * (math.Pow(2, (float64(step) / 12.0)))
}

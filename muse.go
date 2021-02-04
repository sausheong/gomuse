package main

import (
	"errors"
	"fmt"
	"math"
)

var pitch map[string]int // a list of all pitches
var tuneKey map[string][]int
var sharpKeys []string
var flatKeys []string

func init() {
	pitch = make(map[string]int)
	notes := []string{"c", "d", "e", "f", "g", "a", "b"}
	nums := []int{-57, -55, -53, -52, -50, -48, -46}
	for i := 1; i < 8; i++ {
		for j, note := range notes {
			nums[j] = nums[j] + 12
			pitch[fmt.Sprintf("%s%d", note, i)] = nums[j]
		}
	}
	tuneKey = make(map[string][]int)
	initTuneKeys()
}

// Note represents a musical note
type note struct {
	pitch      []int // if it's a chord, there will be more than 1 pitch
	accidental []int // 1 for sharp, -1 for flat, 0 for everything else
	length     float64
	env        envelope
	har        harmonic
	vol        int
}

// tune represents a piece of music
type tune struct {
	key    string
	length float64
	ch1    []note
	ch2    []note
}

// Encode converts the tune to []int data to be used to create a WAV file
func (t tune) encode() (data []int, err error) {
	fmt.Println("encode1")
	// apply key
	acc := 0
	if inKey(sharpKeys, t.key) { // if the key signature is a sharp key
		acc = 1
	} else if inKey(flatKeys, t.key) { // if the key signature is a flat key
		acc = -1
	}
	fmt.Println("encode2")
	channels := [][]note{t.ch1, t.ch2}

	for _, channel := range channels {
		for _, n := range channel {
			for i, pitch := range n.pitch {
				if inNote(tuneKey[t.key], pitch) {
					n.accidental[i] += acc
				}
			}
		}
	}
	fmt.Println("encode3")
	var c1, c2, n []int
	for _, note := range t.ch1 {
		n, err = note.encode()
		if err != nil {
			return
		}
		c1 = append(c1, n...)
	}
	fmt.Println("encode4")
	for _, note := range t.ch2 {
		n, err = note.encode()
		if err != nil {
			return
		}
		c2 = append(c2, n...)
	}
	fmt.Println("encode5")

	data, err = stereo(c1, c2)
	fmt.Println("encode6")
	return
}

// encode the note
func (n note) encode() (data []int, err error) {
	if len(n.pitch) != len(n.accidental) {
		err = errors.New("length of pitches and accidentals not the same")
		return
	}

	// this is a rest
	if len(n.pitch) == 0 && n.length != 0 {
		data = rest(n.length)
		return
	}

	// encode into []int
	var notes [][]int
	for i := 0; i < len(n.pitch); i++ {
		pitch := p(n.pitch[i] + n.accidental[i])
		notes = append(notes, noteData(pitch, n.length, n.env, n.har, n.vol))
	}
	data, err = concat(notes...)
	return
}

// actual note data
func noteData(frequency float64, duration float64, env envelope, har harmonic, vol int) (data []int) {
	for i := 0.0; i < duration; i = i + (1.0 / float64(SampleRate)) {
		x := int(float64(vol) * env(i, duration) * har(frequency*i))
		data = append(data, x)
	}
	return
}

// rest note
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

// initialise the tuneKey array, which is a
// used to apply the key signature to notes
func initTuneKeys() {
	tuneKey["C"] = []int{}
	sharpKeys = []string{"G", "D", "A", "E", "B", "F#", "C#"}
	sharpNotes := []int{pitch["f2"], pitch["c2"], pitch["g2"], pitch["d2"], pitch["a2"], pitch["e2"], pitch["b2"]}
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
	flatNotes := []int{pitch["b2"], pitch["e2"], pitch["a2"], pitch["d2"], pitch["g2"], pitch["c2"], pitch["f2"]}
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

// check if the key is sharp or flat
func inKey(key []string, note string) bool {
	for _, item := range key {
		if item == note {
			return true
		}
	}
	return false
}

// check if the note is in the given key
func inNote(notes []int, note int) bool {
	for _, item := range notes {
		if item == note {
			return true
		}
	}
	return false
}

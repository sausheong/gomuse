package muse

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
	setupPitches()
	setupKeys()
}

// note represents a musical note
type note struct {
	pitch      []int  // if it's a chord, there will be more than 1 pitch
	accidental []int  // 1 for sharp, -1 for flat, 0 for everything else
	explicit   []bool // true when the accidental was written explicitly (#, b or n);
	// an explicit accidental replaces the key signature for that note instead
	// of stacking with it
	length float64
	env    envelope
	har    harmonic
	vol    int
}

// tune represents a piece of music
type tune struct {
	key string
	ch1 []note
	ch2 []note
}

// Encode converts the tune to []int data to be used to create a WAV file
func (t tune) encode() (data []int, err error) {
	if !validKey(t.key) {
		err = fmt.Errorf("unknown key signature - %s ", t.key)
		return
	}

	// apply key
	acc := 0
	if inKey(sharpKeys, t.key) { // if the key signature is a sharp key
		acc = 1
	} else if inKey(flatKeys, t.key) { // if the key signature is a flat key
		acc = -1
	}

	// set up the accidentals for each note in the channels. A note whose
	// accidental was written explicitly (# , b or n) replaces the key
	// signature rather than stacking with it, so it's left untouched here.
	channels := [][]note{t.ch1, t.ch2}
	for _, channel := range channels {
		for _, n := range channel {
			for i, p := range n.pitch {
				if n.explicit[i] {
					continue
				}
				if inNote(tuneKey[t.key], p) {
					n.accidental[i] += acc
				}
			}
		}
	}

	var c1, c2 []int
	c1, err = encodeChannel(t.ch1)
	if err != nil {
		return
	}
	c2, err = encodeChannel(t.ch2)
	if err != nil {
		return
	}
	data, err = stereo(c1, c2)
	return
}

// encodeChannel encodes a whole channel, one note after another
func encodeChannel(notes []note) (data []int, err error) {
	capacity := 0
	for _, n := range notes {
		capacity += sampleCount(n.length)
	}
	data = make([]int, 0, capacity)
	for _, n := range notes {
		var nd []int
		nd, err = n.encode()
		if err != nil {
			return nil, err
		}
		data = append(data, nd...)
	}
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

	// encode into []int, one slice of samples per pitch in the chord
	notes := make([][]int, len(n.pitch))
	for i := 0; i < len(n.pitch); i++ {
		freq := frequency(n.pitch[i] + n.accidental[i])
		notes[i] = noteData(freq, n.length, n.env, n.har, n.vol)
	}
	data, err = concat(notes...)
	return
}

// sampleCount is the exact number of samples a duration (in seconds) maps
// to, rounded to the nearest whole sample. Using this everywhere a duration
// is turned into samples keeps note/rest/channel lengths consistent instead
// of drifting apart through repeated float addition.
func sampleCount(duration float64) int {
	return int(math.Round(duration * float64(sampleRate)))
}

// actual note data
func noteData(frequency float64, duration float64, env envelope, har harmonic, vol int) (data []int) {
	n := sampleCount(duration)
	data = make([]int, n)
	for k := 0; k < n; k++ {
		t := float64(k) / float64(sampleRate)
		data[k] = int(float64(vol) * env(t, duration) * har(frequency*t))
	}
	return
}

// rest note
func rest(duration float64) (data []int) {
	// zero-valued ints are silence, so there's nothing to fill in
	data = make([]int, sampleCount(duration))
	return
}

// chain notes together to create music!
func chain(notes ...[]int) (data []int, err error) {
	total := 0
	for _, n := range notes {
		total += len(n)
	}
	data = make([]int, 0, total)
	for _, n := range notes {
		data = append(data, n...)
	}
	return
}

// concatenate notes together to make chords
func concat(notes ...[]int) (data []int, err error) {
	if len(notes) == 0 {
		return
	}
	// make sure all the notes are the same length
	l := len(notes[0])
	for _, n := range notes {
		if len(n) != l {
			err = errors.New("length of notes are not the same")
			return
		}
	}
	// add up all the notes
	data = make([]int, l)
	for i := 0; i < l; i++ {
		d := 0
		for _, n := range notes {
			d += n[i]
		}
		data[i] = d
	}
	return
}

// returns the frequency of the note
func frequency(step int) float64 {
	return 440.0 * (math.Pow(2, (float64(step) / 12.0)))
}

// setup pitches
func setupPitches() {
	pitch = make(map[string]int)
	notes := []string{"c", "d", "e", "f", "g", "a", "b"}
	nums := []int{-57, -55, -53, -52, -50, -48, -46}
	for i := 1; i < 8; i++ {
		for j, note := range notes {
			nums[j] = nums[j] + 12
			pitch[fmt.Sprintf("%s%d", note, i)] = nums[j]
		}
	}
}

// initialise the tuneKey array, which is used to apply the key signature
// to notes. A key's accidental note applies across every octave (1 to 7),
// so l ranges over all 7 octaves starting from the octave-1 pitch (l=0).
func setupKeys() {
	tuneKey = make(map[string][]int)
	tuneKey["C"] = []int{}
	sharpKeys = []string{"G", "D", "A", "E", "B", "F#", "C#"}
	sharpNotes := []int{pitch["f1"], pitch["c1"], pitch["g1"], pitch["d1"], pitch["a1"], pitch["e1"], pitch["b1"]}
	for i, key := range sharpKeys {
		k := []int{}
		for j := 0; j < i+1; j++ {
			for l := 0; l < 7; l++ {
				k = append(k, sharpNotes[j]+(12*l))
			}
		}
		tuneKey[key] = k
	}

	flatKeys = []string{"F", "Bb", "Eb", "Ab", "Db", "Gb", "Cb"}
	flatNotes := []int{pitch["b1"], pitch["e1"], pitch["a1"], pitch["d1"], pitch["g1"], pitch["c1"], pitch["f1"]}
	for i, key := range flatKeys {
		k := []int{}
		for j := 0; j < i+1; j++ {
			for l := 0; l < 7; l++ {
				k = append(k, flatNotes[j]+(12*l))
			}
		}
		tuneKey[key] = k
	}
}

// validKey reports whether key is a recognised key signature
func validKey(key string) bool {
	if key == "C" {
		return true
	}
	return inKey(sharpKeys, key) || inKey(flatKeys, key)
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

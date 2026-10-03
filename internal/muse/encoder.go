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
	ins    instrument // when set, replaces env and har
	vol    int
	gain   float64 // loudness relative to vol, for accents and balance; 0 means 1
	// release is how long the note keeps sounding after its written length,
	// overlapping whatever comes next
	release float64
	// velocity is how hard the note is played, 0 to 1 (0 means not set);
	// see dynamics.go. It's used for tone; loudness goes through gain.
	velocity float64
	// placed is true when timing.go has set start and hold: start is the
	// note's start in samples from the beginning of the channel, and hold
	// its played length in seconds (instead of length)
	placed bool
	start  int
	hold   float64
	// spread is the delay in seconds between successive pitches of a
	// chord, lowest first, so a chord is rolled instead of struck at once
	spread float64
}

// tune represents a piece of music
type tune struct {
	key string
	ch1 []note
	ch2 []note
	ch3 []note // centre channel, mixed into both sides
	// reverb is the amount of room reverb, 0 to 1
	reverb float64
	// pan places each channel from -1 (left) to 1 (right); nil means the
	// plain layout (C1 left, C2 right, C3 centre), see voice.go
	pan []float64
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
	channels := [][]note{t.ch1, t.ch2, t.ch3}
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

	var cs [3][]int
	var nominal [3]int
	for i, ch := range [][]note{t.ch1, t.ch2, t.ch3} {
		if cs[i], nominal[i], err = encodeChannel(ch); err != nil {
			return
		}
	}
	if err = sameLength(cs, nominal); err != nil {
		return
	}
	// tails ring past the written end by different amounts, so pad every
	// channel out to the longest
	longest := 0
	for _, c := range cs {
		longest = max(longest, len(c))
	}
	for i, c := range cs {
		if len(c) > 0 && len(c) < longest {
			cs[i] = append(c, make([]int, longest-len(c))...)
		}
	}

	var l, r []int
	if t.pan == nil {
		l, r, err = mix(cs[0], cs[1], cs[2])
	} else {
		l, r, err = mixPanned(cs, t.pan)
	}
	if err != nil {
		return
	}
	l, r = reverb(l, r, t.reverb)
	data = interleave(l, r)
	return
}

// sameLength checks that the channels' written lengths (without the tails
// of their last notes) agree to within the tolerance
func sameLength(cs [3][]int, nominal [3]int) error {
	for i := 1; i < 3; i++ {
		if len(cs[i]) == 0 {
			continue
		}
		if d := nominal[i] - nominal[0]; d >= tolerance || -d >= tolerance {
			return fmt.Errorf("channel lengths are different - C1: %d C%d: %d", nominal[0], i+1, nominal[i])
		}
	}
	return nil
}

// encodeChannel encodes a whole channel. Each note starts where the
// previous one's written length ends, and its release tail (if any) is
// added on top of the notes that follow. nominal is the written length of
// the channel in samples; data is longer when the last note has a tail.
func encodeChannel(notes []note) (data []int, nominal int, err error) {
	for _, n := range notes {
		nominal += sampleCount(n.length)
	}
	data = make([]int, nominal)
	at := 0
	for _, n := range notes {
		var nd []int
		if nd, err = n.encode(); err != nil {
			return nil, 0, err
		}
		pos := at
		if n.placed {
			pos = max(0, n.start)
		}
		if end := pos + len(nd); end > len(data) {
			data = append(data, make([]int, end-len(data))...)
		}
		for k, v := range nd {
			data[pos+k] += v
		}
		at += sampleCount(n.length)
	}
	if len(notes) > 0 && notes[len(notes)-1].placed {
		// timing may stretch the channel; its written end moves with it
		last := notes[len(notes)-1]
		nominal = max(0, last.start) + sampleCount(last.hold)
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

	vol := n.vol
	if n.gain > 0 {
		vol = int(math.Round(float64(vol) * n.gain))
	}

	length := n.length
	if n.placed && n.hold > 0 {
		length = n.hold
	}

	// encode into []int, one slice of samples per pitch in the chord; a
	// rolled chord delays each pitch, lowest first, and every pitch is
	// padded to the same total length so they can be added up
	order := pitchOrder(n)
	delay := sampleCount(n.spread)
	notes := make([][]int, len(n.pitch))
	for i := 0; i < len(n.pitch); i++ {
		freq := frequency(n.pitch[i] + n.accidental[i])
		var nd []int
		if n.ins != nil {
			samples := n.ins(freq, length+n.release)
			nd = make([]int, len(samples))
			for k, v := range tone(n, freq, samples) {
				nd[k] = int(float64(vol) * v)
			}
		} else {
			nd = noteData(freq, length, n.env, n.har, vol)
		}
		if lead := order[i] * delay; lead > 0 {
			nd = append(make([]int, lead), nd...)
		}
		notes[i] = nd
	}
	longest := 0
	for _, nd := range notes {
		longest = max(longest, len(nd))
	}
	for i, nd := range notes {
		if len(nd) < longest {
			notes[i] = append(nd, make([]int, longest-len(nd))...)
		}
	}
	data, err = concat(notes...)
	return
}

// pitchOrder ranks a chord's pitches from lowest (0) to highest
func pitchOrder(n note) []int {
	order := make([]int, len(n.pitch))
	for i := range n.pitch {
		pi := n.pitch[i] + n.accidental[i]
		for j := range n.pitch {
			pj := n.pitch[j] + n.accidental[j]
			if pj < pi || (pj == pi && j < i) {
				order[i]++
			}
		}
	}
	return order
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

// note data from an instrument, scaled to the volume
func instrumentData(frequency float64, duration float64, ins instrument, vol int) (data []int) {
	samples := ins(frequency, duration)
	data = make([]int, len(samples))
	for k, v := range samples {
		data[k] = int(float64(vol) * v)
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

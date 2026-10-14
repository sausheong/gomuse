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
	tone           *ToneControls
	attackMS       *float64
	gesture        *Gesture
	pluckPhrase    bool
	noteVibrato    *VibratoShape
	breath         float64
	explicitSlurs  bool
	slurGroup      int
	phrase         *phraseCurve
	sustainHold    float64
	stopPitches    map[int]float64
	instrumentName string
	performance    *Performance
	pitch          []int  // if it's a chord, there will be more than 1 pitch
	accidental     []int  // 1 for sharp, -1 for flat, 0 for everything else
	explicit       []bool // true when the accidental was written explicitly (#, b or n);
	// an explicit accidental replaces the key signature for that note instead
	// of stacking with it
	length      float64
	env         envelope
	har         harmonic
	ins         instrument         // when set, replaces env and har
	velocityIns velocityInstrument // optional timbre response to dynamics
	vol         int
	gain        float64 // loudness relative to vol, for accents and balance; 0 means 1
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
	// kit is the drums struck together on a drum note (see drums.go); a
	// drum note has no pitch. hit counts the drum notes in the part, to
	// pick which variant of each drum it plays.
	kit []string
	hit int
}

// tune represents a piece of music
type tune struct {
	mix       map[string]ChannelMix
	capture   *renderCapture
	mastering *Mastering
	room      *RoomSettings
	// external replaces one pitched channel with a time-aligned neural vocal.
	external map[int][]int
	key      string
	ch1      []note
	ch2      []note
	ch3      []note // centre channel, mixed into both sides
	// drums is the drum part: hits on the kit rather than pitches
	drums []note
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
	channels := t.channels()
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

	for _, ch := range t.channels() {
		prepareStringPerformance(ch)
	}
	var cs [channelCount][]int
	var nominal [channelCount]int
	for i, ch := range t.channels() {
		if samples, ok := t.external[i]; ok {
			cs[i] = samples
			for _, n := range ch {
				nominal[i] += sampleCount(n.length)
			}
			if len(ch) > 0 && ch[len(ch)-1].placed {
				n := ch[len(ch)-1]
				nominal[i] = max(0, n.start) + sampleCount(n.hold)
			}
			continue
		}
		if cs[i], nominal[i], err = encodeChannel(ch); err != nil {
			return
		}
	}
	for c := range cs {
		if settings, ok := t.mix[channelNames[c]]; ok {
			processChannelMix(cs[c], settings)
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
		// the plain layout: C1 left, C2 right, C3 and the drums centre
		if l, r, err = mix(cs[0], cs[1], cs[2]); err != nil {
			return
		}
		if d := cs[drumChannel]; len(d) > 0 {
			if len(l) == 0 {
				l, r = make([]int, len(d)), make([]int, len(d))
			}
			for k, v := range d {
				l[k] += v
				r[k] += v
			}
		}
	} else if l, r, err = mixPanned(cs, t.pan); err != nil {
		return
	}
	if t.capture != nil {
		t.capture.dry(cs, t.pan, l, r)
	}
	if t.room != nil {
		l, r = smallRoom(l, r, cs, t.room, t.reverb)
	} else {
		l, r = reverb(l, r, t.reverb)
	}
	if t.capture != nil {
		gain := t.capture.finish(l, r, masteringGain(l, r, t.mastering))
		scaleStereo(l, r, gain)
	} else {
		masterStereo(l, r, t.mastering)
	}
	data = interleave(l, r)
	return
}

// sameLength checks that the channels' written lengths (without the tails
// of their last notes) agree to within the tolerance
func sameLength(cs [channelCount][]int, nominal [channelCount]int) error {
	// compare against the first channel that has any notes, so a score can
	// be drums alone
	ref := -1
	for i := range cs {
		if len(cs[i]) == 0 {
			continue
		}
		if ref < 0 {
			ref = i
			continue
		}
		if d := nominal[i] - nominal[ref]; d >= tolerance || -d >= tolerance {
			return fmt.Errorf("channel lengths are different - %s: %d %s: %d", channelNames[ref], nominal[ref], channelNames[i], nominal[i])
		}
	}
	return nil
}

// channelCount is how many channels a tune has: C1, C2, C3 and the drums
const channelCount = 4

// drumChannel is the index of the drum part in channelNames
const drumChannel = 3

// channelNames names the tune's channels, in order, as a score writes them
var channelNames = [channelCount]string{"C1", "C2", "C3", "drums"}

// channels returns the tune's channels in the order of channelNames. The
// slices share their notes with the tune, so changing a note through them
// changes the tune.
func (t *tune) channels() [channelCount][]note {
	return [channelCount][]note{t.ch1, t.ch2, t.ch3, t.drums}
}

// encodeChannel encodes a whole channel. Each note starts where the
// previous one's written length ends, and its release tail (if any) is
// added on top of the notes that follow. nominal is the written length of
// the channel in samples; data is longer when the last note has a tail.
// channelCursor renders one musical event at a time while retaining the
// connected wind/bow/pluck state. Both offline and progressive rendering use it.
type channelCursor struct {
	notes                               []note
	index, at, guitarStroke, bassStroke int
	voice                               phraseVoice
	pluck                               pluckedVoice
}

func (c *channelCursor) next() (int, []int, error) {
	n := c.notes[c.index]
	var err error
	if n.instrumentName == "upright-bass" && len(n.pitch) > 0 {
		stroke := c.bassStroke
		c.bassStroke++
		n.velocityIns = func(f, d, v float64) []float64 { return uprightBassStroke(f, d, v, stroke) }
	}
	var nd []int
	pos := c.at
	if n.placed {
		pos = max(0, n.start)
	}
	if !n.placed {
		n.start = pos
	}
	if n.pluckPhrase && n.performance != nil && len(n.pitch) == 1 {
		c.voice.active = false
		if n.gesture.transition() && !c.pluck.active {
			return 0, nil, fmt.Errorf("%s transition needs a contiguous preceding note", n.instrumentName)
		}
		hold := heldTime(n)
		join := false
		if c.index+1 < len(c.notes) {
			next := c.notes[c.index+1]
			nextPos := c.at + sampleCount(n.length)
			if next.placed {
				nextPos = max(0, next.start)
			}
			join = next.pluckPhrase && next.gesture.transition() && next.instrumentName == n.instrumentName && len(next.pitch) == 1 && nextPos > pos && math.Abs(float64(nextPos-pos)/sampleRate-hold) < .04
			if join {
				hold = float64(nextPos-pos) / sampleRate
			}
		}
		if !join && n.performance.Articulation == "let-ring" && (n.gesture == nil || n.gesture.Kind != "mute") {
			hold = math.Max(hold, n.sustainHold)
		}
		nd = c.pluck.render(n, hold, join)
	} else if n.performance != nil && len(n.pitch) > 0 && (n.instrumentName == "piano" || pluckedInstrument(n.instrumentName)) {
		c.pluck.active = false
		c.voice.active = false
		nd = renderStringPerformance(n, c.guitarStroke)
		if n.instrumentName == "guitar" || n.instrumentName == "nylon-guitar" || n.instrumentName == "electric-guitar" {
			c.guitarStroke++
		}
	} else if n.performance != nil && len(n.pitch) == 1 {
		c.pluck.active = false
		if n.gesture.transition() && !c.voice.active {
			return 0, nil, fmt.Errorf("portamento needs a contiguous preceding slurred note")
		}
		hold := n.length
		if n.placed {
			hold = n.hold
		}
		join := false
		if c.index+1 < len(c.notes) && n.performance.Articulation == "legato" {
			next := c.notes[c.index+1]
			nextPos := c.at + sampleCount(n.length)
			if next.placed {
				nextPos = max(0, next.start)
			}
			join = canSlur(n, next) && next.performance != nil && next.performance.Articulation == "legato" && next.instrumentName == n.instrumentName && len(next.pitch) == 1 && nextPos > pos && math.Abs(float64(nextPos-pos)/sampleRate-hold) < 0.04
			if join {
				hold = float64(nextPos-pos) / sampleRate
			}
		}
		nd = c.voice.render(n, hold, join)
	} else {
		c.pluck.active = false
		c.voice.active = false
		if nd, err = n.encode(); err != nil {
			return 0, nil, err
		}
	}

	for k, v := range nd {
		if n.phrase != nil {
			nd[k] = int(float64(v) * dBGain(n.phraseDB(float64(k)/sampleRate)))
		}
	}
	c.at += sampleCount(n.length)
	c.index++
	return pos, nd, nil
}
func channelNominal(notes []note) int {
	n := 0
	for _, event := range notes {
		n += sampleCount(event.length)
	}
	if len(notes) > 0 && notes[len(notes)-1].placed {
		last := notes[len(notes)-1]
		n = max(0, last.start) + sampleCount(last.hold)
	}
	return n
}
func encodeChannel(notes []note) (data []int, nominal int, err error) {
	nominal = channelNominal(notes)
	written := 0
	for _, n := range notes {
		written += sampleCount(n.length)
	}
	data = make([]int, written)
	c := channelCursor{notes: notes}
	for c.index < len(notes) {
		pos, nd, e := c.next()
		if e != nil {
			return nil, 0, e
		}
		if end := pos + len(nd); end > len(data) {
			data = append(data, make([]int, end-len(data))...)
		}
		for k, v := range nd {
			data[pos+k] += v
		}
	}
	return
}

// encode the note
func (n note) encode() (data []int, err error) {
	if len(n.pitch) != len(n.accidental) {
		err = errors.New("length of pitches and accidentals not the same")
		return
	}

	vol := n.vol
	if n.gain > 0 {
		vol = int(math.Round(float64(vol) * n.gain))
	}

	if len(n.kit) > 0 {
		return n.kitData(vol), nil
	}

	// this is a rest
	if len(n.pitch) == 0 && n.length != 0 {
		data = rest(n.length)
		return
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
			var samples []float64
			if n.tone != nil && n.velocityIns != nil {
				samples = controlledSamples(n, freq, length)
			} else if n.velocityIns != nil {
				samples = n.velocityIns(freq, length+n.release, n.velocity)
				if n.release > 0 {
					dampAfterHold(samples, length)
				}
			} else {
				samples = tone(n, freq, n.ins(freq, length+n.release))
			}
			nd = make([]int, len(samples))
			for k, v := range samples {
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

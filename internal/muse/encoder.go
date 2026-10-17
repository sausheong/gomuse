package muse

import (
	"errors"
	"fmt"
	"maps"
	"math"
	"sync"
)

// pitch maps a note name such as "c4" to its distance in semitones from A4
var pitch = map[string]int{}

// keySignatures maps each key, e.g. "G" or "Bb", to the accidental it adds
// to the notes it alters
var keySignatures = map[string]keySignature{}

type keySignature struct {
	shift int          // 1 for a sharp key, -1 for a flat key
	notes map[int]bool // the pitches it alters, in every octave
}

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

// effectiveGain is the note's gain, where 0 means unset and so 1
func (n note) effectiveGain() float64 {
	if n.gain == 0 {
		return 1
	}
	return n.gain
}

// heldTime is how long a note is played, in seconds: its placed hold once
// timing has placed it, or else its written length
func heldTime(n note) float64 {
	if n.placed {
		return n.hold
	}
	return n.length
}

// noteStarts is when each note of a channel starts, in seconds: where
// timing placed it, or else where the written lengths before it end
func noteStarts(notes []note) []float64 {
	starts := make([]float64, len(notes))
	at := 0.0
	for i, n := range notes {
		starts[i] = at
		if n.placed {
			starts[i] = float64(max(0, n.start)) / sampleRate
		}
		at += n.length
	}
	return starts
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
	t.prepareForRender()

	// the channels share nothing, so they're synthesised at the same time
	var cs [channelCount][]int
	var nominal [channelCount]int
	var errs [channelCount]error
	var wg sync.WaitGroup
	for i, ch := range t.channels() {
		if samples, ok := t.external[i]; ok {
			cs[i], nominal[i] = samples, channelNominal(ch)
			continue
		}
		wg.Go(func() { cs[i], nominal[i], errs[i] = encodeChannel(ch) })
	}
	wg.Wait()
	for _, err = range errs {
		if err != nil {
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

// prepareForRender applies the key signature to every note, then works out
// pedal and let-ring sustain from the final pitches. It changes the notes in
// place, so it must run exactly once before a tune is rendered.
func (t *tune) prepareForRender() {
	for _, ch := range t.channels() {
		for _, n := range ch {
			for i, p := range n.pitch {
				// an accidental written explicitly (#, b or n) replaces the
				// key signature rather than stacking with it
				if !n.explicit[i] {
					n.accidental[i] += keyShift(t.key, p)
				}
			}
		}
		prepareStringPerformance(ch)
	}
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

// channelCursor renders one musical event at a time while retaining the
// connected wind/bow/pluck state. Both offline and progressive rendering use it.
type channelCursor struct {
	notes []note
	// index is the next note to render, at where it starts as written
	index, at int
	// guitarStroke and bassStroke count the strokes played so far, so
	// alternate strums and repeated plucks vary
	guitarStroke, bassStroke int
	voice                    phraseVoice  // the connected wind/bow voice
	pluck                    pluckedVoice // the connected plucked string
}

// next renders the next note and returns where it starts, in samples from
// the beginning of the channel, and its samples. A connected note keeps
// the voice of the one before it ringing; anything else resets it.
func (c *channelCursor) next() (int, []int, error) {
	n := c.notes[c.index]
	if n.instrumentName == "upright-bass" && len(n.pitch) > 0 {
		stroke := c.bassStroke
		c.bassStroke++
		n.velocityIns = func(f, d, v float64) []float64 { return uprightBassStroke(f, d, v, stroke) }
	}
	pos := c.at
	if n.placed {
		pos = max(0, n.start)
	} else {
		n.start = pos
	}

	var nd []int
	switch {
	case n.pluckPhrase && n.performance != nil && len(n.pitch) == 1:
		// a plucked note in a phrase of slides, hammer-ons and mutes
		c.voice.active = false
		if n.gesture.transition() && !c.pluck.active {
			return 0, nil, fmt.Errorf("%s transition needs a contiguous preceding note", n.instrumentName)
		}
		hold, join := c.joinNext(n, pos, heldTime(n), func(next note) bool {
			return next.pluckPhrase && next.gesture.transition()
		})
		if !join && n.performance.Articulation == "let-ring" && (n.gesture == nil || n.gesture.Kind != "mute") {
			hold = math.Max(hold, n.sustainHold)
		}
		nd = c.pluck.render(n, hold, join)
	case n.performance != nil && len(n.pitch) > 0 && (n.instrumentName == "piano" || pluckedInstrument(n.instrumentName)):
		// a pedalled piano note, or a strummed or picked string
		c.pluck.active = false
		c.voice.active = false
		nd = renderStringPerformance(n, c.guitarStroke)
		if guitarInstrument(n.instrumentName) {
			c.guitarStroke++
		}
	case n.performance != nil && len(n.pitch) == 1:
		// a sustained wind, bowed or synth note, slurred into the next
		// when both are legato
		c.pluck.active = false
		if n.gesture.transition() && !c.voice.active {
			return 0, nil, fmt.Errorf("portamento needs a contiguous preceding slurred note")
		}
		hold, join := heldTime(n), false
		if n.performance.Articulation == "legato" {
			hold, join = c.joinNext(n, pos, hold, func(next note) bool {
				return canSlur(n, next) && next.performance != nil && next.performance.Articulation == "legato"
			})
		}
		nd = c.voice.render(n, hold, join)
	default:
		c.pluck.active = false
		c.voice.active = false
		var err error
		if nd, err = n.encode(); err != nil {
			return 0, nil, err
		}
	}

	if n.phrase != nil {
		for k, v := range nd {
			nd[k] = int(float64(v) * dBGain(n.phraseDB(float64(k)/sampleRate)))
		}
	}
	c.at += sampleCount(n.length)
	c.index++
	return pos, nd, nil
}

// joinNext reports whether the following note carries on from n, which
// starts at pos and is held for hold seconds: the same instrument, a single
// pitch, starting within 40 ms of where n ends, and connects(next). If it
// does, n is held until the next note starts.
func (c *channelCursor) joinNext(n note, pos int, hold float64, connects func(next note) bool) (float64, bool) {
	if c.index+1 >= len(c.notes) {
		return hold, false
	}
	next := c.notes[c.index+1]
	nextPos := c.at + sampleCount(n.length)
	if next.placed {
		nextPos = max(0, next.start)
	}
	gap := float64(nextPos-pos) / sampleRate
	if connects(next) && next.instrumentName == n.instrumentName && len(next.pitch) == 1 && nextPos > pos && math.Abs(gap-hold) < .04 {
		return gap, true
	}
	return hold, false
}

// channelWritten is the length of a channel's notes in samples, as written
func channelWritten(notes []note) int {
	n := 0
	for _, event := range notes {
		n += sampleCount(event.length)
	}
	return n
}

// channelNominal is the length of a channel in samples without the tail of
// its last note: where the last note ends once timing has placed it, or the
// written length if it hasn't
func channelNominal(notes []note) int {
	if len(notes) > 0 && notes[len(notes)-1].placed {
		last := notes[len(notes)-1]
		return max(0, last.start) + sampleCount(last.hold)
	}
	return channelWritten(notes)
}

// encodeChannel encodes a whole channel. Each note starts where the
// previous one's written length ends, and its release tail (if any) is
// added on top of the notes that follow. nominal is the written length of
// the channel in samples; data is longer when the last note has a tail.
func encodeChannel(notes []note) (data []int, nominal int, err error) {
	nominal = channelNominal(notes)
	data = make([]int, channelWritten(notes))
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

// rest note
func rest(duration float64) (data []int) {
	// zero-valued ints are silence, so there's nothing to fill in
	data = make([]int, sampleCount(duration))
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

// setupPitches names every note from c1 to b7
func setupPitches() {
	// semitones from A4 of each note in octave 4
	octave4 := map[byte]int{'c': -9, 'd': -7, 'e': -5, 'f': -4, 'g': -2, 'a': 0, 'b': 2}
	for octave := 1; octave <= 7; octave++ {
		for name, semitones := range octave4 {
			pitch[fmt.Sprintf("%c%d", name, octave)] = semitones + 12*(octave-4)
		}
	}
}

// setupKeys builds the key signatures. Each key alters the notes of the key
// before it in the circle of fifths plus one more, in every octave.
func setupKeys() {
	keySignatures["C"] = keySignature{}
	addKeys([]string{"G", "D", "A", "E", "B", "F#", "C#"}, "fcgdaeb", 1)
	addKeys([]string{"F", "Bb", "Eb", "Ab", "Db", "Gb", "Cb"}, "beadgcf", -1)
}

func addKeys(keys []string, order string, shift int) {
	notes := map[int]bool{}
	for i, key := range keys {
		base := pitch[order[i:i+1]+"1"]
		for octave := 0; octave < 7; octave++ {
			notes[base+12*octave] = true
		}
		keySignatures[key] = keySignature{shift, maps.Clone(notes)}
	}
}

// validKey reports whether key is a recognised key signature
func validKey(key string) bool {
	_, ok := keySignatures[key]
	return ok
}

// keyShift is the accidental the key signature adds to a written pitch: 1
// in a sharp key, -1 in a flat key, or 0 if the key leaves the pitch alone
func keyShift(key string, p int) int {
	if k := keySignatures[key]; k.notes[p] {
		return k.shift
	}
	return 0
}

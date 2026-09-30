package muse

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// maxChordPitches caps how many pitches a single "-" separated chord may
// stack. encode() allocates one full-length sample slice per pitch in a
// chord, so an unbounded chord width lets a tiny score request an amount of
// memory and CPU wildly out of proportion to its note count - a chord this
// wide is not musically meaningful anyway.
const maxChordPitches = 16

// Score represents the musical score
type Score struct {
	Name     string    `yaml:"name"`
	Key      string    `yaml:"key"`
	Length   float64   `yaml:"length"`
	Envelope string    `yaml:"envelope"`
	Harmonic string    `yaml:"harmonic"`
	Volume   int       `yaml:"volume"`
	Sections []Section `yaml:"sections"`
}

// Section represents a section of music; it has 2 channels for stereo purposes
type Section struct {
	Length   float64  `yaml:"length,omitempty"`
	Envelope string   `yaml:"envelope,omitempty"`
	Harmonic string   `yaml:"harmonic,omitempty"`
	Volume   int      `yaml:"volume,omitempty"`
	C1       []string `yaml:"C1"`
	C2       []string `yaml:"C2"`
}

// ParseFile reads a score file (filename+".yaml") and writes the resulting
// tune to filename+".wav", both relative to the current working directory.
// There is no length limit for the command line tool.
func ParseFile(s *Score, filename string) (name string, err error) {
	scoreFile, err := os.ReadFile(filename + ".yaml")
	if err != nil {
		err = fmt.Errorf("cannot read score file > %v ", err)
		return
	}
	name, err = Parse(s, scoreFile, filename, 0)
	return
}

// Parse reads a score, converts it into a Score struct and writes the
// resulting audio to outfile+".wav". maxSamples caps the number of samples
// allowed in any one channel; maxSamples <= 0 means unlimited. A score that
// would exceed the cap is rejected before any audio is synthesised.
func Parse(s *Score, score []byte, outfile string, maxSamples int) (name string, err error) {
	err = yaml.Unmarshal(score, s)
	if err != nil {
		err = fmt.Errorf("cannot unmarshal score file > %v", err)
		return
	}
	name = s.Name

	t := tune{
		key: s.Key,
		ch1: []note{},
		ch2: []note{},
	}
	var nt note
	for _, section := range s.Sections {
		for _, n := range section.C1 {
			nt, err = makeNote(n, section, *s)
			if err != nil {
				err = fmt.Errorf("[C1] cannot make note > %v ", err)
				return
			}
			t.ch1 = append(t.ch1, nt)
		}

		for _, n := range section.C2 {
			nt, err = makeNote(n, section, *s)
			if err != nil {
				err = fmt.Errorf("[C2] cannot make note > %v ", err)
				return
			}
			t.ch2 = append(t.ch2, nt)
		}
	}

	// reject a score that would be too long before synthesising any audio,
	// computed purely from the notes' durations
	if maxSamples > 0 {
		if n := channelSamples(t.ch1); n > maxSamples {
			err = fmt.Errorf("tune too long > channel 1 needs %d samples, max is %d ", n, maxSamples)
			return
		}
		if n := channelSamples(t.ch2); n > maxSamples {
			err = fmt.Errorf("tune too long > channel 2 needs %d samples, max is %d ", n, maxSamples)
			return
		}
	}

	var data []int
	data, err = t.encode()
	if err != nil {
		err = fmt.Errorf("cannot encode the tune > %v", err)
		return
	}
	err = writeWAV(outfile, data)
	return
}

// channelSamples adds up the number of samples a channel's notes will
// produce, without synthesising any of them. encode() allocates one sample
// slice per pitch in a chord (see note.encode in encoder.go), so a note's
// contribution is its sample count times its chord width, not just once,
// or this cap wouldn't actually bound the memory encode() allocates.
func channelSamples(notes []note) int {
	total := 0
	for _, n := range notes {
		width := len(n.pitch)
		if width == 0 { // a rest still allocates one slice
			width = 1
		}
		total += sampleCount(n.length) * width
	}
	return total
}

// make a note
func makeNote(noteString string, section Section, score Score) (n note, err error) {
	length := section.Length
	if length == 0.0 {
		length = score.Length
	}
	vol := section.Volume
	if vol == 0 {
		vol = score.Volume
	}
	env := section.Envelope
	if env == "" {
		env = score.Envelope
	}
	har := section.Harmonic
	if har == "" {
		har = score.Harmonic
	}

	// make sure envelope and harmonic exist
	envFn, ok := envelopes[env]
	if !ok {
		err = fmt.Errorf("envelope doesn't exist - %s ", env)
		return
	}
	harFn, ok := harmonics[har]
	if !ok {
		err = fmt.Errorf("harmonic doesn't exist - %s ", har)
		return
	}

	// default returned note
	n = note{
		pitch:      []int{},
		accidental: []int{},
		explicit:   []bool{},
		length:     length,
		env:        envFn,
		har:        harFn,
		vol:        vol,
	}

	// if length is explicitly set, separate length of note from the pitches
	nArray := strings.Split(noteString, ":") // split the length away from the notes
	var l float64                            // note length multiplier
	var p string                             // pitch

	// if length of note is explicitly set
	if len(nArray) > 1 {
		l, err = strconv.ParseFloat(nArray[0], 64)
		if err != nil {
			err = fmt.Errorf("cannot parse note duration - %s > %v", nArray[0], err)
			return
		}
		p = nArray[1]
	} else {
		// if length of note is not explicitly set, use the standard one
		l = 1.0
		p = nArray[0]
	}
	n.length = l * length
	if math.IsNaN(n.length) || math.IsInf(n.length, 0) || n.length <= 0 {
		err = fmt.Errorf("invalid note duration - %v ", n.length)
		return
	}

	// handle if note is a single note or a chord
	if strings.Contains(p, "-") {
		// a chord
		pitches := strings.Split(p, "-")
		if len(pitches) > maxChordPitches {
			err = fmt.Errorf("chord has too many notes - %d, max is %d ", len(pitches), maxChordPitches)
			return
		}
		for _, c := range pitches {
			err = process(&n, c)
			if err != nil {
				err = fmt.Errorf("wrong chords formation %s > %v", c, err)
				return
			}
		}
	} else {
		// a single note
		err = process(&n, p)
		if err != nil {
			err = fmt.Errorf("wrong note structure - %s > %v ", p, err)
			return
		}
	}
	return
}

// process the note
func process(n *note, p string) (err error) {
	if p == "z" { // a rest
		return
	}
	if len(p) < 2 {
		err = fmt.Errorf("note doesn't exist, is too short - %s ", p)
		return
	}
	if len(p) > 3 {
		err = fmt.Errorf("note doesn't exist, is too long - %s ", p)
		return
	}

	pt, ok := pitch[p[:2]]
	if !ok {
		err = fmt.Errorf("note doesn't exist - %s ", p)
		return
	}

	// an accidental, if any, is the 3rd character: # (sharp), b (flat) or
	// n (natural, cancels the key signature for this note). Anything else
	// is an error rather than being silently treated as a natural.
	acc := 0
	explicit := false
	if len(p) == 3 {
		switch p[2:] {
		case "#":
			acc = 1
		case "b":
			acc = -1
		case "n":
			acc = 0
		default:
			err = fmt.Errorf("note doesn't exist, invalid accidental - %s ", p)
			return
		}
		explicit = true
	}

	n.pitch = append(n.pitch, pt)
	n.accidental = append(n.accidental, acc)
	n.explicit = append(n.explicit, explicit)
	return
}

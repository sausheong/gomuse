package main

import (
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"

	"gopkg.in/yaml.v2"
)

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
	C1 []string `yaml:"C1"`
	C2 []string `yaml:"C2"`
}

// ParseFile reads a score file and parses it into a Score struct
func ParseFile(s *Score, filename string) (name string, err error) {
	scoreFile, err := ioutil.ReadFile(filename + ".yaml")
	if err != nil {
		err = fmt.Errorf("Cannot read score file > %v ", err)
		return
	}
	name, err = Parse(s, scoreFile, dir+"/"+filename)
	return
}

// Parse reads a score and converts into a Score struct
func Parse(s *Score, score []byte, outfile string) (name string, err error) {
	err = yaml.Unmarshal(score, s)
	if err != nil {
		err = fmt.Errorf("Cannot unmarshal score file > %v", err)
		return
	}
	name = s.Name
	t := tune{
		key:    s.Key,
		length: s.Length,
		ch1:    []note{},
		ch2:    []note{},
	}
	var nt note
	for _, section := range s.Sections {
		for _, n := range section.C1 {
			nt, err = makeNote(n, s.Length, s.Envelope, s.Harmonic, s.Volume)
			if err != nil {
				err = fmt.Errorf("[C1] cannot make note > %v ", err)
				return
			}
			t.ch1 = append(t.ch1, nt)
		}

		for _, n := range section.C2 {
			nt, err = makeNote(n, s.Length, s.Envelope, s.Harmonic, s.Volume)
			if err != nil {
				err = fmt.Errorf("[C2] cannot make note > %v ", err)
				return
			}
			t.ch2 = append(t.ch2, nt)
		}
	}
	var data []int
	data, err = t.encode()
	if err != nil {
		err = fmt.Errorf("Cannot encode the tune > %v", err)
		return
	}
	write(outfile, data)
	return
}

// make a note
func makeNote(noteString string, length float64, env string, har string, vol int) (n note, err error) {
	// default returned note
	n = note{
		pitch:      []int{},
		accidental: []int{},
		length:     length,
		env:        envelopes[env],
		har:        harmonics[har],
		vol:        vol,
	}

	// if length is explicitly set, separate length of note from the pitches
	nArray := strings.Split(noteString, ":") // split the length away from the notes
	var l float64                            // note length
	var p string                             // pitch

	// if length of note is explicitly set
	if len(nArray) > 1 {
		l, err = strconv.ParseFloat(nArray[0], 64)
		p = nArray[1]
		if err != nil {
			err = fmt.Errorf("cannot parse note > %v", err)
		}
	} else {
		// if length of note is not explicitly set, use the standard one
		l = 1.0
		p = nArray[0]
	}
	n.length = l * length

	// handle if note is a single note or a chord
	// a chord
	if len(p) > 3 {
		chord := strings.Split(p, "-")
		if len(chord) < 2 {
			err = fmt.Errorf("wrong chords structure - %s ", chord)
			return
		}
		for _, c := range chord {
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
	if p != "z" {
		if len(p) < 2 {
			err = fmt.Errorf("note doesn't exist, is too short - %s ", p)
			return
		}

		if len(p) > 3 {
			err = fmt.Errorf("note doesn't exist, is too long - %s ", p)
			return
		}
		// check if the note exists
		_, ok := pitch[p[:2]]
		if ok {
			n.pitch = append(n.pitch, pitch[p[:2]])
			if p[len(p)-1:] == "#" {
				n.accidental = append(n.accidental, 1)
			} else if p[len(p)-1:] == "b" {
				n.accidental = append(n.accidental, -1)
			} else {
				n.accidental = append(n.accidental, 0)
			}
		} else {
			err = fmt.Errorf("note doesn't exist - %s ", p)
		}
	}
	return
}

package main

import (
	"io/ioutil"
	"log"
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
		log.Printf("Cannot read score file - #%v ", err)
		return
	}
	name, err = Parse(s, scoreFile, dir+"/"+filename)
	return
}

// Parse reads a score and converts into a Score struct
func Parse(s *Score, score []byte, outfile string) (name string, err error) {

	err = yaml.Unmarshal(score, s)
	if err != nil {
		log.Fatalf("Cannot unmarshal score file - %v", err)
		return
	}

	t := tune{
		key:    s.Key,
		length: s.Length,
		ch1:    []note{},
		ch2:    []note{},
	}

	for _, section := range s.Sections {
		for _, n := range section.C1 {
			note := makeNote(n, s.Length, s.Envelope)
			t.ch1 = append(t.ch1, note)
		}

		for _, n := range section.C2 {
			note := makeNote(n, s.Length, s.Envelope)
			t.ch2 = append(t.ch2, note)
		}
	}
	data, err := t.encode()
	if err != nil {
		log.Printf("Cannot encode the tune - %v", err)
		return s.Name, err
	}
	name = outfile
	write(outfile, data)
	return
}

func makeNote(noteString string, length float64, env string) (n note) {
	// default returned note
	n = note{
		pitch:      []int{},
		accidental: []int{},
		length:     length,
		env:        envelopes[env],
	}

	// if length is explicitly set, separate length of note from the pitches
	nArray := strings.Split(noteString, ":") // split the length away from the notes
	var l float64                            // note length
	var p string                             // pitch
	var err error

	// if length of note is explicitly set
	if len(nArray) > 1 {
		l, err = strconv.ParseFloat(nArray[0], 64)
		p = nArray[1]
		if err != nil {
			panic(err)
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
		for _, c := range chord {
			process(&n, c)
		}
	} else {
		// a single note
		process(&n, p)
	}
	return
}

func process(n *note, p string) {
	if p != "z" {
		n.pitch = append(n.pitch, pitch[p[:2]])
		if p[len(p)-1:] == "#" {
			n.accidental = append(n.accidental, 1)
		} else if p[len(p)-1:] == "b" {
			n.accidental = append(n.accidental, -1)
		} else {
			n.accidental = append(n.accidental, 0)
		}
	}
}

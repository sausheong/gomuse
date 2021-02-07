package main

import (
	"log"
	"testing"
)

func TestCreateTune(t *testing.T) {
	mytune := tune{
		key: "Eb",
		ch1: []note{createNote(pitch["c4"]), createNote(pitch["d4"]), createNote(pitch["e4"])},
		ch2: []note{createNote(pitch["g4"]), createNote(pitch["a4"]), createNote(pitch["b4"])},
	}
	data, err := mytune.encode()
	if err != nil {
		log.Println("Cannot encode the tune - %v", err)
	}
	writeWAV("tune", data)
}

func createNote(p int) (n note) {
	n = note{
		pitch:      []int{p},
		accidental: []int{0},
		length:     (1.0 / 3.0),
		env:        drop,
	}
	return
}

func TestManualCreateNotes(t *testing.T) {
	c := note{
		pitch:      []int{pitch["c4"]},
		accidental: []int{0},
		length:     (1.0 / 3.0),
		env:        drop,
	}

	f := note{
		pitch:      []int{pitch["f4"]},
		accidental: []int{0},
		length:     (1.0 / 3.0),
		env:        drop,
	}

	g := note{
		pitch:      []int{pitch["g4"]},
		accidental: []int{0},
		length:     (1.0 / 3.0),
		env:        drop,
	}

	cNote, err := c.encode()
	fNote, err := f.encode()
	gNote, err := g.encode()

	notes, err := chain(cNote, fNote, gNote)

	cmaj := note{
		pitch:      []int{pitch["c4"], pitch["e4"], pitch["g4"]},
		accidental: []int{0, 0, 0},
		length:     1.0,
		env:        drop,
	}

	cmajChord, err := cmaj.encode()
	data, err := stereo(notes, cmajChord)
	if err != nil {
		log.Println(err)
	}

	writeWAV("test", data)
}

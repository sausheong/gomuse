package main

import "fmt"

func main() {
	parseMusic()
}

func parseMusic() {
	var m Music
	err := Parse(&m, "music.yaml")
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(m.Notes[0].C1[1])
}

func createTune() {
	t := Tune{
		Key:        "Eb",
		NoteLength: 1,
		Channel1:   []Note{createNote(c4), createNote(d4), createNote(e4)},
		Channel2:   []Note{createNote(g4), createNote(a4), createNote(b4)},
	}
	data, err := t.Encode()
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("data:", data)
	fmt.Println("ch1:", t.Channel1)
	fmt.Println("ch2:", t.Channel2)

	write("tune", data)
}

func createNote(n int) (note Note) {
	note = Note{
		Pitch:      []int{n},
		Accidental: []int{0},
		Length:     (1.0 / 3.0),
		Envelope:   drop,
	}
	return
}

func testManualCreateNotes() {
	c := Note{
		Pitch:      []int{c4},
		Accidental: []int{0},
		Length:     (1.0 / 3.0),
		Envelope:   drop,
	}

	f := Note{
		Pitch:      []int{f4},
		Accidental: []int{0},
		Length:     (1.0 / 3.0),
		Envelope:   drop,
	}

	g := Note{
		Pitch:      []int{g4},
		Accidental: []int{0},
		Length:     (1.0 / 3.0),
		Envelope:   drop,
	}

	cNote, err := c.encode()
	fNote, err := f.encode()
	gNote, err := g.encode()

	notes, err := chain(cNote, fNote, gNote)

	cmaj := Note{
		Pitch:      []int{c4, e4, g4},
		Accidental: []int{0, 0, 0},
		Length:     1.0,
		Envelope:   drop,
	}

	cmajChord, err := cmaj.encode()
	data, err := stereo(notes, cmajChord)

	if err != nil {
		fmt.Println(err)
	}

	write("test", data)
}

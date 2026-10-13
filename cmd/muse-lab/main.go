// muse-lab creates exposed, reproducible instrument previews for evaluation.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sausheong/gomuse/internal/muse"
	"gopkg.in/yaml.v3"
)

func main() {
	out := flag.String("out", "", "new directory for scores and rendered WAVs")
	scoreFile := flag.String("score", "", "render an instrumental score and aligned stems instead of an instrument study")
	selected := flag.String("instrument", "all", "registered instrument name, drums, or all (excluding voice and aliases)")
	flag.Parse()
	if *out == "" {
		log.Fatal("use -out <new-directory>")
	}
	if *scoreFile != "" {
		doc, err := os.ReadFile(*scoreFile)
		if err != nil {
			log.Fatal(err)
		}
		if err = muse.RenderBundle(doc, *out); err != nil {
			log.Fatal(err)
		}
		fmt.Println("Rendered master and stems to", *out)
		return
	}
	names := muse.InstrumentNames()
	valid := *selected == "all" || *selected == "drums"
	for _, name := range names {
		if name == *selected {
			valid = true
		}
	}
	if !valid {
		log.Fatalf("unknown instrument %q", *selected)
	}
	if _, err := os.Stat(*out); !os.IsNotExist(err) {
		log.Fatal("output directory must not already exist; preserved benchmarks are never overwritten")
	}
	if err := os.MkdirAll(*out, 0755); err != nil {
		log.Fatal(err)
	}
	if *selected != "all" {
		names = []string{*selected}
	} else {
		names = append(names, "drums")
	}
	zero := 0.0
	manifest := map[string]any{"created_utc": time.Now().UTC().Format(time.RFC3339), "scope": "dry register, dynamics, repetition, sustain and release study", "files": []string{}}
	files := []string{}
	for _, name := range names {
		if *selected == "all" && (name == "voice" || name == "sax" || strings.HasPrefix(name, "scat-")) {
			continue
		}
		low, mid, high := "c3", "c4", "c5"
		switch name {
		case "bass", "upright-bass", "synth-bass":
			low, mid, high = "e1", "a2", "g3"
		case "violin":
			low, mid, high = "g3", "a4", "e6"
		case "cello":
			low, mid, high = "c2", "g3", "c5"
		case "trombone", "bassoon":
			low, mid, high = "c2", "c3", "c4"
		case "french-horn":
			low, mid, high = "c3", "g3", "c5"
		case "oboe":
			low, mid, high = "c4", "a4", "c6"
		case "glockenspiel", "xylophone":
			low, mid, high = "c4", "c5", "c6"
		case "nylon-guitar":
			low, mid, high = "e2", "a3", "e5"
		case "flute":
			low, mid, high = "c4", "c5", "c6"
		case "trumpet":
			low, mid, high = "c4", "g4", "c6"
		case "saxophone", "sax":
			low, mid, high = "b2", "d4", "a5"
		case "clarinet":
			low, mid, high = "e3", "b4", "g5"
		}
		s := muse.Score{Name: name + " benchmark", Key: "C", Length: .32, Instrument: name, Volume: 12000, Humanize: &zero, Reverb: &zero, Pan: map[string]float64{"C1": 0}}
		for _, dynamic := range []string{"p", "mf", "f"} {
			s.Sections = append(s.Sections, muse.Section{Dynamic: dynamic, C1: []string{low, mid, high, "z", ".5:" + mid, ".5:" + mid, ".5:" + mid, ".5:" + mid, "3:" + mid, "z"}})
		}
		if name == "drums" {
			s.Instrument = "piano"
			for i := range s.Sections {
				s.Sections[i].C1 = nil
				s.Sections[i].Drums = []string{"kick", "snare", "hat", "ride", "tom3", "tom1", "crash", "shaker", "tambourine", "conga", "bongo", "claves", "triangle", "brushtap", "2:brushsweep", "z"}
			}
		}
		doc, err := yaml.Marshal(s)
		if err != nil {
			log.Fatal(err)
		}
		base := filepath.Join(*out, name)
		if err = os.WriteFile(base+".yaml", doc, 0644); err != nil {
			log.Fatal(err)
		}
		var parsed muse.Score
		if _, err = muse.Parse(&parsed, doc, base, 0); err != nil {
			log.Fatalf("%s: %v", name, err)
		}
		files = append(files, name)
		fmt.Println("Rendered", name)
	}
	manifest["files"] = files
	doc, _ := yaml.Marshal(manifest)
	if err := os.WriteFile(filepath.Join(*out, "manifest.yaml"), doc, 0644); err != nil {
		log.Fatal(err)
	}
}

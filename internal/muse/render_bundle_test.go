package muse

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-audio/wav"
	"gopkg.in/yaml.v3"
)

func TestRenderBundleAlignedStemsReconstructMaster(t *testing.T) {
	for _, plain := range []bool{false, true} {
		s := expressionScore()
		s.Plain = plain
		s.Volume = 10000
		s.Mastering = &Mastering{RMSDBFS: -20, PeakDBFS: -1}
		s.Room = &RoomSettings{Preset: "small"}
		s.Sections[0].C2 = []string{"g3", "a3", "b3", "c4"}
		s.Sections[0].C3 = []string{"c2", "z", "g2", "z"}
		doc, _ := yaml.Marshal(s)
		dir := filepath.Join(t.TempDir(), "stems")
		if err := RenderBundle(doc, dir); err != nil {
			t.Fatal(err)
		}
		read := func(name string) []int {
			f, err := os.Open(filepath.Join(dir, name+".wav"))
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			b, err := wav.NewDecoder(f).FullPCMBuffer()
			if err != nil {
				t.Fatal(err)
			}
			return b.Data
		}
		master := read("master")
		sum := make([]int, len(master))
		for _, name := range []string{"C1", "C2", "C3", "room"} {
			x := read(name)
			if len(x) != len(master) {
				t.Fatal("stem misaligned")
			}
			for i, v := range x {
				sum[i] += v
				if v == 32767 || v == -32768 {
					t.Fatal("clipped stem")
				}
			}
		}
		for i, v := range master {
			if math.Abs(float64(v-sum[i])) > 3 {
				t.Fatal("stems do not reconstruct master")
			}
		}
		if err := RenderBundle(doc, dir); err == nil {
			t.Fatal("overwrote existing bundle")
		}
	}
}

package muse

import (
	"math"
	"strings"
	"testing"
)

func TestPansDefaults(t *testing.T) {
	note1 := []note{{}}
	three := &tune{ch1: note1, ch2: note1, ch3: note1}
	two := &tune{ch1: note1, ch2: note1}
	one := &tune{ch1: note1}
	eq := func(got, want []float64) bool {
		if len(got) != len(want) {
			return false
		}
		for i := range got {
			if got[i] != want[i] {
				return false
			}
		}
		return true
	}
	if got := pans(&Score{}, three); !eq(got, []float64{0, 0.35, -0.35, 0}) {
		t.Errorf("three parts: %v", got)
	}
	if got := pans(&Score{}, two); !eq(got, []float64{0.3, -0.3, 0, 0}) {
		t.Errorf("two parts: %v", got)
	}
	if got := pans(&Score{}, one); !eq(got, []float64{0, 0, 0, 0}) {
		t.Errorf("one part: %v", got)
	}
}

func TestPansOverrideAndPlain(t *testing.T) {
	note1 := []note{{}}
	three := &tune{ch1: note1, ch2: note1, ch3: note1}
	got := pans(&Score{Pan: map[string]float64{"C1": -0.5, "C3": 1}}, three)
	if len(got) != 4 || got[0] != -0.5 || got[1] != 0.35 || got[2] != 1 {
		t.Errorf("override: %v", got)
	}
	if got := pans(&Score{Plain: true, Pan: map[string]float64{"C1": 1}}, three); got != nil {
		t.Errorf("plain should give nil, got %v", got)
	}
}

func TestValidateParts(t *testing.T) {
	bad := []struct {
		name string
		s    Score
		want string
	}{
		{"channel", Score{Instruments: map[string]string{"C4": "voice"}}, "C4"},
		{"instrument", Score{Instruments: map[string]string{"C1": "kazoo"}}, "kazoo"},
		{"section channel", Score{Sections: []Section{{Instruments: map[string]string{"D1": "piano"}}}}, "D1"},
		{"section instrument", Score{Sections: []Section{{Instruments: map[string]string{"C2": "nope"}}}}, "nope"},
		{"pan channel", Score{Pan: map[string]float64{"L": 0}}, "unknown channel"},
		{"pan high", Score{Pan: map[string]float64{"C1": 1.5}}, "-1"},
		{"pan low", Score{Pan: map[string]float64{"C2": -2}}, "-1"},
		{"pan nan", Score{Pan: map[string]float64{"C2": math.NaN()}}, "-1"},
	}
	for _, c := range bad {
		err := validateParts(&c.s)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: got %v, want an error mentioning %q", c.name, err, c.want)
		}
	}
	ok := Score{
		Instruments: map[string]string{"C1": "voice", "C3": "guitar"},
		Pan:         map[string]float64{"C1": -1, "C2": 1, "C3": 0},
		Sections:    []Section{{Instruments: map[string]string{"C2": "piano"}}},
	}
	if err := validateParts(&ok); err != nil {
		t.Errorf("valid score rejected: %v", err)
	}
}

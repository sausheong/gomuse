package muse

import (
	"math"
	"reflect"
	"testing"
)

func TestSmallRoomSendsAndEarlyReflections(t *testing.T) {
	l, r := make([]int, 4410), make([]int, 4410)
	l[0], r[0] = 10000, 10000
	var cs [channelCount][]int
	cs[0] = append([]int(nil), l...)
	cfg := &RoomSettings{Preset: "small", Sends: map[string]float64{"C1": 0}}
	a, b := smallRoom(l, r, cs, cfg, .4)
	if !reflect.DeepEqual(a, l) || !reflect.DeepEqual(b, r) {
		t.Fatal("zero send changed dry signal")
	}
	cfg.Sends["C1"] = 1
	a, b = smallRoom(l, r, cs, cfg, .4)
	if a[0] != l[0] || b[0] != r[0] {
		t.Fatal("room changed direct impulse")
	}
	if len(a) <= len(l) || reflect.DeepEqual(a, b) {
		t.Fatal("missing stereo tail")
	}
	for _, x := range a[1:sampleCount(.013)] {
		if x != 0 {
			t.Fatal("wet signal before predelay")
		}
	}
	if a[sampleCount(.013)] == 0 {
		t.Fatal("missing early reflection")
	}
	peak := 0
	for _, x := range a {
		peak = max(peak, int(math.Abs(float64(x))))
	}
	if peak > 10000 {
		t.Fatal("unstable impulse response")
	}
	a2, b2 := smallRoom(l, r, cs, cfg, .4)
	if !reflect.DeepEqual(a, a2) || !reflect.DeepEqual(b, b2) {
		t.Fatal("room is not reproducible")
	}
	if cs[0][0] != 10000 {
		t.Fatal("room mutated source")
	}
}
func TestRoomValidationAndPlain(t *testing.T) {
	for _, cfg := range []*RoomSettings{{Preset: "hall"}, {Preset: "small", Sends: map[string]float64{"bad": .5}}, {Preset: "small", Sends: map[string]float64{"C1": math.NaN()}}, {Preset: "small", Sends: map[string]float64{"C1": 1.1}}} {
		s := expressionScore()
		s.Room = cfg
		if Check(s) == nil {
			t.Fatal("accepted invalid room")
		}
	}
	s := expressionScore()
	a, _ := buildTune(s)
	s.Room = &RoomSettings{Preset: "small"}
	b, _ := buildTune(s)
	x, _ := a.encode()
	y, _ := b.encode()
	if !reflect.DeepEqual(x, y) {
		t.Fatal("plain score acquired room")
	}
}
func TestClarinetRegisterSmoothAndBounded(t *testing.T) {
	last := 0.0
	for f := 200.; f <= 1200; f++ {
		x := clarinetRegister(f)
		if x < last || x-last > .003 || x < 0 || x > 1 {
			t.Fatal("discontinuous register curve")
		}
		last = x
	}
	for _, f := range []float64{196, 440, 880} {
		x := clarinet(f, .3, .7)
		for _, v := range x {
			if math.IsNaN(v) || math.IsInf(v, 0) || math.Abs(v) > 1.000001 {
				t.Fatal("invalid clarinet output")
			}
		}
	}
}

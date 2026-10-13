package muse

import (
	"math"
	"reflect"
	"testing"
)

func TestChannelMixFiltersAndBypass(t *testing.T) {
	signal := func(f float64) []int {
		x := make([]int, sampleCount(.5))
		for i := range x {
			x[i] = int(10000 * math.Sin(2*math.Pi*f*float64(i)/sampleRate))
		}
		return x
	}
	energy := func(x []int) float64 {
		sum := 0.
		for _, v := range x[sampleCount(.1):sampleCount(.4)] {
			sum += float64(v) * float64(v)
		}
		return sum
	}
	x := signal(440)
	y := append([]int(nil), x...)
	processChannelMix(y, ChannelMix{})
	if !reflect.DeepEqual(x, y) {
		t.Fatal("mix bypass changed samples")
	}
	low, high := signal(40), signal(1000)
	beforeLow, beforeHigh := energy(low), energy(high)
	processChannelMix(low, ChannelMix{LowCutHz: 150})
	processChannelMix(high, ChannelMix{LowCutHz: 150})
	if energy(low)/beforeLow > .15 || energy(high)/beforeHigh < .8 {
		t.Fatal("low cut did not selectively remove low frequency energy")
	}
	low, high = signal(200), signal(8000)
	beforeLow, beforeHigh = energy(low), energy(high)
	processChannelMix(low, ChannelMix{HighCutHz: 2000})
	processChannelMix(high, ChannelMix{HighCutHz: 2000})
	if energy(high)/beforeHigh > .2 || energy(low)/beforeLow < .8 {
		t.Fatal("high cut has wrong response")
	}
	if high[len(high)-1] != 0 {
		t.Fatal("filter ends abruptly")
	}
}
func TestZeroChannelLevelStaysMuted(t *testing.T) {
	for _, plain := range []bool{false, true} {
		for _, ins := range []string{"piano", "clarinet", "guitar"} {
			s := gestureScore(ins, NoteExpression{Accents: map[int]float64{1: 6}})
			if ins == "piano" {
				s.Performance = map[string]Performance{"C1": {Pedal: "down"}}
			}
			s.Plain = plain
			s.Levels = map[string]float64{"C1": 0}
			s.Calibration = true
			tune, err := buildTune(s)
			if err != nil {
				t.Fatal(err)
			}
			x, _, err := encodeChannel(tune.ch1)
			if err != nil {
				t.Fatal(err)
			}
			for _, v := range x {
				if v != 0 {
					t.Fatalf("%s muted part produced audio", ins)
				}
			}
		}
	}
}
func TestCalibrationStableBoundedAndMixValidation(t *testing.T) {
	for _, ins := range []string{"piano", "clarinet", "flute", "trumpet", "guitar", "cello", "upright-bass"} {
		s := gestureScore(ins, NoteExpression{})
		if ins == "piano" {
			s.Performance = nil
		}
		tune, err := buildTune(s)
		if err != nil {
			t.Fatal(err)
		}
		a, b := instrumentCalibration(tune.ch1[0]), instrumentCalibration(tune.ch1[1])
		if a != b || a < .5 || a > 2 {
			t.Fatal("calibration varies per note or is unbounded")
		}
	}
	for _, m := range []ChannelMix{{GainDB: 13}, {LowCutHz: math.NaN()}, {LowCutHz: 500, HighCutHz: 200}, {TiltDB: 7}} {
		s := expressionScore()
		s.Mix = map[string]ChannelMix{"C1": m}
		if Check(s) == nil {
			t.Fatal("invalid mix settings accepted")
		}
	}
}

package muse

import (
	"math"
	"reflect"
	"testing"
)

func TestMasteringProtectsPeakAndStereoBalance(t *testing.T) {
	l, r := []int{100000, -100000, 50000}, []int{50000, -50000, 25000}
	masterStereo(l, r, &Mastering{RMSDBFS: -12, PeakDBFS: -1})
	for i := range l {
		if math.Abs(float64(l[i])) > 32767*dBGain(-1)+.5 {
			t.Fatal("peak ceiling exceeded")
		}
		if math.Abs(float64(l[i]-2*r[i])) > 1 {
			t.Fatal("stereo ratio changed")
		}
	}
	if l[0] >= 32767 {
		t.Fatal("clipped before normalisation")
	}
}
func TestMasteringTargetBypassAndSilence(t *testing.T) {
	l, r := []int{1000, -1000}, []int{1000, -1000}
	masterStereo(l, r, nil)
	if !reflect.DeepEqual(l, []int{1000, -1000}) {
		t.Fatal("nil mastering changed output")
	}
	masterStereo(l, r, &Mastering{RMSDBFS: -20, PeakDBFS: -1})
	if math.Abs(float64(l[0])-3276.7) > 1 {
		t.Fatal("missed reachable RMS target")
	}
	l, r = []int{0, 0}, []int{0, 0}
	masterStereo(l, r, &Mastering{RMSDBFS: -20, PeakDBFS: -1})
	if l[0] != 0 {
		t.Fatal("silence changed")
	}
	for _, m := range []*Mastering{{0, -1}, {-20, 0}, {math.NaN(), -1}, {-20, math.Inf(1)}} {
		s := expressionScore()
		s.Mastering = m
		if Check(s) == nil {
			t.Fatal("invalid mastering accepted")
		}
	}
}

func TestMasteringCeilingTakesPriorityOverRMS(t *testing.T) {
	l, r := make([]int, 10000), make([]int, 10000)
	l[0], r[0] = 100000, 100000
	masterStereo(l, r, &Mastering{RMSDBFS: -12, PeakDBFS: -1})
	if math.Abs(float64(l[0])-32767*dBGain(-1)) > .5 {
		t.Fatal("peak ceiling did not limit target gain")
	}
}

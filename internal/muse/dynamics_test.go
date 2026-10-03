package muse

import (
	"math"
	"strings"
	"testing"
)

func dynScore(extra string, secs ...string) string {
	return "key: C\nlength: 0.5\ninstrument: piano\nvolume: 10000\n" + extra + "sections:\n" + strings.Join(secs, "")
}

func sec(dyn, hairpin string) string {
	s := "  - "
	if dyn != "" {
		s += "dynamic: " + dyn + "\n    "
	}
	if hairpin != "" {
		s += "hairpin: " + hairpin + "\n    "
	}
	return s + "C2: [c4, d4, e4, f4]\n"
}

func TestDynamicLevelsAreMonotonicAndCapped(t *testing.T) {
	order := []string{"ppp", "pp", "p", "mp", "mf", "f", "ff", "fff"}
	prev := 0.0
	for i, d := range order {
		g := dBGain(dynamicLevels[d])
		if g <= prev {
			t.Errorf("%s gain %v not above previous %v", d, g, prev)
		}
		if i > 0 {
			step := dynamicLevels[d] - dynamicLevels[order[i-1]]
			if step < 3.9 || step > 5 {
				t.Errorf("step to %s is %v dB", d, step)
			}
		}
		prev = g
	}
	if dBGain(dynamicLevels["fff"]) != 1 || velocityOf(0) != 1 {
		t.Error("fff must be gain 1 and velocity 1")
	}
	if v := velocityOf(dynamicLevels["mf"]); math.Abs(v-0.6) > 0.01 {
		t.Errorf("mf velocity = %v", v)
	}
}

func TestDynamicsSetGainAndVelocity(t *testing.T) {
	_, loud := tuneOf(t, dynScore("", sec("fff", ""), sec("p", "")))
	_, soft := tuneOf(t, dynScore("", sec("p", ""), sec("p", "")))
	if loud.ch2[0].velocity != 1 {
		t.Errorf("fff velocity = %v", loud.ch2[0].velocity)
	}
	if loud.ch2[0].gain > 1 || loud.ch2[0].gain <= loud.ch2[4].gain {
		t.Errorf("fff %v should be louder than p %v and at most 1", loud.ch2[0].gain, loud.ch2[4].gain)
	}
	if soft.ch2[0].velocity >= loud.ch2[0].velocity {
		t.Error("p should have a lower velocity than fff")
	}
}

func TestNoMarkingsLeavesGainsUnchanged(t *testing.T) {
	score := dynScore("", sec("", ""), sec("", ""))
	_, got := tuneOf(t, score)
	for _, n := range got.ch2 {
		if n.velocity != 0 {
			t.Fatalf("velocity set without markings: %v", n.velocity)
		}
	}
	if got.ch2[0].gain < 0.8 {
		t.Errorf("unmarked score made quieter: %v", got.ch2[0].gain)
	}
}

func TestBeforeFirstMarkingIsMF(t *testing.T) {
	_, tu := tuneOf(t, dynScore("", sec("", ""), sec("fff", "")))
	if v := tu.ch2[0].velocity; math.Abs(v-0.6) > 0.01 {
		t.Errorf("velocity before first marking = %v", v)
	}
}

func TestMarkingLastsUntilNext(t *testing.T) {
	_, tu := tuneOf(t, dynScore("", sec("p", ""), sec("", ""), sec("f", "")))
	if tu.ch2[4].velocity != tu.ch2[0].velocity {
		t.Error("p should carry into the unmarked section")
	}
	if tu.ch2[8].velocity <= tu.ch2[4].velocity {
		t.Error("f should be louder than p")
	}
}

func TestHairpinInterpolatesToNextMarking(t *testing.T) {
	_, tu := tuneOf(t, dynScore("", sec("p", "cresc"), sec("f", "")))
	prev := 0.0
	for i := 0; i < 4; i++ {
		v := tu.ch2[i].velocity
		if v < prev {
			t.Errorf("crescendo not rising at note %d", i)
		}
		prev = v
	}
	if tu.ch2[0].velocity != velocityOf(dynamicLevels["p"]) {
		t.Error("hairpin should start at the section level")
	}
	if tu.ch2[3].velocity >= velocityOf(dynamicLevels["f"]) {
		t.Error("hairpin should reach the target only at the next section")
	}
	if tu.ch2[4].velocity != velocityOf(dynamicLevels["f"]) {
		t.Error("next section should be at f")
	}
}

func TestHairpinWithoutTargetMovesOneStepAndCarries(t *testing.T) {
	s, _ := tuneOf(t, dynScore("", sec("mf", "dim"), sec("", ""), sec("", "")))
	start, end, ok := sectionLevels(s)
	if !ok {
		t.Fatal("expected marked")
	}
	if end[0] != dynamicLevels["mf"]-dynamicStep || start[1] != end[0] || start[2] != end[0] {
		t.Errorf("start=%v end=%v", start, end)
	}
	// a cresc at fff cannot go above fff
	s2, _ := tuneOf(t, dynScore("", sec("fff", "cresc")))
	_, e2, _ := sectionLevels(s2)
	if e2[0] != 0 {
		t.Errorf("end = %v", e2[0])
	}
}

func TestPlainIgnoresDynamics(t *testing.T) {
	_, tu := tuneOf(t, dynScore("plain: true\n", sec("ppp", ""), sec("fff", "cresc")))
	for _, n := range tu.ch2 {
		if n.velocity != 0 || n.gain != 1 {
			t.Fatalf("plain note has velocity %v gain %v", n.velocity, n.gain)
		}
	}
}

func TestGainNeverAboveOne(t *testing.T) {
	_, tu := tuneOf(t, dynScore("", sec("fff", "cresc"), sec("fff", "")))
	for _, ch := range [][]note{tu.ch1, tu.ch2, tu.ch3} {
		for _, n := range ch {
			if n.gain > 1 {
				t.Fatalf("gain %v", n.gain)
			}
		}
	}
}

func TestPhraseShapingIsSubtleAndFavoursHighLongNotes(t *testing.T) {
	// the 2-beat g4 ends the first phrase; the rest splits off the last c4
	marked := "key: C\nlength: 0.5\ninstrument: piano\nsections:\n  - dynamic: ff\n    C1: [c4, d4, 2:g4, z, c4]\n"
	bare := "key: C\nlength: 0.5\ninstrument: piano\nsections:\n  - C1: [c4, d4, 2:g4, z, c4]\n"
	_, tm := tuneOf(t, marked)
	_, tb := tuneOf(t, bare)
	if len(tm.ch1) != len(tb.ch1) {
		t.Fatal("note counts differ")
	}
	for i := range tm.ch1 {
		pre := tb.ch1[i].gain * dBGain(-4) // unshaped ff gain
		if g := tm.ch1[i].gain; g > pre+1e-9 || g < pre*0.9-1e-9 {
			t.Errorf("note %d gain %v, want within 10%% below %v", i, g, pre)
		}
	}
	// the lone note after the rest is its own phrase: unshaped
	if g, want := tm.ch1[4].gain, tb.ch1[4].gain*dBGain(-4); math.Abs(g-want) > 1e-9 {
		t.Errorf("note after rest gain %v, want unshaped %v", g, want)
	}
	// the top long note keeps its gain; the low short ones give a little
	if g, want := tm.ch1[2].gain, tb.ch1[2].gain*dBGain(-4); math.Abs(g-want) > 1e-9 {
		t.Errorf("top long note gain %v, want %v", g, want)
	}
	if !(tm.ch1[0].gain < tb.ch1[0].gain*dBGain(-4)) {
		t.Error("low short note should be shaped down a little")
	}
	// unmarked scores are not reshaped
	_, tb2 := tuneOf(t, bare)
	for i := range tb.ch1 {
		if tb.ch1[i].gain != tb2.ch1[i].gain {
			t.Fatal("not repeatable")
		}
	}
	if tb.ch1[0].velocity != 0 {
		t.Error("unmarked score should leave velocity unset")
	}
}

func TestPhraseEndsAfterTwoBeatNote(t *testing.T) {
	// c4 | 2:g4 | c4 d4: the long g4 closes a phrase, so the c4 after it
	// starts a new one and is judged against d4, not against g4
	s := &Score{Length: 0.5, Sections: []Section{{C1: []string{"c4", "2:g4", "c4", "d4"}}}}
	notes := []note{
		{pitch: []int{40}, accidental: []int{0}, length: 0.5, gain: 1},
		{pitch: []int{47}, accidental: []int{0}, length: 1, gain: 1},
		{pitch: []int{40}, accidental: []int{0}, length: 0.5, gain: 1},
		{pitch: []int{42}, accidental: []int{0}, length: 0.5, gain: 1},
	}
	shapePhrases(s, notes)
	if notes[3].gain != 1 {
		t.Errorf("d4 is the top of its own phrase, gain %v", notes[3].gain)
	}
	if notes[2].gain >= 1 {
		t.Errorf("c4 should sit below d4 in its phrase, gain %v", notes[2].gain)
	}
	if notes[1].gain != 1 {
		t.Errorf("g4 tops the first phrase, gain %v", notes[1].gain)
	}
}

func TestSectionsOfDifferentLengthScaleOnEveryChannel(t *testing.T) {
	score := "key: C\nlength: 0.5\ninstrument: piano\nsections:\n" +
		"  - dynamic: pp\n    hairpin: cresc\n    C1: [c4, d4, e4, f4]\n    C2: [c3, 2:g3]\n    C3: [c2]\n" +
		"  - dynamic: ff\n    C1: [c4]\n    C2: [c3]\n    C3: [c2]\n"
	_, tu := tuneOf(t, score)
	for name, ch := range map[string][]note{"C1": tu.ch1, "C2": tu.ch2, "C3": tu.ch3} {
		for _, n := range ch {
			if n.gain > 1 || n.gain <= 0 || n.velocity <= 0 || n.velocity > 1 {
				t.Errorf("%s: gain %v velocity %v out of range", name, n.gain, n.velocity)
			}
		}
	}
	// each channel starts at pp and ends its cresc section nearer ff
	if tu.ch3[0].velocity != velocityOf(-24) {
		t.Errorf("C3 single note starts at pp, velocity %v", tu.ch3[0].velocity)
	}
	if !(tu.ch2[1].velocity > tu.ch2[0].velocity) || !(tu.ch1[3].velocity > tu.ch1[0].velocity) {
		t.Error("hairpin should rise by beat position on C1 and C2 alike")
	}
	if tu.ch1[3].velocity < tu.ch2[0].velocity {
		t.Error("C1's last note is later in the hairpin than C2's first")
	}
}

func TestToneUnchangedWithoutVelocity(t *testing.T) {
	in := []float64{0, 0.5, -0.5, 0}
	out := tone(note{}, 440, in)
	for i := range in {
		if out[i] != in[i] {
			t.Fatal("changed with velocity 0")
		}
	}
}

func sine(freq float64, n int) []float64 {
	x := make([]float64, n)
	for k := range x {
		env := math.Sin(math.Pi * float64(k) / float64(n-1))
		x[k] = env * (math.Sin(2*math.Pi*freq*float64(k)/sampleRate) + 0.8*math.Sin(2*math.Pi*8*freq*float64(k)/sampleRate)) / 1.8
	}
	return x
}

func energy(x []float64) float64 {
	e := 0.0
	for _, v := range x {
		e += v * v
	}
	return e
}

func TestToneSofterIsDarkerAndSafe(t *testing.T) {
	in := sine(220, 20000)
	soft := tone(note{velocity: 0.2}, 220, in)
	med := tone(note{velocity: 0.6}, 220, in)
	loud := tone(note{velocity: 1}, 220, in)
	if len(soft) != len(in) {
		t.Fatal("length changed")
	}
	if !(energy(soft) < energy(med) && energy(med) < energy(loud)) {
		t.Errorf("energy not rising with velocity: %v %v %v", energy(soft), energy(med), energy(loud))
	}
	if energy(loud) < 0.95*energy(in) {
		t.Errorf("fff should be nearly unchanged: %v vs %v", energy(loud), energy(in))
	}
	for _, out := range [][]float64{soft, med, loud} {
		peak, sum := 0.0, 0.0
		for _, v := range out {
			peak = math.Max(peak, math.Abs(v))
			sum += v
		}
		if peak > 1 {
			t.Errorf("peak %v", peak)
		}
		if out[0] != 0 || out[len(out)-1] != 0 {
			t.Error("must start and end at 0")
		}
		if math.Abs(sum/float64(len(out))) > 1e-3 {
			t.Errorf("DC offset %v", sum/float64(len(out)))
		}
	}
	// input untouched
	if in[1] != sine(220, 20000)[1] {
		t.Error("input modified")
	}
}

func TestToneHighPitchIsLeftAlone(t *testing.T) {
	in := []float64{0, 0.3, -0.3, 0}
	out := tone(note{velocity: 1}, 20000, in)
	if &out[0] != &in[0] {
		t.Error("expected unchanged samples when cutoff is above range")
	}
}

func TestValidateDynamics(t *testing.T) {
	for _, tc := range []struct{ dyn, hp, want string }{
		{"loud", "", "section 2: unknown dynamic"},
		{"", "swell", "section 2: unknown hairpin"},
		{"MF", "", "section 2"},
	} {
		s := &Score{Sections: []Section{{}, {Dynamic: tc.dyn, Hairpin: tc.hp}}}
		err := validateDynamics(s)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%+v: err = %v", tc, err)
		}
	}
	ok := &Score{Sections: []Section{{Dynamic: "ppp", Hairpin: "cresc"}, {Dynamic: "fff", Hairpin: "dim"}, {}}}
	if err := validateDynamics(ok); err != nil {
		t.Error(err)
	}
}

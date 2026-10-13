package muse

import (
	"fmt"
	"math"
)

// Mastering uses one shared gain for the entire stereo render, before integer
// clipping. A peak ceiling takes priority over the requested RMS target.
// It does not compress dynamics and is not a perceptual LUFS normaliser.
type Mastering struct {
	RMSDBFS  float64 `yaml:"rms_dbfs"`
	PeakDBFS float64 `yaml:"peak_dbfs"`
}

func validateMastering(s *Score) error {
	if s.Mastering == nil {
		return nil
	}
	if !finiteRange(s.Mastering.RMSDBFS, -60, -6) || !finiteRange(s.Mastering.PeakDBFS, -12, -.1) {
		return fmt.Errorf("mastering requires rms_dbfs -60..-6 and peak_dbfs -12..-0.1")
	}
	return nil
}
func masterStereo(l, r []int, m *Mastering) { scaleStereo(l, r, masteringGain(l, r, m)) }
func masteringGain(l, r []int, m *Mastering) float64 {
	if m == nil || len(l) == 0 {
		return 1
	}
	peak, sum := 0.0, 0.0
	for i := range l {
		for _, sample := range []int{l[i], r[i]} {
			x := float64(sample)
			peak = math.Max(peak, math.Abs(x))
			sum += x * x
		}
	}
	if peak == 0 {
		return 1
	}
	rms := math.Sqrt(sum / float64(2*len(l)))
	return math.Min(32767*dBGain(m.RMSDBFS)/rms, 32767*dBGain(m.PeakDBFS)/peak)
}
func scaleStereo(l, r []int, gain float64) {
	if gain == 1 {
		return
	}
	for i := range l {
		l[i] = int(math.Round(float64(l[i]) * gain))
		r[i] = int(math.Round(float64(r[i]) * gain))
	}
}

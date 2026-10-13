package muse

import "testing"

func TestElectricGuitarSustainAndPitch(t *testing.T) {
	x := electricGuitar(220, 2, 0.7)
	if power(x, 1000, 15000, 220) < 10*power(x, 1000, 15000, 230) {
		t.Fatal("electric guitar is not centred on the requested pitch")
	}
	if rms(x, sampleRate, sampleRate+sampleRate/5) < 0.08*rms(x, 1000, 8000) {
		t.Fatal("electric guitar loses its sustained tone too quickly")
	}
	if x[len(x)-1] != 0 {
		t.Fatal("electric guitar does not end silently")
	}
}

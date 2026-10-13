# Verification

Implemented scope: procedural scat phrases in pure Go; continuous oscillator and tract state; vowel extensions; effort-dependent harmonic balance and mouth shape; short pitch transitions; consonant articulation; breaths in rests; phrase effort arc; per-syllable emphasis; global and section YAML controls; relaxed/playful/assertive comparisons.

Validation on 5 October 2026:

- `go test -race ./internal/muse -run TestScat -count=1`: passed.
- `go test ./... -skip TestSampleScoresProduceValidWAV -count=1`: all non-server packages passed; server tests could not bind loopback in the sandbox. A separate `go test ./internal/server -count=1` with loopback access passed.
- The full-suite run exercised all embedded sample-score renders without sample-score failures. It used an earlier version of the new example test with an overly small allocation cap; that failure was corrected and the example test passed in both focused runs above. Neural sample fixtures use a deterministic fake singer so these checks need no model download.
- `go vet ./...` and `git diff --check`: passed.
- All four demos were freshly rendered after the final synthesis change. WAV measurements are in `metrics.json`: 44.1 kHz, zero clipped samples. Dry comparisons are eight seconds, mono, RMS-matched; the band demo is 26.529 seconds, stereo.

Limits: stylised doo/bah/wah syllables, one procedural vocal channel, no arbitrary English pronunciation or learned human performance. Automated checks cannot establish perceived emotion. The supplied listening examples are the acceptance material for that subjective judgement.

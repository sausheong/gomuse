# Phrase-performance comparison

`before.wav` and `after.wav` are the same 60–84 second chorus excerpt from
`how_deep_is_your_love_chamber_pop.wav` and `how_deep_is_your_love_expressive.wav`.
Both cuts have identical 10 ms edge fades and are matched to -23 dBFS whole-file
RMS, not perceptual LUFS. Neither clips. The full arrangements have identical
musical events and duration; the new version enables stateful clarinet and
violin performance. No other instrument or singing backend is altered.

Listen for sustained connections, smoother note boundaries and swells on held
notes. The new renderer keeps phase, noise, pitch smoothing, harmonic state and
vibrato time across legato notes, and resets after rests or detached notes.
Its fixed gain replaces per-note peak normalisation, so mix balance may differ.
Signal tests establish continuity, timing and bounds; musical preference still
requires listening.

WAVs are local generated files, ignored by Git. Regenerate the full versions
with `go run ./cmd/muse scores/how_deep_is_your_love_chamber_pop` and
`go run ./cmd/muse scores/how_deep_is_your_love_expressive`.

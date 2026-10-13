# Instrument palette expansion

This expansion implements the high- and medium-priority recommendations from the instrument review. The later-priority orchestral and regional instruments remain outside this change.

## Coverage

| Recommendation | Engine names | Evidence |
|---|---|---|
| Trombone and French horn | `trombone`, `french-horn` | Native velocity synthesis, distinct resonance profiles, connected phrasing, tongue and fall gestures |
| Oboe and bassoon | `oboe`, `bassoon` | Native velocity synthesis, double-reed resonance profiles, connected phrasing and tongue gestures |
| String ensemble | `string-ensemble` | Five detuned bow voices with independent noise seeds, attack times and vibrato depths; chord support |
| Nylon-string guitar | `nylon-guitar` | Low-stiffness plucked spectrum and body resonances; strum, let-ring, palm-mute, slide and hammer-on support |
| Tuned percussion | `glockenspiel`, `xylophone`, `tubular-bells` | Distinct modal ratios, strike-strength response and decay profiles |
| Shaker and tambourine | `shaker`, `tambourine` | Bead noise and ringing jingle modes with strike variants |
| Brush kit | `brushtap`, `brushsweep` | Separate tap and sustained sweep; use with existing kick and cymbals |
| Additional percussion | `conga`, `bongo`, `claves`, `triangle` | Membrane, wood and metal modes; strike variants |
| Organ controls | `tone.C1.drawbars`, `tone.C1.rotary` | Nine drawbars plus rotor rate/depth, validated and exercised by audio tests |
| Electric-piano controls | `tone.C1.tremolo` | Rate/depth with score-time LFO phase |
| Synth controls | `tone.C1.synth` | Four waveforms, detune, attack/release, cutoff, resonance and filter-envelope amount/decay |

There are now 29 main pitched instruments, six experimental scat presets and 20 percussion sounds. `sax` remains an alias of `saxophone`. Tone controls use score/section channel maps; section entries replace inherited entries. See the README for units, ranges, defaults and full YAML examples.

## Verification

`palette_expansion_test.go` checks distinct sound output, all added percussion, variable sweep duration, simultaneous hits, new wind/guitar performance, control routing, invalid combinations and parameter ranges. It also checks drawbar harmonic selection, rotary and tremolo modulation, synth wave spectra, cutoff attenuation, resonant emphasis, filter-envelope direction and decay time, attack and detune, bounded samples, silent endpoints and release-budget accounting.

The existing all-instrument tests include every new registration. Their coverage includes native velocity response, short and zero-length notes, low/high pitches, note/chord rendering, normalization and silent endpoints. Mallet brightness tests use each instrument's actual upper mode, rather than assuming a second harmonic. The quality matrix includes the new instruments' intended registers. Drum tests now use a genuinely unknown drum name in rejection cases, since bongo is supported, and provide a duration when testing sweeps.

The full `make check` includes formatting, vet, the package tests and `TestSampleScoresProduceValidWAV`, which renders every embedded YAML score. Neural score fixtures use a deterministic test singer and do not validate a neural model. Focused race tests cover new instruments, controls, percussion and quality matrices.

Final validation on 13 October 2026 passed:

- `GOCACHE=/private/tmp/gomuse-go-cache make check`, including all 68 embedded sample scores and HTTP server tests. The Muse package completed in 540.849 seconds.
- Focused race checks for the expansion, controls, percussion, tuning, quality matrix, gestures and alternating guitar strums.
- `git diff --check` and the independent audio report, with final WAV hashes verified against the saved report.

## Auditions

- `scores/instrument_palette_3.yaml` / `.wav`: all nine new pitched instruments at soft and loud dynamics, eight new percussion sounds, then chamber, bossa and horn/bell ensembles. Original phrases, 76.496 seconds.
- `scores/tone_controls_demo.yaml` / `.wav`: paired default/controlled organ and electric-piano phrases, followed by default and edited synth sounds. Original phrases, 30.460 seconds.
- `examples/palette-expansion-audio.json`: independently measured PCM metadata and SHA-256 hashes for both WAVs. Both are stereo, 44.1 kHz, 16-bit PCM and contain zero clipped samples. Palette peak is −13.978 dBFS; controls-demo peak is −1.993 dBFS.

The palette deliberately leaves headroom for exposed soft/hard comparisons. Audio metrics establish technical properties, not acoustic realism or listening acceptance.

## Boundaries

These remain procedural approximations. The ensemble does not simulate seating or coordinated orchestral bowing, and uses independent polyphonic notes rather than the solo bowed phrase renderer. The organ rotor is a mono Doppler/amplitude approximation, without stereo cabinet acoustics or speed acceleration. Custom organ registrations omit the default key-contact transient. Synth resonance uses a stable linear filter, without analogue saturation; custom patches currently reject sustained `performance` to prevent silent bypass. There are no named patch libraries or arbitrary parameter automation in this change.

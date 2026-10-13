# Muse sound-quality programme

Objective: complete the seven-area instrumental sound-quality programme, verify
its integrated behaviour and deliver `scores/misty_final.yaml` and a fresh WAV.
Singing and articles are excluded; no commit or push is part of this delivery.

## Completion audit

The implementation, render evidence and full-project checks below have passed.
All seven areas and the final audio deliverable are complete within the documented
procedural-synthesis scope. Human listening preference remains a separate judgement.

| Requirement | Implementation and direct evidence | Status |
|---|---|---|
| Every instrument and drums reviewed across dynamics, registers, repetition and releases | `baseline-v1/` and `after-v1/` each contain 19 canonical pitched studies plus drums, with manifests and zero-clipping reports. `gestures/drum-kit.wav` exposes all 12 kit pieces at three dynamics. `TestInstrumentQualityMatrix` covers all canonical presets at three representative registers and velocities. Existing pitch, envelope, family-character and synth alias tests cover tuning, decay and spectral properties. | Verified |
| Family-specific sound refinements | Existing string-body/hammer/pluck, wind/bow, keyboard/mallet and band-limited synth models retained after review. Distinct stateful profiles added for flute, brass/saxophone and cello; connected plucked string renderer; drum velocity now darkens softer strikes at equal gain. `string_body_test.go`, `new_instruments_test.go`, `next_instruments_test.go`, `quality_matrix_test.go` and `TestDrumVelocityColour`. | Verified |
| Idiomatic guitar, bass, brass and bow techniques | `gesture.go`/`plucked_phrase.go`: hammer-on, slide, mute, tongue, fall, bow-change, portamento; existing strum, palm-mute and pedal. Nine ordinary/gesture demonstrations in `gestures/`. Behavioural tests verify changed audio, retained timeline/phase, attack onset, invalid combinations and actual falling brass pitch. | Verified |
| Continuous expression | Phrase gain follows performed time; tone, selective delayed vibrato, breaths, slur groups and attack duration across compatible sustained instruments. `note_expression_test.go`, `advanced_expression_test.go`, `performance_test.go`, `TestSustainedFamiliesShareExpression`. | Verified |
| Shared timing with individual placement | Shared section rubato/ritardando, bounded channel offsets, deterministic variation and chord spreads. `ensemble_test.go` verifies common boundaries, offsets, determinism, monotonic time mapping and chord placement. | Verified |
| Three reviewed arrangements | `scores/misty_final.yaml`, `ensembles/acoustic_ballad.yaml`, `ensembles/fuller_ensemble.yaml`. Register, voicing, bass, density and fill decisions documented in evaluation README; `arrangement-audit.json` proves aligned sections and preserved Misty melody. All three fresh WAVs have zero clipping. | Rendered and structurally verified |
| Mix and acoustics | Optional fixed-reference calibration, channel gain/low-cut/high-cut/tilt, shared small room/sends, RMS target subordinate to peak ceiling. Tests verify response, bypass, mute, calibration bounds, room behaviour, stereo ratio and peak priority. | Verified |
| Reusable authoring and evaluation | Performance/expression presets, live-registry preview CLI, stem bundle CLI, immutable-output comparison tool, hashed PCM reports. Inline/preset equivalence, aligned stem reconstruction and Python tool tests pass. README documents schema and limits. | Verified |
| Final Misty audio and comparisons | `scores/misty_final.wav`: 139.972585 s, stereo 44.1 kHz, peak -1.000152 dBFS, RMS -24.501992 dBFS, no clipped samples. `misty-final-stems/`, `misty-comparison/` and `matched-instruments/` provide exports and common-level comparisons. | Verified |

Evidence paths abbreviated above are relative to `examples/quality-programme/`
unless an explicit source path is given. The top-level evaluation README explains
how to reproduce the outputs and how to interpret the comparisons.

## Verification log

- Focused mix, calibration, mute, drum dynamics and bundle tests passed.
- All Go packages other than the HTTP server passed the initial fast run; the
  server was blocked by sandbox loopback permissions. The full check was then
  launched with local access, rather than treating this as an application defect.
- `TestInstrumentQualityMatrix` and `TestBrassFallLowersAudiblePitch` passed.
- Targeted race tests for sustained profiles, gestures, attack, presets, ensemble,
  mixing, mastering, stems, drums and the quality matrix passed (12.544 s).
- Python audio-tool tests passed, including common peak headroom, matched RMS,
  clipping detection and refusing to overwrite existing output.
- `git diff --check` passed.
- After benchmark: 20 WAVs, zero clipped samples. Gesture/complete-kit studies:
  10 WAVs, zero clipped samples. Final stem bundle: five WAVs, zero clipped samples.
- Full `make check`: passed. Formatting, vet and all packages passed, including
  every sample-score render (Muse package 459.062 s) and HTTP server tests. The
  quality matrix and brass-fall tests added during that run also passed separately
  and under the race detector; subsequent vet and diff checks passed.
- Real Misty stem reconstruction: maximum error two 16-bit PCM units; bundle master
  is byte-identical to `scores/misty_final.wav`.
- All 20 matched instrument pairs and the Misty comparison have matching RMS
  within 0.01 dB and verified source hashes.

## Scope and limits

The dry baseline was saved after the earlier instrument improvement waves. Some
legacy presets therefore remain byte-identical in the final comparison; retaining
a sound after review is deliberate. New phrase behaviour is opt-in and is exposed
in gesture demonstrations instead of changing existing scores silently.

Calibration is a bounded reference-level correction, not perceptual loudness
normalisation. Mastering uses common gain without compression: Misty reaches the
peak ceiling before its requested -23 dBFS RMS, retaining transient dynamics.
RMS-matched comparisons are not LUFS matches. Numerical checks do not establish
human listening preference or equivalence to recordings of acoustic instruments.

Procedural synthesis remains the engine. The sample/hybrid decision and its
licensing/streaming implications are recorded in the evaluation README as a
separate possible project, not an unfinished dependency of this programme.

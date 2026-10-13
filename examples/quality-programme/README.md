# Muse instrument and expression evaluation

The programme combines the earlier instrument refinements with continuous
performance controls, ensemble timing, mixing and reproducible evaluation.
Singing and article work are outside this programme.

## Start here

- `../../scores/misty_final.yaml` and `.wav`: complete clarinet, piano and upright
  bass arrangement. `misty-final-stems/` contains its aligned dry parts and shared
  room return. `misty-comparison/` compares seconds 14–44 with preserved Misty 6,
  at a common RMS level. The timing intentionally differs slightly with rubato.
- `ensembles/acoustic_ballad.yaml` and `.wav`: eight-bar original flute, guitar
  and upright bass ballad. Flute occupies C5–C6, guitar arpeggiates below it,
  bass establishes roots/fifths and approaches the last tonic. Breaths follow
  phrases; the final cadence broadens. Guitar ringing is contained by successive
  string attacks instead of a blanket sustain pedal.
- `ensembles/fuller_ensemble.yaml` and `.wav`: eight-bar original trumpet,
  electric piano, bass and drums study. Compact mid-register voicings leave
  space above for trumpet and below for bass. The first melody enters after
  a rest; the central phrases build to mf. One tom fill leads into the final
  held tonic; the drum part then leaves room for the release. The explicit
  trumpet fall is confined to a phrase ending.
- `gestures/`: exposed ordinary-versus-expressive phrases demonstrating all
  gesture categories. Each YAML is editable and has a corresponding WAV.

## Instrument coverage

`baseline-v1/` and `after-v1/` contain the same dry studies of all 19 canonical
pitched presets and the drum kit. Each pitched study has three registers,
three dynamics, repeated notes, sustain and silence after release.
`matched-instruments/` holds RMS-matched comparisons with common peak headroom.
The baseline was captured after the earlier piano, bass, wind, keyboard, mallet,
string and synth refinements; it is not the original engine. Unchanged legacy
presets therefore deliberately have identical comparisons. New opt-in connected
playing is demonstrated in `gestures/`, not enabled silently in old scores.

The family models retain different mechanisms: piano string beating and hammer
excitation; guitar pluck-position filtering and damping; upright body resonance;
wind register/air/attack profiles; bowed harmonic bodies and sustained envelopes;
electric-piano tine modes; organ drawbars/key contact; vibraphone resonator
modulation; marimba fast upper-mode decay; harp register-dependent ringing;
band-limited synth oscillator banks; and seeded drum noise/modal bodies.
The final drum pass adds softer spectral excitation for quiet strikes.

Automated tests check pitch, finite samples, envelopes, register/dynamic coverage,
synth folded harmonics, gesture pitch motion, determinism, timing and mix behaviour.
Audio reports include file hashes, duration, channels, sample rate, peak, RMS,
crest factor, DC and clipping. These establish technical properties, not a human
listening preference. RMS matching is not perceptual LUFS matching.

## Reproduce

```sh
go run ./cmd/muse-lab -out /tmp/muse-preview-new
go run ./cmd/muse-lab -score scores/misty_final.yaml -out /tmp/misty-stems-new
python3 scripts/audio_report.py /tmp/muse-preview-new --out /tmp/preview-report.json
python3 scripts/match_audio.py before.wav after.wav --out /tmp/comparison-new
```

Use a fresh output directory; benchmark and stem tools refuse to overwrite one.
The incomplete initial `baseline/` directory is not acceptance evidence.

## Procedural synthesis and a future hybrid engine

This programme retains procedural synthesis. It provides small, reproducible
assets and direct control of transitions, but simplified resonances and excitation
cannot reproduce every acoustic instrument or performance technique. The current
benchmarks do not establish a need for one particular sample library. A hybrid
backend should be a separate decision based on matched listening comparisons,
with explicit sample licences, velocity layers, round-robin attacks and streaming
requirements. No third-party samples are required by this delivery.

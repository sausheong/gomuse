# Existing instrument quality pass

`palette.yaml` is an original audition score, captured before and after the
changes. The preserved `before.wav` uses the previous engine. Running the score
again uses the current engine and cannot reproduce that baseline.

Use `before-matched.wav` and `after-matched.wav` for comparison. `compare.py`
pads the shorter tail with silence and matches whole-file RMS to -23 dBFS.
This controls overall level, not perceptual loudness or individual instrument
levels. `metrics.json` records duration, format, gain, peak and clipping counts.

| Start | Audition |
|---|---|
| 0.0 s | Piano |
| 6.4 s | Guitar |
| 12.8 s | Bass |
| 19.2 s | Trumpet |
| 25.6 s | Saxophone |
| 32.0 s | Individual kit pieces, then combined hits |
| 38.4 s | Saxophone, piano, bass and drums ensemble |

Each pitched solo repeats a phrase softly and loudly. Piano hammer brightness,
guitar excitation, bass pickup colour and horn breath now respond inside their
synthesis models. Piano has register-dependent two/three-string unisons;
guitar has broad body resonances. All five use note-off damping and bypass
the redundant generic velocity filter.

Kick and toms have additional head modes; snare adds a short shell transient;
hi-hat and crash add damped metallic modes; ride has more resonant components.
Cowbell uses explicitly bounded odd harmonics. All kit samples have a brief
contact ramp to remove first-sample jumps. Clap and rim retain their previous
body models with that onset correction.

Automated tests check tuning, strike/breath response, decay, spectral balance,
bounded finite samples, sample timing and rendering. These are procedural
approximations; listening is still needed to judge musical realism. Singing
synthesis was left unchanged.

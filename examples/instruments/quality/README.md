# Instrument quality comparison

The same original score, `palette.yaml`, before and after the quality pass.
The baseline WAV was preserved before editing synthesis. Regenerating the
score now uses the revised engine; it cannot recreate the baseline engine.

Listen to `before-matched.wav` and `after-matched.wav`. Both are stereo 44.1 kHz,
53.9777 seconds and matched to -23 dBFS whole-file RMS. The shorter revised
release tail is padded with silence for comparison. This is RMS matching,
not perceptual LUFS matching, and individual instruments may differ in level.
Neither file clips. `metrics.json` contains measurements and `compare.py`
recreates the comparison files from the preserved raw renders.

| Time | Instrument |
|---|---|
| 0–8 seconds | Electric piano |
| 8–16 seconds | Organ |
| 16–24 seconds | Vibraphone |
| 24–32 seconds | Flute |
| 32–40 seconds | Clarinet |
| 40–52 seconds | Three short ensembles, followed by the room tail |

Each solo phrase plays softly and then loudly. Listen for the electric piano's
richer strike and changing decay, the organ's brief contact click and consistent
drawbar colour, the vibraphone's softer high modes, and less abrupt woodwind air
attacks. Notes now damp from note-off instead of sustaining through the release
allowance. Native velocity synthesis bypasses the generic second tone filter.

Automated checks cover tuning, finite bounded output, instrument spectra,
velocity response, sustain/decay, release continuity and render integration.
They do not establish subjective realism. Organ rotary effects, continuous
phrase articulation and sampled acoustic realism are outside this pass.

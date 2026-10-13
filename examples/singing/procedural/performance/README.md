# Hand-shaped performance experiment

Two performances of the same eight notes and syllables in the same register, lasting six seconds each. Both start from the relaxed voice source. There is no neural model, recording, reverb or backing track.

- **Tentative:** a 300 ms hesitation, an extra pause after the high note, slow pressure attacks, shallow vowel opening, breathy releases, and pitch that approaches notes from below before trailing down.
- **Exuberant:** a 50 ms lead-in, compressed pickups, stronger consonants, rapidly opening vowels, pitch overshoots and a sustained final note falling 330 cents.

These are deliberately exaggerated performance studies, not evidence that a listener will perceive those emotions. Pitch and syllable order are identical; rhythm, note durations, articulation and vocal-tract trajectories differ. Both WAVs are mono and matched to whole-file RMS 0.1. This controls overall level, not every moment's loudness or subjective loudness.

Render and prepare:

```sh
for name in tentative exuberant; do
  go run ./cmd/muse examples/singing/procedural/performance/$name
done
python3 examples/singing/procedural/performance/prepare_audio.py
```

The preparation script needs NumPy. `build_scores.py` (PyYAML) reproduces the authored YAMLs. `metrics.json` records the final WAV measurements. Re-render before preparing audio.

## Authoring gestures

Each section can specify `scat.gestures`, with exactly one entry per syllable, including `{}` for rests. Empty entries retain normal behaviour. Each note can have:

- `pitch`: 2–64 evenly spaced points, cents from the score pitch, within −600 to +600. Replaces the automatic scoop/vibrato.
- `mouth`: 2–64 points from 0 (oo) to 1 (ah), overriding the syllable's normal vowel path.
- `effort`, `air`: 2–64 points from 0 to 1, replacing the preset's effort or breathiness trajectory.
- `attack`, `release`: fade durations in seconds, 0–1. Zero retains the ordinary envelope. Fades multiply the existing phrase envelope and consonant closure. When both fades cover a short note, they overlap.

Curves use smooth interpolation with zero slope at each control point. They are local to the note, including `+` extensions; use matching endpoints if continuity matters. Explicit note lengths and rests author hesitation, anticipation and release space without moving the other parts.

Validation checks malformed curves and alignment. A synthesis test verifies the authored pitch actually moves the output fundamental; both example scores are exercised by rendering tests. These do not test perceived emotion.

The next study, [Finding My Feet](../finding_my_feet/README.md), develops this approach into four connected musical phrases, supplied as exposed voice and a quiet jazz arrangement.

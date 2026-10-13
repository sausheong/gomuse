# Finding My Feet

An original four-phrase scat study developing the hand-shaped tentative/exuberant experiment. The melody grows from a hesitant opening into a question, a firmer response and an exuberant high point with a falling final cadence. It uses the existing Go gesture renderer, without adding emotion presets or changing the synthesis engine.

Approximate phrase starts are 0, 6, 12 and 18 seconds. The final phrase slows gently through Muse's existing final ritardando. Written gaps, note lengths, pitch bends, vowel openings, effort, breathiness and releases are authored together. The middle phrases have intermediate and individually adjusted contours; the second ends with a rising question and the third answers with a smaller fall. Only the final cadence uses the broad downward release.

`voice.yaml` exposes the procedural voice without reverb or accompaniment. `band.yaml` uses the same vocal performance with quiet piano, bass and drums, plus light room reverb. `voice.wav` is mono; `band.wav` is stereo. A single gain adjustment per complete recording retains the rise in effort across phrases. Both target whole-file RMS 0.12; this is not a perceptual loudness guarantee.

Reproduce from the repository root:

```sh
python3 examples/singing/procedural/finding_my_feet/build.py
for name in voice band; do
  go run ./cmd/muse examples/singing/procedural/finding_my_feet/$name
done
python3 examples/singing/procedural/finding_my_feet/prepare_audio.py
```

The authoring script requires PyYAML and NumPy; the preparation script requires NumPy. Go synthesis itself requires neither. Re-render before running preparation. Measurements are saved in `metrics.json`; preparation rejects already-clipped input. Both score renders completed successfully. The previous six-second examples are preserved.

This remains a stylised scat voice. The intended emotional arc describes the composition, not an established listener response.

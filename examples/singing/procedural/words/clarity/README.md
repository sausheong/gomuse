# Word clarity comparison

The same five-second phrase, with identical notes, gestures and syllable timing. `before.wav` preserves the version the listener correctly transcribed. `clear.yaml` enables `scat.clarity: 1`.

Clarity is a word-only control from 0 to 1, available globally or per section. It defaults to zero, so existing scores keep their sound. Explicit section zero overrides a global setting.

At maximum clarity:

- The vocal-tract resonances track vowel changes faster, reducing transition blur.
- Upper vowel resonances receive more weight, revealing vowel identity.
- Fricatives and stop releases receive more weight; the low nasal component receives less.
- A 40 ms pressure release separates complete words. A `+` vowel continuation does not introduce another word boundary.

This changes articulation and spectral balance, not just volume. Both delivered files are mono, five seconds at 44.1 kHz, RMS-matched to 0.1, with zero clipped samples. Measurements are in `metrics.json`. The unprompted local recogniser returns the correct words for both; that does not by itself establish a perceived clarity improvement. The listening comparison is the relevant quality check.

Reproduce from the repository root:

```sh
for name in before clear; do
  go run ./cmd/muse examples/singing/procedural/words/clarity/$name
done
python3 examples/singing/procedural/words/clarity/prepare_audio.py
```

The preparation script requires NumPy. Re-render before preparing. Validation covers control bounds and inheritance, rendering, and exact vowel continuity across tied notes with clarity both disabled and enabled. Existing scat tests also run with race detection.

## Compact single-syllable ending

`compact.yaml` responds to the listener hearing “sta-ay”. It shortens “stay” from 1.4 to 0.7 seconds and uses one note instead of a tied two-note figure. The renderer now caps the final vowel change in hey/say/stay at 80 ms, sustaining the main vowel until then. The complete compact clip lasts 4.3 seconds. The previous WAVs remain preserved for listening comparison; rerendering their YAMLs with the updated renderer will also use the new vowel rule.

```sh
go run ./cmd/muse examples/singing/procedural/words/clarity/compact
python3 examples/singing/procedural/words/clarity/prepare_audio.py compact
```

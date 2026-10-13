# Reusable procedural phonemes

This experimental layer accepts explicit pronunciations instead of adding each word to a fixed lexicon. It synthesises consonants, a held vowel, and optional final consonants in Go. Existing scat syllables and the earlier word lexicon remain available.

```yaml
scat:
  channel: C1
  clarity: 0.7
sections:
- C1: [g3, a3, g3, z]
  C2: [4:z]
  scat:
    syllables: [a, live, '+', '']
    phonemes: [[AH], [L, AY, V], [], []]
```

The example pronounces “alive” across two syllables, extends the second syllable over another note, and only releases the final V at the end of that extension. Complete scores also require the usual name, key, length and instrument fields; see `come_alive.yaml`.

## Symbols and alignment

Vowels: `AA AE AH AO EH ER IH IY UH UW EY AY OW AW OY`.

Consonants: `HH M N NG Y W S Z F V L R P B T D K G`.

`phonemes` is an optional list aligned one-to-one with `syllables`. A non-empty pronunciation accepts an arbitrary non-empty syllable label; the label is not converted to speech. Each pronunciation needs exactly one vowel and at most four consonants on either side. Split multisyllable words across notes. Symbols are uppercase without stress numbers. Rests and `+` extensions must use empty phoneme lists. Empty lists on ordinary notes fall back to the existing scat/word lexicon; unsupported labels are rejected.

Consonants have bounded durations. On very short notes the onset and coda are compressed, leaving at least 45 percent of the syllable for the vowel. Diphthong endings stay compact. Tied notes share the complete syllable span, so neither the onset nor the coda is repeated. Explicit coda releases use shorter final fades to avoid masking their release. The oscillator and resonators retain state between notes. Lip, tongue and velar stop models use different noise spectra; liquids, glides and nasals use changing resonance targets. These are approximate synthesis mechanisms, not recorded phonemes.

Pitch, effort, air and clarity controls remain available. Automatic text-to-phoneme conversion is not implemented.

## Original demo

`come_alive.yaml` contains an 18-second original vocal phrase with manually specified pronunciations. `lyrics.txt` holds the words. The WAV is mono, 44.1 kHz, RMS-matched to 0.1 with no clipped samples; see `metrics.json`.

```sh
python3 examples/singing/procedural/phonemes/build.py
go run ./cmd/muse examples/singing/procedural/phonemes/come_alive
python3 examples/singing/procedural/phonemes/prepare_audio.py
```

Python authoring/preparation requires PyYAML/NumPy. Go synthesis does not. Re-render before preparing audio.

## Validation and current limits

Tests cover invalid symbols, missing/multiple vowels, alignment, illegal pronunciations on extensions, bounded consonants on held notes, coda releases, exact continuity of tied syllables, and the complete 27-note demo. The approved compact word sample remains PCM-identical when re-rendered with this layer present.

The new demo is **not yet a successful intelligibility result**. Unprompted recognition catches some phrases and misses others; `asr.json` records the first pass and `asr-second-pass.json` the second revision and `asr-third-pass.json` the current revision. Changes strengthened stops and made stop/liquid clusters transition into the liquid rather than an intervening vowel. They improved recognition of “bright” but did not produce an overall clean transcript. The goal remains open; numerical tests establish correct operation, not clear pronunciation or natural singing.


## Articulation comparison

`comparison/airflow_before.wav` and `airflow_after.wav` contain “fee, see, he, vie, zoo”, with matching pitch, timing and RMS level. `phrase_before.wav` and `phrase_after.wav` compare the complete 18-second demo. All four files are mono 44.1 kHz with zero clipped samples; measurements are in `comparison/metrics.json`.

The third pass lengthens F/S airflow to 120 ms with a sustained noise envelope, and restores familiar H, nasal and glide onset behaviour from the earlier word prototype. A regression test compares their voicing, aspiration, nasal contribution and formant trajectories. Recognition now recovers “We can rise” but still confuses “ride” with “rise” and misses much of the first two lines. This is mixed evidence, not proof of improved perceived clarity. Listener comparison remains necessary.

The subsequent coda pass removes abrupt voicing changes at vowel-to-fricative boundaries and adds continuous closing formant trajectories for final stops. Tests verify vowel continuity and the closure/release distinction between D and Z. `comparison/phrase_before_codas.wav` and `phrase_after_codas.wav` retain matched 18-second renders. The current `come_alive.wav` is this coda revision; `asr-codas.json` records its recognition output. It still substitutes “rise” for “ride”, so this revision is a continuity correction, not a demonstrated intelligibility gain. Listening feedback is outstanding on both comparisons.

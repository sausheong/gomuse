# Procedural word experiment

This extends the existing Go phrase voice with a small explicit pronunciation lexicon. It is not general text-to-speech. Supported lowercase tokens are `hey`, `oh`, `my`, `you`, `stay`, `me`, `we`, `see`, `say`, `no`, `so`, `hi`. They go directly in `scat.syllables`, alongside `doo`, `bah`, `wah`, `+` and empty rests. Unsupported spellings are rejected.

Each word has a bounded consonant onset and a sustained vowel nucleus. Diphthongs change towards their second vowel near the end of the complete word, including tied `+` notes. `stay` has a fricative, stop closure and release; `my`/`me` and `no` have nasal onsets; `you`/`we` have voiced formant transitions; `hey`/`hi` begin with aspiration. All audio is generated from oscillators, deterministic noise and resonators in Go. No sampled speech or neural backend is used by the synthesiser.

Pitch, effort, air and attack/release gestures remain available. For words, a `mouth` curve adds a modest formant shift instead of replacing vowel identity with the old oo-to-ah path. Consonant timing is capped in seconds rather than stretched in proportion to long sung notes. Short notes compress the onset to leave room for the vowel.

The formant-sequence approach and the distinction between consonant and vowel transitions are described in the [eSpeak phoneme documentation](https://espeak.sourceforge.net/phontab.html). The parameters and implementation here are experimental authoring choices, not copied eSpeak code or voice data.

## Reproduce

```sh
python3 examples/singing/procedural/words/build.py
for name in vocabulary phrase check_one check_two check_three first_words; do
  go run ./cmd/muse examples/singing/procedural/words/$name
done
python3 examples/singing/procedural/words/prepare_audio.py
```

The helper scripts need PyYAML and NumPy; the Go renderer does not. Preparation converts to mono and matches whole-file RMS to 0.1. Re-render before preparing. `metrics.json` records measured duration, sample rate, channels, peak and clipping.

Tests cover consonant timing on long notes, late diphthong transitions, exact continuity across tied vowels, score rendering, existing scat regressions and race checks. These establish synthesis behaviour, not intelligibility. Listener transcription and independent recognition remain acceptance checks.

An optional `evaluate.py` uses faster-whisper locally, with English selected but no lyric prompt, hotwords or vocabulary constraint. The evaluator is isolated in `.local/word-eval`; it is not a Go runtime dependency. Recognition of a short synthetic phrase is only supporting evidence and can be wrong or hallucinated.

## Current evidence and limits

The first-word milestone is demonstrated by `first_words.wav`: the five starting words appear in a new, expressive phrase with a vowel carried over two pitches. The local recogniser recovered all five words, without a lyric prompt, both from the raw render and the final prepared WAV. See `asr-new-phrase.json` and `asr-final.json`; the latter includes audio SHA-256 digests.

Recognition is **not reliable across every context**. The repeated six-second `phrase.wav` triggered decoder repetition after level preparation, despite a correct transcription before preparation. The vocabulary list still loses some consonants, and `check_two` gains an unwanted final consonant. These failures are preserved in the reports. `check_one` and `check_three` are correctly recognised. This is evidence for a working small-vocabulary prototype, not proof of general English singing, human realism, or consistent listener intelligibility.

The final controls were chosen after strengthening sibilants, preserving more of the y/w transition, and extending the h breath-to-voicing transition. Earlier reports record unsuccessful revisions rather than concealing them. No recognition text was fed back into synthesis or provided to the decoder as a prompt.

To reproduce the final independent diagnostic (optional Python evaluator plus FFmpeg):

```sh
.local/word-eval/bin/python examples/singing/procedural/words/evaluate.py \
  --model .local/word-eval/base.en \
  --output examples/singing/procedural/words/asr-final.json \
  examples/singing/procedural/words/first_words.wav \
  examples/singing/procedural/words/phrase.wav \
  examples/singing/procedural/words/check_one.wav \
  examples/singing/procedural/words/check_two.wav \
  examples/singing/procedural/words/check_three.wav \
  examples/singing/procedural/words/vocabulary.wav
```

Validation: all scat tests passed with race detection; the broader Muse tests passed excluding the unchanged, slow embedded sample-score suite. The new word-score tests render all six word examples. All prepared files are 44.1 kHz mono with no clipped samples. The final five-word demo is five seconds long.

## Follow-up checks

`asr-current-beam.json` and `asr-current-greedy.json` describe the current final audio, including SHA-256. The five-word demo is correctly decoded with both beam search and greedy decoding. Greedy decoding also recovers the repeated phrase that previously produced a decoder repetition loop. No prompt, hotwords, vocabulary constraints or repetition suppression were used. Changing a decoder is a diagnostic comparison, not an improvement to the sound.

A candidate gave the stop release its own noise resonance. It did not fix the isolated-word error and introduced an unwanted final consonant in one decoding of the five-word phrase. The candidate was rejected and the earlier renderer restored. `asr-stop-release.json` and `asr-stop-candidate-*.json` preserve those results.

`robustness/` and `asr-range.json` record further tests. The current slow, quick, low and high examples do not transcribe reliably. A slow-phrase success occurred with the rejected stop-release candidate; it is not a result for the final renderer. These are failed stress tests, not supported intelligibility guarantees. `asr-range-normalized.json` repeats the check after converting to mono and matching levels; the failures remain. They show why this milestone remains a small-vocabulary sung-word prototype rather than a general English singer.

The initial feasibility objective is demonstrated by the new five-word phrase, which includes all five proposed starting words, authored expression and vowel-only continuation across two pitches. Recognition is independent supporting evidence. Human listening acceptance and naturalness are not claimed, and the recorded broader failures remain open work for extending this prototype.

The overall listening goal remains open. Human transcription has been requested but not received. Decoder settings and small level changes can substantially change ASR results, so further claims about perceived intelligibility need listener evidence rather than choosing only successful recognition outputs.

## Clarity control

[The clarity comparison](clarity/README.md) adds `scat.clarity: 0..1` for sharper word transitions and more prominent consonant/vowel detail. The original sound is preserved at zero. The user correctly transcribed the original five-word phrase; the newer clarity variant is a separate listening comparison.

# Procedural scat voice

The pure Go phrase renderer needs no model, recording or external backend. It generates stylised `doo`, `bah` and `wah` syllables. A small experimental English word layer is also available below; arbitrary lyrics and human voice reproduction remain unsupported.

## Phrase controls

```yaml
name: Phrase example
key: C
length: 0.5
instrument: piano
scat:
  channel: C1
  delivery: playful
  effort: 0.6
  breathiness: 0.15
  articulation: crisp
  phrasing: connected
  breaths: 0.2
sections:
- C1: [g3, a3, g3, z]
  C2: [4:z]
  scat:
    syllables: [doo, '+', bah, '']
    emphasis: [0.3, 0.7, 0.9, 0]
```

`scat.channel` selects one monophonic C1/C2/C3 part, replacing its instrument sound. Every section needs a `scat.syllables` entry for each note, including `''` for rests. `+` extends the previous vowel without repeating its consonant; it cannot start a phrase. Other parts retain their instruments. Neural `singing` and procedural `scat` cannot be combined in one score.

Delivery presets are `relaxed` (default), `playful`, and `assertive`. Each changes source brightness, aspiration, articulation, pitch scoops and vibrato. `effort`, `breathiness` and `breaths` range from 0 to 1; explicit zero disables the corresponding contribution. `articulation` is `gentle` or `crisp`; `phrasing` is `connected` or `detached`. Detached phrasing shortens each note, including extensions; use connected phrasing for sustained melismas. Optional `emphasis` values range from 0 to 1 and must match the syllable count.

Sections can override the same controls and select another delivery. Resolution starts with the selected section or global delivery preset, applies explicit global controls, then explicit section controls. Thus a global explicit effort remains in force when a section changes delivery unless that section overrides effort too.

The renderer preserves oscillator phase and vocal-tract filter state across connected notes, glides into pitches, changes mouth shape and harmonic balance with effort, and adds a phrase effort arc. Consonant closures and bursts articulate new syllables. Breaths fit into written rests of at least 120 ms. It shares the score's key, tempo map and dynamics; vocal timing avoids independent random note jitter. Limits are 10 minutes per score and 30 seconds per uninterrupted vocal phrase. Insert rests for longer songs.

## Listening comparison

`procedural/relaxed.yaml`, `playful.yaml` and `assertive.yaml` contain the same eight-second melody. Their delivered WAVs are mono and RMS-matched to 0.1 so loudness alone cannot explain their differences. `jazz_demo.yaml` puts the three deliveries in sequence over piano, bass and drums.

```sh
for name in relaxed playful assertive jazz_demo; do
  go run ./cmd/muse examples/singing/procedural/$name
done
python3 examples/singing/procedural/prepare_audio.py
```

The final Python step needs NumPy; it prepares listening files, not synthesis. Rerender before preparing them. Measured durations, levels and clipping counts are in `procedural/metrics.json`. Tests cover continuity, spectral changes, delivery differences beyond gain, rest breaths, validation, cancellation and rendering. These checks establish working controls; perceived emotion still needs listening judgement.

## Note-level instruments

The earlier instruments remain available: `scat-doo-relaxed`, `scat-doo-punchy`, `scat-bah-relaxed`, `scat-bah-punchy`, `scat-wah-relaxed`, `scat-wah-punchy`. They restart their articulation on each note. Existing `scores/scat_lab*.yaml` examples are preserved. Prefer the phrase renderer above for connected expression.

For deliberately hand-shaped timing and note-by-note pitch, vowel, effort and breath curves, see the [tentative/exuberant experiment](procedural/performance/README.md). These six-second comparisons go beyond the uniform delivery presets.

An experimental [word layer](procedural/words/README.md) adds a small explicit English vocabulary with consonant onsets and held vowels. General English pronunciation remains unsupported; word intelligibility is still under evaluation.

The [explicit phoneme layer](procedural/phonemes/README.md) supports manually authored pronunciations, final consonants, clusters and multisyllable words split across notes. It includes an original 18-second development demo; broader intelligibility remains work in progress.

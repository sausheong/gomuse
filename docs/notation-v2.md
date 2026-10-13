# Muse 2: write the music once

Use `version: 2` for named tracks, reusable patterns and sections, and an explicit
playback order. Old scores, including `parts`/`playing` scores, remain supported.
The renderer receives an expanded score; your source is never rewritten.

Start with [the short example](../scores/notation_v2_demo.yaml), then
[The Living Years](../scores/the_living_years_expressive.yaml).

```yaml
version: 2
name: A small song
key: C
tempo: 96
meter: 4/4
volume: 5000
tracks:
  melody: {instrument: flute, playing: lyrical-wind}
  keys: {instrument: piano, playing: gentle-piano, level: 0.6}
  kit: {instrument: drums, level: 0.5}
patterns:
  tune: "c5 d5 e5 g5 |"
  harmony: "4:c4-e4-g4 |"
  backbeat: "kick hat snare hat |"
sections:
  verse:
    bars: 1
    dynamic: mp
    parts:
      melody: {use: tune}
      keys: {use: harmony}
  chorus:
    extends: verse
    dynamic: mf
    parts:
      kit: {use: backbeat}
form: [verse, {use: chorus, repeat: 2}]
```

## Tracks and mix

Track names are yours to choose. The first three pitched tracks, in their written
order, map to C1, C2 and C3. The one track with `instrument: drums` maps to the kit
and does not consume a pitched channel. The existing three-pitched-track limit
still applies. Instruments use the existing renderer names.

Each track accepts `instrument`, `playing`, `level` (0–1) and `pan` (−1 to 1).
A playing style is for pitched instruments only. Omitted track levels and pans
use the renderer's existing channel defaults; set them when migrating an exact mix.

Global settings are `name`, `key`, `tempo`, `meter`, `volume`, `humanize`, `swing`
and `reverb`. Tempo is quarter-note BPM, including in compound meters. Meter
is a numerator from 1–32 and a power-of-two denominator from 1–32; default 4/4.
Tempo is required and must be 1–1000. Keep engine-specific mastering, room,
singing and scat configurations in the legacy format for now: version 2 rejects
unsupported fields instead of silently dropping them.

## Notes, bars and phrases

Existing note tokens are unchanged: `c4`, `0.5:d4`, `2:c4-e4-g4`, `z`, `d4@lean`.
Parts accept a string, a phrase object, or a list of strings/phrase objects:

```yaml
melody:
  - notes: >-
      c5 d5 e5 g5 |
      2:e5 2:c5 |
    breath_ms: 100
  - "4:z |"
```

Wrapping a string changes no music. A new list entry is a new phrase, and can
change breaths, slurs and expression. Keep musical phrase boundaries intentional.
The same `unphrased`, `peak_db`, `breath_ms` and `attack_ms` controls described
in the [phrase guide](editable-scores.md) remain available. Rest-only phrases
cannot have sounding-note overrides. Avoid putting internal rests inside a
slurred phrase; split at the rest.

`|` validates a full bar against the meter. Bar counting continues across phrase
entries, so a phrase can end mid-bar. Without separators, only part/section totals
are checked. Every part must have the same duration. A section's `bars` is an
optional additional check. Use `beats` instead for a pickup or irregular section;
omit full-bar separators around that partial measure. Do not specify both.

**Omitted tracks are silent for that section.** Silence lasts as long as the
explicit `bars`/`beats`, or the supplied parts. A completely silent section needs
an explicit duration. In a section variant, `track: off` silences an inherited
part. It does not change that instrument's later parts.

## Patterns and repetition

Patterns use exactly the same musical content as parts. They can contain other
patterns. A reference expands into fresh phrases each time it is used:

```yaml
patterns:
  bass_ab: "a2 e3 a2 e3 |"
  bass_db: "d3 a3 d3 a3 |"
  verse_bass:
    - {use: bass_ab, repeat: 2}
    - {use: bass_db, repeat: 2}

# Inside a section's parts:
bass: {use: verse_bass, repeat: 2}
```

For a sequence made entirely of references, this is equivalent and easier to scan:

```yaml
bass:
  sequence: [{use: bass_ab, repeat: 2}, {use: bass_db, repeat: 2}]
```

A `sequence` item is a pattern name or a `{use, repeat}` reference. A normal list
item that is a string is always **notes**, not a pattern name. This avoids guessing
whether a word names music or references a definition. Drum patterns use the same
reference/sequence syntax but accept only drum notes, not pitched expression.

`repeat` is 1–1024. Pattern/inheritance cycles are errors. Expansion is capped at
200,000 note/bar tokens and 4,096 played sections, before audio is allocated.
Use named references rather than YAML aliases for musical patterns.

## Sections, variants and form

`sections` defines named music. `form` is the order in which it plays:

```yaml
form: [intro, verse, verse_second_ending, chorus, verse, chorus, coda]
```

Form entries accept the same `{use: section_name, repeat: 2}` reference syntax.
A section can extend another section; changed tracks replace that entire track's
part, while unmentioned tracks are inherited. Other section settings inherit:

```yaml
verse_second_ending:
  extends: verse
  parts:
    melody: {use: melody_second_ending}
    kit: off
```

Sections accept `label`, `bars` or `beats`, `dynamic`, `tempo`, `playing`,
`hairpin`, `ritardando`, `rubato`, `fermata` and `parts`. A `playing` override maps
track names to styles. Dynamics persist in playback order, as in legacy Muse.
Other omitted section controls return to score defaults. Inherited settings are
explicitly copied from the parent before those playback rules apply.

Performance boundaries still matter: a section-level crescendo/ritardando applies
across the section, and section starts affect accents. Preserve section boundaries
when migrating for exact performance. Bar-positioned automation within a section
is not part of this version.

## Reusable playing styles

`styles` defines custom styles; built-ins such as `lyrical-wind` remain available.
Styles can `extend` another custom or built-in style. Map fields merge recursively;
lists, including held-note rules, replace the parent's entire list.

```yaml
styles:
  reflective:
    extends: lyrical-wind
    breath_ms: 80
    shape: {peak_db: 0.5}
  chorus:
    extends: reflective
    shape: {peak_db: 1.2, tone: {peak: 0.18}}
marks:
  lean: {accent: 0.9}
```

Track selection uses `playing: reflective`. Marks still attach directly to notes,
so accents move with the notes when a phrase is edited. Instrument compatibility
is validated by the existing performance engine.

## Render, inspect and migrate

```sh
go run ./cmd/muse scores/notation_v2_demo
go run ./cmd/muse -expand /tmp/demo-expanded.yaml scores/notation_v2_demo
```

`-expand` validates and writes a standalone legacy score without synthesising
audio. It refuses to overwrite an existing file, including the source. This is
useful for inspecting references and comparing migrations.

Unknown fields in version 2 are errors. Invalid bars report section, track and
bar; unknown references and cycles fail before rendering. Existing unversioned
YAML uses its original decoder and renderer.

The Living Years retains a phrase-format reference next to the version 2 score.
Its migration preserves notes, phrase expression, section timing and mix, while
sharing repeated accompaniment, drum patterns and section variants. Adjacent
rests in omitted tracks may become a single equal-length rest. The previous
indexed-expression reference is retained separately as well.

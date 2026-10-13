# Editing a musical score

For named tracks, reusable patterns and song form, see [Muse 2](notation-v2.md).
This guide describes the compatible phrase layer also used inside Muse 2 parts.

Start with `scores/misty_final.yaml`. It uses musical phrases and reusable playing
styles, rather than separate event-number tables. The earlier expanded version
and audio are preserved in `examples/quality-programme/authoring-reference/`.

```yaml
name: A short phrase
key: C
length: 0.5
instrument: clarinet
volume: 12000
playing: {C1: lyrical-wind}
marks:
  lean: {accent: 1}
sections:
- label: Opening
  dynamic: mp
  parts:
    C1:
    - notes: c4 d4@lean 2:e4
    - notes: g4 e4 2:c4
      peak_db: 0.3
      breath_ms: 110
```

Each `notes` line is one musical phrase. Separate notes with spaces. The usual
pitch, accidental, chord and duration notation is unchanged. A bare note lasts
one beat, `2:e4` lasts two, and `z` is a rest. Optional standalone `|` separators
help you see bars; they do not create events or enforce bar length. YAML folded
strings (`notes: >-`) let a long phrase span multiple source lines.

Insert or move a note in its phrase: Muse recalculates phrase boundaries, slurs,
held-note expression and end breaths. `d4@lean` keeps the named accent on D even
when earlier notes are inserted. Multiple marks may follow a note; later marks
win for any setting they both specify. Marks can set `accent` in dB, `vibrato`,
`pedal` or `gesture`, using the existing detailed control formats.

A scalar line such as `- c4 d4 2:e4` is shorthand for `- notes: c4 d4 2:e4`.
Use a separate phrase line for each melodic breath. A rests-only line adds no
phrase shaping. For a run of sounding notes that should have no phrase curve,
slur, attack override or end breath, set `unphrased: true`; note-style defaults
and attached marks still apply. Parts in a section must retain matching musical
durations, just as with the original format.

## Playing styles

Select a style per channel with `playing`. A section-level `playing` entry
replaces the score-level choice for that section.

| Style | Behaviour |
|---|---|
| `lyrical-wind` | Legato; phrase volume/tone arch; held notes receive progressively stronger accents and delayed vibrato; 38 ms phrase attack and 65 ms end breath. Use with supported wind instruments. |
| `gentle-piano` | Restrained note accent and pedal on each chord; rests of a quarter beat or shorter lift the pedal. Longer rests allow the preceding chord to ring. |
| `dry-piano` | Pedal up, with no extra accents or phrase shaping. |
| `two-feel-bass` | Subdued default accent. Put a named emphasis on selected bass notes; the style does not invent or move notes. |
| `plain` | No style-generated performance or expression. Global humanisation/mix settings still apply. |

`peak_db`, `breath_ms` and `attack_ms` on a phrase override its style. An explicit
zero breath or attack disables that generated control. `peak_db` requires a style
with a phrase shape; attack and breath require a compatible performance mode.

Define another reusable style under `playing_styles`. A custom definition replaces
a same-named built-in in this score; it does not partially merge with it:

```yaml
playing: {C1: gentle-flute}
playing_styles:
  gentle-flute:
    performance: {articulation: legato, vibrato: 2}
    notes: {accent: -1, vibrato: {depth: 0}}
    held:
    - min_beats: 2
      accent: 0.5
      vibrato: {depth: 5, delay_ms: 350, rise_ms: 500, rate_hz: 4.8}
    shape:
      start_db: -2
      peak_db: 0.5
      end_db: -3
      peak_at: 0.6
      tone: {start: -0.2, peak: 0.2, end: -0.3}
    slur: true
    attack_ms: 40
    breath_ms: 80
```

Held-note rules use written beats, in increasing `min_beats` order. Matching rules
apply after `notes`; attached marks apply last. Optional `pedal_lift_rest` is the
maximum rest length, in beats, that releases piano pedal (zero disables this rule).
Style phrase shapes have no event-number `from` or `to`: Muse derives these from
the phrase's first and last sounding notes.

## Compatibility and verification

Legacy `C1`/`C2`/`C3` lists and indexed expression remain supported unchanged.
Use either `parts` or a legacy note list for a given channel in a section; mixing
both, or attaching an indexed expression table to the same written part, is an
error. Playing styles apply to `parts`. The new format currently supports
instrumental scores; singing and scat continue to use the existing format.

Muse expands the written score in memory and never rewrites the editable source.
Misty's compact score has been checked against the preserved expanded score for
notes, expression and playing controls, then rendered and compared byte for byte.
The authoring change preserves its audio exactly. The defaults are a starting
point for editing, not a claim that one phrasing suits every song.

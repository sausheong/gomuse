# Muse

Muse turns music scores written in YAML into stereo WAV files. It uses nothing but Go and some simple maths: every note is built from sine waves, either shaped by an envelope and coloured by harmonics, or synthesised whole by an instrument (piano, guitar, voice, trumpet, saxophone). A score is then played the way a person would play it, with dynamics, phrasing, loose timing, stereo placement and a room, unless you ask for it exactly as written.

It also comes with `muse-convert`, which turns sheet music (images or PDFs) into scores using Claude's vision.

You can use it as a command-line tool, or as a web app where you write a score, listen to it, download it and share it. Try it live at https://muse.sausheong.com.

The design is explained in these articles:

- [Make Music With Maths and Go (Part I)](https://medium.com/geekculture/make-music-with-maths-and-go-i-abfdbcf73b65): generating notes from sine waves, envelopes and harmonics
- [Make Music With Maths and Go (Part II)](https://medium.com/geekculture/make-music-with-maths-and-go-ii-7d49d438b83c): the score format, the CLI and web app, and sharing through Open Graph

## Quick start

Requires Go 1.26 or later.

```shell
git clone https://github.com/sausheong/gomuse
cd gomuse

go run ./cmd/muse scores/scale   # writes scores/scale.wav from scores/scale.yaml
go run ./cmd/muse -s             # starts the web app on http://localhost:8888
```

Or use the Makefile (`make help` lists every target):

```shell
make build              # builds bin/muse
make run SCORE=scores/tears
make serve              # web app on :8888, tunes stored in ./data
make render             # renders every sample score to scores/*.wav
make build-convert      # builds bin/muse-convert, the sheet music converter
make convert PAGES="notation/misty_*.jpeg" OUT=scores/misty.yaml
make check              # gofmt check, go vet and tests
make build-linux        # static linux/amd64 binary for deployment
```

The templates, static files and sample scores are built into the binary, so `bin/muse` runs from any directory.

## How it works

Each note is turned into 16-bit samples at 44.1 kHz. With an envelope and a harmonic:

```
sample(t) = volume × envelope(t, duration) × harmonic(frequency × t)
```

An instrument instead synthesises the whole note at once, so each overtone can change on its own over the note (see [Instruments](#instruments)).

- **Frequency** uses equal temperament, counted in semitones from A4 (440 Hz): `frequency = 440 × 2^(n/12)`. For example, `a4` is 440 Hz and `c4` is 9 semitones below it, about 261.6 Hz.
- **Envelope** shapes the note's loudness over its duration, for example a fade-out (`drop`) or a swell (`round`).
- **Harmonic** adds overtones to the fundamental frequency to change the timbre.
- **Chords** are the samples of their notes added together.
- **Expression:** unless a score sets `plain: true`, it's played a little like a person would play it, not exactly as written (see [Expression](#expression)).
- **Channels:** a score has up to three channels, `C1`, `C2` and `C3`, usually the melody, the accompaniment and the bass. They're placed across the stereo field (the melody in the middle) and mixed into a stereo WAV; a `plain` score instead puts `C1` on the left, `C2` on the right and `C3` in the centre (see [Singing voice and stereo placement](#singing-voice-and-stereo-placement)). Samples are clamped to the 16-bit range, so loud chords clip rather than wrap around.

## Command-line tool

```shell
muse <path/to/score>    # without the .yaml extension
```

This reads `<score>.yaml` and writes `<score>.wav` next to it, relative to the current directory. On success it prints the tune's name and how long it took. On failure it prints the error to stderr and exits with a non-zero status. There's no length limit on the command line.

## Web app

```shell
muse -s [-addr 0.0.0.0:8888] [-data data]
```

| Flag | Default | Meaning |
|---|---|---|
| `-s` | off | Start the web app instead of rendering a score |
| `-addr` | `0.0.0.0:8888` | Address to listen on |
| `-data` | `data` | Folder for generated tunes and their scores |

| Route | What it does |
|---|---|
| `GET /` | Score editor with links to the sample scores |
| `GET /sample/{name}` | Opens `scores/{name}.yaml` in the editor |
| `POST /create` | Renders the score, then shows a player, download link and share link |
| `GET /share/{id}` | Public page for a tune, with Open Graph `music.song` tags for social previews |
| `GET /static/...` | Embedded CSS, JS and images |
| `GET /static/tunes/{id}.wav` | Generated tunes, served from the data folder |

- **Where files go:**
  - Generated tunes are saved as `<data>/tunes/<id>.wav`, and their scores as `<data>/scores/<id>.yaml`. Both folders are created when the server starts. The default `data/` folder is ignored by git.
  - Tunes keep their `/static/tunes/<id>.wav` URLs, so share links from earlier versions still work.
  - **Upgrading an existing deployment:** tunes used to be stored in `static/tunes/` and `static/scores/` next to the binary. Move them to `<data>/tunes/` and `<data>/scores/` so old share links keep working.
- **Re-creating a tune:** pressing "re-create" overwrites the same tune, but only if your browser holds the signed edit cookie it received when the tune was created. Anyone else, or anyone after a server restart, gets a new tune with a new ID. Shared links can't be used to overwrite someone else's tune.
- **Limits:**
  - Requests are capped at 256 KiB.
  - Each channel can have at most 6,000,000 samples (about 2 minutes 16 seconds). The length is checked before any audio is generated.
  - A chord can have at most 16 notes.
  - Longer tunes can be rendered with the command-line tool.
- **Security:** all output is HTML-escaped, and tune IDs and sample names are validated before they're used in any file path.

## Score format

A score is a YAML document:

```yaml
name: C major scale with C major chord progression
key: C
length: 0.5        # seconds per beat
envelope: round
harmonic: first
volume: 5000
sections: [
  {
    envelope: drawl,        # section overrides
    volume: 2000,
    C1: [c4, d4, e4, f4],
    C2: [2:c4-e4-g4, 2:f4-a4-c4],
  },
  {
    harmonic: second,
    C1: [g4, a4, b4, c5],
    C2: [2:g4-b4-d4, 2:z],
  }
]
```

### Fields

| Field | Level | Meaning |
|---|---|---|
| `name` | score | Title shown in the web app and share previews |
| `key` | score | Key signature (see below) |
| `length` | score, section | Length of one beat in seconds |
| `envelope` | score, section | Envelope name (see below) |
| `harmonic` | score, section | Harmonic name (see below) |
| `instrument` | score, section | Instrument name (see below). Replaces `envelope` and `harmonic`, which can then be left out |
| `volume` | score, section | Peak amplitude of a single note. Every envelope and harmonic peaks at 1, so a note never exceeds `volume`. Chord notes add up, so keep `volume × notes in the largest chord` below 32767 to avoid clipping |
| `reverb` | score | Amount of room reverb, from `0` (none) to `1`. Defaults to `0.25` |
| `plain` | score | `true` plays the score exactly as written: no ring-over, accents, balance, dynamics, timing changes, panning or reverb |
| `humanize` | score | Small random timing variation, from `0` (none) to `1`. Defaults to `0.5` |
| `swing` | score | Swing for off-beat eighths, from `0` (straight) to `1` (triplet feel) |
| `instruments` | score, section | An instrument per channel, e.g. `{C1: voice}` |
| `pan` | score | Stereo position per channel, from `-1` (left) to `1` (right), e.g. `{C1: 0}` |
| `levels` | score | Loudness per channel, from `0` to `1`, e.g. `{C2: 0.6, C3: 0.55}`. Replaces the default balance, and applies even when `plain` |
| `dynamic` | section | Dynamic marking: `ppp pp p mp mf f ff fff`. Lasts until the next one |
| `hairpin` | section | `cresc` or `dim`: moves smoothly towards the next section's dynamic |
| `ritardando` | section | Slows the section gradually: `0.2` ends it 20% slower. Negative speeds up |
| `fermata` | section | Holds the end of the section this many beats longer |
| `sections` | score | List of sections, played in order |
| `C1`, `C2` | section | Notes for the left and right channels |
| `C3` | section | Optional notes for a centre channel, mixed into both sides |

A section inherits every setting it doesn't set itself from the score. If `C2` and `C3` are empty everywhere, `C1` plays on both sides. All channels should be the same length; a difference under 1,500 samples (about 34 ms) is padded with silence, and anything bigger is an error. Each section is best kept to one bar: accents and dynamics follow sections, so a long section only gets one strong downbeat.

### Notes

A note is `<letter><octave>[accidental]`:

- **Letter:** `a` to `g`, lowercase.
- **Octave:** `1` to `7`. `c4` is middle C and `a4` is 440 Hz.
- **Accidental:**
  - `#` sharp, e.g. `f4#`
  - `b` flat, e.g. `b3b`
  - `n` natural, e.g. `f4n`

Other building blocks:

| Syntax | Meaning |
|---|---|
| `z` | A rest |
| `2:c4` | Scales the note's length relative to the beat. Any positive number works, e.g. `0.5:e4` or `0.33:g4` |
| `c4-e4-g4` | A chord: notes joined with `-` play together, up to 16 notes. Each note can have its own accidental, e.g. `c4#-e4-g4#` |
| `3:a3-d4-f4` | A length prefix applies to the whole chord |

### Key signatures

Supported keys: `C`, the sharp keys `G D A E B F# C#`, and the flat keys `F Bb Eb Ab Db Gb Cb`.

The key signature applies to every octave. A note written without an accidental follows the key: in `D`, `f4` plays F♯. A written accidental replaces the key's accidental for that note, so it never adds to it:

- `f4n` plays F natural.
- `f4#` plays F♯.
- `f4b` plays F♭.

### Instruments

An instrument synthesises each note as a whole instead of combining an envelope with a harmonic. That lets every overtone change at its own rate: in a real plucked or struck string the high overtones die away much faster than the fundamental, so the note starts bright and turns mellow; in a horn they grow faster as the player blows harder. Like envelopes, every instrument peaks at 1 and ends at 0.

| Name | Sound |
|---|---|
| `guitar` | A plucked steel string. Plucked a fifth of the way along, so every 5th overtone is missing. The overtones run very slightly sharp, and higher ones fade faster |
| `piano` | A hammered string. The hammer strikes a seventh of the way along and its felt softens the top. Above the bass, two strings tuned 1.6 cents apart beat gently against each other. Each overtone decays in two stages (a quick drop, then a long tail), and higher notes die away faster |
| `voice` | A sung "ah" with vowel formants and a delayed vibrato. Best for melodies; see [Singing voice and stereo placement](#singing-voice-and-stereo-placement) |
| `trumpet` | A bright brass tone. Tongued firmly, starts a little under the note as the lips find the pitch, and is mostly played straight, with only a touch of vibrato on long notes |
| `saxophone` (or `sax`) | A darker, reedy tone with a nasal colour. Slides up into each note, sings with a clear vibrato, and the breath is heard all through the note |

```yaml
instrument: piano     # instead of envelope and harmonic
sections:
  - C1: [d5, e5, f5]
  - instrument: guitar   # a section can switch instrument
    C1: [a4, b4, 2:c5]
```

The trumpet and saxophone share one model (in `internal/muse/winds.go`): blowing harder doesn't just make a horn louder, it makes it brighter, because the upper overtones grow much faster than the low ones. So each note starts dark and opens up as it swells. Use them like the voice, on a melody: `instruments: {C1: trumpet}`, or `muse-convert -melody sax`.

The guitar and piano live in `internal/muse/instrument.go`, and build each note from a list of partials: sine components with a frequency, a level and one or more exponential decay stages. The voice (`voice.go`) and the winds (`winds.go`) are sustained tones whose pitch moves (a scoop into the note, vibrato), so they follow the fundamental's phase and derive the overtones from it. To add an instrument, write a function that returns a normalised note and register it in the `instruments` map, with a ring-over time in `releases`.

### Expression

A score played exactly as written sounds mechanical: every note equally loud, cut off the instant the next one starts, in a room with no echo. Unless a score sets `plain: true`, Muse adds four things a player and a room would:

- **Ring-over:** instrument notes keep sounding after their written length and overlap the notes that follow, the way strings ring on and a pianist's fingers overlap from key to key: 0.8 s for guitar, 0.3 s for piano, about 0.13 s for the voice and saxophone and 0.08 s for the trumpet, for legato. Envelopes are shapes that already end at silence, so they're left alone.
- **Balance:** when a score has more than one part, `C2` is played at 0.8 and `C3` at 0.7 of the volume of `C1`, which usually carries the melody. That's only about 2 dB of difference, so for a singer over a busy accompaniment set your own with `levels`, e.g. `levels: {C2: 0.6, C3: 0.55}`.
- **Accents:** the first note of each section (ideally a bar) is the strongest, notes on a beat are played at 0.92 and notes between beats at 0.85. Each note's loudness is also lowered by up to 5% at random, so repeated notes aren't identical. The randomness has a fixed seed, so a score sounds the same every time it's rendered.
- **Reverb:** a room reverb (after Freeverb) adds a tail of echoes that darken as they fade, slightly different on each side for width. Set the amount with `reverb`.

None of these ever make a note louder than `volume`, but reverb and overlapping tails add to the level, so leave some headroom.

### Dynamics

Give a section `dynamic:` (ppp, pp, p, mp, mf, f, ff, fff) and that level holds until the next marking. Before the first marking the level is mf. Each step is 4 dB, and fff is full `volume`. `hairpin: cresc` or `hairpin: dim` moves the level smoothly, by beat position, from the section's level to the next section's `dynamic:`. If the next section has none, it moves one step up or down and carries on from there.

Softer notes are also darker, as on a real piano, guitar or voice: a velocity-dependent low-pass takes the top off. A score with no markings is left exactly as before. Once a score has any marking, the melody (C1) also gets gentle phrase shaping: the high and long notes of a phrase are slightly fuller, and no note is turned down more than 10%.

`volume` sets the fff level, so a marked score is quieter than the same score unmarked: mf is about 12 dB down, and the darkening takes off a little more. Raise `volume` to make up for it. The converter does this automatically.

### Timing

Unless a score sets `plain: true`, timing is not machine-exact:

- **Humanize:** each note starts a few milliseconds early or late around its place on the beat (about 6 ms spread at `humanize: 1`, never more than 20 ms). The error never accumulates. The melody (C1) leans slightly ahead of the beat. Chords roll lowest note first, over about 5–10 ms for piano and 12–20 ms for guitar (a strum).
- **Swing:** `swing:` delays off-beat eighths by `swing` × 1/6 beat, so `1` gives a 2:1 triplet feel. The on-beat note is held longer by the same amount, so nothing overlaps.
- **Ritardando:** `ritardando: r` on a section slows it gradually, so it ends `r` slower (`0.5` = 50% slower at the end). Negative values, down to just above `-0.5`, speed up; the maximum is `1`. The next section returns to tempo. If the last section sets none, a gentle final ritardando of `0.2` is applied.
- **Fermata:** `fermata: f` (0–8) holds the end of the section `f` beats, lengthening the notes sounding then. All the fermatas in a score may add at most 60 seconds.

All channels share one tempo map, so they stay together at every section boundary. The length limits count the time as played, not as written.

### Singing voice and stereo placement

`instruments: {C1: voice}`, at the score level or in a section, gives a channel its own instrument; keys are `C1`, `C2` or `C3`. A section's own `instrument` beats the score's per-channel map, and a section's per-channel map beats both.

`voice` is a sung "ah": a glottal source shaped by warm vowel formants (about 650, 1080 and 2650 Hz). It fades in softly, holds the pitch steady for the first 0.2 s, then grows a vibrato (about 5.5 Hz, ±45 cents). It has a slight drift and shimmer, and a touch of breath at the onset. Its notes overlap the next by 0.12 s, for legato.

Unless the score is `plain`, the channels are placed in the stereo field instead of hard left and right:

- three channels: C1 in the centre, C2 at +0.35 (right) and C3 at −0.35 (left), like a singer in front of a piano
- two channels: C1 at +0.3 and C2 at −0.3
- one channel: centre

`pan: {C1: 0, C2: 0.5}` overrides any channel. The default layout is a little louder than plain mixing, because a centred channel plays at full level on both sides.

### Envelopes

Every envelope stays between 0 and 1, peaks at 1, and ends at 0, so notes never click.

| Name | Shape |
|---|---|
| `flat` | Constant volume, with a 10 ms fade at the end |
| `drop` | Starts loud and fades out (a quarter cosine) |
| `rise` | Starts silent and swells to full volume, then a 10 ms fade at the end |
| `round` | Swells, then fades (a half sine) |
| `triangle` | Rises linearly to the middle of the note, then falls linearly |
| `tadpole` | A quick attack (the head), then a rippling decay to silence (the tail) |
| `combi` | A mix of `round` and `drop` |
| `diamond` | A tall swell followed by a smaller second one |
| `drawl` | Decays quickly, then slowly, lingering before it fades out |
| `tempered` | `drop` shaped by `drawl`'s curve, a soft plucked decay |

### Harmonics

Each harmonic is scaled to peak at 1.

| Name | Waveform |
|---|---|
| `first` | Pure sine: the fundamental only |
| `second` | Fundamental plus the 2nd harmonic |
| `third` | Fundamental plus the 2nd and 3rd harmonics |
| `stringed` | Harmonics 1–4 weighted 3 : 1.5 : 0.25 : 0.125, for a plucked-string sound |

Envelopes live in `internal/muse/envelope.go` and harmonics in `internal/muse/harmonic.go`. Each is a small pure function registered in a map, so adding one takes a few lines.

## Converting sheet music

`muse-convert` transcribes sheet music into a score. It sends each page to Claude, which reads the notation and writes it out in the score format; the converter then checks the result and asks for fixes before writing the file.

```shell
make build-convert
bin/muse-convert -o scores/misty.yaml notation/misty_01.jpeg notation/misty_02.jpeg notation/misty_03.jpeg
bin/muse scores/misty
```

- **Input:** PDF, PNG, JPEG, GIF or WebP files of one piece, given in playing order. Each file is transcribed in one request; a PDF goes in whole, so split very long PDFs into several files. Images larger than the model needs (over 2576 px on the long side, or over 3.5 MB) are shrunk first.
- **Staves to channels:** each staff of a system becomes a channel, counted from the top. One staff gives `C1`; two staves (a piano score) give `C1` and `C2`; three staves (voice and piano) give `C1`, `C2` and `C3`. More than three staves put the rest into `C3` as chords. A staff that is missing for a few bars, like the voice in an introduction, is filled with rests so the channels stay in step.
- **Checks:** every page must parse, every note must be valid, every channel used in the score must appear in every section, and the channels of each section must add up to the same number of beats. A page that fails is sent back with the list of problems, up to `-retries` times. Anything still wrong is listed on stderr and the command exits with status 1, but the score is written anyway so you can fix it by hand.
- **Settings:** the name, key and tempo come from the music, along with dynamics, hairpins, ritardandos and fermatas. With three staves, the top one (the voice) is sung with the `voice` instrument; change that with `-melody`, e.g. `-melody sax`. The instrument (`piano` by default) and volume come from flags; by default the volume is the loudest the widest chords allow without clipping.
- **Accuracy:** the transcription is good but not exact. Listen to the result and compare it with the music, especially accidentals, ties and dense chords.

| Flag | Default | Meaning |
|---|---|---|
| `-o` | `<first input>.yaml` | Output score file |
| `-env` | `.env` | File with `ANTHROPIC_*` settings. Variables already in the environment win |
| `-model` | `$ANTHROPIC_MODEL`, else `claude-opus-5-5` | Claude model |
| `-effort` | `high` | Thinking effort: `low`, `medium`, `high`, `xhigh` or `max`. Empty uses the model's default |
| `-retries` | `2` | Times to ask for fixes when a page fails the checks |
| `-name` | title on the music | Score name |
| `-instrument` | `piano` | Instrument. Set it to empty (`-instrument=`) to use `-envelope` and `-harmonic` |
| `-envelope`, `-harmonic` | `tempered`, `stringed` | Used when `-instrument` is empty |
| `-melody` | `voice` | Instrument for the top staff when there are three staves, e.g. `trumpet` or `sax`. Empty to use `-instrument` |
| `-volume` | auto | Volume |
| `-q` | off | Don't print progress |

Credentials come from `ANTHROPIC_API_KEY`, or `ANTHROPIC_AUTH_TOKEN` for a proxy, and `ANTHROPIC_BASE_URL` points it at a proxy such as LiteLLM. Keep them in `.env`, which git ignores:

```shell
ANTHROPIC_AUTH_TOKEN=...
ANTHROPIC_BASE_URL="https://your-litellm-proxy"
ANTHROPIC_MODEL="claude-opus-5-5"
```

- **Other models:** through a proxy like LiteLLM, which translates the Anthropic API, `-model` can name any vision model the proxy serves, such as a Gemini model. Claude has read pitches more accurately in practice.
- **Failures:** each request is tried three times. A provider's content filter can block a transcription of published music; a request that keeps failing stops the conversion. The pages already done are still written, and the command says which page it stopped at and exits with status 1.

## Sample scores

The `scores/` folder contains these scores. Those marked † are longer than the web app's limit (about 2 minutes 16 seconds), so render them with the command-line tool.

| File | Tune | Key | Sound |
|---|---|---|---|
| `scale.yaml` | C major scale with a C major chord progression | C | round / first |
| `after_all.yaml` | After All † | D | tempered / stringed |
| `after_all_guitar.yaml` | After All, first chorus | D | guitar |
| `after_all_piano.yaml` | After All, first chorus | D | piano |
| `after_all_voice.yaml` | After All, the whole arrangement, converted by `muse-convert` † | D | voice and piano |
| `beauty.yaml` | Beauty and the Beast | F | round / first |
| `cheek.yaml` | Cheek to Cheek | C | drop / stringed |
| `every.yaml` | Every Breath You Take | A | tempered / stringed |
| `how_deep_is_your_love.yaml` | How Deep Is Your Love, arranged with sustained verses, at 88 bpm † | E♭ | saxophone and piano |
| `march.yaml` | Turkish March | C | drop / stringed |
| `memory.yaml` | Memory | C | round / first |
| `misty.yaml` | Misty, converted by `muse-convert` † | E♭ | voice and piano |
| `sweet.yaml` | Sweet Child O' Mine | D | drawl / stringed |
| `tears.yaml` | Tears in Heaven (excerpt) | D | tempered / stringed |
| `tears.full.yaml` | Tears in Heaven (full) † | D | tempered / stringed |
| `the_light_you_left.yaml` | The Light You Left, an original piano miniature | F | piano |
| `velvet_after_midnight.yaml` | Velvet After Midnight, an original jazz miniature with swing | C | voice and piano |

In the web app, open any of these at `/sample/<name>`, e.g. `/sample/tears`. Every `.yaml` file in `scores/` is built into the binary, so whatever is there is served by the web app.

## Project layout

```
cmd/muse/            entry point: CLI rendering and the -s web app flags
cmd/muse-convert/    entry point: sheet music to score converter
internal/muse/       audio engine
  parser.go            YAML scores, notes, chords, length checks
  encoder.go           pitches, key signatures, synthesis of notes, rests and chords
  envelope.go          envelope functions
  harmonic.go          harmonic functions
  instrument.go        guitar and piano instruments
  expression.go        ring-over, balance and accents
  dynamics.go          dynamic markings, hairpins, tone of soft notes
  timing.go            tempo map, ritardando, fermata, swing, humanize
  voice.go             the voice instrument and stereo placement
  winds.go             trumpet and saxophone
  reverb.go            room reverb
  wav.go               stereo interleaving, clipping, WAV writing
internal/convert/    sheet music transcription with Claude
  convert.go           requests, page continuation, fix-up loop
  prompt.go            the score format and staff-to-channel rules for the model
  score.go             assembling pages, checks, automatic volume
  image.go             shrinking oversized images
  env.go               reading .env
internal/server/     web app: routes, handlers, edit cookies
web/                 embedded web assets
  templates/           HTML page templates
  static/              CSS, JavaScript (CodeMirror editor), images
scores/              sample scores (also embedded for /sample/{name})
notation/            sheet music pages to convert
Makefile             build, run, test and release tasks
```

Dependencies: [go-audio/wav](https://github.com/go-audio/wav), [rs/xid](https://github.com/rs/xid), [yaml.v3](https://gopkg.in/yaml.v3) and, for `muse-convert`, [anthropic-sdk-go](https://github.com/anthropics/anthropic-sdk-go).

## Development

```shell
make check              # gofmt check, go vet, tests
make race               # tests with the race detector
make cover              # coverage report
```

Every test is hermetic: it writes only to temporary folders. The tests cover:

- pitch and frequency maths
- exact sample counts
- clipping
- key signatures and accidentals
- parser errors
- rendering every sample score and decoding the WAV back
- instruments: normalisation and tuning for all of them, decay for the guitar and piano, formants and vibrato for the voice, brightening and vibrato for the winds
- expression: ring-over, balance, levels, accents, dynamics and hairpins, the tone of soft notes, the tempo map (ritardando, fermata, swing, humanize), panning and reverb, and that `plain` turns them off
- the converter's checks, page assembly, automatic volume, image shrinking and `.env` loading (the calls to Claude aren't tested)
- the web handlers via `httptest`, including escaping, ID validation, 404s and size limits

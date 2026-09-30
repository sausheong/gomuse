# Muse

Muse turns music scores written in YAML into stereo WAV files. It uses nothing but Go and some simple maths: every note is a sine wave, shaped by an envelope and coloured by harmonics.

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
make check              # gofmt check, go vet and tests
make build-linux        # static linux/amd64 binary for deployment
```

The templates, static files and sample scores are built into the binary, so `bin/muse` runs from any directory.

## How it works

Each note is turned into 16-bit samples at 44.1 kHz:

```
sample(t) = volume × envelope(t, duration) × harmonic(frequency × t)
```

- **Frequency** uses equal temperament, counted in semitones from A4 (440 Hz): `frequency = 440 × 2^(n/12)`. For example, `a4` is 440 Hz and `c4` is 9 semitones below it, about 261.6 Hz.
- **Envelope** shapes the note's loudness over its duration, for example a fade-out (`drop`) or a swell (`round`).
- **Harmonic** adds overtones to the fundamental frequency to change the timbre.
- **Chords** are the samples of their notes added together.
- **Channels:** the two channels, `C1` and `C2`, are interleaved into a stereo WAV. Samples are clamped to the 16-bit range, so loud chords clip rather than wrap around.

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
| `volume` | score, section | Amplitude. Keep the total below 32767 to avoid clipping; chords and the richer harmonics add up |
| `sections` | score | List of sections, played in order |
| `C1`, `C2` | section | Notes for the left and right channels |

A section inherits every setting it doesn't set itself from the score. If `C2` is empty everywhere, `C1` plays in both channels. Both channels should be the same length. A difference under 1,500 samples (about 34 ms) is trimmed; anything bigger is an error.

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

### Envelopes

| Name | Shape |
|---|---|
| `flat` | Constant volume |
| `drop` | Starts loud and fades out (a quarter cosine) |
| `rise` | Starts silent and swells (a quarter sine) |
| `round` | Swells, then fades (a half sine) |
| `triangle` | One full triangle-wave cycle: up, down through zero to negative, and back |
| `tadpole` | Swells slowly, peaks near the end, then cuts off sharply |
| `combi` | A mix of `round` and `drop` |
| `diamond` | A mix of two triangle waves |
| `drawl` | A slow logarithmic decay |
| `tempered` | `drop` × `drawl`, a soft plucked decay |

### Harmonics

| Name | Waveform |
|---|---|
| `first` | Pure sine: the fundamental only |
| `second` | Fundamental plus the 2nd harmonic |
| `third` | Fundamental plus the 2nd and 3rd harmonics |
| `stringed` | A weighted mix of a sub-harmonic and harmonics 1–4, for a plucked-string sound |

Envelopes live in `internal/muse/envelope.go` and harmonics in `internal/muse/harmonic.go`. Each is a small pure function registered in a map, so adding one takes a few lines.

## Sample scores

The `scores/` folder contains:

| File | Tune | Key | Envelope / harmonic |
|---|---|---|---|
| `scale.yaml` | C major scale with a C major chord progression | C | round / first |
| `beauty.yaml` | Beauty and the Beast | F | round / first |
| `cheek.yaml` | Cheek to Cheek | C | drop / stringed |
| `every.yaml` | Every Breath You Take | A | tempered / stringed |
| `march.yaml` | Turkish March | C | drop / stringed |
| `memory.yaml` | Memory | C | round / first |
| `sweet.yaml` | Sweet Child O' Mine | D | drawl / stringed |
| `tears.yaml` | Tears in Heaven (excerpt) | D | tempered / stringed |
| `tears.full.yaml` | Tears in Heaven (full) | D | tempered / stringed |

In the web app, open any of these at `/sample/<name>`, e.g. `/sample/tears`.

## Project layout

```
cmd/muse/            entry point: CLI rendering and the -s web app flags
internal/muse/       audio engine
  parser.go            YAML scores, notes, chords, length checks
  encoder.go           pitches, key signatures, synthesis of notes, rests and chords
  envelope.go          envelope functions
  harmonic.go          harmonic functions
  wav.go               stereo interleaving, clipping, WAV writing
internal/server/     web app: routes, handlers, edit cookies
web/                 embedded web assets
  templates/           HTML page templates
  static/              CSS, JavaScript (CodeMirror editor), images
scores/              sample scores (also embedded for /sample/{name})
Makefile             build, run, test and release tasks
```

Dependencies: [go-audio/wav](https://github.com/go-audio/wav), [rs/xid](https://github.com/rs/xid) and [yaml.v3](https://gopkg.in/yaml.v3).

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
- the web handlers via `httptest`, including escaping, ID validation, 404s and size limits

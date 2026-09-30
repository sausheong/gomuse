# Muse

Create music using Go and YAML-based scores. Check it out live at https://muse.sausheong.com.

Also check out these articles:

- [Make Music With Maths and Go (Part I)](https://medium.com/geekculture/make-music-with-maths-and-go-i-abfdbcf73b65)
- [Make Music With Maths and Go (Part II)](https://medium.com/geekculture/make-music-with-maths-and-go-ii-7d49d438b83c)

## Build & run

Requires Go 1.26+.

```shell
go build .              # build the muse binary

go run . scores/scale   # create scores/scale.wav from scores/scale.yaml
go run . -s             # start the web app on :8888
```

The command line tool reads `<name>.yaml` and writes `<name>.wav` next to it,
relative to the current directory. The web app serves the "try it" page and
lets you create, download and share tunes.

## Score format

A score is YAML with a name, a key signature, defaults for length/envelope/
harmonic/volume, and a list of sections. Each section has two channels, `C1`
and `C2`, for stereo. A section (and its notes) can override any of the
score-level defaults.

```yaml
name: C major scale
key: C
length: 0.5      # seconds per note, unless overridden
envelope: round   # shape of the note - see envelope.go for the full list
harmonic: first   # harmonics added to the fundamental - see harmonic.go
volume: 5000
sections: [
  {
    C1: [c4, d4, e4, f4],
    C2: [2:c4-e4-g4, 2:f4-a4-c4],
  }
]
```

Each note is `<pitch><octave>`, e.g. `c4`, optionally followed by an
accidental:

- `#` sharp, e.g. `f4#`
- `b` flat, e.g. `f4b`
- `n` natural, e.g. `f4n` - cancels a sharp/flat coming from the key
  signature for just that note
- `z` a rest

A note can be prefixed with `<multiple>:` to scale its length relative to the
section/score default, e.g. `2:c4` plays for twice as long. Several pitches
joined with `-`, e.g. `c4-e4-g4`, play together as a chord.

See `envelope.go` and `harmonic.go` for the full list of envelope shapes and
harmonics you can use.

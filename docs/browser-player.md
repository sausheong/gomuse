# Browser player

Muse can synthesize scores entirely in the browser. The same Go engine is
compiled to WebAssembly and runs in a worker; an AudioWorklet plays stereo PCM
from memory. Scores and audio stay in the browser. No rendering service, plugin,
or instrument downloads are required after the player assets load.

## Build and open

With Go and Make installed:

```sh
make serve
```

Open `/player` on the Muse server, or use **Play in browser** in the existing
score editor. That link carries the current editor contents into the player.
`make build` and `make build-linux` build and embed the browser assets too.
Direct `go build` users must run `make player` first.

For a standalone player:

```sh
make player-bundle
python3 -m http.server 8000 --bind 127.0.0.1 --directory bin/player
```

Open `http://localhost:8000`. Deploy the contents of `bin/player` to any HTTPS
static host to serve the same player without a Go server. Serve `.wasm` as
`application/wasm` where possible. AudioWorklet requires HTTPS or localhost;
opening `index.html` as a `file:` URL is unsupported. This is a static bundle,
not an installable offline PWA; it has no service worker or persistent cache.

The generated `muse.wasm` and matching `wasm_exec.js` are build outputs and are
not checked in. Rebuild both together when changing Go versions or engine code.

## Playback modes and controls

- **While rendering** synthesizes successive blocks and begins playback after
  roughly one second of audio is ready. Voice continuity, overlapping releases,
  timing, channel filters, panning and room state persist across blocks.
- **After the full render** uses the existing export renderer in memory,
  including whole-song RMS mastering. It starts after rendering completes.
- Pause and resume retain the playhead. Stop returns to the start while any
  background render continues. Cancel discards the render without changing YAML.
- Seeking within rendered audio is immediate. Seeking ahead waits for those
  samples; the playhead does not advance through an underrun.
- Volume, pan and brightness change the live output without re-rendering.
  They do not edit the score or affect the downloaded WAV.
- Download WAV saves the rendered audio as 44.1 kHz, stereo, 16-bit PCM. Save YAML
  saves the editor contents. Both are local browser downloads.

## Studio workspace

The score editor marks changes made since the last render. The persistent
transport shows a waveform from the rendered PCM and keeps playback available
while editing. Listening controls and expandable render settings sit beside
the editor on desktop and below it on narrow screens.

Press Command/Ctrl + Enter to render. Press Space outside an input, button or
disclosure to pause or resume. These shortcuts leave normal editor typing and
native control keyboard behaviour intact.

## Current boundaries

The browser accepts scores up to 2 MB and ten minutes of written/performed
music, with an additional limit of two hours of synthesized voices per
channel. Release and room tails can extend the final duration. Rendered PCM is
retained in memory for seeking and WAV downloads, so this is progressive
synthesis and playback rather than an unbounded network audio stream.

Progressive mode omits whole-song mastering because its loudness measurement
requires the complete song. Choose the full-render mode when matching a mastered
CLI export matters. Progressive mode also retains terminal tail silence that
export may trim. Live output controls and device resampling can affect what you
hear without changing rendered PCM.

Individual notes/chords are still synthesized as units. Long or dense events
can delay a block, and slower devices may buffer. Scat vocals are supported but
require their vocal track to be prepared first. Neural singing depends on a
local external model and is not available in the browser; render those scores
with the CLI and play the resulting WAV.

## Verification

```sh
make player-test  # requires Node.js; actual WASM engine and worklet checks
make check        # Go formatting, vet and complete test suite
```

Go tests compare progressive PCM with offline PCM across block sizes, instruments,
connected voices, channel filters and rooms. The JavaScript checks exercise the
actual WebAssembly engine and AudioWorklet pause, resampling, seeking, underruns
and completion. Browser checks cover the player controls and error recovery.

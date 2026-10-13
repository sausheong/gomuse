# Optional neural singing (experimental)

Muse can replace one melody channel with locally rendered neural singing. The
Go binary remains self-contained for ordinary scores. Only the CLI explicitly
starts a backend; the web server never runs a command supplied by a score.

## Current verification status

The complete CLI path has rendered the original demo with **Nishiren Gard AI
DiffSinger v1.2 by Gardanana**, its bundled PC-NSF-HiFiGAN vocoder, and its learned
duration, pitch, and tension predictors on macOS arm64. The updated render took about eight seconds.
The mix is 21.446 seconds, stereo 44.1 kHz, peak 31,367/32,768, with zero clipped
samples. The raw vocal is 20.3 seconds, mono 44.1 kHz, peak 27,852/32,768, with
zero clipped samples. The mix includes instrumental and reverb tails.

The Go integration, English phoneme preparation, and predictor input handling
have automated tests. The adapter performs real ONNX inference; it does not
substitute speech or a test waveform. **Listening acceptance remains open**:
these measurements do not establish intelligibility or naturalness. Leif DS v111
Lush has also passed a real-model render; see its setup below. Other voicebanks
have not been tested.

## Install the optional runtime

Run from the repository root, using Python 3.13 (tested on macOS arm64):

```sh
uv venv --python python3.13 .local/singing/venv
uv pip install --python .local/singing/venv/bin/python -r scripts/singing/requirements.txt
```

Install an OpenUtau-style DiffSinger ONNX voicebank and a matching ONNX vocoder
under `.local/singing/`. Models and the Python environment are git-ignored.
The adapter supports a 44.1 kHz acoustic model with `tokens`, `durations`, `f0`,
and optionally `languages`, `spk_embed`, `gender`, `velocity`, `steps`, `speedup`,
and `depth`. Additional `energy`, `tension`, `breathiness`, or `voicing` inputs
are predicted automatically when the bank includes a compatible `dsvariance`
encoder and predictor. Bundled `dsdur` and `dspitch` models are also used
automatically when both are present. Explicit model-appropriate constant values can override
them via `--controls`. Other
required inputs fail clearly. Models requiring separate shallow-diffusion inputs
are not supported by this first adapter.

Voicebank for the local demo: [Nishiren Gard AI DiffSinger v1.2 by Gardanana](https://github.com/Gardanana/Nishiren-AI-Diffsinger/releases/tag/v1.2).
This release includes its matching PC-NSF-HiFiGAN vocoder. Keep the archive and
its credits intact; the adapter reads the published model files without editing
them. [Voicebank terms](https://github.com/Gardanana/Nishiren-AI-Diffsinger/blob/main/terms-of-service.md)
permit non-commercial use, require the voicebank name when publishing, and
require individual approval for commercial use. Do not use generated audio as
training data without the author's approval.

```sh
curl -L --fail --continue-at - --output .local/singing/nishiren.zip \
  https://github.com/Gardanana/Nishiren-AI-Diffsinger/releases/download/v1.2/Nishiren.Diffsinger.v1.2.zip
unzip .local/singing/nishiren.zip -d .local/singing/
```

The downloaded v1.2 archive passed all ZIP CRC checks. Its locally measured
SHA-256 is `402bed565c6c78226ca9249b889d7c703534c186feb8615b93ac8a6edb121040`
(a reproducibility record, not a publisher-signed checksum).

LIEE was considered first, but its bundled terms prohibit AI-generated music;
it is not the voicebank used for this AI-authored demonstration.

Vocoder releases: [OpenVPI vocoders](https://github.com/openvpi/vocoders/releases).
Use the version required by the bank, including matching mel parameters and
logarithm base. The adapter converts between `mel_base: e` and `mel_base: '10'`;
missing bases default to `e`, so explicitly annotate old log10 configurations.
OpenVPI publishes these pretrained vocoders under CC BY-NC-SA 4.0; this local
experiment does not establish suitability for commercial distribution. Voicebank
terms are separate from the inference code's licence.

After extraction, create a configuration using the actual paths:

```sh
.local/singing/venv/bin/python scripts/singing/configure.py \
  --voicebank ".local/singing/Nishiren Diffsinger v1.2" \
  --vocoder ".local/singing/Nishiren Diffsinger v1.2/dsvocoder/vocoder.yaml" \
  --prosody neural
```

For Nishiren v1.2, use the bank directory and its bundled
`dsvocoder/vocoder.yaml`; the default Standard voice and learned tension curve
are selected automatically. `--prosody neural` requires the duration and pitch
models; `auto` uses them when both configurations exist, and `score` explicitly
selects the original fixed-pitch/heuristic-timing baseline. `--expression` accepts
0..1 (default 1) to control the pitch predictor's expressiveness. These options
are accepted by both `configure.py` and the adapter. Reconfigure after changing
them so the cache key reflects the new settings. The variance encoder must accept supplied phoneme
durations; models needing additional word or language conditioning fail clearly.
Without a compatible predictor, required variance controls must be supplied
explicitly using values appropriate for that bank. The configuration fingerprints model files,
backend source, and dependency versions for cache invalidation. Re-run it after
changing any of these. Changing command arguments also changes the cache key.

## Leif Lush

[Leif DS v111](https://leifly.carrd.co/) is another tested local voicebank, voiced
by JaimeSynth and configured by tigermeat. Download the DiffSinger archive from
the publisher's page and extract it under `.local/singing/`. Keep its bundled
`extra/LICENSE.txt` and credits. The licence permits this non-commercial test;
commercial usage requires a licence from the rights holder.

The 397,383,419-byte archive passed ZIP CRC verification. Its locally measured
SHA-256 is `7fdce935c34544f890de4a83163b7cb6d6d23cbed1787b0bcb81f8b42727b30d`.
This is a reproducibility record, not a publisher-signed checksum.

```sh
.local/singing/venv/bin/python scripts/singing/configure.py \
  --voicebank .local/singing/Leif_DS_v111 \
  --vocoder .local/singing/Leif_DS_v111/dsvocoder/vocoder.yaml \
  --speaker src/leif_lush --prosody neural \
  --output .local/singing/leif-lush.json

go run ./cmd/muse -singer .local/singing/leif-lush.json scores/beyond_the_sea_leif_lush
```

The acoustic, duration, and pitch models all select the Lush speaker embedding.
Their individual language maps supply IDs for language-prefixed phonemes;
shared silence tokens use zero. Leif uses its bundled `pc_tgm_hifigan_100`
vocoder and does not request separate tension/energy/breathiness conditioning.
The score retains the user-supplied English lyrics, melody and band arrangement.
The delivered WAV is additionally peak-normalised to 85% full scale, as was the
Nishiren comparison. Generated audio and model files remain git-ignored.

## Write and render a score

See [`stay_with_me.yaml`](../../examples/singing/stay_with_me.yaml), an original
20.3-second vocal timeline over piano, with a tempo change, ritardando, rests,
and held vowels. WAV duration may include additional instrumental/reverb tails.

```yaml
singing: {channel: C1, language: en}
sections:
  - C1: [c4, e4, g4, a4, z]
    lyrics: [stay, with, me, +, ""]
```

- One lyric per melody event, including `""` for each rest.
- `+` carries the preceding vowel over another note. It cannot follow a rest.
- A lyric is one syllable. English words use CMUdict; unknown words and multiple
  syllables require explicit phonemes, e.g. `"[s t ey]"` or syllable splitting.
- The phone inventory must match the bank. CMU phones are lowercased and stress
  digits removed; exact phones or `en/`-prefixed phones are supported. Bank-specific
  alias dictionaries are not yet interpreted automatically.
- Only one monophonic singing channel, English, phrases up to 30 seconds, and
  scores up to 10 minutes. Insert a rest/breath between phrases.
- Every active channel must fill each section to the same duration.
- Singing follows the shared tempo map, swing, ritardando, and fermata. Random
  note-onset jitter is omitted for the vocal to keep syllables contiguous.
- Voice level and section dynamics are applied in Go; existing panning and
  reverb are applied in the final mix. A zero vocal level is silent.

Validate/export timing without any model:

```sh
go run ./cmd/muse -singing-plan .local/singing/stay-plan.json examples/singing/stay_with_me
```

Render after configuring a compatible singer:

```sh
go run ./cmd/muse -singer .local/singing/singer.json examples/singing/stay_with_me
```

The result is `examples/singing/stay_with_me.wav`. The local comparison also
retains `stay_with_me_before.wav` and `stay_with_me_vocal_before.wav`; the
updated isolated vocal is `stay_with_me_vocal.wav`. These generated WAVs are
git-ignored. This command fails explicitly
when the model is missing or incompatible. Ordinary `muse` rendering of a score
with `singing` also fails instead of silently replacing its lyrics with “ah”.

## Backend contract and limitations

The local JSON configuration contains a `command` argument array. No shell is
used. Relative executable paths and `cache_dir` resolve against the config file;
arguments run with that folder as the working directory. Muse appends
`--request <temporary-json>` and `--output <temporary-wav>`.

Protocol v1 supplies absolute note start/duration in seconds, resolved MIDI
pitches, lyrics, a sample rate, and total duration. Return exactly that many
frames as mono 16-bit PCM at 44.1 kHz, including rests. Muse validates format and
length, applies score gains, and mixes the vocal in place of the selected channel.
The subprocess has a timeout, cancellation, bounded log capture, and validated
WAV caching. Temporary files are removed on failure. Cache files are the raw
vocal before Muse levels and effects; they are not a final mixed vocal stem.

With Nishiren, the duration predictor allocates phoneme lengths within each
syllable, then those lengths are fitted to the written syllable duration. Individual
consonants are capped at 75 ms and onset/closing clusters at 120 ms (rounded
down to whole frames). Excess duration goes to the vowel; short syllables reserve
at least 60% for the vowel where one-frame consonant minima permit. This avoids
stretching the initial “s” of a long “sail”.
Cumulative frame rounding preserves the score timeline. Held vowels share one
syllable across their pitch notes. The pitch predictor receives the complete
note sequence, rest flags, and a smoothed MIDI guide; its output is converted
to Hz before tension prediction, acoustic synthesis, and vocoding. Duration and
pitch predictors with one speaker use that shared predictor voice; multi-speaker
predictors require a matching acoustic speaker. Published model files are unchanged.

This does not yet implement learned English phonemisation, vowel-onset anchoring
across syllable boundaries, or continuous score dynamics within a held note.
It does not guarantee bit-for-bit neural determinism. A fixed ONNX seed is requested; cached renders
are reused exactly. Rendering is CPU-only, one phrase at a time. Whole-score
vocal buffers are retained, so this is not a streaming/real-time engine.

## Tests

```sh
go test ./internal/muse ./internal/singing -run 'Test(Singing|Backend|Config)'
.local/singing/venv/bin/python -m unittest discover -s scripts/singing -p 'test_*.py'
```

Transport tests use an explicitly synthetic helper process and make no network
calls. Acceptance still requires real audio: intelligible lyrics, pitch and
rhythm alignment, natural vowel continuation, and duration/peak/clipping checks.

Architecture references: [OpenVPI DiffSinger](https://github.com/openvpi/DiffSinger),
[OpenUtau DiffSinger support](https://www.openutau.com/DiffSinger-support/), and
[DiffSinger UTAU's ONNX input handling](https://github.com/bingcheng1998/diffsinger_utau).

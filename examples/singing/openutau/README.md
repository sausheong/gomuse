# Beyond the Sea: native OpenUtau comparison

Rendered with OpenUtau macOS ARM64 beta 0.1.571.0, Leif DS v1.1.1, voice colour 02 Lush, DIFFS EN phonemizer, at 108 BPM. Native neural pitch was generated for all 17 notes and saved in the USTX project. DiffSinger acoustic settings were depth 0.60 and 20 steps.

## Listen

- `beyond_the_sea_openutau_15s_matched.wav`: native OpenUtau phrase renders assembled into the passage.
- `beyond_the_sea_adapter_15s_matched.wav`: existing GoMuse adapter performance, with constant gain adjustment to match RMS over the common singing window.

Both files are 15.56 seconds, mono, 44.1 kHz, PCM16, with no clipped samples. No compression, reverb or pitch post-processing was applied. Listening acceptance remains with the user; signal checks do not establish perceptual quality.

The native project uses whole words and `+` for subsequent syllables. It includes a 162 ms break before “And”. This compares the native workflow including timing and pitch, rather than isolating one variable. Original full-song scores remain unchanged.

## Export provenance

OpenUtau successfully rendered both phrases during playback. Its popup export menu was inaccessible through the desktop controls, so the native cached PCM phrase renders were copied directly. They were placed by their score end times, accounting for DiffSinger's eight-frame tail padding. Alignment is approximate to the model frame duration (11.6 ms); this is not a GUI mixdown export. Raw phrase files and comparison_metrics.json preserve provenance.

## Reopen

OpenUtau Additional Singer Path is `/Users/sausheong/projects/gomuse/.local/singing`. Open `beyond_the_sea_leif_15s.ustx`; it contains the generated pitch curves. The application is at `/Users/sausheong/projects/gomuse/.local/singing/OpenUtau.app`.

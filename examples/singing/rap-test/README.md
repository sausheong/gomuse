# Four-bar spoken rap workflow prototype

Uses the installed macOS Reed (English US) speech voice, not Leif. Four complete spoken lines were rendered using `say` at 235, 225, 245 and 220 words per minute respectively. Input text is in lyrics.txt and the original AIFF files are retained.

assemble.py trims outer padding conservatively and uses FFmpeg atempo to fit each line within a 92 BPM bar while preserving speech pitch. It leaves a short rest at each bar boundary and applies one global peak-normalisation. No fixed MIDI pitches, word-by-word splicing, or added musical vibrato are used. performance.json records placement and stretch factors.

four_bars_dry.wav is the requested vocal audition. four_bars_with_click.wav adds only a quiet beat reference, not an instrumental arrangement. Both are mono PCM16 at 44.1 kHz, about 11 seconds, without clipping.

This tests a phrase-based speech workflow. It does not establish convincing rap delivery: internal stresses and cadence come from general-purpose TTS, not a rap-trained model. No production rap-track feature has been added to Muse yet. Listen before deciding whether to integrate or replace the speech engine.

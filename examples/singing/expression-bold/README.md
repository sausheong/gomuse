# Sun in My Shoes: exaggerated expression audition

A separate version of the four-line chorus. The original song and subtle expression audition are preserved.

Compared with the subtle audition: accented notes scoop up by 100 cents over 220 ms (previously 35 cents over 120 ms); held endings use 55-cent, 5.2 Hz vibrato (previously 17 cents), followed by an 80-cent falling release. Endings are extended by 340 ms, leaving approximately 129 ms of the written rest. Pitch contours are applied before acoustic synthesis.

The smooth post-render gain envelope uses +7 dB word accents and -6 dB for selected connecting words, with a 3 dB within-note swell and 7 dB release. These deliberate exaggerations test the voice's expressive range; they are not emotional conditioning or a different voice model. Both files are RMS-matched after shaping, with common peak headroom. No compression or reverb is added. Listening feedback determines whether this is musically useful.

Run render.py with the local singing virtual environment. Requests, raw audio, frame-level pitch diagnostics and metrics are retained.

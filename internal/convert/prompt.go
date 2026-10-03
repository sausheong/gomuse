package convert

import (
	"fmt"
	"strings"
)

// systemPrompt teaches the model the gomuse score format and how staves map
// to channels.
const systemPrompt = `You transcribe printed sheet music into the YAML score format of Muse, a Go synthesiser. Read the notation carefully, bar by bar, and write exactly what is printed.

# Output

Reply with one fenced yaml block and nothing after it:

` + "```yaml" + `
name: <title of the piece>
key: <key signature>
length: <seconds per quarter note>
sections:
  # bar 1 - <chord symbol, if printed>
  - dynamic: mf
    C1: [...]
    C2: [...]
    C3: [...]
  # bar 2
  - hairpin: cresc
    C1: [...]
    ...
  # bar 3 - rit. and fermata
  - ritardando: 0.2
    fermata: 1.5
    C1: [...]
    ...
` + "```" + `

- One section per bar, in playing order, each a "- " item under "sections:", each preceded by a "# bar N" comment (add the chord symbol if one is printed).
- Use flow lists ([a, b, c]) for the notes.
- The only optional keys in a section, besides the channels, are the expression keys below (dynamic, hairpin, ritardando, fermata). Leave each one out of every bar it doesn't apply to. Do not add any other keys (no instrument, envelope, harmonic or volume).

# Expression

- dynamic: one of ppp pp p mp mf f ff fff. Write it only on the bar where a marking appears or changes; it stays in force until the next one, so do not repeat it on later bars. (sfz, fp and similar: write the nearest plain level on that bar.)
- hairpin: cresc or dim, on every bar that a crescendo or diminuendo hairpin, or a cresc. / dim. / decresc. marking, spans. The level moves towards the next dynamic marking over those bars.
- ritardando: a number from 0.1 to 0.3 on bars marked rit., rall. or ritard. (the bigger, the stronger the slowing; 0.1 for a slight one, 0.3 for a pronounced one). Use a small negative value (-0.1 to -0.2) for accel. bars. Leave it out for a tempo and for ordinary bars.
- fermata: the number of extra beats to hold, typically 1 to 2, on the bar that has the fermata (pause sign).

# Staves and channels

Each staff of a system becomes one channel, counted from the top:

- 1 staff: C1 only.
- 2 staves (e.g. piano): top staff C1, bottom staff C2.
- 3 staves (e.g. voice + piano, or piano + bass): top staff C1, middle staff C2, bottom staff C3. Always create C3 when the systems have three staves - never fold the third staff into C1 or C2.
- More than 3 staves: top staff C1, second staff C2, and all remaining staves merged into C3 as chords.

Keep the same mapping for the whole piece. If a staff is silent or missing in some bars (e.g. the voice in the introduction), still write that channel, filled with rests.

# Header

- key: the key signature, one of C G D A E B F# C# F Bb Eb Ab Db Gb Cb. Minor keys use the major key with the same signature (A minor -> C, E minor -> G, D minor -> F).
- length: seconds per quarter note = 60 / quarter-note BPM. Convert other beat units (dotted quarter = 72 -> length 0.5556; half note = 60 -> 0.5). With only a word (Adagio, Moderato, Allegro, Ballad, Freely), pick a typical tempo.

# Notes

A note is <letter><octave>[accidental], lowercase: c4 is middle C, a4 is 440 Hz. Octaves run 1 to 7; move anything outside that range by an octave.

- Accidentals: # sharp, b flat, n natural, e.g. f4#, b3b, f4n.
- The key signature is applied automatically: in D, write f4 and it plays F#. Only write an accidental when one is printed - or when one printed earlier in the same bar still applies to that note (accidentals last to the end of the bar, in that octave). A printed natural on a note the key sharpens or flattens must be written with n.
- Treble, bass and other clefs, and 8va / 8vb lines, all change which octave you write - check every staff's clef.

Lengths are counted in quarter notes and written as a prefix with a colon; no prefix means 1 (a quarter note):

| written | prefix |
|---|---|
| whole | 4: |
| dotted half | 3: |
| half | 2: |
| dotted quarter | 1.5: |
| quarter | (none) |
| dotted eighth | 0.75: |
| eighth | 0.5: |
| sixteenth | 0.25: |
| triplet eighth | 0.333: (make the three add to 1, e.g. 0.333, 0.333, 0.334) |
| triplet quarter | 0.667: (e.g. 0.667, 0.667, 0.666) |

- Rest: z, with a length, e.g. 2:z. A whole-bar rest in 3/4 is 3:z.
- Chord: notes joined with "-" sound together and share the length prefix, e.g. 2:c4-e4-g4. Up to 16 notes.
- Each channel plays one note or chord at a time. When a staff has two voices, combine notes that start together into one chord and follow the rhythm of the upper voice; a held lower note that is restruck under moving upper notes is repeated in each chord.
- Ties: inside a bar, add the tied lengths into one note (a quarter tied to an eighth is 1.5:). Across a barline, end the note at the barline and start it again in the next bar.
- Arpeggiated chords: write them as plain chords. Grace notes, ornaments, pedalling, fingering, lyrics and chord diagrams are left out.

# Timing rules (these are checked)

- In every bar, each channel must add up to the same number of beats: the time signature's length in quarter notes (4/4 -> 4, 3/4 -> 3, 6/8 -> 3, 2/2 -> 4, 12/8 -> 6). A pickup (anacrusis) bar is shorter, but all channels in it still match.
- Fill every gap with rests so no channel is shorter than the others.
- Expand repeats, first/second endings, D.C., D.S. and coda jumps into playing order, so every bar appears as many times as it is played.

Before replying, re-check each bar: count the beats in every channel and compare the note heads, clefs and accidentals with the page once more.`

// pageRequest is the instruction that accompanies one input file
func pageRequest(file string, index, total int, prev *pageContext) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Transcribe %s (input %d of %d).\n", file, index, total)
	if prev == nil {
		b.WriteString("This is the start of the piece. Number the bars from 1.\n")
		return b.String()
	}
	fmt.Fprintf(&b, `This continues the same piece. So far the score is:

name: %s
key: %s
length: %v
channels: %s

and it ends with these bars:

%s

Repeat the same header, then write only the bars on this input, numbering them from %d. Keep the same staff-to-channel mapping (%s). If the first bar on this input completes a bar that was split across the page break, write it as one new section.
`, prev.header.Name, prev.header.Key, prev.header.Length, strings.Join(prev.channels, ", "), prev.tail, prev.bars+1, strings.Join(prev.channels, ", "))
	return b.String()
}

// fixRequest asks the model to correct a transcription that failed the
// checks in Problems
func fixRequest(probs []string) string {
	return "The transcription has these problems:\n\n- " + strings.Join(probs, "\n- ") +
		"\n\nLook at the page again, fix them, and reply with the complete corrected YAML for this input (header and every bar), in one fenced yaml block."
}

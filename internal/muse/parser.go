package muse

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// maxChordPitches caps how many pitches a single "-" separated chord may
// stack. encode() allocates one full-length sample slice per pitch in a
// chord, so an unbounded chord width lets a tiny score request an amount of
// memory and CPU wildly out of proportion to its note count - a chord this
// wide is not musically meaningful anyway.
const maxChordPitches = 16

// Score represents the musical score
type Score struct {
	Tone               map[string]ToneControls   `yaml:"tone,omitempty"`
	Playing            map[string]string         `yaml:"playing,omitempty"`
	PlayingStyles      map[string]PlayingStyle   `yaml:"playing_styles,omitempty"`
	Marks              map[string]NoteMark       `yaml:"marks,omitempty"`
	Calibration        bool                      `yaml:"calibration,omitempty"`
	Mix                map[string]ChannelMix     `yaml:"mix,omitempty"`
	PerformancePresets map[string]Performance    `yaml:"performance_presets,omitempty"`
	ExpressionPresets  map[string]NoteExpression `yaml:"expression_presets,omitempty"`
	Ensemble           *Ensemble                 `yaml:"ensemble,omitempty"`
	Mastering          *Mastering                `yaml:"mastering,omitempty"`
	Room               *RoomSettings             `yaml:"room,omitempty"`
	Performance        map[string]Performance    `yaml:"performance,omitempty"`
	Name               string                    `yaml:"name"`
	Key                string                    `yaml:"key"`
	Length             float64                   `yaml:"length"`
	Envelope           string                    `yaml:"envelope"`
	Harmonic           string                    `yaml:"harmonic"`
	Instrument         string                    `yaml:"instrument,omitempty"`
	Volume             int                       `yaml:"volume"`
	Reverb             *float64                  `yaml:"reverb,omitempty"` // 0 to 1; unset means defaultReverb
	Plain              bool                      `yaml:"plain,omitempty"`  // play exactly as written: no ring-over, accents or reverb
	// Humanize scales the small random timing and loudness variations, 0 to
	// 1; unset means the default (see timing.go)
	Humanize *float64 `yaml:"humanize,omitempty"`
	// Swing delays every off-beat eighth note, 0 (straight) to 1 (full
	// triplet swing)
	Swing float64 `yaml:"swing,omitempty"`
	// Instruments sets an instrument per channel, e.g. {C1: voice}, which
	// replaces Instrument for that channel
	Instruments map[string]string `yaml:"instruments,omitempty"`
	// Pan places each channel from -1 (left) to 1 (right), e.g. {C1: 0};
	// unset channels get the defaults in voice.go
	Pan map[string]float64 `yaml:"pan,omitempty"`
	// Levels sets the loudness of each channel from 0 to 1, e.g. {C2: 0.6},
	// in place of the default balance (see channelGains in expression.go)
	Levels   map[string]float64 `yaml:"levels,omitempty"`
	Sections []Section          `yaml:"sections"`
	Singing  *Singing           `yaml:"singing,omitempty"`
	Scat     *Scat              `yaml:"scat,omitempty"`
}

// Section represents a section of music. C1 and C2 are the left and right
// channels; C3, if present, is a centre channel mixed into both.
type Section struct {
	Tone        map[string]ToneControls    `yaml:"tone,omitempty"`
	Playing     map[string]string          `yaml:"playing,omitempty"`
	Label       string                     `yaml:"label,omitempty"`
	Parts       map[string][]WrittenPhrase `yaml:"parts,omitempty"`
	Rubato      float64                    `yaml:"rubato,omitempty"`
	Expression  map[string]NoteExpression  `yaml:"expression,omitempty"`
	Performance map[string]Performance     `yaml:"performance,omitempty"`
	Length      float64                    `yaml:"length,omitempty"`
	Envelope    string                     `yaml:"envelope,omitempty"`
	Harmonic    string                     `yaml:"harmonic,omitempty"`
	Instrument  string                     `yaml:"instrument,omitempty"`
	Volume      int                        `yaml:"volume,omitempty"`
	// Dynamic is the dynamic marking at the start of the section: ppp, pp,
	// p, mp, mf, f, ff or fff; it lasts until the next one (see dynamics.go)
	Dynamic string `yaml:"dynamic,omitempty"`
	// Hairpin is "cresc" or "dim": the dynamic moves smoothly over the
	// section towards the next section's dynamic
	Hairpin string `yaml:"hairpin,omitempty"`
	// Ritardando slows the section down gradually: 0.2 means the end of the
	// section is played 20% slower (negative values speed up, accelerando)
	Ritardando float64 `yaml:"ritardando,omitempty"`
	// Fermata holds the last note of the section this many beats longer
	Fermata float64 `yaml:"fermata,omitempty"`
	// Instruments sets an instrument per channel for this section
	Instruments map[string]string `yaml:"instruments,omitempty"`
	// Drums is the drum part: each entry names the drums struck together,
	// e.g. kick-hat or 0.5:snare (see drums.go)
	Drums  []string     `yaml:"drums,omitempty"`
	C1     []string     `yaml:"C1"`
	C2     []string     `yaml:"C2"`
	C3     []string     `yaml:"C3,omitempty"`
	Lyrics []string     `yaml:"lyrics,omitempty"`
	Scat   *ScatSection `yaml:"scat,omitempty"`
}

// Parse reads a score, converts it into a Score struct and writes the
// resulting audio to outfile+".wav". maxSamples caps the number of samples
// allowed in any one channel; maxSamples <= 0 means unlimited. A score that
// would exceed the cap is rejected before any audio is synthesised.
func Parse(s *Score, score []byte, outfile string, maxSamples int) (name string, err error) {
	return ParseWithSinger(context.Background(), s, score, outfile, maxSamples, nil)
}

// ParseWithSinger enables an explicitly supplied, local singing backend.
// Parse and the web server never execute a backend from score contents.
func ParseWithSinger(ctx context.Context, s *Score, score []byte, outfile string, maxSamples int, singer Singer) (name string, err error) {
	err = yaml.Unmarshal(score, s)
	if err != nil {
		err = fmt.Errorf("cannot unmarshal score file > %v", err)
		return
	}
	name = s.Name
	if s.Singing != nil && singer == nil {
		return name, fmt.Errorf("this score requires a neural singer; render with muse -singer <local-config.json> <score>")
	}

	var t tune
	if t, err = buildTune(s); err != nil {
		return
	}

	// reject a score that would be too long before synthesising any audio,
	// computed purely from the notes' durations
	if maxSamples > 0 {
		needs := [channelCount]string{"channel 1 needs", "channel 2 needs", "channel 3 needs", "the drums need"}
		for c, ch := range t.channels() {
			if n := channelSamples(ch); n > maxSamples {
				err = fmt.Errorf("tune too long > %s %d samples, max is %d ", needs[c], n, maxSamples)
				return
			}
		}
	}

	if s.Singing != nil {
		if err = attachSinging(ctx, s, &t, singer); err != nil {
			return name, err
		}
	}
	if s.Scat != nil {
		if err = attachScat(ctx, s, &t); err != nil {
			return name, err
		}
	}
	var data []int
	data, err = t.encode()
	if err != nil {
		err = fmt.Errorf("cannot encode the tune > %v", err)
		return
	}
	err = writeWAV(outfile, data)
	return
}

// Check validates a score's settings and every note in every channel
// without synthesising any audio.
func Check(s *Score) error {
	t, err := buildTune(s)
	if err != nil {
		return err
	}
	if !validKey(t.key) {
		return fmt.Errorf("unknown key signature - %s ", t.key)
	}
	return nil
}

// buildTune turns every note string in the score into a note
func buildTune(s *Score) (tune, error) {
	expanded, err := expandWrittenScore(s)
	if err != nil {
		return tune{}, err
	}
	s = expanded
	if err := validateSinging(s); err != nil {
		return tune{}, err
	}
	for _, validate := range []func(*Score) error{validateTone, validateMix, validateEnsemble, validateMastering, validateRoom, validateNoteExpression, validateDynamics, validateTiming, validateParts, validatePerformance, validateScat} {
		if err := validate(s); err != nil {
			return tune{}, err
		}
	}
	// C1 to C3 start empty rather than nil; the drums stay nil without notes
	chs := [channelCount][]note{{}, {}, {}, nil}
	for _, section := range s.Sections {
		for c, name := range channelNames {
			for _, n := range sectionChannel(section, c) {
				var nt note
				var err error
				if c == drumChannel {
					nt, err = makeDrumNote(n, section, *s)
					nt.hit = len(chs[c])
				} else {
					nt, err = makeChannelNote(n, section, *s, name)
				}
				if err != nil {
					return tune{key: s.Key}, fmt.Errorf("[%s] cannot make note > %v ", name, err)
				}
				chs[c] = append(chs[c], nt)
			}
		}
	}
	t := tune{key: s.Key, ch1: chs[0], ch2: chs[1], ch3: chs[2], drums: chs[3]}
	expressive(s, &t)
	applyDynamics(s, &t)
	applyTiming(s, &t)
	applyNoteExpression(s, &t)
	applyCalibration(s, &t)
	for _, ch := range t.channels() {
		prepareStringPerformance(ch)
	}
	if err := validatePreparedGestures(&t); err != nil {
		return tune{}, err
	}
	t.reverb = reverbAmount(s)
	t.room = s.Room
	t.mastering = s.Mastering
	t.mix = s.Mix
	t.pan = pans(s, &t)
	return t, nil
}

// channelSamples adds up the number of samples a channel's notes will
// produce, without synthesising any of them. encode() allocates one sample
// slice per pitch in a chord (see note.encode in encoder.go), so a note's
// contribution is its sample count times its chord width, not just once,
// or this cap wouldn't actually bound the memory encode() allocates.
func channelSamples(notes []note) int {
	total := 0
	for _, n := range notes {
		width := len(n.pitch)
		if width == 0 { // a rest still allocates one slice
			width = 1
		}
		// timing (timing.go) may play a note longer than written, through a
		// ritardando or fermata, and a rolled chord delays its upper notes
		played := n.length
		if n.placed && n.hold > 0 {
			played = n.hold
		}
		played = math.Max(played, n.sustainHold)
		played += n.release + n.spread*float64(width-1)
		if n.performance != nil && pluckedInstrument(n.instrumentName) {
			played += 0.12
		}
		if len(n.kit) > 0 {
			// every drum rings for its own length, however short the note
			total += max(sampleCount(played), drumLength(n)) * len(n.kit)
			continue
		}
		total += sampleCount(played) * width
	}
	return total
}

// makeChannelNote makes a note for one channel, using that channel's
// instrument if the section or score sets one in Instruments
func makeChannelNote(noteString string, section Section, score Score, channel string) (note, error) {
	if ins, ok := section.Instruments[channel]; ok {
		section.Instrument = ins
	} else if ins, ok := score.Instruments[channel]; ok && section.Instrument == "" {
		section.Instrument = ins
	}
	n, err := makeNote(noteString, section, score)
	if err != nil {
		return n, err
	}
	p, err := performanceFor(score, section, channel)
	if err != nil {
		return n, err
	}
	if p != nil && p.enabled() {
		if err := validateInstrumentPerformance(n, *p); err != nil {
			return n, err
		}
		n.performance = p
		// Include the phrase renderer's tail in pre-render sample limits,
		// including plain scores whose legacy release is zero.
		n.release = math.Max(n.release, 0.1)
		if sustainedInstrument(n.instrumentName) {
			n.release = math.Max(n.release, profileFor(n.instrumentName, 440, .7).release)
		}
		if n.instrumentName == "piano" || pluckedInstrument(n.instrumentName) {
			n.release = math.Max(n.release, 0.18)
		}
	}
	if err := toneFor(score, section, channel, &n); err != nil {
		return n, err
	}
	return n, nil
}

// sectionBeat is the length of a beat in a section, in seconds: the
// section's own, or else the score's
func sectionBeat(s Score, sec Section) float64 {
	if sec.Length != 0 {
		return sec.Length
	}
	return s.Length
}

// sectionVolume is a section's volume: its own, or else the score's
func sectionVolume(s Score, sec Section) int {
	if sec.Volume != 0 {
		return sec.Volume
	}
	return s.Volume
}

// make a note
func makeNote(noteString string, section Section, score Score) (n note, err error) {
	length := sectionBeat(score, section)
	vol := sectionVolume(score, section)
	env := section.Envelope
	if env == "" {
		env = score.Envelope
	}
	har := section.Harmonic
	if har == "" {
		har = score.Harmonic
	}

	ins := section.Instrument
	if ins == "" {
		ins = score.Instrument
	}

	// an instrument shapes the whole note itself, and replaces the envelope
	// and harmonic; without one, both must exist
	var envFn envelope
	var harFn harmonic
	var insFn instrument
	if ins != "" {
		var ok bool
		if insFn, ok = instruments[ins]; !ok {
			err = fmt.Errorf("instrument doesn't exist - %s ", ins)
			return
		}
	} else {
		var ok bool
		if envFn, ok = envelopes[env]; !ok {
			err = fmt.Errorf("envelope doesn't exist - %s ", env)
			return
		}
		if harFn, ok = harmonics[har]; !ok {
			err = fmt.Errorf("harmonic doesn't exist - %s ", har)
			return
		}
	}

	// default returned note
	n = note{
		pitch:          []int{},
		accidental:     []int{},
		explicit:       []bool{},
		length:         length,
		env:            envFn,
		har:            harFn,
		ins:            insFn,
		velocityIns:    velocityInstruments[ins],
		instrumentName: ins,
		vol:            vol,
		gain:           1,
	}
	if ins != "" && !score.Plain {
		n.release = releases[ins]
	}

	// if length is explicitly set, separate length of note from the pitches
	nArray := strings.Split(noteString, ":") // split the length away from the notes
	var l float64                            // note length multiplier
	var p string                             // pitch

	// if length of note is explicitly set
	if len(nArray) > 1 {
		l, err = strconv.ParseFloat(nArray[0], 64)
		if err != nil {
			err = fmt.Errorf("cannot parse note duration - %s > %v", nArray[0], err)
			return
		}
		p = nArray[1]
	} else {
		// if length of note is not explicitly set, use the standard one
		l = 1.0
		p = nArray[0]
	}
	n.length = l * length
	if math.IsNaN(n.length) || math.IsInf(n.length, 0) || n.length <= 0 {
		err = fmt.Errorf("invalid note duration - %v ", n.length)
		return
	}

	// handle if note is a single note or a chord
	if strings.Contains(p, "-") {
		// a chord
		pitches := strings.Split(p, "-")
		if len(pitches) > maxChordPitches {
			err = fmt.Errorf("chord has too many notes - %d, max is %d ", len(pitches), maxChordPitches)
			return
		}
		for _, c := range pitches {
			err = process(&n, c)
			if err != nil {
				err = fmt.Errorf("wrong chords formation %s > %v", c, err)
				return
			}
		}
	} else {
		// a single note
		err = process(&n, p)
		if err != nil {
			err = fmt.Errorf("wrong note structure - %s > %v ", p, err)
			return
		}
	}
	return
}

// process the note
func process(n *note, p string) (err error) {
	if p == "z" { // a rest
		return
	}
	if len(p) < 2 {
		err = fmt.Errorf("note doesn't exist, is too short - %s ", p)
		return
	}
	if len(p) > 3 {
		err = fmt.Errorf("note doesn't exist, is too long - %s ", p)
		return
	}

	pt, ok := pitch[p[:2]]
	if !ok {
		err = fmt.Errorf("note doesn't exist - %s ", p)
		return
	}

	// an accidental, if any, is the 3rd character: # (sharp), b (flat) or
	// n (natural, cancels the key signature for this note). Anything else
	// is an error rather than being silently treated as a natural.
	acc := 0
	explicit := false
	if len(p) == 3 {
		switch p[2:] {
		case "#":
			acc = 1
		case "b":
			acc = -1
		case "n":
			acc = 0
		default:
			err = fmt.Errorf("note doesn't exist, invalid accidental - %s ", p)
			return
		}
		explicit = true
	}

	n.pitch = append(n.pitch, pt)
	n.accidental = append(n.accidental, acc)
	n.explicit = append(n.explicit, explicit)
	return
}

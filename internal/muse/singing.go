package muse

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"gopkg.in/yaml.v3"
)

// Singing assigns lyrics to one monophonic channel. Backend paths and commands
// are deliberately absent: only the local CLI configuration can select code.
type Singing struct {
	Channel  string `yaml:"channel"`
	Language string `yaml:"language"`
}

type VocalNote struct {
	Start    float64 `json:"start"`
	Duration float64 `json:"duration"`
	MIDI     int     `json:"midi"`
	Lyric    string  `json:"lyric"`
	Gain     float64 `json:"gain"`
}

type VocalPhrase struct {
	Notes []VocalNote `json:"notes"`
}

type VocalRequest struct {
	Version    int           `json:"version"`
	SampleRate int           `json:"sample_rate"`
	Language   string        `json:"language"`
	Duration   float64       `json:"duration"`
	Phrases    []VocalPhrase `json:"phrases"`
}

// Singer renders a mono, 44.1 kHz timeline including rests. Samples must be
// finite and in [-1,1]. The caller applies score dynamics and stereo placement.
type Singer interface {
	Render(context.Context, VocalRequest) ([]float64, error)
}

// SingingPlan validates and exports the exact vocal timeline without loading a
// model or synthesising audio. Useful for backend setup and pronunciation checks.
func SingingPlan(doc []byte) ([]byte, error) {
	var s Score
	if err := yaml.Unmarshal(doc, &s); err != nil {
		return nil, err
	}
	if s.Singing == nil {
		return nil, fmt.Errorf("score has no singing part")
	}
	t, err := buildTune(&s)
	if err != nil {
		return nil, err
	}
	if !validKey(t.key) {
		return nil, fmt.Errorf("unknown key signature - %s", t.key)
	}
	r, err := vocalRequest(&s, &t)
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(r, "", "  ")
}

func singingChannel(s *Score) int {
	if s.Singing != nil {
		for i, c := range channelNames[:3] {
			if c == s.Singing.Channel {
				return i
			}
		}
	}
	return -1
}

func validateSinging(s *Score) error {
	if s.Singing == nil {
		for _, sec := range s.Sections {
			if len(sec.Lyrics) > 0 {
				return fmt.Errorf("lyrics require a singing channel")
			}
		}
		return nil
	}
	c := singingChannel(s)
	if c < 0 {
		return fmt.Errorf("singing.channel must be C1, C2 or C3")
	}
	if s.Singing.Language != "en" {
		return fmt.Errorf("singing.language must be en for this experimental backend")
	}
	if len(s.Sections) == 0 {
		return fmt.Errorf("singing requires sections")
	}
	continuing := false
	sung := 0
	var used [channelCount]bool
	for _, sec := range s.Sections {
		for ch := range channelCount {
			used[ch] = used[ch] || len(sectionChannel(sec, ch)) > 0
		}
	}
	for i, sec := range s.Sections {
		// A shared phrase timeline requires bar alignment, not just equal totals.
		var reference float64
		for ch := range channelCount {
			if !used[ch] {
				continue
			}
			var duration float64
			for _, token := range sectionChannel(sec, ch) {
				var n note
				var err error
				if ch == drumChannel {
					n, err = makeDrumNote(token, sec, *s)
				} else {
					n, err = makeChannelNote(token, sec, *s, channelNames[ch])
				}
				if err != nil {
					return fmt.Errorf("section %d %s: %w", i+1, channelNames[ch], err)
				}
				duration += n.length
			}
			if duration == 0 {
				return fmt.Errorf("section %d: fill %s with rests while the singer is active", i+1, channelNames[ch])
			}
			if reference == 0 {
				reference = duration
			} else if math.Abs(reference-duration) > 1.0/sampleRate {
				return fmt.Errorf("section %d: singing requires aligned channel durations", i+1)
			}
		}
		ns := sectionChannel(sec, c)
		if len(ns) == 0 || len(ns) != len(sec.Lyrics) {
			return fmt.Errorf("section %d: lyrics must have one entry per %s note, including rests", i+1, s.Singing.Channel)
		}
		for j, token := range ns {
			n, err := makeChannelNote(token, sec, *s, s.Singing.Channel)
			if err != nil {
				return fmt.Errorf("section %d note %d: %w", i+1, j+1, err)
			}
			lyric := strings.TrimSpace(sec.Lyrics[j])
			switch len(n.pitch) {
			case 0:
				if lyric != "" {
					return fmt.Errorf("section %d note %d: a rest needs an empty lyric", i+1, j+1)
				}
				continuing = false
			case 1:
				if lyric == "" {
					return fmt.Errorf("section %d note %d: missing lyric (use + to extend a vowel)", i+1, j+1)
				}
				if lyric == "+" && !continuing {
					return fmt.Errorf("section %d note %d: + needs a preceding sung syllable without a rest", i+1, j+1)
				}
				if len(lyric) > 128 {
					return fmt.Errorf("section %d note %d: lyric is too long", i+1, j+1)
				}
				continuing = true
				sung++
			default:
				return fmt.Errorf("section %d note %d: singing requires a single pitch, not a chord", i+1, j+1)
			}
		}
	}
	if sung == 0 {
		return fmt.Errorf("singing requires at least one sung note")
	}
	return nil
}

// vocalRequest uses the shared tempo map, without random onset jitter. Singing
// needs contiguous syllables; consonant anticipation belongs to the phonemizer.
func vocalRequest(s *Score, t *tune) (VocalRequest, error) {
	c := singingChannel(s)
	tm := buildTempoMap(s, t)
	req := VocalRequest{Version: 1, SampleRate: sampleRate, Language: s.Singing.Language}
	ns := t.channels()[c]
	at, i := 0.0, 0
	var phrase VocalPhrase
	flush := func() {
		if len(phrase.Notes) > 0 {
			req.Phrases = append(req.Phrases, phrase)
			phrase = VocalPhrase{}
		}
	}
	for si, sec := range s.Sections {
		u := 0.0
		for j := range sectionChannel(sec, c) {
			n := ns[i]
			i++
			start, end := at+u, at+u+n.length
			if !s.Plain {
				start, end = tm[si].at(u, false), tm[si].at(u+n.length, true)
			}
			u += n.length
			req.Duration = math.Max(req.Duration, end)
			if len(n.pitch) == 0 {
				flush()
				continue
			}
			p := n.pitch[0] + n.accidental[0]
			if !n.explicit[0] && inNote(tuneKey[t.key], n.pitch[0]) {
				if inKey(sharpKeys, t.key) {
					p++
				} else if inKey(flatKeys, t.key) {
					p--
				}
			}
			// Muse pitches count semitones from A4; MIDI A4 is 69.
			gain := float64(n.vol) * n.gain / 32768
			if lv, ok := s.Levels[s.Singing.Channel]; ok && lv == 0 {
				gain = 0
			}
			if !isFinite(gain) || gain < 0 {
				return req, fmt.Errorf("singing volume must be non-negative and finite")
			}
			phrase.Notes = append(phrase.Notes, VocalNote{start, end - start, p + 69, strings.TrimSpace(sec.Lyrics[j]), gain})
		}
		at += u
	}
	flush()
	if req.Duration > 600 {
		return req, fmt.Errorf("vocal rendering is limited to 10 minutes per score")
	}
	for i, p := range req.Phrases {
		last := p.Notes[len(p.Notes)-1]
		if last.Start+last.Duration-p.Notes[0].Start > 30 {
			return req, fmt.Errorf("vocal phrase %d exceeds 30 seconds; add a breath/rest", i+1)
		}
	}
	return req, nil
}

func isFinite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

func attachSinging(ctx context.Context, s *Score, t *tune, singer Singer) error {
	if !validKey(t.key) {
		return fmt.Errorf("unknown key signature - %s", t.key)
	}
	req, err := vocalRequest(s, t)
	if err != nil {
		return err
	}
	x, err := singer.Render(ctx, req)
	if err != nil {
		return fmt.Errorf("neural singing: %w", err)
	}
	expected := sampleCount(req.Duration)
	if len(x) != expected {
		return fmt.Errorf("neural singing returned %d samples, want %d", len(x), expected)
	}
	out := make([]int, len(x))
	for _, v := range x {
		if math.IsNaN(v) || math.IsInf(v, 0) || math.Abs(v) > 1 {
			return fmt.Errorf("neural singing returned invalid audio samples")
		}
	}
	// Retain pre-onset consonants and short phrase tails while following explicit
	// levels/dynamics. Within a phrase the last onset's gain applies until next.
	for pi, p := range req.Phrases {
		begin := max(0, sampleCount(p.Notes[0].Start-0.15))
		last := p.Notes[len(p.Notes)-1]
		end := min(len(x), sampleCount(last.Start+last.Duration+0.15))
		if pi+1 < len(req.Phrases) {
			end = min(end, max(0, sampleCount(req.Phrases[pi+1].Notes[0].Start-0.15)))
		}
		ni := 0
		for k := begin; k < end; k++ {
			for ni+1 < len(p.Notes) && k >= sampleCount(p.Notes[ni+1].Start) {
				ni++
			}
			out[k] = int(math.Round(x[k] * 32768 * p.Notes[ni].Gain))
		}
	}
	t.external = map[int][]int{singingChannel(s): out}
	return nil
}

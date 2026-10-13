package muse

// Version 2 is an authoring layer. Compilation produces the same Score used by
// the original renderer; neither rendering nor expansion rewrites source files.
import (
	"bytes"
	"fmt"
	"math"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const notationMaxEvents = 200000
const notationMaxSections = 4096

type notationTrack struct {
	Instrument string   `yaml:"instrument"`
	Playing    string   `yaml:"playing"`
	Level      *float64 `yaml:"level"`
	Pan        *float64 `yaml:"pan"`
}
type notationScore struct {
	Version  int                      `yaml:"version"`
	Name     string                   `yaml:"name"`
	Key      string                   `yaml:"key"`
	Tempo    float64                  `yaml:"tempo"`
	Meter    string                   `yaml:"meter"`
	Volume   int                      `yaml:"volume"`
	Humanize *float64                 `yaml:"humanize"`
	Swing    float64                  `yaml:"swing"`
	Reverb   *float64                 `yaml:"reverb"`
	Tracks   map[string]notationTrack `yaml:"tracks"`
	Styles   map[string]yaml.Node     `yaml:"styles"`
	Marks    map[string]NoteMark      `yaml:"marks"`
	Patterns map[string]yaml.Node     `yaml:"patterns"`
	Sections map[string]yaml.Node     `yaml:"sections"`
	Form     []yaml.Node              `yaml:"form"`
}
type notationSection struct {
	Label      string               `yaml:"label"`
	Bars       int                  `yaml:"bars"`
	Beats      float64              `yaml:"beats"`
	Dynamic    string               `yaml:"dynamic"`
	Tempo      float64              `yaml:"tempo"`
	Hairpin    string               `yaml:"hairpin"`
	Ritardando float64              `yaml:"ritardando"`
	Rubato     float64              `yaml:"rubato"`
	Fermata    float64              `yaml:"fermata"`
	Playing    map[string]string    `yaml:"playing"`
	Parts      map[string]yaml.Node `yaml:"parts"`
}
type notationRef struct {
	Use    string `yaml:"use"`
	Repeat *int   `yaml:"repeat"`
}

func strictNotation(n *yaml.Node, dst any) error {
	b, err := yaml.Marshal(n)
	if err != nil {
		return err
	}
	d := yaml.NewDecoder(bytes.NewReader(b))
	d.KnownFields(true)
	return d.Decode(dst)
}
func notationField(n *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return n.Content[i+1]
		}
	}
	return nil
}

// UnmarshalYAML keeps legacy decoding intact while routing explicit version 2
// documents through the authoring compiler, including callers in the web app.
func (s *Score) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind != yaml.MappingNode {
		return fmt.Errorf("score must be a mapping")
	}
	version := notationField(n, "version")
	if version == nil {
		if notationField(n, "tracks") != nil || notationField(n, "form") != nil {
			return fmt.Errorf("named tracks and form require version: 2")
		}
		type legacy Score
		var v legacy
		if err := n.Decode(&v); err != nil {
			return err
		}
		*s = Score(v)
		return nil
	}
	var v notationScore
	if err := strictNotation(n, &v); err != nil {
		return err
	}
	if v.Version != 2 {
		return fmt.Errorf("unsupported score version %d", v.Version)
	}
	result, err := compileNotation(n, v)
	if err != nil {
		return err
	}
	*s = *result
	return nil
}

// ExpandScore returns a validated, standalone legacy score with authoring
// references and styles resolved. It performs no audio synthesis.
func ExpandScore(doc []byte) ([]byte, error) {
	var s Score
	if err := yaml.Unmarshal(doc, &s); err != nil {
		return nil, err
	}
	expanded, err := expandWrittenScore(&s)
	if err != nil {
		return nil, err
	}
	expanded.Playing = nil
	expanded.PlayingStyles = nil
	expanded.Marks = nil
	for i := range expanded.Sections {
		expanded.Sections[i].Parts = nil
		expanded.Sections[i].Playing = nil
	}
	if err = Check(expanded); err != nil {
		return nil, err
	}
	return yaml.Marshal(expanded)
}

// Overlay maps recursively for styles. Section parts are replaced per track,
// so overriding a reference cannot accidentally inherit a phrase's controls.
func notationOverlay(base, over *yaml.Node, recursive bool) *yaml.Node {
	r := *base
	r.Content = append([]*yaml.Node(nil), base.Content...)
	for i := 0; i < len(over.Content); i += 2 {
		key, val := over.Content[i], over.Content[i+1]
		if key.Value == "extends" {
			continue
		}
		found := false
		for j := 0; j < len(r.Content); j += 2 {
			if r.Content[j].Value == key.Value {
				prev := r.Content[j+1]
				if prev.Kind == yaml.MappingNode && val.Kind == yaml.MappingNode && (recursive || key.Value == "parts" || key.Value == "playing") {
					r.Content[j+1] = notationOverlay(prev, val, recursive)
				} else {
					r.Content[j+1] = val
				}
				found = true
				break
			}
		}
		if !found {
			r.Content = append(r.Content, key, val)
		}
	}
	return &r
}
func resolveNotation(name string, library map[string]yaml.Node, builtin map[string]yaml.Node, stack map[string]bool, recursive bool) (*yaml.Node, error) {
	if stack[name] || len(stack) > 32 {
		return nil, fmt.Errorf("cyclic or too-deep inheritance at %q", name)
	}
	n, ok := library[name]
	if !ok {
		n, ok = builtin[name]
	}
	if !ok {
		return nil, fmt.Errorf("unknown definition %q", name)
	}
	if n.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("definition %q must be a mapping", name)
	}
	if parent := notationField(&n, "extends"); parent != nil {
		stack[name] = true
		base, err := resolveNotation(parent.Value, library, builtin, stack, recursive)
		delete(stack, name)
		if err != nil {
			return nil, err
		}
		return notationOverlay(base, &n, recursive), nil
	}
	return &n, nil
}
func notationRepeat(n *yaml.Node) (string, int, error) {
	if n.Kind == yaml.ScalarNode {
		if n.Tag != "!!str" {
			return "", 0, fmt.Errorf("reference must be a name")
		}
		return n.Value, 1, nil
	}
	var r notationRef
	if err := strictNotation(n, &r); err != nil {
		return "", 0, err
	}
	count := 1
	if r.Repeat != nil {
		count = *r.Repeat
	}
	if r.Use == "" || count < 1 || count > 1024 {
		return "", 0, fmt.Errorf("use requires a name and repeat must be 1..1024")
	}
	return r.Use, count, nil
}
func expandNotationPart(n *yaml.Node, patterns map[string]yaml.Node, stack map[string]bool, budget *int) ([]WrittenPhrase, error) {
	if n.Kind == yaml.AliasNode {
		return nil, fmt.Errorf("use named patterns instead of YAML aliases")
	}
	if n.Kind == yaml.MappingNode && notationField(n, "sequence") != nil {
		var seq struct {
			Sequence []yaml.Node `yaml:"sequence"`
		}
		if err := strictNotation(n, &seq); err != nil {
			return nil, err
		}
		if len(seq.Sequence) == 0 {
			return nil, fmt.Errorf("sequence must not be empty")
		}
		var out []WrittenPhrase
		for _, item := range seq.Sequence {
			name, repeat, err := notationRepeat(&item)
			if err != nil {
				return nil, err
			}
			var ref yaml.Node
			if err = ref.Encode(map[string]any{"use": name, "repeat": repeat}); err != nil {
				return nil, err
			}
			phrases, err := expandNotationPart(&ref, patterns, stack, budget)
			if err != nil {
				return nil, err
			}
			out = append(out, phrases...)
		}
		return out, nil
	}
	if n.Kind == yaml.MappingNode && notationField(n, "use") != nil {
		name, count, err := notationRepeat(n)
		if err != nil {
			return nil, err
		}
		if stack[name] || len(stack) > 32 {
			return nil, fmt.Errorf("cyclic or too-deep pattern %q", name)
		}
		target, ok := patterns[name]
		if !ok {
			return nil, fmt.Errorf("unknown pattern %q", name)
		}
		stack[name] = true
		var out []WrittenPhrase
		for j := 0; j < count; j++ {
			p, err := expandNotationPart(&target, patterns, stack, budget)
			if err != nil {
				return nil, err
			}
			out = append(out, p...)
		}
		delete(stack, name)
		return out, nil
	}
	if n.Kind == yaml.SequenceNode {
		var out []WrittenPhrase
		for _, child := range n.Content {
			p, err := expandNotationPart(child, patterns, stack, budget)
			if err != nil {
				return nil, err
			}
			out = append(out, p...)
		}
		return out, nil
	}
	var p WrittenPhrase
	if n.Kind == yaml.ScalarNode {
		if n.Tag != "!!str" {
			return nil, fmt.Errorf("notes must be text")
		}
		p.Notes = n.Value
	} else if err := strictNotationPhrase(n, &p); err != nil {
		return nil, err
	}
	tokens := strings.Fields(p.Notes)
	if len(tokens) == 0 {
		return nil, fmt.Errorf("empty phrase")
	}
	*budget += len(tokens)
	if *budget > notationMaxEvents {
		return nil, fmt.Errorf("expanded score exceeds %d tokens", notationMaxEvents)
	}
	return []WrittenPhrase{p}, nil
}

// WrittenPhrase has a custom decoder, so validate its keys explicitly.
func strictNotationPhrase(n *yaml.Node, p *WrittenPhrase) error {
	if n.Kind != yaml.MappingNode {
		return fmt.Errorf("part must contain notes or a pattern reference")
	}
	for i := 0; i < len(n.Content); i += 2 {
		switch n.Content[i].Value {
		case "notes", "unphrased", "peak_db", "breath_ms", "attack_ms":
		default:
			return fmt.Errorf("unknown phrase field %q", n.Content[i].Value)
		}
	}
	return n.Decode(p)
}
func notationDuration(p []WrittenPhrase, meter float64) (float64, error) {
	total, bar := 0., 0.
	number := 1
	for _, phrase := range p {
		for _, token := range strings.Fields(phrase.Notes) {
			if token == "|" {
				if math.Abs(bar-meter) > 1e-7 {
					return 0, fmt.Errorf("bar %d has %g beats, expected %g", number, bar, meter)
				}
				bar = 0
				number++
				continue
			}
			raw := strings.Split(token, "@")[0]
			beats := 1.
			if prefix, _, ok := strings.Cut(raw, ":"); ok {
				var err error
				beats, err = strconv.ParseFloat(prefix, 64)
				if err != nil || !finiteRange(beats, 0, 1e9) || beats <= 0 {
					return 0, fmt.Errorf("invalid duration in %q", token)
				}
			}
			total += beats
			bar += beats
		}
	}
	// With separators, any unfinished final bar must also be complete.
	if number > 1 && bar != 0 && math.Abs(bar-meter) > 1e-7 {
		return 0, fmt.Errorf("bar %d has %g beats, expected %g", number, bar, meter)
	}
	return total, nil
}
func compileNotation(root *yaml.Node, v notationScore) (*Score, error) {
	if !finiteRange(v.Tempo, 1, 1000) {
		return nil, fmt.Errorf("tempo must be 1..1000 BPM")
	}
	meter := 4.
	if v.Meter != "" {
		a, b, ok := strings.Cut(v.Meter, "/")
		num, e1 := strconv.Atoi(a)
		den, e2 := strconv.Atoi(b)
		if !ok || e1 != nil || e2 != nil || num < 1 || num > 32 || den < 1 || den > 32 || den&(den-1) != 0 {
			return nil, fmt.Errorf("invalid meter %q", v.Meter)
		}
		meter = float64(num) * 4 / float64(den)
	}
	s := &Score{Name: v.Name, Key: v.Key, Length: 60 / v.Tempo, Volume: v.Volume, Humanize: v.Humanize, Swing: v.Swing, Reverb: v.Reverb, Marks: v.Marks, Instruments: map[string]string{}, Playing: map[string]string{}, Levels: map[string]float64{}, Pan: map[string]float64{}, PlayingStyles: map[string]PlayingStyle{}}
	builtin := map[string]yaml.Node{}
	for name, style := range defaultPlayingStyles() {
		var n yaml.Node
		if err := n.Encode(style); err != nil {
			return nil, err
		}
		builtin[name] = n
	}
	for name := range v.Styles {
		n, err := resolveNotation(name, v.Styles, builtin, map[string]bool{}, true)
		if err != nil {
			return nil, fmt.Errorf("style %s: %w", name, err)
		}
		var st PlayingStyle
		if err = strictNotation(n, &st); err != nil {
			return nil, err
		}
		s.PlayingStyles[name] = st
	}
	trackNode := notationField(root, "tracks")
	if trackNode == nil || len(v.Tracks) == 0 {
		return nil, fmt.Errorf("tracks are required")
	}
	channels := map[string]string{}
	count := 0
	for i := 0; i < len(trackNode.Content); i += 2 {
		name := trackNode.Content[i].Value
		t := v.Tracks[name]
		ch := "drums"
		if t.Instrument != "drums" {
			count++
			if count > 3 {
				return nil, fmt.Errorf("at most three pitched tracks are supported")
			}
			ch = fmt.Sprintf("C%d", count)
			if t.Instrument == "" {
				return nil, fmt.Errorf("track %s: instrument is required", name)
			}
			s.Instruments[ch] = t.Instrument
			if t.Playing != "" {
				s.Playing[ch] = t.Playing
			}
		} else {
			for _, existing := range channels {
				if existing == "drums" {
					return nil, fmt.Errorf("only one drums track is supported")
				}
			}
			if t.Playing != "" {
				return nil, fmt.Errorf("drums do not support playing styles")
			}
		}
		channels[name] = ch
		if t.Level != nil {
			s.Levels[ch] = *t.Level
		}
		if t.Pan != nil {
			s.Pan[ch] = *t.Pan
		}
	}
	if len(v.Form) == 0 {
		return nil, fmt.Errorf("form is required")
	}
	budget := 0
	for _, item := range v.Form {
		name, repeats, err := notationRepeat(&item)
		if err != nil {
			return nil, fmt.Errorf("form: %w", err)
		}
		node, err := resolveNotation(name, v.Sections, nil, map[string]bool{}, false)
		if err != nil {
			return nil, err
		}
		var def notationSection
		if err = strictNotation(node, &def); err != nil {
			return nil, fmt.Errorf("section %s: %w", name, err)
		}
		for r := 0; r < repeats; r++ {
			if len(s.Sections) >= notationMaxSections {
				return nil, fmt.Errorf("form exceeds %d sections", notationMaxSections)
			}
			sec := Section{Label: name, Dynamic: def.Dynamic, Hairpin: def.Hairpin, Ritardando: def.Ritardando, Rubato: def.Rubato, Fermata: def.Fermata, Playing: map[string]string{}, Parts: map[string][]WrittenPhrase{}}
			if def.Label != "" {
				sec.Label = def.Label
			}
			if def.Tempo != 0 {
				if !finiteRange(def.Tempo, 1, 1000) {
					return nil, fmt.Errorf("section %s: invalid tempo", name)
				}
				sec.Length = 60 / def.Tempo
			}
			if def.Bars < 0 || def.Bars > 4096 || !finiteRange(def.Beats, 0, 16384) || def.Bars > 0 && def.Beats > 0 {
				return nil, fmt.Errorf("section %s: use positive bars or beats, not both", name)
			}
			duration := def.Beats
			if def.Bars > 0 {
				duration = float64(def.Bars) * meter
			}
			lengths := map[string]float64{}
			for track, n := range def.Parts {
				ch, ok := channels[track]
				if !ok {
					return nil, fmt.Errorf("section %s: unknown track %s", name, track)
				}
				if n.Kind == yaml.ScalarNode && n.Value == "off" {
					continue
				}
				phrases, err := expandNotationPart(&n, v.Patterns, map[string]bool{}, &budget)
				if err != nil {
					return nil, fmt.Errorf("section %s, track %s: %w", name, track, err)
				}
				beats, err := notationDuration(phrases, meter)
				if err != nil {
					return nil, fmt.Errorf("section %s, track %s: %w", name, track, err)
				}
				lengths[track] = beats
				if duration == 0 {
					duration = beats
				}
				if ch == "drums" {
					for _, p := range phrases {
						if p.Unphrased || p.PeakDB != nil || p.BreathMS != nil || p.AttackMS != nil {
							return nil, fmt.Errorf("drum phrases cannot use pitched expression")
						}
						for _, tok := range strings.Fields(p.Notes) {
							if tok != "|" {
								sec.Drums = append(sec.Drums, tok)
							}
						}
					}
				} else {
					sec.Parts[ch] = phrases
				}
			}
			if duration <= 0 {
				return nil, fmt.Errorf("section %s: empty; specify bars for a silent section", name)
			}
			for track, beats := range lengths {
				if math.Abs(beats-duration) > 1e-7 {
					return nil, fmt.Errorf("section %s, track %s: %g beats, expected %g", name, track, beats, duration)
				}
			}
			for track, style := range def.Playing {
				ch, ok := channels[track]
				if !ok || ch == "drums" {
					return nil, fmt.Errorf("section %s: invalid playing track %s", name, track)
				}
				sec.Playing[ch] = style
			}
			for _, ch := range channels {
				rest := fmt.Sprintf("%g:z", duration)
				if ch == "drums" {
					if len(sec.Drums) == 0 {
						sec.Drums = []string{rest}
					}
				} else if _, ok := sec.Parts[ch]; !ok {
					sec.Parts[ch] = []WrittenPhrase{{Notes: rest}}
				}
			}
			s.Sections = append(s.Sections, sec)
		}
	}
	return s, nil
}

package muse

import "sort"

// InstrumentNames lists registered procedural instruments. Aliases and the
// legacy voice are included so clients can validate against the actual engine.
func InstrumentNames() []string {
	names := make([]string, 0, len(instruments))
	for name := range instruments {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

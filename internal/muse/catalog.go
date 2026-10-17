package muse

import (
	"maps"
	"slices"
)

// InstrumentNames lists registered procedural instruments. Aliases and the
// legacy voice are included so clients can validate against the actual engine.
func InstrumentNames() []string {
	return slices.Sorted(maps.Keys(instruments))
}

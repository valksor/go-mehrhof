// Package configcheck provides config drift detection by comparing
// two settings configs field-by-field and reporting differences.
package configcheck

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
)

// Drift represents a single configuration difference between
// a reference config and an actual config.
type Drift struct {
	Path        string `json:"path"`
	Expected    any    `json:"expected"`
	Actual      any    `json:"actual"`
	Description string `json:"description"`
}

// Check recursively compares two flat maps (from JSON-unmarshaled settings)
// and returns all differences. Keys in reference that are missing or differ
// in actual are reported as Drift entries.
func Check(reference, actual map[string]any) []Drift {
	var drifts []Drift
	checkRecursive("", reference, actual, &drifts)

	slices.SortFunc(drifts, func(a, b Drift) int {
		return cmp.Compare(a.Path, b.Path)
	})

	return drifts
}

func checkRecursive(prefix string, reference, actual map[string]any, drifts *[]Drift) {
	keys := slices.Sorted(maps.Keys(reference))

	for _, key := range keys {
		path := key
		if prefix != "" {
			path = prefix + "." + key
		}

		refVal := reference[key]
		actVal, exists := actual[key]

		if !exists {
			*drifts = append(*drifts, Drift{
				Path:        path,
				Expected:    refVal,
				Actual:      nil,
				Description: fmt.Sprintf("key %q missing from actual config", path),
			})

			continue
		}

		// If both are nested maps, recurse.
		refMap, refIsMap := refVal.(map[string]any)
		actMap, actIsMap := actVal.(map[string]any)

		if refIsMap && actIsMap {
			checkRecursive(path, refMap, actMap, drifts)

			continue
		}

		if fmt.Sprintf("%v", refVal) != fmt.Sprintf("%v", actVal) {
			*drifts = append(*drifts, Drift{
				Path:        path,
				Expected:    refVal,
				Actual:      actVal,
				Description: fmt.Sprintf("value differs at %q", path),
			})
		}
	}
}

package settings

import (
	"cmp"
	"fmt"
	"maps"
	"reflect"
	"slices"
)

// ConfigDrift represents a single configuration difference between
// a reference config and an actual config.
type ConfigDrift struct {
	Path        string `json:"path"`
	Expected    any    `json:"expected"`
	Actual      any    `json:"actual"`
	Description string `json:"description"`
}

// CheckConfigDrift recursively compares two maps (from JSON-unmarshaled settings)
// and returns all differences. Keys in reference that are missing or differ
// in actual are reported as ConfigDrift entries.
func CheckConfigDrift(reference, actual map[string]any) []ConfigDrift {
	var drifts []ConfigDrift
	checkDriftRecursive("", reference, actual, &drifts)

	slices.SortFunc(drifts, func(a, b ConfigDrift) int {
		return cmp.Compare(a.Path, b.Path)
	})

	return drifts
}

func checkDriftRecursive(prefix string, reference, actual map[string]any, drifts *[]ConfigDrift) {
	keys := slices.Sorted(maps.Keys(reference))

	for _, key := range keys {
		path := key
		if prefix != "" {
			path = prefix + "." + key
		}

		refVal := reference[key]
		actVal, exists := actual[key]

		if !exists {
			*drifts = append(*drifts, ConfigDrift{
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
			checkDriftRecursive(path, refMap, actMap, drifts)

			continue
		}

		if !reflect.DeepEqual(refVal, actVal) {
			*drifts = append(*drifts, ConfigDrift{
				Path:        path,
				Expected:    refVal,
				Actual:      actVal,
				Description: fmt.Sprintf("value differs at %q", path),
			})
		}
	}
}

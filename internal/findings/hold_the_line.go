package findings

// LineRange represents a contiguous range of lines [Start, End] inclusive.
type LineRange struct {
	Start int
	End   int
}

// ClassifyByDiff marks each finding as introduced or pre-existing based on
// whether its File:Line falls within the changed hunks. Findings without a
// file or line are marked as unknown. The diffHunks map keys are file paths
// relative to the repository root.
func ClassifyByDiff(findings []Finding, diffHunks map[string][]LineRange) []Finding {
	result := make([]Finding, len(findings))
	copy(result, findings)

	for i := range result {
		if result[i].File == "" || result[i].Line == 0 {
			result[i].Origin = OriginUnknown

			continue
		}

		hunks, ok := diffHunks[result[i].File]
		if !ok {
			// File was not changed in the diff — pre-existing.
			result[i].Origin = OriginPreExisting

			continue
		}

		if lineInHunks(result[i].Line, hunks) {
			result[i].Origin = OriginIntroduced
		} else {
			result[i].Origin = OriginPreExisting
		}
	}

	return result
}

// FilterIntroduced returns only findings marked as introduced (new in the
// current change). Findings with unknown origin are also included because
// they cannot be proven pre-existing.
func FilterIntroduced(findings []Finding) []Finding {
	var result []Finding
	for _, f := range findings {
		if f.Origin == OriginIntroduced || f.Origin == OriginUnknown {
			result = append(result, f)
		}
	}

	return result
}

// lineInHunks reports whether line falls within any of the given ranges.
func lineInHunks(line int, hunks []LineRange) bool {
	for _, h := range hunks {
		if line >= h.Start && line <= h.End {
			return true
		}
	}

	return false
}

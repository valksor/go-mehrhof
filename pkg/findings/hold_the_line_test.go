package findings

import "testing"

func TestClassifyByDiff(t *testing.T) {
	hunks := map[string][]LineRange{
		"main.go": {{Start: 10, End: 20}, {Start: 50, End: 55}},
		"util.go": {{Start: 1, End: 5}},
	}

	input := []Finding{
		{File: "main.go", Line: 15, Message: "in changed hunk"},
		{File: "main.go", Line: 30, Message: "outside hunk"},
		{File: "main.go", Line: 50, Message: "at hunk start"},
		{File: "main.go", Line: 55, Message: "at hunk end"},
		{File: "other.go", Line: 1, Message: "unchanged file"},
		{File: "", Line: 0, Message: "no location"},
		{File: "util.go", Line: 3, Message: "in util hunk"},
	}

	result := ClassifyByDiff(input, hunks)

	expected := []Origin{
		OriginIntroduced,
		OriginPreExisting,
		OriginIntroduced,
		OriginIntroduced,
		OriginPreExisting,
		OriginUnknown,
		OriginIntroduced,
	}

	for i, want := range expected {
		if result[i].Origin != want {
			t.Errorf("finding[%d] (%s) origin = %q, want %q", i, result[i].Message, result[i].Origin, want)
		}
	}
}

func TestClassifyByDiffDoesNotMutateInput(t *testing.T) {
	hunks := map[string][]LineRange{
		"a.go": {{Start: 1, End: 10}},
	}
	input := []Finding{{File: "a.go", Line: 5}}
	result := ClassifyByDiff(input, hunks)

	if input[0].Origin != "" {
		t.Error("ClassifyByDiff mutated original slice")
	}
	if result[0].Origin != OriginIntroduced {
		t.Errorf("result origin = %q, want %q", result[0].Origin, OriginIntroduced)
	}
}

func TestFilterIntroduced(t *testing.T) {
	findings := []Finding{
		{Origin: OriginIntroduced, Message: "new"},
		{Origin: OriginPreExisting, Message: "old"},
		{Origin: OriginUnknown, Message: "unknown"},
		{Origin: OriginPreExisting, Message: "old2"},
		{Origin: OriginIntroduced, Message: "new2"},
	}

	result := FilterIntroduced(findings)
	if len(result) != 3 {
		t.Fatalf("expected 3 findings, got %d", len(result))
	}
	if result[0].Message != "new" {
		t.Errorf("result[0] = %q, want %q", result[0].Message, "new")
	}
	if result[1].Message != "unknown" {
		t.Errorf("result[1] = %q, want %q", result[1].Message, "unknown")
	}
	if result[2].Message != "new2" {
		t.Errorf("result[2] = %q, want %q", result[2].Message, "new2")
	}
}

func TestFilterIntroducedEmpty(t *testing.T) {
	result := FilterIntroduced(nil)
	if len(result) != 0 {
		t.Errorf("expected 0 findings, got %d", len(result))
	}
}

func TestFilterIntroducedAllPreExisting(t *testing.T) {
	findings := []Finding{
		{Origin: OriginPreExisting},
		{Origin: OriginPreExisting},
	}
	result := FilterIntroduced(findings)
	if len(result) != 0 {
		t.Errorf("expected 0 findings, got %d", len(result))
	}
}

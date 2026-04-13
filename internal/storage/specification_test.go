package storage

import (
	"strings"
	"testing"
)

func newTestSpecStore(t *testing.T) *SpecStore {
	t.Helper()

	return NewSpecStore(newTestStore(t))
}

func TestSaveLoadSpecification(t *testing.T) {
	ss := newTestSpecStore(t)

	content := "# My Specification\n\nImplement X feature."
	if err := ss.SaveSpecification("task-1", 1, content); err != nil {
		t.Fatalf("SaveSpecification() error = %v", err)
	}

	got, err := ss.LoadSpecification("task-1", 1)
	if err != nil {
		t.Fatalf("LoadSpecification() error = %v", err)
	}
	if got != content {
		t.Errorf("LoadSpecification() = %q, want %q", got, content)
	}
}

func TestSaveSpecification_InvalidTaskID(t *testing.T) {
	ss := newTestSpecStore(t)

	tests := []struct {
		name   string
		taskID string
	}{
		{"path traversal", "../traversal"},
		{"empty", ""},
		{"with slash", "task/name"},
		{"with backslash", `task\name`},
		{"with spaces", "task name"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ss.SaveSpecification(tt.taskID, 1, "content")
			if err == nil {
				t.Errorf("SaveSpecification(%q) expected error, got nil", tt.taskID)
			}
		})
	}
}

func TestIsValidTaskID(t *testing.T) {
	tests := []struct {
		id    string
		valid bool
	}{
		{"task-123", true},
		{"task_abc", true},
		{"TASK123", true},
		{"task-abc-123", true},
		{"", false},
		{"task/path", false},
		{"../traversal", false},
		{"task name", false},
		{`task\name`, false},
	}

	for _, tt := range tests {
		got := isValidTaskID(tt.id)
		if got != tt.valid {
			t.Errorf("isValidTaskID(%q) = %v, want %v", tt.id, got, tt.valid)
		}
	}
}

func TestParseSpecification_NoFrontmatter(t *testing.T) {
	ss := newTestSpecStore(t)

	content := "# Spec Title\n\nThis is the specification."
	if err := ss.SaveSpecification("task-1", 1, content); err != nil {
		t.Fatal(err)
	}

	spec, err := ss.ParseSpecification("task-1", 1)
	if err != nil {
		t.Fatalf("ParseSpecification() error = %v", err)
	}

	if spec.Number != 1 {
		t.Errorf("Number = %d, want 1", spec.Number)
	}
	if spec.Title != "Spec Title" {
		t.Errorf("Title = %q, want Spec Title", spec.Title)
	}
	if spec.Status != SpecStatusDraft {
		t.Errorf("Status = %q, want %q", spec.Status, SpecStatusDraft)
	}
}

func TestParseSpecification_WithFrontmatter(t *testing.T) {
	ss := newTestSpecStore(t)

	content := "---\nstatus: ready\ntitle: Custom Title\n---\n\n# Ignored Header\n\nContent."
	if err := ss.SaveSpecification("task-1", 1, content); err != nil {
		t.Fatal(err)
	}

	spec, err := ss.ParseSpecification("task-1", 1)
	if err != nil {
		t.Fatal(err)
	}

	if spec.Status != "ready" {
		t.Errorf("Status = %q, want ready", spec.Status)
	}
}

func TestSaveSpecificationWithMeta(t *testing.T) {
	ss := newTestSpecStore(t)

	spec := &Specification{
		Number:  1,
		Status:  SpecStatusReady,
		Content: "# Spec\n\nContent.",
	}

	if err := ss.SaveSpecificationWithMeta("task-1", spec); err != nil {
		t.Fatalf("SaveSpecificationWithMeta() error = %v", err)
	}

	raw, err := ss.LoadSpecification("task-1", 1)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(raw, "---\n") {
		t.Error("SaveSpecificationWithMeta() did not write YAML frontmatter")
	}
	if spec.CreatedAt.IsZero() {
		t.Error("CreatedAt not set")
	}
	if spec.UpdatedAt.IsZero() {
		t.Error("UpdatedAt not set")
	}
}

func TestUpdateSpecificationStatus(t *testing.T) {
	ss := newTestSpecStore(t)

	spec := &Specification{Number: 1, Status: SpecStatusDraft, Content: "# Spec\n\nContent."}
	if err := ss.SaveSpecificationWithMeta("task-1", spec); err != nil {
		t.Fatal(err)
	}

	if err := ss.UpdateSpecificationStatus("task-1", 1, SpecStatusDone); err != nil {
		t.Fatalf("UpdateSpecificationStatus() error = %v", err)
	}

	got, err := ss.ParseSpecification("task-1", 1)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != SpecStatusDone {
		t.Errorf("Status = %q, want %q", got.Status, SpecStatusDone)
	}
	if got.CompletedAt.IsZero() {
		t.Error("CompletedAt is zero after done, want non-zero")
	}
}

func TestUpdateSpecificationStatus_DoneOnly(t *testing.T) {
	ss := newTestSpecStore(t)

	spec := &Specification{Number: 1, Status: SpecStatusDraft, Content: "# Spec\n\nContent."}
	if err := ss.SaveSpecificationWithMeta("task-1", spec); err != nil {
		t.Fatal(err)
	}

	if err := ss.UpdateSpecificationStatus("task-1", 1, SpecStatusReady); err != nil {
		t.Fatal(err)
	}

	got, _ := ss.ParseSpecification("task-1", 1)
	if !got.CompletedAt.IsZero() {
		t.Error("CompletedAt should be zero for non-done status")
	}
}

func TestListSpecifications_Empty(t *testing.T) {
	ss := newTestSpecStore(t)

	specs, err := ss.ListSpecifications("task-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(specs) != 0 {
		t.Errorf("ListSpecifications() empty = %v, want []", specs)
	}
}

func TestListSpecifications_Sorted(t *testing.T) {
	ss := newTestSpecStore(t)

	for _, n := range []int{3, 1, 2} {
		if err := ss.SaveSpecification("task-1", n, "content"); err != nil {
			t.Fatal(err)
		}
	}

	specs, err := ss.ListSpecifications("task-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(specs) != 3 {
		t.Fatalf("ListSpecifications() len = %d, want 3", len(specs))
	}
	for i, want := range []int{1, 2, 3} {
		if specs[i] != want {
			t.Errorf("specs[%d] = %d, want %d", i, specs[i], want)
		}
	}
}

func TestNextSpecificationNumber(t *testing.T) {
	ss := newTestSpecStore(t)

	n, err := ss.NextSpecificationNumber("task-1")
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("NextSpecificationNumber() empty = %d, want 1", n)
	}

	if err := ss.SaveSpecification("task-1", 1, "content"); err != nil {
		t.Fatal(err)
	}

	n, err = ss.NextSpecificationNumber("task-1")
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("NextSpecificationNumber() after one = %d, want 2", n)
	}
}

func TestGetLatestSpecificationContent_Empty(t *testing.T) {
	ss := newTestSpecStore(t)

	content, num, err := ss.GetLatestSpecificationContent("task-1")
	if err != nil {
		t.Fatal(err)
	}
	if content != "" || num != 0 {
		t.Errorf("GetLatestSpecificationContent() empty = (%q, %d), want (\"\", 0)", content, num)
	}
}

func TestGetLatestSpecificationContent(t *testing.T) {
	ss := newTestSpecStore(t)

	for i, c := range []string{"first", "second", "third"} {
		if err := ss.SaveSpecification("task-1", i+1, c); err != nil {
			t.Fatal(err)
		}
	}

	content, num, err := ss.GetLatestSpecificationContent("task-1")
	if err != nil {
		t.Fatal(err)
	}
	if num != 3 {
		t.Errorf("num = %d, want 3", num)
	}
	if content != "third" {
		t.Errorf("content = %q, want third", content)
	}
}

func TestGatherSpecificationsContent(t *testing.T) {
	ss := newTestSpecStore(t)

	for i, c := range []string{"spec one content", "spec two content"} {
		if err := ss.SaveSpecification("task-1", i+1, c); err != nil {
			t.Fatal(err)
		}
	}

	gathered, err := ss.GatherSpecificationsContent("task-1")
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(gathered, "Specification 1") {
		t.Error("GatherSpecificationsContent() missing Specification 1 header")
	}
	if !strings.Contains(gathered, "Specification 2") {
		t.Error("GatherSpecificationsContent() missing Specification 2 header")
	}
	if !strings.Contains(gathered, "spec one content") {
		t.Error("GatherSpecificationsContent() missing spec 1 content")
	}
}

func TestListSpecificationsWithStatus(t *testing.T) {
	ss := newTestSpecStore(t)

	for i, status := range []string{SpecStatusDraft, SpecStatusReady, SpecStatusDone} {
		spec := &Specification{Number: i + 1, Status: status, Content: "# Spec\n\nContent."}
		if err := ss.SaveSpecificationWithMeta("task-1", spec); err != nil {
			t.Fatal(err)
		}
	}

	specs, err := ss.ListSpecificationsWithStatus("task-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(specs) != 3 {
		t.Fatalf("ListSpecificationsWithStatus() len = %d, want 3", len(specs))
	}

	statuses := map[string]bool{}
	for _, spec := range specs {
		statuses[spec.Status] = true
	}
	for _, want := range []string{SpecStatusDraft, SpecStatusReady, SpecStatusDone} {
		if !statuses[want] {
			t.Errorf("missing status %q in result", want)
		}
	}
}

func TestGetSpecificationsSummary(t *testing.T) {
	ss := newTestSpecStore(t)

	for i, status := range []string{SpecStatusDraft, SpecStatusDraft, SpecStatusReady} {
		spec := &Specification{Number: i + 1, Status: status, Content: "# Spec\n\nContent."}
		if err := ss.SaveSpecificationWithMeta("task-1", spec); err != nil {
			t.Fatal(err)
		}
	}

	summary, err := ss.GetSpecificationsSummary("task-1")
	if err != nil {
		t.Fatal(err)
	}

	if summary[SpecStatusDraft] != 2 {
		t.Errorf("draft count = %d, want 2", summary[SpecStatusDraft])
	}
	if summary[SpecStatusReady] != 1 {
		t.Errorf("ready count = %d, want 1", summary[SpecStatusReady])
	}
	if summary[SpecStatusDone] != 0 {
		t.Errorf("done count = %d, want 0", summary[SpecStatusDone])
	}
}

func TestDeleteSpecification(t *testing.T) {
	ss := newTestSpecStore(t)

	// Non-existent is not an error
	if err := ss.DeleteSpecification("task-1", 99); err != nil {
		t.Errorf("DeleteSpecification() non-existent error = %v, want nil", err)
	}

	if err := ss.SaveSpecification("task-1", 1, "content"); err != nil {
		t.Fatal(err)
	}

	if err := ss.DeleteSpecification("task-1", 1); err != nil {
		t.Fatalf("DeleteSpecification() error = %v", err)
	}

	specs, _ := ss.ListSpecifications("task-1")
	if len(specs) != 0 {
		t.Errorf("ListSpecifications() after delete = %v, want empty", specs)
	}
}

func TestSpecificationCount(t *testing.T) {
	ss := newTestSpecStore(t)

	n, err := ss.SpecificationCount("task-1")
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("SpecificationCount() empty = %d, want 0", n)
	}

	for i := 1; i <= 3; i++ {
		if err := ss.SaveSpecification("task-1", i, "content"); err != nil {
			t.Fatal(err)
		}
	}

	n, err = ss.SpecificationCount("task-1")
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Errorf("SpecificationCount() = %d, want 3", n)
	}
}

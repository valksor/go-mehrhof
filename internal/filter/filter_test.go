package filter_test

import (
	"testing"
	"time"

	"github.com/valksor/kvelmo/internal/filter"
	"github.com/valksor/kvelmo/internal/page"
)

type task struct {
	ID       string
	Status   string
	Tags     []string
	Created  time.Time
	Priority int
}

var (
	now   = time.Now()
	tasks = []task{
		{ID: "t1", Status: "active", Tags: []string{"bug", "urgent"}, Created: now.Add(-2 * time.Hour), Priority: 3},
		{ID: "t2", Status: "done", Tags: []string{"feature"}, Created: now.Add(-1 * time.Hour), Priority: 1},
		{ID: "t3", Status: "active", Tags: []string{"bug"}, Created: now.Add(-30 * time.Minute), Priority: 2},
		{ID: "t4", Status: "paused", Tags: []string{"feature", "urgent"}, Created: now.Add(-10 * time.Minute), Priority: 5},
		{ID: "t5", Status: "active", Tags: []string{"docs"}, Created: now, Priority: 1},
	}

	statusAccessor   = func(t task) any { return t.Status }
	idAccessor       = func(t task) any { return t.ID }
	priorityAccessor = func(t task) any { return t.Priority }
)

func TestEmptyFilterMatchesEverything(t *testing.T) {
	f := filter.New[task]()
	result := f.Apply(tasks)

	if len(result) != len(tasks) {
		t.Fatalf("expected %d items, got %d", len(tasks), len(result))
	}
}

func TestEq(t *testing.T) {
	f := filter.New[task]().Eq(statusAccessor, "active")
	result := f.Apply(tasks)

	if len(result) != 3 {
		t.Fatalf("expected 3 active tasks, got %d", len(result))
	}

	for _, item := range result {
		if item.Status != "active" {
			t.Errorf("expected status active, got %s", item.Status)
		}
	}
}

func TestNeq(t *testing.T) {
	f := filter.New[task]().Neq(statusAccessor, "active")
	result := f.Apply(tasks)

	if len(result) != 2 {
		t.Fatalf("expected 2 non-active tasks, got %d", len(result))
	}

	for _, item := range result {
		if item.Status == "active" {
			t.Error("expected non-active status")
		}
	}
}

func TestContainsCaseInsensitive(t *testing.T) {
	f := filter.New[task]().Contains(idAccessor, "T") // uppercase T, IDs are lowercase t
	result := f.Apply(tasks)

	if len(result) != 5 {
		t.Fatalf("expected 5 tasks (case-insensitive), got %d", len(result))
	}
}

func TestContainsSubstring(t *testing.T) {
	f := filter.New[task]().Contains(statusAccessor, "act")
	result := f.Apply(tasks)

	if len(result) != 3 {
		t.Fatalf("expected 3 tasks containing 'act', got %d", len(result))
	}
}

func TestIn(t *testing.T) {
	f := filter.New[task]().In(statusAccessor, "active", "paused")
	result := f.Apply(tasks)

	if len(result) != 4 {
		t.Fatalf("expected 4 tasks, got %d", len(result))
	}
}

func TestSince(t *testing.T) {
	cutoff := now.Add(-45 * time.Minute)
	f := filter.New[task]().Since(func(t task) time.Time { return t.Created }, cutoff)
	result := f.Apply(tasks)

	// t3 (-30m), t4 (-10m), t5 (now) are after cutoff
	if len(result) != 3 {
		t.Fatalf("expected 3 tasks since cutoff, got %d", len(result))
	}
}

func TestBefore(t *testing.T) {
	cutoff := now.Add(-45 * time.Minute)
	f := filter.New[task]().Before(func(t task) time.Time { return t.Created }, cutoff)
	result := f.Apply(tasks)

	// t1 (-2h), t2 (-1h) are before cutoff
	if len(result) != 2 {
		t.Fatalf("expected 2 tasks before cutoff, got %d", len(result))
	}
}

func TestHasTag(t *testing.T) {
	f := filter.New[task]().HasTag(func(t task) []string { return t.Tags }, "urgent")
	result := f.Apply(tasks)

	if len(result) != 2 {
		t.Fatalf("expected 2 urgent tasks, got %d", len(result))
	}
}

func TestMultiplePredicatesAND(t *testing.T) {
	f := filter.New[task]().
		Eq(statusAccessor, "active").
		HasTag(func(t task) []string { return t.Tags }, "bug")

	result := f.Apply(tasks)

	// t1 (active+bug), t3 (active+bug)
	if len(result) != 2 {
		t.Fatalf("expected 2 active+bug tasks, got %d", len(result))
	}
}

func TestMatchSingleItem(t *testing.T) {
	f := filter.New[task]().Eq(statusAccessor, "done")

	if !f.Match(tasks[1]) {
		t.Error("expected t2 to match")
	}

	if f.Match(tasks[0]) {
		t.Error("expected t1 not to match")
	}
}

func TestApplyPaginated(t *testing.T) {
	f := filter.New[task]().Eq(statusAccessor, "active")
	p := page.Pagination{Page: 1, PerPage: 2}
	result := f.ApplyPaginated(tasks, p)

	if result.Total != 3 {
		t.Fatalf("expected total 3, got %d", result.Total)
	}

	if len(result.Items) != 2 {
		t.Fatalf("expected 2 items on page 1, got %d", len(result.Items))
	}

	if !result.HasNext {
		t.Error("expected HasNext to be true")
	}

	// Page 2
	p.Page = 2
	result = f.ApplyPaginated(tasks, p)

	if len(result.Items) != 1 {
		t.Fatalf("expected 1 item on page 2, got %d", len(result.Items))
	}

	if result.HasNext {
		t.Error("expected HasNext to be false on last page")
	}
}

func TestCustomWherePredicate(t *testing.T) {
	f := filter.New[task]().Where(func(item task) bool {
		return item.Priority > 2
	})
	result := f.Apply(tasks)

	// t1 (3), t4 (5)
	if len(result) != 2 {
		t.Fatalf("expected 2 high-priority tasks, got %d", len(result))
	}
}

func TestSortByString(t *testing.T) {
	items := make([]task, len(tasks))
	copy(items, tasks)

	filter.Sort(items, filter.SortByString(func(t task) string { return t.ID }, false))

	if items[0].ID != "t1" || items[4].ID != "t5" {
		t.Errorf("expected ascending ID order, got %s...%s", items[0].ID, items[4].ID)
	}

	filter.Sort(items, filter.SortByString(func(t task) string { return t.ID }, true))

	if items[0].ID != "t5" || items[4].ID != "t1" {
		t.Errorf("expected descending ID order, got %s...%s", items[0].ID, items[4].ID)
	}
}

func TestSortByTime(t *testing.T) {
	items := make([]task, len(tasks))
	copy(items, tasks)

	filter.Sort(items, filter.SortByTime(func(t task) time.Time { return t.Created }, false))

	if items[0].ID != "t1" {
		t.Errorf("expected oldest first (t1), got %s", items[0].ID)
	}

	if items[4].ID != "t5" {
		t.Errorf("expected newest last (t5), got %s", items[4].ID)
	}
}

func TestSortByInt(t *testing.T) {
	items := make([]task, len(tasks))
	copy(items, tasks)

	filter.Sort(items, filter.SortByInt(func(t task) int { return t.Priority }, true))

	if items[0].Priority != 5 {
		t.Errorf("expected highest priority first (5), got %d", items[0].Priority)
	}
}

func TestContainsNonStringField(t *testing.T) {
	f := filter.New[task]().Contains(priorityAccessor, "3")
	result := f.Apply(tasks)

	// Priority is int, not string, so accessor returns int - Contains should return false
	if len(result) != 0 {
		t.Fatalf("expected 0 matches for non-string field, got %d", len(result))
	}
}

func TestInEmptyValues(t *testing.T) {
	f := filter.New[task]().In(statusAccessor)
	result := f.Apply(tasks)

	if len(result) != 0 {
		t.Fatalf("expected 0 matches for empty In set, got %d", len(result))
	}
}

func TestApplyEmptySlice(t *testing.T) {
	f := filter.New[task]().Eq(statusAccessor, "active")
	result := f.Apply(nil)

	if len(result) != 0 {
		t.Fatalf("expected 0 items from nil slice, got %d", len(result))
	}
}

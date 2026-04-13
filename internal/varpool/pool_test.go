package varpool

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSetAndGet(t *testing.T) {
	p := New()

	p.Set("name", "alice", "test")
	p.Set("age", 30.0, "test")
	p.Set("active", true, "test")
	p.Set("tags", []string{"a", "b"}, "test")

	tests := []struct {
		name     string
		key      string
		wantType VarType
	}{
		{"string", "name", TypeString},
		{"number", "age", TypeNumber},
		{"bool", "active", TypeBool},
		{"json", "tags", TypeJSON},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, ok := p.Get(tt.key)
			if !ok {
				t.Fatalf("key %q not found", tt.key)
			}
			if v.Type != tt.wantType {
				t.Fatalf("expected type %s, got %s", tt.wantType, v.Type)
			}
			if v.SetBy != "test" {
				t.Fatalf("expected SetBy 'test', got %q", v.SetBy)
			}
		})
	}
}

func TestGetMissing(t *testing.T) {
	p := New()

	if _, ok := p.Get("missing"); ok {
		t.Fatal("expected missing key to return false")
	}

	if s := p.GetString("missing"); s != "" {
		t.Fatalf("expected empty string, got %q", s)
	}

	if n := p.GetNumber("missing"); n != 0 {
		t.Fatalf("expected 0, got %f", n)
	}

	if b := p.GetBool("missing"); b {
		t.Fatal("expected false")
	}
}

func TestGetWrongType(t *testing.T) {
	p := New()
	p.Set("num", 42.0, "test")

	if s := p.GetString("num"); s != "" {
		t.Fatalf("GetString on number should return empty, got %q", s)
	}

	p.Set("str", "hello", "test")

	if n := p.GetNumber("str"); n != 0 {
		t.Fatalf("GetNumber on string should return 0, got %f", n)
	}
}

func TestDeleteAndClear(t *testing.T) {
	p := New()
	p.Set("a", "1", "test")
	p.Set("b", "2", "test")

	p.Delete("a")

	if _, ok := p.Get("a"); ok {
		t.Fatal("a should be deleted")
	}

	if p.Len() != 1 {
		t.Fatalf("expected 1 variable, got %d", p.Len())
	}

	p.Clear()

	if p.Len() != 0 {
		t.Fatal("expected 0 after clear")
	}
}

func TestListSorted(t *testing.T) {
	p := New()
	p.Set("c", "3", "test")
	p.Set("a", "1", "test")
	p.Set("b", "2", "test")

	vars := p.List()
	if len(vars) != 3 {
		t.Fatalf("expected 3 vars, got %d", len(vars))
	}

	if vars[0].Name != "a" || vars[1].Name != "b" || vars[2].Name != "c" {
		t.Fatalf("expected sorted [a b c], got [%s %s %s]", vars[0].Name, vars[1].Name, vars[2].Name)
	}
}

func TestSaveAndLoad(t *testing.T) {
	p := New()
	p.Set("greeting", "hello", "test")
	p.Set("count", 42.0, "test")

	path := filepath.Join(t.TempDir(), "varpool.json")

	if err := p.Save(path); err != nil {
		t.Fatalf("save: %v", err)
	}

	p2 := New()
	if err := p2.Load(path); err != nil {
		t.Fatalf("load: %v", err)
	}

	if p2.GetString("greeting") != "hello" {
		t.Fatalf("expected 'hello', got %q", p2.GetString("greeting"))
	}

	if p2.GetNumber("count") != 42.0 {
		t.Fatalf("expected 42, got %f", p2.GetNumber("count"))
	}
}

func TestJSONRoundTrip(t *testing.T) {
	p := New()
	p.Set("x", "val", "test")

	data, err := p.MarshalJSON()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	p2 := New()
	if err := p2.UnmarshalJSON(data); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if p2.GetString("x") != "val" {
		t.Fatalf("expected 'val' after round trip, got %q", p2.GetString("x"))
	}
}

func TestCloneIndependence(t *testing.T) {
	p := New()
	p.Set("k", "original", "test")

	clone := p.Clone()
	clone.Set("k", "modified", "clone")

	if p.GetString("k") != "original" {
		t.Fatal("clone mutation should not affect original")
	}

	if clone.GetString("k") != "modified" {
		t.Fatal("clone should have modified value")
	}
}

func TestAppend(t *testing.T) {
	p := New()

	// Append to non-existent key creates a new slice.
	if err := p.Append("items", "first", "test"); err != nil {
		t.Fatalf("Append to new key: %v", err)
	}
	v, ok := p.Get("items")
	if !ok {
		t.Fatal("items key should exist after Append")
	}
	if v.Type != TypeJSON {
		t.Fatalf("expected type json, got %s", v.Type)
	}
	slice, ok := v.Value.([]any)
	if !ok || len(slice) != 1 || slice[0] != "first" {
		t.Fatalf("expected [first], got %v", v.Value)
	}

	// Append to existing slice.
	if err := p.Append("items", "second", "test"); err != nil {
		t.Fatalf("Append to existing: %v", err)
	}
	v, _ = p.Get("items")
	slice, ok = v.Value.([]any)
	if !ok {
		t.Fatal("expected []any after second Append")
	}
	if len(slice) != 2 {
		t.Fatalf("expected 2 items, got %d", len(slice))
	}

	// Append to non-slice should error.
	p.Set("str", "hello", "test")
	if err := p.Append("str", "fail", "test"); err == nil {
		t.Fatal("expected error appending to string variable")
	}
}

func TestIncrement(t *testing.T) {
	p := New()

	// Increment non-existent key creates it.
	if err := p.Increment("counter", 5.0, "test"); err != nil {
		t.Fatalf("Increment new key: %v", err)
	}
	if p.GetNumber("counter") != 5.0 {
		t.Fatalf("expected 5.0, got %f", p.GetNumber("counter"))
	}

	// Increment existing number.
	if err := p.Increment("counter", 3.0, "test"); err != nil {
		t.Fatalf("Increment existing: %v", err)
	}
	if p.GetNumber("counter") != 8.0 {
		t.Fatalf("expected 8.0, got %f", p.GetNumber("counter"))
	}

	// Increment non-number should error.
	p.Set("name", "alice", "test")
	if err := p.Increment("name", 1.0, "test"); err == nil {
		t.Fatal("expected error incrementing string variable")
	}
}

func TestDecrement(t *testing.T) {
	p := New()
	p.Set("counter", 10.0, "test")

	if err := p.Decrement("counter", 3.0, "test"); err != nil {
		t.Fatalf("Decrement: %v", err)
	}
	if p.GetNumber("counter") != 7.0 {
		t.Fatalf("expected 7.0, got %f", p.GetNumber("counter"))
	}
}

func TestClearScope(t *testing.T) {
	p := New()
	p.SetScoped(ScopePlan, "spec", "v1", "test")
	p.SetScoped(ScopePlan, "draft", "d1", "test")
	p.SetScoped(ScopeImplement, "result", "ok", "test")
	p.Set("global_key", "keep", "test")

	if p.Len() != 4 {
		t.Fatalf("expected 4 vars, got %d", p.Len())
	}

	p.ClearScope(ScopePlan)

	if p.Len() != 2 {
		t.Fatalf("expected 2 vars after ClearScope(plan), got %d", p.Len())
	}

	// Plan-scoped vars should be gone
	if _, ok := p.GetScoped(ScopePlan, "spec"); ok {
		t.Fatal("plan.spec should be cleared")
	}
	if _, ok := p.GetScoped(ScopePlan, "draft"); ok {
		t.Fatal("plan.draft should be cleared")
	}

	// Other scopes and unscoped vars should remain
	if p.GetScopedString(ScopeImplement, "result") != "ok" {
		t.Fatal("implement.result should be untouched")
	}
	if p.GetString("global_key") != "keep" {
		t.Fatal("global_key should be untouched")
	}
}

func TestClearScopeEmpty(t *testing.T) {
	p := New()
	p.Set("unscoped", "val", "test")

	// ClearScope on a scope with no vars should be a no-op
	p.ClearScope(ScopeReview)

	if p.Len() != 1 {
		t.Fatalf("expected 1 var, got %d", p.Len())
	}
}

func TestSetWithTTL(t *testing.T) {
	p := New()

	p.SetWithTTL("temp", "value", "test", 50*time.Millisecond)

	// Immediately available
	v, ok := p.Get("temp")
	if !ok {
		t.Fatal("expected temp to exist immediately")
	}
	if v.Value != "value" {
		t.Fatalf("expected value, got %v", v.Value)
	}
	if v.ExpiresAt.IsZero() {
		t.Fatal("expected non-zero ExpiresAt")
	}

	// Wait for expiry
	time.Sleep(60 * time.Millisecond)

	_, ok = p.Get("temp")
	if ok {
		t.Fatal("expected temp to be expired")
	}
	if p.GetString("temp") != "" {
		t.Fatal("GetString should return empty for expired var")
	}
}

func TestSetWithTTL_ZeroMeansNoExpiry(t *testing.T) {
	p := New()

	p.SetWithTTL("persistent", "val", "test", 0)

	v, ok := p.Get("persistent")
	if !ok {
		t.Fatal("expected persistent to exist")
	}
	if !v.ExpiresAt.IsZero() {
		t.Fatal("zero TTL should mean no expiry")
	}
}

func TestCleanExpired(t *testing.T) {
	p := New()

	p.SetWithTTL("expires-soon", "a", "test", 10*time.Millisecond)
	p.SetWithTTL("expires-later", "b", "test", 5*time.Second)
	p.Set("no-expiry", "c", "test")

	time.Sleep(15 * time.Millisecond)

	removed := p.CleanExpired()
	if removed != 1 {
		t.Fatalf("expected 1 removed, got %d", removed)
	}
	if p.Len() != 2 {
		t.Fatalf("expected 2 remaining vars, got %d", p.Len())
	}

	// Verify the right one was removed
	if _, ok := p.Get("expires-soon"); ok {
		t.Fatal("expires-soon should be removed")
	}
	if _, ok := p.Get("expires-later"); !ok {
		t.Fatal("expires-later should still exist")
	}
	if _, ok := p.Get("no-expiry"); !ok {
		t.Fatal("no-expiry should still exist")
	}
}

func TestExecutionScoped(t *testing.T) {
	p := New()

	p.SetExecutionScoped("exec-1", "results_1", "data1", "graph")
	p.SetExecutionScoped("exec-1", "status_1", "running", "graph")
	p.SetExecutionScoped("exec-2", "results_2", "data2", "graph")
	p.Set("global", "val", "system")

	if p.Len() != 4 {
		t.Fatalf("expected 4 vars, got %d", p.Len())
	}

	// Verify scoped vars are accessible
	v, ok := p.Get("results_2")
	if !ok {
		t.Fatal("expected results_2 to exist")
	}
	if v.ExecutionID != "exec-2" {
		t.Fatalf("expected exec-2, got %s", v.ExecutionID)
	}

	// Clear exec-1 vars
	p.ClearExecution("exec-1")

	if p.Len() != 2 {
		t.Fatalf("expected 2 vars after clearing exec-1, got %d", p.Len())
	}

	if _, ok := p.Get("results_1"); ok {
		t.Fatal("results_1 (exec-1) should be cleared")
	}
	if _, ok := p.Get("status_1"); ok {
		t.Fatal("status_1 (exec-1) should be cleared")
	}
	if _, ok := p.Get("results_2"); !ok {
		t.Fatal("results_2 (exec-2) should still exist")
	}
	if _, ok := p.Get("global"); !ok {
		t.Fatal("global (no execution) should still exist")
	}
}

func TestClearExecution_NoMatch(t *testing.T) {
	p := New()
	p.Set("safe", "val", "test")

	p.ClearExecution("nonexistent")

	if p.Len() != 1 {
		t.Fatal("should not remove non-execution-scoped vars")
	}
}

func TestExpiredGetNumber(t *testing.T) {
	p := New()

	p.SetWithTTL("counter", 42.0, "test", 10*time.Millisecond)

	if p.GetNumber("counter") != 42.0 {
		t.Fatal("expected 42 before expiry")
	}

	time.Sleep(15 * time.Millisecond)

	if p.GetNumber("counter") != 0 {
		t.Fatal("expected 0 after expiry")
	}
}

func TestExpiredGetBool(t *testing.T) {
	p := New()

	p.SetWithTTL("flag", true, "test", 10*time.Millisecond)

	if !p.GetBool("flag") {
		t.Fatal("expected true before expiry")
	}

	time.Sleep(15 * time.Millisecond)

	if p.GetBool("flag") {
		t.Fatal("expected false after expiry")
	}
}

func TestConcurrentAccess(t *testing.T) {
	p := New()
	var wg sync.WaitGroup

	for i := range 100 {
		wg.Add(1)

		go func(n int) {
			defer wg.Done()
			key := "key"
			p.Set(key, float64(n), "goroutine")
			_ = p.GetNumber(key)
			_ = p.List()
			_ = p.Len()
		}(i)
	}

	wg.Wait()

	if p.Len() != 1 {
		t.Fatalf("expected 1 key, got %d", p.Len())
	}
}

func TestSummaryEmpty(t *testing.T) {
	p := New()
	if got := p.Summary(); got != "" {
		t.Errorf("Summary() on empty pool = %q, want empty", got)
	}
}

func TestSummaryGroupsByScope(t *testing.T) {
	p := New()
	p.SetScoped(ScopePlan, "spec", "Build a widget", "planner")
	p.SetScoped(ScopeImplement, "files", "main.go", "implementer")

	got := p.Summary()

	if !strings.Contains(got, "### plan") {
		t.Error("Summary missing plan scope header")
	}
	if !strings.Contains(got, "### implement") {
		t.Error("Summary missing implement scope header")
	}
	if !strings.Contains(got, "**spec**") {
		t.Error("Summary missing spec key")
	}
	if !strings.Contains(got, "**files**") {
		t.Error("Summary missing files key")
	}
}

func TestSummaryExcludesSystem(t *testing.T) {
	p := New()
	p.SetScoped(ScopeSystem, "task_id", "abc123", "system")
	p.SetScoped(ScopePlan, "spec", "test", "planner")

	// Default: excludes system.
	got := p.Summary()
	if strings.Contains(got, "task_id") {
		t.Error("Summary should exclude sys.* vars by default")
	}
	if !strings.Contains(got, "spec") {
		t.Error("Summary should include non-system vars")
	}

	// WithSystemVars: includes system.
	got = p.Summary(WithSystemVars())
	if !strings.Contains(got, "task_id") {
		t.Error("Summary with WithSystemVars() should include sys.* vars")
	}
}

func TestSummaryExcludesInternal(t *testing.T) {
	p := New()
	p.Set("_graph_partial_results_plan", "cached", "system")
	p.SetScoped(ScopePlan, "spec", "test", "planner")

	got := p.Summary()
	if strings.Contains(got, "_graph_partial") {
		t.Error("Summary should exclude keys starting with _")
	}
	if !strings.Contains(got, "spec") {
		t.Error("Summary should include normal keys")
	}
}

func TestSummaryExcludesExpired(t *testing.T) {
	p := New()
	p.SetWithTTL("temp_val", "gone", "test", 1*time.Millisecond) // expires almost immediately
	time.Sleep(5 * time.Millisecond)                             // wait for expiry
	p.SetScoped(ScopePlan, "spec", "stays", "planner")

	got := p.Summary()
	if strings.Contains(got, "temp_val") {
		t.Error("Summary should exclude expired vars")
	}
	if !strings.Contains(got, "stays") {
		t.Error("Summary should include non-expired vars")
	}
}

func TestSummaryTruncatesLongValues(t *testing.T) {
	p := New()
	longVal := strings.Repeat("x", 300)
	p.SetScoped(ScopePlan, "big", longVal, "test")

	got := p.Summary(WithMaxValueLen(50))
	if strings.Contains(got, longVal) {
		t.Error("Summary should truncate long values")
	}
	if !strings.Contains(got, "…") {
		t.Error("Summary should end truncated values with …")
	}
}

func TestSummaryMaxBytes(t *testing.T) {
	p := New()
	for i := range 100 {
		p.SetScoped(ScopePlan, fmt.Sprintf("var_%03d", i), strings.Repeat("v", 50), "test")
	}

	got := p.Summary(WithMaxBytes(500))
	// Pre-check content fits: entries stop before exceeding maxBytes.
	// Only the truncation marker "\n_(truncated)_\n" (16 bytes) is appended after.
	if len(got) > 500+16 {
		t.Errorf("Summary exceeded maxBytes + marker overhead: got %d bytes", len(got))
	}
	if !strings.Contains(got, "_(truncated)_") {
		t.Error("Summary should indicate truncation")
	}
}

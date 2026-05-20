package socket

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/valksor/kvelmo/internal/catalog"
	"github.com/valksor/kvelmo/internal/notify"
	"github.com/valksor/kvelmo/internal/taskgroup"
)

// --- handleStrategyList ---

func TestHandleStrategyList(t *testing.T) {
	g := newTestGlobalSocket(t)
	resp, err := g.handleStrategyList(context.Background(), &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleStrategyList: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Message)
	}
	if len(resp.Result) == 0 {
		t.Error("expected non-empty strategy list result")
	}
}

// --- handleProvidersList ---

func TestHandleProvidersList(t *testing.T) {
	withTempHome(t)
	g := newTestGlobalSocket(t)
	resp, err := g.handleProvidersList(context.Background(), &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleProvidersList: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Message)
	}
	var result struct {
		Providers []map[string]any `json:"providers"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(result.Providers) == 0 {
		t.Error("expected at least one provider in list")
	}
}

// --- task group handlers ---

func TestTaskGroupCoordinatorSetGet(t *testing.T) {
	orig := GetTaskGroupCoordinator()
	t.Cleanup(func() { SetTaskGroupCoordinator(orig) })

	store := taskgroup.NewStore(t.TempDir())
	coord := taskgroup.NewCoordinator(store)
	SetTaskGroupCoordinator(coord)

	if GetTaskGroupCoordinator() != coord {
		t.Error("GetTaskGroupCoordinator did not return the set coordinator")
	}
}

func TestHandleTaskGroup_NotConfigured(t *testing.T) {
	orig := GetTaskGroupCoordinator()
	SetTaskGroupCoordinator(nil)
	t.Cleanup(func() { SetTaskGroupCoordinator(orig) })

	g := newTestGlobalSocket(t)
	ctx := context.Background()

	// Create/Status/Add/Submit/Remove all error when unconfigured.
	for _, tc := range []struct {
		name string
		fn   func(context.Context, *Request) (*Response, error)
	}{
		{"create", g.handleTaskGroupCreate},
		{"status", g.handleTaskGroupStatus},
		{"add", g.handleTaskGroupAdd},
		{"submit", g.handleTaskGroupSubmit},
		{"remove", g.handleTaskGroupRemove},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := tc.fn(ctx, &Request{ID: "1", Params: json.RawMessage(`{"id":"g1","label":"x","task_id":"t1"}`)})
			if err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			if resp.Error == nil {
				t.Errorf("%s should error when coordinator unconfigured", tc.name)
			}
		})
	}

	// List returns empty (not an error) when unconfigured.
	resp, err := g.handleTaskGroupList(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if resp.Error != nil {
		t.Errorf("list should not error when unconfigured: %s", resp.Error.Message)
	}
}

func TestHandleTaskGroup_FullLifecycle(t *testing.T) {
	orig := GetTaskGroupCoordinator()
	t.Cleanup(func() { SetTaskGroupCoordinator(orig) })

	store := taskgroup.NewStore(t.TempDir())
	SetTaskGroupCoordinator(taskgroup.NewCoordinator(store))

	g := newTestGlobalSocket(t)
	ctx := context.Background()

	// Create
	resp, err := g.handleTaskGroupCreate(ctx, &Request{ID: "1", Params: json.RawMessage(`{"label":"my-group"}`)})
	if err != nil || resp.Error != nil {
		t.Fatalf("create: err=%v resp=%+v", err, resp.Error)
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(resp.Result, &created); err != nil {
		t.Fatalf("unmarshal create: %v", err)
	}
	if created.ID == "" {
		t.Fatal("created group has no ID")
	}

	// List
	resp, _ = g.handleTaskGroupList(ctx, &Request{ID: "2"})
	if resp.Error != nil {
		t.Errorf("list: %s", resp.Error.Message)
	}

	// Add task
	addParams := mustJSON(t, map[string]string{"id": created.ID, "task_id": "t1", "project_dir": "/p", "state": "implementing"})
	resp, _ = g.handleTaskGroupAdd(ctx, &Request{ID: "3", Params: addParams})
	if resp.Error != nil {
		t.Errorf("add: %s", resp.Error.Message)
	}

	// Status
	statusParams := mustJSON(t, map[string]string{"id": created.ID})
	resp, _ = g.handleTaskGroupStatus(ctx, &Request{ID: "4", Params: statusParams})
	if resp.Error != nil {
		t.Errorf("status: %s", resp.Error.Message)
	}

	// Remove
	resp, _ = g.handleTaskGroupRemove(ctx, &Request{ID: "5", Params: statusParams})
	if resp.Error != nil {
		t.Errorf("remove: %s", resp.Error.Message)
	}
}

func TestHandleTaskGroupCreate_EmptyLabel(t *testing.T) {
	orig := GetTaskGroupCoordinator()
	t.Cleanup(func() { SetTaskGroupCoordinator(orig) })
	SetTaskGroupCoordinator(taskgroup.NewCoordinator(taskgroup.NewStore(t.TempDir())))

	g := newTestGlobalSocket(t)
	resp, _ := g.handleTaskGroupCreate(context.Background(), &Request{ID: "1", Params: json.RawMessage(`{"label":""}`)})
	if resp.Error == nil {
		t.Error("expected error for empty label")
	}
}

// --- notifier setters + handleNotifyTest ---

func TestNotifierSetGet(t *testing.T) {
	orig := GetNotifier()
	t.Cleanup(func() { SetNotifier(orig) })

	SetNotifier(nil)
	if GetNotifier() != nil {
		t.Error("expected nil notifier")
	}

	n := notify.New(nil, false)
	SetNotifier(n)
	if GetNotifier() != n {
		t.Error("GetNotifier did not return set notifier")
	}
}

func TestHandleNotifyTest_NotEnabled(t *testing.T) {
	orig := GetNotifier()
	SetNotifier(nil)
	t.Cleanup(func() { SetNotifier(orig) })

	g := newTestGlobalSocket(t)
	resp, err := g.handleNotifyTest(context.Background(), &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleNotifyTest: %v", err)
	}
	var result map[string]any
	_ = json.Unmarshal(resp.Result, &result)
	if sent, _ := result["sent"].(float64); sent != 0 {
		t.Errorf("sent = %v, want 0 when notifier nil", result["sent"])
	}
}

func TestHandleNotifyTest_Enabled(t *testing.T) {
	orig := GetNotifier()
	t.Cleanup(func() { SetNotifier(orig) })
	SetNotifier(notify.New(nil, false))

	g := newTestGlobalSocket(t)
	resp, err := g.handleNotifyTest(context.Background(), &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleNotifyTest: %v", err)
	}
	var result map[string]any
	_ = json.Unmarshal(resp.Result, &result)
	if sent, _ := result["sent"].(float64); sent != 1 {
		t.Errorf("sent = %v, want 1 when notifier set", result["sent"])
	}
}

// mustJSON marshals v to json.RawMessage, failing the test on error.
func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	return b
}

// --- catalog setter ---

func TestSetCatalog(t *testing.T) {
	c := catalog.New(filepath.Join(t.TempDir(), "templates"))
	SetCatalog(c) // exercises the setter; no panic expected
}

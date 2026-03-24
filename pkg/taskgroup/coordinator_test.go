package taskgroup

import (
	"os"
	"path/filepath"
	"testing"
)

func tempStore(t *testing.T) *Store {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "groups")
	return NewStore(dir)
}

func TestCreateGroup(t *testing.T) {
	c := NewCoordinator(tempStore(t))

	g, err := c.CreateGroup("api + client")
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	if g.ID == "" {
		t.Error("expected non-empty ID")
	}
	if g.Label != "api + client" {
		t.Errorf("Label = %q, want %q", g.Label, "api + client")
	}
	if g.Status != StatusActive {
		t.Errorf("Status = %q, want %q", g.Status, StatusActive)
	}
	if len(g.Tasks) != 0 {
		t.Errorf("Tasks len = %d, want 0", len(g.Tasks))
	}
}

func TestAddTask(t *testing.T) {
	c := NewCoordinator(tempStore(t))
	g, err := c.CreateGroup("test")
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}

	ref := TaskRef{ProjectDir: "/tmp/api", TaskID: "task-1", State: "loaded"}
	if err := c.AddTask(g.ID, ref); err != nil {
		t.Fatalf("AddTask: %v", err)
	}

	got, err := c.GetGroup(g.ID)
	if err != nil {
		t.Fatalf("GetGroup: %v", err)
	}
	if len(got.Tasks) != 1 {
		t.Fatalf("Tasks len = %d, want 1", len(got.Tasks))
	}
	if got.Tasks[0].TaskID != "task-1" {
		t.Errorf("TaskID = %q, want %q", got.Tasks[0].TaskID, "task-1")
	}

	// Duplicate add should fail
	err = c.AddTask(g.ID, ref)
	if err == nil {
		t.Error("expected error on duplicate add")
	}
}

func TestAllReady(t *testing.T) {
	tests := []struct {
		name   string
		states []string
		want   bool
	}{
		{name: "empty group", states: nil, want: false},
		{name: "all reviewing", states: []string{"reviewing", "reviewing"}, want: true},
		{name: "one implementing", states: []string{"reviewing", "implementing"}, want: false},
		{name: "mixed ready states", states: []string{"reviewing", "submitted", "completed"}, want: true},
		{name: "single loaded", states: []string{"loaded"}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &Group{Status: StatusActive}
			for i, s := range tt.states {
				g.Tasks = append(g.Tasks, TaskRef{
					TaskID: "t" + string(rune('0'+i)),
					State:  s,
				})
			}
			if got := g.AllReady(); got != tt.want {
				t.Errorf("AllReady() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCanSubmit_GroupNotReady(t *testing.T) {
	c := NewCoordinator(tempStore(t))
	g, err := c.CreateGroup("test")
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	if err := c.AddTask(g.ID, TaskRef{ProjectDir: "/a", TaskID: "t1", State: "implementing"}); err != nil {
		t.Fatal(err)
	}
	if err := c.AddTask(g.ID, TaskRef{ProjectDir: "/b", TaskID: "t2", State: "reviewing"}); err != nil {
		t.Fatal(err)
	}

	ok, err := c.CanSubmit("t1", true)
	if ok {
		t.Error("expected CanSubmit to return false when group not ready")
	}
	if err == nil {
		t.Error("expected error describing blocked submission")
	}

	// Move t1 to reviewing
	c.UpdateTaskState("t1", "reviewing")
	ok, err = c.CanSubmit("t1", true)
	if !ok {
		t.Errorf("expected CanSubmit to return true, got error: %v", err)
	}
}

func TestCanSubmit_NoGroup(t *testing.T) {
	c := NewCoordinator(tempStore(t))

	ok, err := c.CanSubmit("orphan-task", true)
	if !ok {
		t.Errorf("expected CanSubmit to return true for task not in any group, err: %v", err)
	}
}

func TestCanSubmit_SyncDisabled(t *testing.T) {
	c := NewCoordinator(tempStore(t))
	g, err := c.CreateGroup("test")
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	if err := c.AddTask(g.ID, TaskRef{ProjectDir: "/a", TaskID: "t1", State: "implementing"}); err != nil {
		t.Fatal(err)
	}

	// Even though group not ready, sync is disabled
	ok, err := c.CanSubmit("t1", false)
	if !ok {
		t.Errorf("expected CanSubmit to return true with sync disabled, err: %v", err)
	}
}

func TestStorage_SaveLoad(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "groups")
	store := NewStore(dir)

	c1 := NewCoordinator(store)
	g, err := c1.CreateGroup("persist-test")
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	if err := c1.AddTask(g.ID, TaskRef{ProjectDir: "/x", TaskID: "tx", State: "loaded"}); err != nil {
		t.Fatal(err)
	}

	// Create a new coordinator from the same store to test persistence
	c2 := NewCoordinator(store)
	loaded, err := c2.GetGroup(g.ID)
	if err != nil {
		t.Fatalf("GetGroup after reload: %v", err)
	}
	if loaded.Label != "persist-test" {
		t.Errorf("Label = %q, want %q", loaded.Label, "persist-test")
	}
	if len(loaded.Tasks) != 1 {
		t.Fatalf("Tasks len = %d, want 1", len(loaded.Tasks))
	}
	if loaded.Tasks[0].TaskID != "tx" {
		t.Errorf("TaskID = %q, want %q", loaded.Tasks[0].TaskID, "tx")
	}
}

func TestRemoveGroup(t *testing.T) {
	store := tempStore(t)
	c := NewCoordinator(store)
	g, err := c.CreateGroup("to-delete")
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}

	if err := c.RemoveGroup(g.ID); err != nil {
		t.Fatalf("RemoveGroup: %v", err)
	}

	if _, err := c.GetGroup(g.ID); err == nil {
		t.Error("expected error after removing group")
	}

	// File should be gone
	path := filepath.Join(store.dir, g.ID+".json")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("expected file to be deleted, got err: %v", err)
	}
}

func TestRemoveTask(t *testing.T) {
	c := NewCoordinator(tempStore(t))
	g, err := c.CreateGroup("test")
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	if err := c.AddTask(g.ID, TaskRef{ProjectDir: "/a", TaskID: "t1", State: "loaded"}); err != nil {
		t.Fatal(err)
	}
	if err := c.AddTask(g.ID, TaskRef{ProjectDir: "/b", TaskID: "t2", State: "loaded"}); err != nil {
		t.Fatal(err)
	}

	if err := g.RemoveTask("t1"); err != nil {
		t.Fatalf("RemoveTask: %v", err)
	}
	if len(g.Tasks) != 1 {
		t.Errorf("Tasks len = %d, want 1", len(g.Tasks))
	}
	if g.Tasks[0].TaskID != "t2" {
		t.Errorf("remaining task = %q, want t2", g.Tasks[0].TaskID)
	}

	// Remove non-existent
	if err := g.RemoveTask("t99"); err == nil {
		t.Error("expected error removing non-existent task")
	}
}

func TestSubmitGroup(t *testing.T) {
	c := NewCoordinator(tempStore(t))
	g, err := c.CreateGroup("submit-test")
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	if err := c.AddTask(g.ID, TaskRef{ProjectDir: "/a", TaskID: "t1", State: "reviewing"}); err != nil {
		t.Fatal(err)
	}
	if err := c.AddTask(g.ID, TaskRef{ProjectDir: "/b", TaskID: "t2", State: "reviewing"}); err != nil {
		t.Fatal(err)
	}

	if err := c.SubmitGroup(g.ID); err != nil {
		t.Fatalf("SubmitGroup: %v", err)
	}

	got, _ := c.GetGroup(g.ID)
	if got.Status != StatusSubmitted {
		t.Errorf("Status = %q, want %q", got.Status, StatusSubmitted)
	}
}

func TestSubmitGroup_NotReady(t *testing.T) {
	c := NewCoordinator(tempStore(t))
	g, err := c.CreateGroup("test")
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	if err := c.AddTask(g.ID, TaskRef{ProjectDir: "/a", TaskID: "t1", State: "implementing"}); err != nil {
		t.Fatal(err)
	}

	if err := c.SubmitGroup(g.ID); err == nil {
		t.Error("expected error submitting group that is not ready")
	}
}

func TestListGroups(t *testing.T) {
	c := NewCoordinator(tempStore(t))
	if _, err := c.CreateGroup("g1"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.CreateGroup("g2"); err != nil {
		t.Fatal(err)
	}

	groups := c.ListGroups()
	if len(groups) != 2 {
		t.Errorf("ListGroups len = %d, want 2", len(groups))
	}
}

func TestFindGroupForTask(t *testing.T) {
	c := NewCoordinator(tempStore(t))
	g, err := c.CreateGroup("finder")
	if err != nil {
		t.Fatal(err)
	}
	if err := c.AddTask(g.ID, TaskRef{ProjectDir: "/a", TaskID: "findme", State: "loaded"}); err != nil {
		t.Fatal(err)
	}

	found := c.FindGroupForTask("findme")
	if found == nil {
		t.Fatal("expected to find group for task")
	}
	if found.ID != g.ID {
		t.Errorf("found group ID = %q, want %q", found.ID, g.ID)
	}

	if c.FindGroupForTask("nonexistent") != nil {
		t.Error("expected nil for task not in any group")
	}
}

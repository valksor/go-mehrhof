package screenshot_test

import (
	"encoding/base64"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/valksor/kvelmo/internal/screenshot"
	"github.com/valksor/kvelmo/internal/testutil"
)

// minimalPNG is a 1×1 white pixel PNG used as test image data.
// Encoded to avoid a large binary literal; decoding at init time is negligible.
var minimalPNG = func() []byte {
	const encoded = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAAAAAA6fptVAAAACklEQVQI12NgAAAAAgAB4iG8MwAAAABJRU5ErkJggg=="
	b, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		panic("minimalPNG: invalid base64: " + err.Error())
	}

	return b
}()

func newTestStore(t *testing.T) *screenshot.Store {
	t.Helper()

	return screenshot.NewStore(testutil.TempDir(t))
}

func TestNewStore(t *testing.T) {
	store := newTestStore(t)
	if store == nil {
		t.Fatal("NewStore() returned nil")
	}
}

func TestSave_PNG(t *testing.T) {
	store := newTestStore(t)
	ss, err := store.Save("task1", minimalPNG, screenshot.SaveOptions{
		Source: screenshot.SourceAgent,
	})
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if ss.TaskID != "task1" {
		t.Errorf("TaskID = %q, want %q", ss.TaskID, "task1")
	}
	if ss.Format != screenshot.FormatPNG {
		t.Errorf("Format = %q, want %q", ss.Format, screenshot.FormatPNG)
	}
	if ss.Source != screenshot.SourceAgent {
		t.Errorf("Source = %q, want %q", ss.Source, screenshot.SourceAgent)
	}
	if ss.ID == "" {
		t.Error("ID should not be empty")
	}
	if ss.SizeBytes != int64(len(minimalPNG)) {
		t.Errorf("SizeBytes = %d, want %d", ss.SizeBytes, len(minimalPNG))
	}
	if ss.Width == 0 || ss.Height == 0 {
		t.Errorf("expected non-zero dimensions, got %dx%d", ss.Width, ss.Height)
	}
}

func TestSave_DefaultsFormatToPNG(t *testing.T) {
	store := newTestStore(t)
	ss, err := store.Save("task1", []byte("data"), screenshot.SaveOptions{})
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if ss.Format != screenshot.FormatPNG {
		t.Errorf("Format = %q, want default %q", ss.Format, screenshot.FormatPNG)
	}
}

func TestSave_JPEG(t *testing.T) {
	store := newTestStore(t)
	ss, err := store.Save("task1", []byte("data"), screenshot.SaveOptions{
		Format: screenshot.FormatJPEG,
	})
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if ss.Format != screenshot.FormatJPEG {
		t.Errorf("Format = %q, want %q", ss.Format, screenshot.FormatJPEG)
	}
	if !strings.HasSuffix(ss.Filename, ".jpeg") {
		t.Errorf("Filename %q should end with .jpeg", ss.Filename)
	}
}

func TestSave_CreatesIndex(t *testing.T) {
	store := newTestStore(t)
	if _, err := store.Save("task1", []byte("data"), screenshot.SaveOptions{}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	list, err := store.List("task1")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != 1 {
		t.Errorf("List() len = %d, want 1", len(list))
	}
}

func TestList_Empty(t *testing.T) {
	store := newTestStore(t)
	list, err := store.List("task1")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != 0 {
		t.Errorf("List() len = %d, want 0 for empty store", len(list))
	}
}

func TestList_SortedNewestFirst(t *testing.T) {
	store := newTestStore(t)
	if _, err := store.Save("task1", []byte("first"), screenshot.SaveOptions{}); err != nil {
		t.Fatalf("Save first error = %v", err)
	}
	// Sleep long enough for timestamps to differ across all platforms.
	time.Sleep(50 * time.Millisecond)
	if _, err := store.Save("task1", []byte("second"), screenshot.SaveOptions{}); err != nil {
		t.Fatalf("Save second error = %v", err)
	}

	list, err := store.List("task1")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("List() len = %d, want 2", len(list))
	}
	if !list[0].Timestamp.After(list[1].Timestamp) {
		t.Error("List() should be sorted newest-first")
	}
}

func TestGet_Found(t *testing.T) {
	store := newTestStore(t)
	saved, err := store.Save("task1", []byte("data"), screenshot.SaveOptions{})
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := store.Get("task1", saved.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.ID != saved.ID {
		t.Errorf("Get() ID = %q, want %q", got.ID, saved.ID)
	}
}

func TestGet_NotFound(t *testing.T) {
	store := newTestStore(t)
	_, err := store.Get("task1", "nonexistent")
	if err == nil {
		t.Error("Get() should return error for unknown ID")
	}
}

func TestDelete_Found(t *testing.T) {
	store := newTestStore(t)
	saved, err := store.Save("task1", []byte("data"), screenshot.SaveOptions{})
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	if err := store.Delete("task1", saved.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// File should no longer exist.
	if _, err := os.Stat(saved.Path); !os.IsNotExist(err) {
		t.Error("screenshot file should be deleted from disk")
	}

	// Index should be empty.
	list, err := store.List("task1")
	if err != nil {
		t.Fatalf("List() after Delete() error = %v", err)
	}
	if len(list) != 0 {
		t.Errorf("List() after Delete() len = %d, want 0", len(list))
	}
}

func TestDelete_NotFound(t *testing.T) {
	store := newTestStore(t)
	err := store.Delete("task1", "nonexistent")
	if err == nil {
		t.Error("Delete() should return error for unknown ID")
	}
}

func TestGetPath_Found(t *testing.T) {
	store := newTestStore(t)
	saved, err := store.Save("task1", []byte("data"), screenshot.SaveOptions{})
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	path, err := store.GetPath("task1", saved.ID)
	if err != nil {
		t.Fatalf("GetPath() error = %v", err)
	}
	if path == "" {
		t.Error("GetPath() should return non-empty path")
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("GetPath() returned path %q that does not exist", path)
	}
}

func TestConcurrentSave(t *testing.T) {
	store := newTestStore(t)
	const goroutines = 10
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for range goroutines {
		go func() {
			defer wg.Done()
			if _, err := store.Save("task1", []byte("data"), screenshot.SaveOptions{}); err != nil {
				t.Errorf("concurrent Save() error = %v", err)
			}
		}()
	}

	wg.Wait()

	list, err := store.List("task1")
	if err != nil {
		t.Fatalf("List() after concurrent saves error = %v", err)
	}
	if len(list) != goroutines {
		t.Errorf("List() len = %d, want %d after %d concurrent saves", len(list), goroutines, goroutines)
	}
}

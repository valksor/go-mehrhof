//go:build e2e

package screenshot

import (
	"os"
	"path/filepath"
	"testing"
)

func TestE2E_ScreenshotLifecycle(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)
	taskID := "e2e-test-task"

	// Create a minimal valid PNG (1x1 pixel)
	pngData := createMinimalPNG()

	// Save screenshot
	ss, err := store.Save(taskID, pngData, SaveOptions{
		Format: FormatPNG,
		Source: "e2e-test",
		Step:   "planning",
		Agent:  "test-agent",
	})
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if ss.ID == "" {
		t.Error("screenshot ID should not be empty")
	}
	if ss.Width != 1 || ss.Height != 1 {
		t.Errorf("dimensions = %dx%d, want 1x1", ss.Width, ss.Height)
	}
	if ss.Source != "e2e-test" {
		t.Errorf("Source = %q, want e2e-test", ss.Source)
	}
	t.Logf("Saved screenshot: %s (%d bytes)", ss.ID, ss.SizeBytes)

	// Verify file exists on disk
	if _, err := os.Stat(ss.Path); os.IsNotExist(err) {
		t.Errorf("screenshot file does not exist: %s", ss.Path)
	}

	// List screenshots
	list, err := store.List(taskID)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("List length = %d, want 1", len(list))
	}
	if list[0].ID != ss.ID {
		t.Errorf("List[0].ID = %q, want %q", list[0].ID, ss.ID)
	}

	// Get screenshot
	got, err := store.Get(taskID, ss.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Path != ss.Path {
		t.Errorf("Get().Path = %q, want %q", got.Path, ss.Path)
	}

	// Save another screenshot
	ss2, err := store.Save(taskID, pngData, SaveOptions{
		Format: FormatPNG,
		Source: "e2e-test",
		Step:   "implementing",
	})
	if err != nil {
		t.Fatalf("Save second: %v", err)
	}

	// List should now have 2
	list, err = store.List(taskID)
	if err != nil {
		t.Fatalf("List after second save: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("List length = %d, want 2", len(list))
	}

	// Delete first screenshot
	if err := store.Delete(taskID, ss.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// File should be gone
	if _, err := os.Stat(ss.Path); !os.IsNotExist(err) {
		t.Errorf("screenshot file should be deleted: %s", ss.Path)
	}

	// List should have 1
	list, err = store.List(taskID)
	if err != nil {
		t.Fatalf("List after delete: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("List length = %d, want 1", len(list))
	}
	if list[0].ID != ss2.ID {
		t.Errorf("remaining screenshot ID = %q, want %q", list[0].ID, ss2.ID)
	}

	// Get deleted screenshot should fail
	_, err = store.Get(taskID, ss.ID)
	if err == nil {
		t.Error("Get deleted screenshot should return error")
	}

	// Verify index file exists on disk
	indexPath := filepath.Join(tmpDir, "screenshots", taskID, "index.json")
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		t.Errorf("index file does not exist: %s", indexPath)
	}
}

// createMinimalPNG returns a valid 1x1 pixel PNG.
func createMinimalPNG() []byte {
	return []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, // PNG signature
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52, // IHDR chunk
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, // 1x1
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53, // 8-bit RGB
		0xDE, 0x00, 0x00, 0x00, 0x0C, 0x49, 0x44, 0x41, // IDAT chunk
		0x54, 0x08, 0xD7, 0x63, 0xF8, 0xCF, 0xC0, 0x00, // compressed data
		0x00, 0x00, 0x02, 0x00, 0x01, 0xE2, 0x21, 0xBC, //
		0x33, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, // IEND chunk
		0x44, 0xAE, 0x42, 0x60, 0x82,
	}
}

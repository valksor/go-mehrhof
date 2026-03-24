package conductor

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewSpecWatcher(t *testing.T) {
	t.Run("records baseline mod times for existing files", func(t *testing.T) {
		dir := t.TempDir()
		a := filepath.Join(dir, "spec-a.md")
		b := filepath.Join(dir, "spec-b.md")

		if err := os.WriteFile(a, []byte("aaa"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(b, []byte("bbb"), 0o644); err != nil {
			t.Fatal(err)
		}

		sw := newSpecWatcher([]string{a, b})
		if len(sw.files) != 2 {
			t.Fatalf("files tracked = %d, want 2", len(sw.files))
		}
		for _, path := range []string{a, b} {
			if _, ok := sw.files[path]; !ok {
				t.Errorf("missing baseline for %s", path)
			}
		}
	})

	t.Run("empty paths produces empty map", func(t *testing.T) {
		sw := newSpecWatcher([]string{})
		if len(sw.files) != 0 {
			t.Fatalf("files tracked = %d, want 0", len(sw.files))
		}
	})

	t.Run("nonexistent paths are skipped", func(t *testing.T) {
		sw := newSpecWatcher([]string{"/no/such/file.txt", "/also/missing.md"})
		if len(sw.files) != 0 {
			t.Fatalf("files tracked = %d, want 0", len(sw.files))
		}
	})
}

func TestSpecWatcher_Check(t *testing.T) {
	t.Run("returns false when files unchanged", func(t *testing.T) {
		dir := t.TempDir()
		f := filepath.Join(dir, "spec.md")
		if err := os.WriteFile(f, []byte("content"), 0o644); err != nil {
			t.Fatal(err)
		}

		sw := newSpecWatcher([]string{f})
		if sw.Check() {
			t.Error("Check() = true on unchanged file, want false")
		}
	})

	t.Run("returns true after file modification", func(t *testing.T) {
		dir := t.TempDir()
		f := filepath.Join(dir, "spec.md")
		if err := os.WriteFile(f, []byte("content"), 0o644); err != nil {
			t.Fatal(err)
		}

		sw := newSpecWatcher([]string{f})

		// Advance the mod time by 1 second.
		future := time.Now().Add(1 * time.Second)
		if err := os.Chtimes(f, future, future); err != nil {
			t.Fatal(err)
		}

		if !sw.Check() {
			t.Error("Check() = false after modification, want true")
		}
	})

	t.Run("returns false with empty file set", func(t *testing.T) {
		sw := newSpecWatcher([]string{})
		if sw.Check() {
			t.Error("Check() = true with no files, want false")
		}
	})
}

func TestSpecWatcher_Changed(t *testing.T) {
	t.Run("false initially", func(t *testing.T) {
		dir := t.TempDir()
		f := filepath.Join(dir, "spec.md")
		if err := os.WriteFile(f, []byte("content"), 0o644); err != nil {
			t.Fatal(err)
		}

		sw := newSpecWatcher([]string{f})
		if sw.Changed() {
			t.Error("Changed() = true before any Check, want false")
		}
	})

	t.Run("true after Check detects modification", func(t *testing.T) {
		dir := t.TempDir()
		f := filepath.Join(dir, "spec.md")
		if err := os.WriteFile(f, []byte("content"), 0o644); err != nil {
			t.Fatal(err)
		}

		sw := newSpecWatcher([]string{f})

		future := time.Now().Add(1 * time.Second)
		if err := os.Chtimes(f, future, future); err != nil {
			t.Fatal(err)
		}

		sw.Check()
		if !sw.Changed() {
			t.Error("Changed() = false after Check detected change, want true")
		}
	})
}

func TestSpecWatcher_Reset(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "spec.md")
	if err := os.WriteFile(f, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}

	sw := newSpecWatcher([]string{f})

	// Trigger a change.
	future := time.Now().Add(1 * time.Second)
	if err := os.Chtimes(f, future, future); err != nil {
		t.Fatal(err)
	}
	sw.Check()
	if !sw.Changed() {
		t.Fatal("precondition: Changed() should be true")
	}

	sw.Reset()
	if sw.Changed() {
		t.Error("Changed() = true after Reset, want false")
	}
}

func TestSpecWatcher_NilReceiver(t *testing.T) {
	var sw *specWatcher

	t.Run("Check returns false", func(t *testing.T) {
		if sw.Check() {
			t.Error("nil.Check() = true, want false")
		}
	})

	t.Run("Changed returns false", func(t *testing.T) {
		if sw.Changed() {
			t.Error("nil.Changed() = true, want false")
		}
	})

	t.Run("Reset does not panic", func(t *testing.T) {
		sw.Reset() // must not panic
	})
}

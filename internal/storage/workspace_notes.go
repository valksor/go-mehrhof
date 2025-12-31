package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// NotesPath returns the path to notes.md
func (w *Workspace) NotesPath(taskID string) string {
	return filepath.Join(w.WorkPath(taskID), notesFileName)
}

// AppendNote adds a note to notes.md
func (w *Workspace) AppendNote(taskID, content, state string) error {
	notesPath := w.NotesPath(taskID)

	// Read existing content
	existing, _ := os.ReadFile(notesPath)

	// Format new note
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	stateTag := ""
	if state != "" {
		stateTag = fmt.Sprintf(" [%s]", state)
	}
	newNote := fmt.Sprintf("\n## %s%s\n\n%s\n", timestamp, stateTag, content)

	// Use strings.Builder for efficient concatenation
	var b strings.Builder
	b.Grow(len(existing) + len(newNote))
	b.Write(existing)
	b.WriteString(newNote)
	return os.WriteFile(notesPath, []byte(b.String()), 0o644)
}

// ReadNotes reads the notes file content
func (w *Workspace) ReadNotes(taskID string) (string, error) {
	data, err := os.ReadFile(w.NotesPath(taskID))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

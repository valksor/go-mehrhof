package taskgroup

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Store persists groups as JSON files in a directory.
type Store struct {
	dir string
}

// NewStore creates a Store in the given directory.
// The directory is created if it does not exist.
func NewStore(dir string) *Store {
	return &Store{dir: dir}
}

// Save persists a group to a JSON file named by its ID.
func (s *Store) Save(g *Group) error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return fmt.Errorf("save group: create dir: %w", err)
	}
	data, err := json.MarshalIndent(g, "", "  ")
	if err != nil {
		return fmt.Errorf("save group: marshal: %w", err)
	}
	path := filepath.Join(s.dir, g.ID+".json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("save group: write: %w", err)
	}
	return nil
}

// Load reads a group by ID from a JSON file.
func (s *Store) Load(id string) (*Group, error) {
	path := filepath.Join(s.dir, id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("load group: %w", err)
	}
	var g Group
	if err := json.Unmarshal(data, &g); err != nil {
		return nil, fmt.Errorf("load group: unmarshal: %w", err)
	}
	return &g, nil
}

// LoadAll reads all group JSON files from the store directory.
func (s *Store) LoadAll() ([]*Group, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("load all groups: %w", err)
	}
	var groups []*Group
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".json")
		g, err := s.Load(id)
		if err != nil {
			continue // skip corrupt files
		}
		groups = append(groups, g)
	}
	return groups, nil
}

// Delete removes a group file by ID.
func (s *Store) Delete(id string) error {
	path := filepath.Join(s.dir, id+".json")
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("delete group: %w", err)
	}
	return nil
}

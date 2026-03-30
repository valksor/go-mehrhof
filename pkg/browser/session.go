package browser

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
)

// BrowserProfilesDir returns the directory for global browser profiles.
func BrowserProfilesDir() string {
	return filepath.Join(Paths(), "browser-profiles")
}

// WorktreesDir returns the directory for worktree-specific state.
func WorktreesDir() string {
	return filepath.Join(Paths(), "worktrees")
}

// GlobalProfilePath returns the path to a global profile's state file.
func GlobalProfilePath(profileName string) string {
	return filepath.Join(BrowserProfilesDir(), profileName+".json")
}

// WorktreeHash generates a hash for a worktree path.
func WorktreeHash(worktreePath string) string {
	h := sha256.Sum256([]byte(worktreePath))

	return hex.EncodeToString(h[:8]) // First 8 bytes = 16 hex chars
}

// WorktreeStatePath returns the path to worktree-specific browser state.
func WorktreeStatePath(worktreeHash string) string {
	return filepath.Join(WorktreesDir(), worktreeHash, "browser.json")
}

// BrowserState represents the browser state that can be persisted.
// This matches playwright-cli's state-save format.
type BrowserState struct {
	// Cookies from all domains
	Cookies []Cookie `json:"cookies,omitempty"`

	// LocalStorage entries per origin
	LocalStorage map[string]map[string]string `json:"localStorage,omitempty"`

	// SessionStorage entries per origin (not persisted across sessions)
	SessionStorage map[string]map[string]string `json:"sessionStorage,omitempty"`
}

// Cookie represents a browser cookie.
type Cookie struct {
	Name     string  `json:"name"`
	Value    string  `json:"value"`
	Domain   string  `json:"domain"`
	Path     string  `json:"path"`
	Expires  float64 `json:"expires,omitempty"`
	HTTPOnly bool    `json:"httpOnly,omitempty"`
	Secure   bool    `json:"secure,omitempty"`
	SameSite string  `json:"sameSite,omitempty"`
}

// LoadState loads browser state from a file.
func LoadState(path string) (*BrowserState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &BrowserState{}, nil
		}

		return nil, err
	}

	var state BrowserState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}

	return &state, nil
}

// Save writes browser state to a file.
func (s *BrowserState) Save(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
}

// MergeState creates a merged state file for browser operation.
// It combines global profile state with worktree-specific state.
// Returns the path to the merged state file (in a temp location).
func MergeState(profileName, worktreeHash string) (string, error) {
	// Load global profile
	globalPath := GlobalProfilePath(profileName)
	globalState, err := LoadState(globalPath)
	if err != nil {
		return "", fmt.Errorf("load global profile: %w", err)
	}

	// Load worktree state
	wtPath := WorktreeStatePath(worktreeHash)
	wtState, err := LoadState(wtPath)
	if err != nil {
		return "", fmt.Errorf("load worktree state: %w", err)
	}

	// Merge: worktree state takes precedence
	merged := mergeStates(globalState, wtState)

	// Write to temp file
	tempDir := filepath.Join(Paths(), "tmp")
	if err := os.MkdirAll(tempDir, 0o755); err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}

	tempPath := filepath.Join(tempDir, fmt.Sprintf("merged-%s.json", worktreeHash))
	if err := merged.Save(tempPath); err != nil {
		return "", fmt.Errorf("save merged state: %w", err)
	}

	return tempPath, nil
}

// mergeStates combines two browser states, with b taking precedence.
func mergeStates(a, b *BrowserState) *BrowserState {
	merged := &BrowserState{
		Cookies:      make([]Cookie, 0),
		LocalStorage: make(map[string]map[string]string),
	}

	// Build cookie map for deduplication (domain+path+name is unique)
	cookieMap := make(map[string]Cookie)

	for _, c := range a.Cookies {
		key := fmt.Sprintf("%s|%s|%s", c.Domain, c.Path, c.Name)
		cookieMap[key] = c
	}

	// b cookies override a cookies
	for _, c := range b.Cookies {
		key := fmt.Sprintf("%s|%s|%s", c.Domain, c.Path, c.Name)
		cookieMap[key] = c
	}

	for _, c := range cookieMap {
		merged.Cookies = append(merged.Cookies, c)
	}

	// Merge localStorage
	for origin, entries := range a.LocalStorage {
		if merged.LocalStorage[origin] == nil {
			merged.LocalStorage[origin] = make(map[string]string)
		}
		maps.Copy(merged.LocalStorage[origin], entries)
	}

	for origin, entries := range b.LocalStorage {
		if merged.LocalStorage[origin] == nil {
			merged.LocalStorage[origin] = make(map[string]string)
		}
		maps.Copy(merged.LocalStorage[origin], entries)
	}

	return merged
}

// ExtractWorktreeState extracts worktree-specific changes from merged state.
// It saves only the cookies/storage that differ from the global profile.
func ExtractWorktreeState(mergedPath, profileName, worktreeHash string) error {
	// Load merged state (after browser operations)
	merged, err := LoadState(mergedPath)
	if err != nil {
		return fmt.Errorf("load merged state: %w", err)
	}

	// Load original global profile
	globalPath := GlobalProfilePath(profileName)
	global, err := LoadState(globalPath)
	if err != nil {
		return fmt.Errorf("load global profile: %w", err)
	}

	// Extract diff: only keep cookies/storage not in global
	wtState := diffStates(global, merged)

	// Save worktree state
	wtPath := WorktreeStatePath(worktreeHash)
	if err := wtState.Save(wtPath); err != nil {
		return fmt.Errorf("save worktree state: %w", err)
	}

	return nil
}

// diffStates returns entries in b that are not in a or are different.
func diffStates(a, b *BrowserState) *BrowserState {
	diff := &BrowserState{
		Cookies:      make([]Cookie, 0),
		LocalStorage: make(map[string]map[string]string),
	}

	// Build map of a cookies
	aCookies := make(map[string]Cookie)
	for _, c := range a.Cookies {
		key := fmt.Sprintf("%s|%s|%s", c.Domain, c.Path, c.Name)
		aCookies[key] = c
	}

	// Find cookies in b that are new or changed
	for _, c := range b.Cookies {
		key := fmt.Sprintf("%s|%s|%s", c.Domain, c.Path, c.Name)
		if ac, ok := aCookies[key]; !ok || ac.Value != c.Value {
			diff.Cookies = append(diff.Cookies, c)
		}
	}

	// Find localStorage entries in b that are new or changed
	for origin, entries := range b.LocalStorage {
		aEntries := a.LocalStorage[origin]
		for k, v := range entries {
			if aEntries == nil || aEntries[k] != v {
				if diff.LocalStorage[origin] == nil {
					diff.LocalStorage[origin] = make(map[string]string)
				}
				diff.LocalStorage[origin][k] = v
			}
		}
	}

	return diff
}

// UpdateGlobalProfile updates the global profile with auth-related state.
// This should be called when explicitly saving auth state.
func UpdateGlobalProfile(profileName string, state *BrowserState) error {
	path := GlobalProfilePath(profileName)

	return state.Save(path)
}

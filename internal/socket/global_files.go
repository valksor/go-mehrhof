package socket

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/valksor/kvelmo/internal/search"
)

// skipDirs contains common non-source directories to skip during file walks.
var skipDirs = map[string]bool{
	"node_modules": true, "vendor": true, "dist": true,
	"build": true, "__pycache__": true, ".git": true,
}

// --- Browse Handler ---

// BrowseParams holds params for browse.
type BrowseParams struct {
	Path  string `json:"path"`
	Files bool   `json:"files"` // include .md/.txt files
}

// BrowseEntry represents a file or directory entry.
type BrowseEntry struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"is_dir"`
}

func (g *GlobalSocket) handleBrowse(ctx context.Context, req *Request) (*Response, error) {
	var params BrowseParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params"), nil //nolint:nilerr // JSON-RPC error response
		}
	}

	// Build list of allowed roots: home directory + registered project paths
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, "cannot determine home directory"), nil //nolint:nilerr // JSON-RPC error response
	}

	allowedRoots := []string{homeDir}
	g.mu.RLock()
	for _, w := range g.worktrees {
		allowedRoots = append(allowedRoots, w.Path)
	}
	g.mu.RUnlock()

	path := params.Path
	if path == "" {
		path = homeDir
	}

	// Validate path is within allowed roots
	path, err = ValidatePathWithRoots(allowedRoots, path)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "access denied: path outside allowed directories"), nil //nolint:nilerr // JSON-RPC error response
	}

	info, err := os.Stat(path)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "path not found"), nil //nolint:nilerr // JSON-RPC error response
	}
	if !info.IsDir() {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "not a directory"), nil
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, "cannot read directory"), nil //nolint:nilerr // JSON-RPC error response
	}

	result := []BrowseEntry{}
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue // skip hidden
		}

		if e.IsDir() {
			result = append(result, BrowseEntry{
				Name:  name,
				Path:  filepath.Join(path, name),
				IsDir: true,
			})
		} else if params.Files {
			// Include .md and .txt files
			ext := strings.ToLower(filepath.Ext(name))
			if ext == ".md" || ext == ".txt" {
				result = append(result, BrowseEntry{
					Name:  name,
					Path:  filepath.Join(path, name),
					IsDir: false,
				})
			}
		}
	}

	return NewResultResponse(req.ID, map[string]any{
		keyPath:    path,
		"parent":   filepath.Dir(path),
		keyEntries: result,
	})
}

// --- File Handlers ---

// FilesListParams holds params for files.list.
type FilesListParams struct {
	Path       string   `json:"path"`
	Extensions []string `json:"extensions,omitempty"` // Filter by extension
	MaxDepth   int      `json:"max_depth,omitempty"`  // Max directory depth
}

// FileEntry represents a file for autocomplete/mentions.
type FileEntry struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	RelPath string `json:"rel_path"`
	IsDir   bool   `json:"is_dir"`
	Size    int64  `json:"size,omitempty"`
}

// Searchable interface for internal/search hybrid fuzzy matching.
func (f FileEntry) SearchTitle() string        { return f.Name }
func (f FileEntry) SearchDescription() string  { return f.RelPath }
func (f FileEntry) SearchTags() []string       { return nil }
func (f FileEntry) SearchStatus() string       { return "" }
func (f FileEntry) SearchCreatedAt() time.Time { return time.Time{} }
func (f FileEntry) SearchPriority() int        { return 0 }

func (g *GlobalSocket) handleFilesList(ctx context.Context, req *Request) (*Response, error) {
	var params FilesListParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params"), nil //nolint:nilerr // JSON-RPC error response
		}
	}

	// Build list of allowed roots from registered projects
	var allowedRoots []string
	g.mu.RLock()
	for _, w := range g.worktrees {
		allowedRoots = append(allowedRoots, w.Path)
	}
	g.mu.RUnlock()

	path := params.Path
	if path == "" {
		// Use first registered project as default
		if len(allowedRoots) > 0 {
			path = allowedRoots[0]
		}
	}

	if path == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "no path specified and no projects registered"), nil
	}

	// Validate path is within registered projects
	basePath, err := ValidatePathWithRoots(allowedRoots, path)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "access denied: path outside registered projects"), nil //nolint:nilerr // JSON-RPC error response
	}

	maxDepth := params.MaxDepth
	if maxDepth <= 0 {
		maxDepth = 3
	}

	entries := []FileEntry{}

	_ = filepath.WalkDir(basePath, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // Continue walking on individual file errors
		}

		// Skip hidden files/dirs
		if strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}

			return nil
		}

		// Calculate depth
		relPath, _ := filepath.Rel(basePath, p)
		depth := strings.Count(relPath, string(filepath.Separator))
		if depth > maxDepth {
			if d.IsDir() {
				return filepath.SkipDir
			}

			return nil
		}

		// Skip common non-source directories
		if d.IsDir() && skipDirs[d.Name()] {
			return filepath.SkipDir
		}

		// Filter by extensions if specified
		if len(params.Extensions) > 0 && !d.IsDir() {
			ext := strings.ToLower(filepath.Ext(d.Name()))
			found := false
			for _, e := range params.Extensions {
				if ext == e || ext == "."+e {
					found = true

					break
				}
			}
			if !found {
				return nil
			}
		}

		info, _ := d.Info()
		var size int64
		if info != nil && !d.IsDir() {
			size = info.Size()
		}

		entries = append(entries, FileEntry{
			Name:    d.Name(),
			Path:    p,
			RelPath: relPath,
			IsDir:   d.IsDir(),
			Size:    size,
		})

		// Limit results
		if len(entries) >= 500 {
			return filepath.SkipAll
		}

		return nil
	})

	return NewResultResponse(req.ID, map[string]any{
		keyPath:    basePath,
		keyEntries: entries,
	})
}

// FilesSearchParams holds params for files.search.
type FilesSearchParams struct {
	Query      string `json:"query"`
	Path       string `json:"path,omitempty"`
	MaxResults int    `json:"max_results,omitempty"`
}

func (g *GlobalSocket) handleFilesSearch(ctx context.Context, req *Request) (*Response, error) {
	var params FilesSearchParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error()), nil
	}

	if params.Query == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "query is required"), nil
	}

	// Build list of allowed roots from registered projects
	var allowedRoots []string
	g.mu.RLock()
	for _, w := range g.worktrees {
		allowedRoots = append(allowedRoots, w.Path)
	}
	g.mu.RUnlock()

	path := params.Path
	if path == "" {
		// Use first registered project as default
		if len(allowedRoots) > 0 {
			path = allowedRoots[0]
		}
	}

	if path == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "no path specified and no projects registered"), nil
	}

	// Validate path is within registered projects
	basePath, err := ValidatePathWithRoots(allowedRoots, path)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "access denied: path outside registered projects"), nil //nolint:nilerr // JSON-RPC error response
	}

	maxResults := params.MaxResults
	if maxResults <= 0 {
		maxResults = 20
	}

	// Phase B: Collect all candidate entries up to a cap of 500.
	const collectCap = 500
	candidates := []FileEntry{}

	_ = filepath.WalkDir(basePath, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // Continue walking on individual file errors
		}

		// Skip hidden and common non-source dirs
		if strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}

			return nil
		}
		if d.IsDir() && skipDirs[d.Name()] {
			return filepath.SkipDir
		}

		relPath, _ := filepath.Rel(basePath, p)
		candidates = append(candidates, FileEntry{
			Name:    d.Name(),
			Path:    p,
			RelPath: relPath,
			IsDir:   d.IsDir(),
		})

		if len(candidates) >= collectCap {
			return filepath.SkipAll
		}

		return nil
	})

	// Rank candidates using hybrid fuzzy + exact search, then unwrap.
	ranked := search.Search(candidates, params.Query, search.Options{})
	entries := make([]FileEntry, 0, min(len(ranked), maxResults))
	for i, r := range ranked {
		if i >= maxResults {
			break
		}
		entries = append(entries, r.Item)
	}

	return NewResultResponse(req.ID, map[string]any{
		"query":    params.Query,
		keyPath:    basePath,
		keyEntries: entries,
	})
}

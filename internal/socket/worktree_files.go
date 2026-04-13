package socket

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/valksor/kvelmo/internal/screenshot"
)

// --- Browse Handler ---

// WorktreeBrowseParams holds params for browse.
type WorktreeBrowseParams struct {
	Path  string `json:"path"`
	Files bool   `json:"files"` // include .md/.txt files
}

// WorktreeBrowseEntry represents a file or directory entry.
type WorktreeBrowseEntry struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"is_dir"`
}

// handleWorktreeFilesList lists files in the worktree for mentions/autocomplete.
// Mirrors the global handleFilesList but scoped to this worktree's path.
func (w *WorktreeSocket) handleWorktreeFilesList(_ context.Context, req *Request) (*Response, error) {
	var params struct {
		Path       string   `json:"path"`
		Extensions []string `json:"extensions,omitempty"`
		MaxDepth   int      `json:"max_depth,omitempty"`
	}
	if req.Params != nil {
		_ = json.Unmarshal(req.Params, &params)
	}

	basePath := w.path
	if params.Path != "" {
		// Join to worktree root first, then verify the resolved path is still within it.
		joined := filepath.Join(w.path, filepath.Clean(params.Path))
		resolved, absErr := filepath.Abs(joined)
		if absErr != nil {
			return nil, fmt.Errorf("resolve path: %w", absErr)
		}
		if !strings.HasPrefix(resolved+string(filepath.Separator), w.path+string(filepath.Separator)) {
			return NewErrorResponse(req.ID, -32602, "path outside worktree"), nil
		}
		basePath = resolved
	}

	maxDepth := params.MaxDepth
	if maxDepth <= 0 || maxDepth > 10 {
		maxDepth = 3
	}

	type fileEntry struct {
		Name  string `json:"name"`
		Path  string `json:"path"`
		IsDir bool   `json:"is_dir"`
		Size  int64  `json:"size,omitempty"`
	}

	var entries []fileEntry
	skipDirs := map[string]bool{
		"node_modules": true, "vendor": true, "dist": true,
		"build": true, "__pycache__": true, ".git": true,
	}

	_ = filepath.WalkDir(basePath, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // Continue on individual errors
		}
		if strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}

			return nil
		}
		relPath, _ := filepath.Rel(basePath, p)
		depth := strings.Count(relPath, string(filepath.Separator))
		if depth > maxDepth {
			if d.IsDir() {
				return filepath.SkipDir
			}

			return nil
		}
		if d.IsDir() && skipDirs[d.Name()] {
			return filepath.SkipDir
		}
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
		entries = append(entries, fileEntry{
			Name:  d.Name(),
			Path:  relPath,
			IsDir: d.IsDir(),
			Size:  size,
		})

		return nil
	})

	return NewResultResponse(req.ID, map[string]any{"files": entries})
}

func (w *WorktreeSocket) handleBrowse(ctx context.Context, req *Request) (*Response, error) {
	var params WorktreeBrowseParams
	if req.Params != nil {
		_ = json.Unmarshal(req.Params, &params)
	}

	path := params.Path
	if path == "" {
		path = w.path // default to worktree path
	}
	path = filepath.Clean(path)

	// Validate path is within worktree to prevent path traversal
	path, err := ValidatePathWithRoots([]string{w.path}, path)
	if err != nil {
		return NewErrorResponse(req.ID, -32602, "access denied: path outside worktree"), nil //nolint:nilerr // JSON-RPC error response
	}

	info, err := os.Stat(path)
	if err != nil {
		return NewErrorResponse(req.ID, -32602, "path not found"), nil //nolint:nilerr // JSON-RPC error response
	}
	if !info.IsDir() {
		return NewErrorResponse(req.ID, -32602, "not a directory"), nil
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return NewErrorResponse(req.ID, -32603, "cannot read directory"), nil //nolint:nilerr // JSON-RPC error response
	}

	result := []WorktreeBrowseEntry{}
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue // skip hidden
		}

		if e.IsDir() {
			result = append(result, WorktreeBrowseEntry{
				Name:  name,
				Path:  filepath.Join(path, name),
				IsDir: true,
			})
		} else if params.Files {
			// Include .md and .txt files
			ext := strings.ToLower(filepath.Ext(name))
			if ext == ".md" || ext == ".txt" {
				result = append(result, WorktreeBrowseEntry{
					Name:  name,
					Path:  filepath.Join(path, name),
					IsDir: false,
				})
			}
		}
	}

	return NewResultResponse(req.ID, map[string]any{
		"path":    path,
		"parent":  filepath.Dir(path),
		"entries": result,
	})
}

// --- Screenshot Handlers ---

type ScreenshotListParams struct {
	TaskID string `json:"task_id"`
}

func (w *WorktreeSocket) handleScreenshotsList(ctx context.Context, req *Request) (*Response, error) {
	var params ScreenshotListParams
	if req.Params != nil {
		_ = json.Unmarshal(req.Params, &params)
	}

	// Get task ID from params or current work unit
	taskID := params.TaskID
	if taskID == "" && w.conductor != nil {
		if wu := w.conductor.WorkUnit(); wu != nil {
			taskID = wu.ID
		}
	}

	if taskID == "" {
		return NewResultResponse(req.ID, map[string]any{
			"screenshots": []screenshot.Screenshot{},
		})
	}

	screenshots, err := w.screenshots.List(taskID)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}

	return NewResultResponse(req.ID, map[string]any{
		"screenshots": screenshots,
	})
}

type ScreenshotGetParams struct {
	TaskID       string `json:"task_id"`
	ScreenshotID string `json:"screenshot_id"`
}

func (w *WorktreeSocket) handleScreenshotsGet(ctx context.Context, req *Request) (*Response, error) {
	var params ScreenshotGetParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params"), nil //nolint:nilerr // JSON-RPC error response
	}

	// Get task ID from params or current work unit
	taskID := params.TaskID
	if taskID == "" && w.conductor != nil {
		if wu := w.conductor.WorkUnit(); wu != nil {
			taskID = wu.ID
		}
	}

	if taskID == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "task_id required"), nil
	}

	if params.ScreenshotID == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "screenshot_id required"), nil
	}

	ss, err := w.screenshots.Get(taskID, params.ScreenshotID)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}

	return NewResultResponse(req.ID, ss)
}

type ScreenshotCaptureParams struct {
	TaskID string `json:"task_id,omitempty"`
	Source string `json:"source"` // "agent" or "user"
	Step   string `json:"step,omitempty"`
	Agent  string `json:"agent,omitempty"`
	Format string `json:"format,omitempty"` // "png" or "jpeg"
	Data   string `json:"data"`             // base64 encoded image
}

func (w *WorktreeSocket) handleScreenshotsCapture(ctx context.Context, req *Request) (*Response, error) {
	var params ScreenshotCaptureParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params"), nil //nolint:nilerr // JSON-RPC error response
	}

	// Get task ID from params or current work unit
	taskID := params.TaskID
	if taskID == "" && w.conductor != nil {
		if wu := w.conductor.WorkUnit(); wu != nil {
			taskID = wu.ID
		}
	}

	if taskID == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "task_id required (no active task)"), nil
	}

	if params.Data == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "data required"), nil
	}

	// Decode base64 image data
	imageData, err := base64.StdEncoding.DecodeString(params.Data)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid base64 data"), nil //nolint:nilerr // JSON-RPC error response
	}

	source := params.Source
	if source == "" {
		source = screenshot.SourceUser
	}

	opts := screenshot.SaveOptions{
		Source: source,
		Step:   params.Step,
		Agent:  params.Agent,
		Format: params.Format,
	}

	ss, err := w.screenshots.Save(taskID, imageData, opts)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}

	// Emit screenshot_captured event to all subscribers
	w.emitEvent("screenshot_captured", ss)

	return NewResultResponse(req.ID, ss)
}

type ScreenshotDeleteParams struct {
	TaskID       string `json:"task_id"`
	ScreenshotID string `json:"screenshot_id"`
}

func (w *WorktreeSocket) handleScreenshotsDelete(ctx context.Context, req *Request) (*Response, error) {
	var params ScreenshotDeleteParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params"), nil //nolint:nilerr // JSON-RPC error response
	}

	// Get task ID from params or current work unit
	taskID := params.TaskID
	if taskID == "" && w.conductor != nil {
		if wu := w.conductor.WorkUnit(); wu != nil {
			taskID = wu.ID
		}
	}

	if taskID == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "task_id required"), nil
	}

	if params.ScreenshotID == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "screenshot_id required"), nil
	}

	if err := w.screenshots.Delete(taskID, params.ScreenshotID); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}

	// Emit screenshot_deleted event to all subscribers
	w.emitEvent("screenshot_deleted", map[string]string{
		"id":      params.ScreenshotID,
		"task_id": taskID,
	})

	return NewResultResponse(req.ID, map[string]any{
		"success": true,
	})
}

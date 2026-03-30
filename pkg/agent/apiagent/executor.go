package apiagent

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/valksor/kvelmo/pkg/agent"
)

// ToolExecutor runs tools locally in the agent's working directory.
type ToolExecutor struct {
	WorkDir           string
	PermissionHandler agent.PermissionHandler
	ExecTimeout       time.Duration // Per-tool timeout (default: 2m)
}

// Execute runs a tool and returns its output.
// Returns (output, error). Checks permissions before executing write operations.
func (e *ToolExecutor) Execute(ctx context.Context, toolName string, input map[string]any) (string, error) {
	if e.WorkDir == "" {
		return "", errors.New("tool executor: WorkDir is not set")
	}

	// Map API tool names to kvelmo permission names
	// (e.g., "write_file" → "write", "edit_file" → "edit")
	permTool := permissionToolName(toolName)

	// Check permissions
	req := agent.PermissionRequest{
		ID:    fmt.Sprintf("api-%d", time.Now().UnixNano()),
		Tool:  permTool,
		Input: input,
	}

	result := agent.EvaluatePermission(req)
	if result.DangerLevel == 2 { // permission.Dangerous
		return "", fmt.Errorf("dangerous operation denied: %s", result.DangerReason)
	}

	if !result.Approved && e.PermissionHandler != nil {
		if !e.PermissionHandler(req) {
			return "", fmt.Errorf("permission denied for tool: %s", toolName)
		}
	}

	timeout := e.ExecTimeout
	if timeout == 0 {
		timeout = 2 * time.Minute
	}

	// Normalize tool name (accept both PascalCase and snake_case)
	canonical := canonicalToolName(toolName)
	switch canonical {
	case "bash":
		return e.execBash(ctx, input, timeout)
	case "read":
		return e.execReadFile(input)
	case "write":
		return e.execWriteFile(input)
	case "edit":
		return e.execEditFile(input)
	case "glob":
		return e.execGlob(input)
	case "grep":
		return e.execGrep(ctx, input, timeout)
	case "ls":
		return e.execListDir(input)
	default:
		return "", fmt.Errorf("unknown tool: %s", toolName)
	}
}

func (e *ToolExecutor) execBash(ctx context.Context, input map[string]any, timeout time.Duration) (string, error) {
	command, _ := input["command"].(string)
	if command == "" {
		return "", errors.New("bash: command is required")
	}

	if t, ok := input["timeout"].(float64); ok && t > 0 {
		timeout = time.Duration(t) * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-c", command)
	cmd.Dir = e.WorkDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	output := stdout.String()
	if stderr.Len() > 0 {
		if output != "" {
			output += "\n"
		}
		output += "STDERR: " + stderr.String()
	}

	if err != nil {
		return output, fmt.Errorf("bash: %w\n%s", err, output)
	}

	return output, nil
}

func (e *ToolExecutor) execReadFile(input map[string]any) (string, error) {
	path, _ := input["path"].(string)
	if path == "" {
		return "", errors.New("read_file: path is required")
	}

	path, resolveErr := e.resolvePath(path)
	if resolveErr != nil {
		return "", fmt.Errorf("read_file: %w", resolveErr)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read_file: %w", err)
	}

	lines := strings.Split(string(data), "\n")

	offset := 0
	if o, ok := input["offset"].(float64); ok && o > 0 {
		offset = int(o) - 1 // Convert 1-based to 0-based
		if offset >= len(lines) {
			return "", fmt.Errorf("read_file: offset %d exceeds file length %d", int(o), len(lines))
		}
	}

	limit := len(lines) - offset
	if l, ok := input["limit"].(float64); ok && l > 0 {
		limit = int(l)
	}

	end := min(offset+limit, len(lines))

	var buf strings.Builder
	for i := offset; i < end; i++ {
		fmt.Fprintf(&buf, "%6d\t%s\n", i+1, lines[i])
	}

	return buf.String(), nil
}

func (e *ToolExecutor) execWriteFile(input map[string]any) (string, error) {
	path, _ := input["path"].(string)
	if path == "" {
		return "", errors.New("write_file: path is required")
	}

	content, _ := input["content"].(string)

	path, resolveErr := e.resolvePath(path)
	if resolveErr != nil {
		return "", fmt.Errorf("write_file: %w", resolveErr)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("write_file: create directory: %w", err)
	}

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("write_file: %w", err)
	}

	return fmt.Sprintf("Wrote %d bytes to %s", len(content), path), nil
}

func (e *ToolExecutor) execEditFile(input map[string]any) (string, error) {
	path, _ := input["path"].(string)
	if path == "" {
		return "", errors.New("edit_file: path is required")
	}

	oldStr, _ := input["old_string"].(string)
	newStr, _ := input["new_string"].(string)

	if oldStr == "" {
		return "", errors.New("edit_file: old_string is required")
	}

	path, resolveErr := e.resolvePath(path)
	if resolveErr != nil {
		return "", fmt.Errorf("edit_file: %w", resolveErr)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("edit_file: %w", err)
	}

	content := string(data)
	count := strings.Count(content, oldStr)

	if count == 0 {
		return "", fmt.Errorf("edit_file: old_string not found in %s", path)
	}
	if count > 1 {
		return "", fmt.Errorf("edit_file: old_string found %d times in %s (must be unique)", count, path)
	}

	content = strings.Replace(content, oldStr, newStr, 1)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("edit_file: %w", err)
	}

	return "Edited " + path, nil
}

func (e *ToolExecutor) execGlob(input map[string]any) (string, error) {
	pattern, _ := input["pattern"].(string)
	if pattern == "" {
		return "", errors.New("glob: pattern is required")
	}

	dir := e.WorkDir
	if p, ok := input["path"].(string); ok && p != "" {
		resolved, resolveErr := e.resolvePath(p)
		if resolveErr != nil {
			return "", fmt.Errorf("glob: %w", resolveErr)
		}
		dir = resolved
	}

	// For patterns with **, walk the directory tree and match
	if strings.Contains(pattern, "**") {
		return e.globWalk(dir, pattern)
	}

	// Simple glob
	matches, err := filepath.Glob(filepath.Join(dir, pattern))
	if err != nil {
		return "", fmt.Errorf("glob: %w", err)
	}

	// Make paths relative to dir
	for i, m := range matches {
		if rel, err := filepath.Rel(dir, m); err == nil {
			matches[i] = rel
		}
	}

	if len(matches) == 0 {
		return "No matches found", nil
	}

	return strings.Join(matches, "\n"), nil
}

// globWalk handles ** patterns by walking the directory tree.
func (e *ToolExecutor) globWalk(dir, pattern string) (string, error) {
	// Extract the file pattern after the last **
	// e.g., "**/*.go" → match "*.go" against every file
	parts := strings.SplitN(pattern, "**", 2)
	suffix := strings.TrimPrefix(parts[len(parts)-1], "/")

	var matches []string

	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil //nolint:nilerr // Intentionally skip inaccessible entries in glob walk
		}

		rel, relErr := filepath.Rel(dir, path)
		if relErr != nil {
			return relErr
		}

		if d.IsDir() {
			// Skip hidden directories
			if strings.HasPrefix(d.Name(), ".") && d.Name() != "." {
				return filepath.SkipDir
			}

			return nil
		}

		if suffix == "" {
			matches = append(matches, rel)

			return nil
		}

		matched, matchErr := filepath.Match(suffix, d.Name())
		if matchErr != nil {
			return matchErr
		}
		if matched {
			matches = append(matches, rel)
		}

		return nil
	})
	if err != nil {
		return "", fmt.Errorf("glob: %w", err)
	}

	if len(matches) == 0 {
		return "No matches found", nil
	}

	// Limit output
	if len(matches) > 500 {
		result := strings.Join(matches[:500], "\n")

		return result + fmt.Sprintf("\n... (%d more files)", len(matches)-500), nil
	}

	return strings.Join(matches, "\n"), nil
}

func (e *ToolExecutor) execGrep(ctx context.Context, input map[string]any, timeout time.Duration) (string, error) {
	pattern, _ := input["pattern"].(string)
	if pattern == "" {
		return "", errors.New("grep: pattern is required")
	}

	dir := e.WorkDir
	if p, ok := input["path"].(string); ok && p != "" {
		resolved, resolveErr := e.resolvePath(p)
		if resolveErr != nil {
			return "", fmt.Errorf("grep: %w", resolveErr)
		}
		dir = resolved
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	args := []string{"-rn", "--color=never"}

	if g, ok := input["glob"].(string); ok && g != "" {
		args = append(args, "--include="+g)
	}

	args = append(args, pattern, dir)

	cmd := exec.CommandContext(ctx, "grep", args...)

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	_ = cmd.Run() // grep returns exit 1 for no matches

	output := stdout.String()
	if output == "" {
		return "No matches found", nil
	}

	// Limit output to prevent overwhelming the model
	lines := strings.Split(output, "\n")
	if len(lines) > 200 {
		output = strings.Join(lines[:200], "\n") + fmt.Sprintf("\n... (%d more lines)", len(lines)-200)
	}

	return output, nil
}

func (e *ToolExecutor) execListDir(input map[string]any) (string, error) {
	dir := e.WorkDir
	if p, ok := input["path"].(string); ok && p != "" {
		resolved, resolveErr := e.resolvePath(p)
		if resolveErr != nil {
			return "", fmt.Errorf("list_dir: %w", resolveErr)
		}
		dir = resolved
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("list_dir: %w", err)
	}

	var buf strings.Builder
	for _, entry := range entries {
		if entry.IsDir() {
			fmt.Fprintf(&buf, "%s/\n", entry.Name())
		} else {
			fmt.Fprintf(&buf, "%s\n", entry.Name())
		}
	}

	return buf.String(), nil
}

// resolvePath resolves a path relative to the working directory.
// Enforces containment within WorkDir to prevent path traversal.
func (e *ToolExecutor) resolvePath(path string) (string, error) {
	var resolved string
	if filepath.IsAbs(path) {
		resolved = filepath.Clean(path)
	} else {
		resolved = filepath.Clean(filepath.Join(e.WorkDir, path))
	}

	// Verify the resolved path is within the working directory
	workDir := filepath.Clean(e.WorkDir)
	if resolved != workDir && !strings.HasPrefix(resolved, workDir+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q escapes working directory", path)
	}

	return resolved, nil
}

// canonicalToolName normalizes tool names to a canonical lowercase form.
// Accepts both PascalCase (Write, Read, Edit) and snake_case (write_file, read_file).
func canonicalToolName(toolName string) string {
	switch strings.ToLower(toolName) {
	case "bash":
		return "bash"
	case "read", "read_file", "readfile":
		return "read"
	case "write", "write_file", "writefile":
		return "write"
	case "edit", "edit_file", "editfile":
		return "edit"
	case "glob":
		return "glob"
	case "grep":
		return "grep"
	case "ls", "list_dir", "listdir":
		return "ls"
	default:
		return strings.ToLower(toolName)
	}
}

// permissionToolName maps tool names to kvelmo permission names.
func permissionToolName(toolName string) string {
	return canonicalToolName(toolName)
}

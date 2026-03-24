package conductor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/valksor/kvelmo/pkg/codegraph"
	"github.com/valksor/kvelmo/pkg/git"
)

// ResolvedContext holds the result of resolving a ContextItem.
type ResolvedContext struct {
	Label   string // Section heading for the prompt
	Content string // Resolved content
}

// ContextResolver resolves ContextItems into content for prompt assembly.
// Dependencies are injected so the resolver is testable without a full Conductor.
type ContextResolver struct {
	WorktreeRoot string           // Absolute path to worktree root (containment boundary)
	Repo         *git.Repository  // Git operations (may be nil)
	Graph        *codegraph.Graph // Symbol index (may be nil — fallback to grep)
}

// Resolve resolves a single ContextItem to content.
// Returns an error if the reference is invalid (file missing, path escape, etc.).
func (r *ContextResolver) Resolve(ctx context.Context, item ContextItem) (ResolvedContext, error) {
	switch item.Type {
	case ContextTypeFile:
		return r.resolveFile(item)
	case ContextTypeSymbol:
		return r.resolveSymbol(ctx, item)
	case ContextTypeCommit:
		return r.resolveCommit(ctx, item)
	case ContextTypeBranch:
		return r.resolveBranch(ctx, item)
	case ContextTypeTerminal:
		return r.resolveTerminal(item)
	case ContextTypeURL:
		// URL resolution is deferred to Item 4 (Docs/Terminal Context).
		// For now, return the URL as-is for the agent to fetch if needed.
		label := item.Label
		if label == "" {
			label = "Reference URL"
		}

		return ResolvedContext{
			Label:   label,
			Content: item.Ref,
		}, nil
	default:
		return ResolvedContext{}, fmt.Errorf("unknown context type: %q", item.Type)
	}
}

// ResolveAll resolves a slice of ContextItems, collecting results and errors.
// Items that fail to resolve are returned in the errors slice but do not block
// resolution of other items.
func (r *ContextResolver) ResolveAll(ctx context.Context, items []ContextItem) ([]ResolvedContext, []error) {
	var results []ResolvedContext
	var errs []error
	for _, item := range items {
		resolved, err := r.Resolve(ctx, item)
		if err != nil {
			errs = append(errs, fmt.Errorf("context %s %q: %w", item.Type, item.Ref, err))

			continue
		}
		results = append(results, resolved)
	}

	return results, errs
}

// containedPath resolves a path relative to the worktree root and verifies containment.
// Returns the absolute path or an error if the path escapes the worktree.
func (r *ContextResolver) containedPath(ref string) (string, error) {
	if r.WorktreeRoot == "" {
		return "", errors.New("worktree root not set")
	}

	root, err := filepath.Abs(r.WorktreeRoot)
	if err != nil {
		return "", fmt.Errorf("resolve worktree root: %w", err)
	}
	root = filepath.Clean(root)
	if evaledRoot, evalErr := filepath.EvalSymlinks(root); evalErr == nil {
		root = evaledRoot
	}

	// Reject absolute paths outright
	if filepath.IsAbs(ref) {
		return "", fmt.Errorf("absolute paths not allowed: %q", ref)
	}

	resolved := filepath.Clean(filepath.Join(root, ref))

	// Eval symlinks to prevent symlink-based escapes
	evaled, err := filepath.EvalSymlinks(resolved)
	if err != nil {
		// If file doesn't exist yet, fall back to lexical check
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("resolve path: %w", err)
		}
		evaled = resolved
	}

	if evaled != root && !strings.HasPrefix(evaled, root+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q escapes worktree", ref)
	}

	return evaled, nil
}

// resolveFile reads a file (or file:line range) from the worktree.
// Ref formats: "path/to/file.go", "path/to/file.go:42", "path/to/file.go:40-60".
func (r *ContextResolver) resolveFile(item ContextItem) (ResolvedContext, error) {
	ref := item.Ref
	filePath, lineSpec := ParseFileRef(ref)

	absPath, err := r.containedPath(filePath)
	if err != nil {
		return ResolvedContext{}, err
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return ResolvedContext{}, fmt.Errorf("read file: %w", err)
	}

	content := string(data)
	label := filePath

	// Extract specific lines if requested
	if lineSpec != "" {
		lines := strings.Split(content, "\n")
		start, end, parseErr := parseLineSpec(lineSpec, len(lines))
		if parseErr != nil {
			return ResolvedContext{}, fmt.Errorf("invalid line spec %q: %w", lineSpec, parseErr)
		}
		content = strings.Join(lines[start-1:end], "\n")
		label = fmt.Sprintf("%s (lines %d-%d)", filePath, start, end)
	}

	if item.Label != "" {
		label = item.Label
	}

	return ResolvedContext{Label: label, Content: content}, nil
}

// resolveSymbol looks up a code symbol via codegraph (with grep fallback).
func (r *ContextResolver) resolveSymbol(ctx context.Context, item ContextItem) (ResolvedContext, error) {
	name := item.Ref

	// Try codegraph first
	if r.Graph != nil {
		symbols, err := r.Graph.QuerySymbol(ctx, name)
		if err == nil && len(symbols) > 0 {
			return r.formatSymbolResults(name, symbols, item.Label)
		}
	}

	// Fallback: grep for the symbol in the worktree
	return r.grepSymbol(ctx, name, item.Label)
}

// formatSymbolResults formats codegraph symbol query results.
func (r *ContextResolver) formatSymbolResults(name string, symbols []codegraph.Symbol, label string) (ResolvedContext, error) {
	var parts []string
	for _, sym := range symbols {
		parts = append(parts, fmt.Sprintf("- %s %s (%s:%d, package %s)", sym.Kind, sym.Name, sym.File, sym.Line, sym.Package))
	}

	if label == "" {
		label = "Symbol: " + name
	}

	return ResolvedContext{
		Label:   label,
		Content: strings.Join(parts, "\n"),
	}, nil
}

// grepSymbol falls back to grep when codegraph is unavailable.
func (r *ContextResolver) grepSymbol(ctx context.Context, name, label string) (ResolvedContext, error) {
	if r.WorktreeRoot == "" {
		return ResolvedContext{}, fmt.Errorf("symbol %q not found — run 'kvelmo codegraph index' to build the symbol index", name)
	}

	cmd := exec.CommandContext(ctx, "grep", "-rFn", "--include=*.go", "--include=*.ts", "--include=*.tsx",
		"--include=*.js", "--include=*.py", "--include=*.rs", "--", name, r.WorktreeRoot)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	_ = cmd.Run() // grep returns non-zero when no matches — that's fine

	output := strings.TrimSpace(stdout.String())
	if output == "" {
		return ResolvedContext{}, fmt.Errorf("symbol %q not found — run 'kvelmo codegraph index' to build the symbol index", name)
	}

	// Limit grep output to first 20 matches
	lines := strings.Split(output, "\n")
	total := len(lines)
	if total > 20 {
		lines = lines[:20]
	}

	// Strip worktree root prefix for cleaner display
	for i, line := range lines {
		lines[i] = strings.TrimPrefix(line, r.WorktreeRoot+string(filepath.Separator))
	}

	if total > 20 {
		lines = append(lines, fmt.Sprintf("... (%d more matches)", total-20))
	}

	if label == "" {
		label = fmt.Sprintf("Symbol: %s (grep)", name)
	}

	return ResolvedContext{
		Label:   label,
		Content: strings.Join(lines, "\n"),
	}, nil
}

// resolveCommit resolves a git commit reference to its message and diff summary.
func (r *ContextResolver) resolveCommit(ctx context.Context, item ContextItem) (ResolvedContext, error) {
	if r.Repo == nil {
		return ResolvedContext{}, errors.New("git repository not available")
	}

	info, err := r.Repo.CommitInfo(ctx, item.Ref)
	if err != nil {
		return ResolvedContext{}, fmt.Errorf("commit %q: %w", item.Ref, err)
	}

	// Get diff stat for the commit
	diffStat, _ := r.Repo.DiffAgainst(ctx, item.Ref+"~1.."+item.Ref, true)

	label := item.Label
	if label == "" {
		sha := info.SHA
		if len(sha) > 8 {
			sha = sha[:8]
		}
		label = "Commit " + sha
	}

	content := fmt.Sprintf("SHA: %s\nAuthor: %s\nDate: %s\nMessage: %s", info.SHA, info.Author, info.Date, info.Message)
	if diffStat != "" {
		content += "\n\n" + diffStat
	}

	return ResolvedContext{Label: label, Content: content}, nil
}

// resolveBranch resolves a branch reference to its latest commit info.
func (r *ContextResolver) resolveBranch(ctx context.Context, item ContextItem) (ResolvedContext, error) {
	if r.Repo == nil {
		return ResolvedContext{}, errors.New("git repository not available")
	}

	info, err := r.Repo.CommitInfo(ctx, item.Ref)
	if err != nil {
		return ResolvedContext{}, fmt.Errorf("branch %q: %w", item.Ref, err)
	}

	label := item.Label
	if label == "" {
		label = "Branch: " + item.Ref
	}

	content := fmt.Sprintf("Latest commit: %s\nAuthor: %s\nDate: %s\nMessage: %s", info.SHA, info.Author, info.Date, info.Message)

	return ResolvedContext{Label: label, Content: content}, nil
}

// resolveTerminal returns terminal output as-is (content is in Ref for terminal type).
func (r *ContextResolver) resolveTerminal(item ContextItem) (ResolvedContext, error) {
	if item.Ref == "" {
		return ResolvedContext{}, errors.New("empty terminal output")
	}

	label := item.Label
	if label == "" {
		label = "Terminal Output"
	}

	return ResolvedContext{Label: label, Content: item.Ref}, nil
}

// ParseFileRef splits "path/to/file.go:42" into ("path/to/file.go", "42").
func ParseFileRef(ref string) (string, string) {
	// Handle Windows-style paths by only splitting on the last colon
	// that's followed by digits (line spec)
	for i := len(ref) - 1; i >= 0; i-- {
		if ref[i] == ':' {
			rest := ref[i+1:]
			// Check if it looks like a line spec (digits, or digits-digits)
			if isLineSpec(rest) {
				return ref[:i], rest
			}
		}
	}

	return ref, ""
}

// isLineSpec returns true if s looks like "42" or "40-60" (positive integers only).
func isLineSpec(s string) bool {
	if s == "" {
		return false
	}
	parts := strings.SplitN(s, "-", 2)
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 1 {
			return false
		}
	}

	return true
}

// parseLineSpec parses "42" or "40-60" into 1-based start/end line numbers.
func parseLineSpec(spec string, totalLines int) (int, int, error) {
	parts := strings.SplitN(spec, "-", 2)
	start, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid line number: %w", err)
	}
	end := start
	if len(parts) == 2 {
		end, err = strconv.Atoi(parts[1])
		if err != nil {
			return 0, 0, fmt.Errorf("invalid end line: %w", err)
		}
	}

	if start < 1 {
		start = 1
	}
	if end > totalLines {
		end = totalLines
	}
	if start > end {
		return 0, 0, fmt.Errorf("start line %d > end line %d", start, end)
	}

	return start, end, nil
}

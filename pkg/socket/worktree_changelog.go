package socket

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/valksor/kvelmo/pkg/changelog"
)

// changelogEntry is the JSON-serializable changelog entry returned by the RPC.
type changelogEntry struct {
	SHA      string `json:"sha"`
	Message  string `json:"message"`
	Author   string `json:"author"`
	Date     string `json:"date"`
	Category string `json:"category"`
	Body     string `json:"body,omitempty"`
}

// handleChangelogGenerate produces a changelog from commits between two git refs.
func (w *WorktreeSocket) handleChangelogGenerate(ctx context.Context, req *Request) (*Response, error) {
	if w.repo == nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, "no git repository"), nil
	}

	var params struct {
		Source string `json:"source"`
		Target string `json:"target"`
		Full   bool   `json:"full"`
		Note   string `json:"note"`
	}
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params"), nil //nolint:nilerr // JSON-RPC error response
		}
	}

	if params.Source == "" || params.Target == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "source and target are required"), nil
	}

	return w.changelog(ctx, req.ID, params.Source, params.Target, params.Note, params.Full)
}

func (w *WorktreeSocket) changelog(ctx context.Context, reqID, source, target, note string, full bool) (*Response, error) {
	var entries []changelogEntry
	if full {
		commits, err := w.repo.CommitsBetweenFull(ctx, source, target)
		if err != nil {
			return NewErrorResponse(reqID, ErrCodeInternal, fmt.Sprintf("list commits: %v", err)), nil
		}
		entries = make([]changelogEntry, 0, len(commits))
		for _, c := range commits {
			entries = append(entries, changelogEntry{
				SHA:      c.SHA,
				Message:  c.Message,
				Author:   c.Author,
				Date:     c.Date,
				Category: changelog.Categorize(c.Message),
				Body:     c.Body,
			})
		}
	} else {
		commits, err := w.repo.CommitsBetween(ctx, source, target)
		if err != nil {
			return NewErrorResponse(reqID, ErrCodeInternal, fmt.Sprintf("list commits: %v", err)), nil
		}
		entries = make([]changelogEntry, 0, len(commits))
		for _, c := range commits {
			entries = append(entries, changelogEntry{
				SHA:      c.SHA,
				Message:  c.Message,
				Author:   c.Author,
				Date:     c.Date,
				Category: changelog.Categorize(c.Message),
			})
		}
	}

	md := renderChangelogMarkdown(entries, note, full)

	resp := map[string]any{
		"entries":  entries,
		"markdown": md,
	}
	if note != "" {
		resp["note"] = note
	}

	return NewResultResponse(reqID, resp)
}

// shortSHA returns the first 8 characters of a SHA.
func shortSHA(sha string) string {
	if len(sha) > 8 {
		return sha[:8]
	}

	return sha
}

// categoryOrder defines the canonical ordering for changelog categories.
var categoryOrder = []string{"Added", "Changed", "Fixed", "Removed"}

// groupByCategory groups changelog entries by their category.
func groupByCategory(entries []changelogEntry) map[string][]changelogEntry {
	groups := make(map[string][]changelogEntry)
	for _, e := range entries {
		groups[e.Category] = append(groups[e.Category], e)
	}

	return groups
}

// renderChangelogMarkdown renders entries as grouped markdown.
// When full is true, body text is included beneath each entry.
func renderChangelogMarkdown(entries []changelogEntry, note string, full bool) string {
	if len(entries) == 0 {
		return ""
	}

	groups := groupByCategory(entries)
	var b strings.Builder

	if note != "" {
		fmt.Fprintf(&b, "> %s\n\n", note)
	}

	for _, cat := range categoryOrder {
		items, ok := groups[cat]
		if !ok {
			continue
		}
		fmt.Fprintf(&b, "### %s\n\n", cat)
		for _, e := range items {
			fmt.Fprintf(&b, "- %s (%s)\n", e.Message, shortSHA(e.SHA))
			if full && e.Body != "" {
				for _, line := range strings.Split(e.Body, "\n") {
					fmt.Fprintf(&b, "  %s\n", line)
				}
			}
		}
		b.WriteString("\n")
	}

	return b.String()
}

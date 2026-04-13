package socket

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/valksor/kvelmo/internal/git"
	"github.com/valksor/kvelmo/internal/storage"
	"github.com/valksor/kvelmo/settings"
)

// taskExportParams holds the parameters for task.export.
type taskExportParams struct {
	Format string `json:"format"` // "json" or "md"
}

// taskExportResult holds the full export bundle for JSON format.
type taskExportResult struct {
	Task           taskExportMeta        `json:"task"`
	Specifications []taskExportSpec      `json:"specifications"`
	ChatHistory    []storage.ChatMessage `json:"chat_history"`
	Checkpoints    []CheckpointInfo      `json:"checkpoints"`
	FileChanges    []git.FileStatus      `json:"file_changes"`
	Reviews        []storage.Review      `json:"reviews"`
	ExportedAt     string                `json:"exported_at"`
}

// taskExportMeta holds task metadata for the export.
type taskExportMeta struct {
	ID          string            `json:"id"`
	ExternalID  string            `json:"external_id,omitempty"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	State       string            `json:"state"`
	Branch      string            `json:"branch,omitempty"`
	Source      *taskExportSource `json:"source,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	PRID        string            `json:"pr_id,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	CreatedAt   string            `json:"created_at"`
	UpdatedAt   string            `json:"updated_at"`
}

// taskExportSource holds the task source info.
type taskExportSource struct {
	Provider  string `json:"provider"`
	Reference string `json:"reference"`
	URL       string `json:"url,omitempty"`
	Content   string `json:"content,omitempty"`
}

// taskExportSpec holds a specification file's path and content.
type taskExportSpec struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

func (w *WorktreeSocket) handleTaskExport(ctx context.Context, req *Request) (*Response, error) {
	if w.conductor == nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, "no conductor configured"), nil
	}

	var params taskExportParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params"), nil //nolint:nilerr // JSON-RPC error response
		}
	}
	if params.Format == "" {
		params.Format = "json"
	}

	wu := w.conductor.WorkUnit()
	if wu == nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, "no task loaded"), nil
	}

	state := w.conductor.State()

	// Collect task metadata.
	meta := taskExportMeta{
		ID:          wu.ID,
		ExternalID:  wu.ExternalID,
		Title:       wu.Title,
		Description: wu.Description,
		State:       string(state),
		Branch:      wu.Branch,
		Tags:        wu.Tags,
		PRID:        wu.PRID,
		Metadata:    wu.Metadata,
		CreatedAt:   wu.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   wu.UpdatedAt.Format(time.RFC3339),
	}
	if wu.Source != nil {
		meta.Source = &taskExportSource{
			Provider:  wu.Source.Provider,
			Reference: wu.Source.Reference,
			URL:       wu.Source.URL,
			Content:   wu.Source.Content,
		}
	}

	// Collect specifications.
	specs := make([]taskExportSpec, 0, len(wu.Specifications))
	for _, path := range wu.Specifications {
		content, err := os.ReadFile(path)
		if err != nil {
			slog.Warn("task.export: failed to read specification", "path", path, "error", err)

			continue
		}
		specs = append(specs, taskExportSpec{Path: path, Content: string(content)})
	}

	// Collect chat history.
	chatMessages := w.loadChatHistory(wu.ID)

	// Collect checkpoints.
	checkpoints := w.enrichCheckpoints(ctx, wu.Checkpoints)

	// Collect file changes.
	fileChanges := w.collectFileChanges(ctx)

	// Collect reviews.
	reviews := w.collectReviews()

	now := time.Now().Format(time.RFC3339)

	if params.Format == "md" {
		md := renderTaskExportMarkdown(meta, specs, chatMessages, checkpoints, fileChanges, reviews, now)

		return NewResultResponse(req.ID, map[string]string{
			"markdown": md,
		})
	}

	result := taskExportResult{
		Task:           meta,
		Specifications: specs,
		ChatHistory:    chatMessages,
		Checkpoints:    checkpoints,
		FileChanges:    fileChanges,
		Reviews:        reviews,
		ExportedAt:     now,
	}

	return NewResultResponse(req.ID, result)
}

// loadChatHistory loads chat messages for the given task from storage.
func (w *WorktreeSocket) loadChatHistory(taskID string) []storage.ChatMessage {
	effective, _, _, err := settings.LoadEffective(w.path)
	saveInProject := false
	if err == nil && effective != nil {
		saveInProject = settings.BoolValue(effective.Storage.SaveInProject, false)
	}
	store := storage.NewStore(w.path, saveInProject)
	chatStore := storage.NewChatStore(store)

	history, err := chatStore.LoadHistory(taskID)
	if err != nil {
		slog.Debug("task.export: failed to load chat history", "task", taskID, "error", err)

		return []storage.ChatMessage{}
	}

	return history.Messages
}

// enrichCheckpoints converts checkpoint SHAs to CheckpointInfo with commit metadata.
func (w *WorktreeSocket) enrichCheckpoints(ctx context.Context, shas []string) []CheckpointInfo {
	result := make([]CheckpointInfo, 0, len(shas))
	for _, sha := range shas {
		info := CheckpointInfo{SHA: sha}
		if w.repo != nil {
			if entry, err := w.repo.CommitInfo(ctx, sha); err == nil {
				info.Message = entry.Message
				info.Author = entry.Author
				info.Timestamp = entry.Date
			}
		}
		result = append(result, info)
	}

	return result
}

// collectFileChanges returns the list of changed files from git.
func (w *WorktreeSocket) collectFileChanges(ctx context.Context) []git.FileStatus {
	if w.repo == nil {
		return []git.FileStatus{}
	}
	files, err := w.repo.DiffFilesWithStatus(ctx)
	if err != nil {
		slog.Debug("task.export: failed to get file changes", "error", err)

		return []git.FileStatus{}
	}

	return files
}

// collectReviews returns reviews for the current task.
func (w *WorktreeSocket) collectReviews() []storage.Review {
	if w.conductor == nil {
		return []storage.Review{}
	}
	reviews, err := w.conductor.ListReviews()
	if err != nil {
		slog.Debug("task.export: failed to list reviews", "error", err)

		return []storage.Review{}
	}

	return reviews
}

// renderTaskExportMarkdown renders the export bundle as a markdown document.
func renderTaskExportMarkdown(
	meta taskExportMeta,
	specs []taskExportSpec,
	chat []storage.ChatMessage,
	checkpoints []CheckpointInfo,
	files []git.FileStatus,
	reviews []storage.Review,
	exportedAt string,
) string {
	var sb strings.Builder

	sb.WriteString("# Task Export: " + html.EscapeString(meta.Title) + "\n\n")
	sb.WriteString("**ID:** " + html.EscapeString(meta.ID) + "\n")
	sb.WriteString("**State:** " + html.EscapeString(meta.State) + "\n")
	if meta.Branch != "" {
		sb.WriteString("**Branch:** " + html.EscapeString(meta.Branch) + "\n")
	}
	if meta.Source != nil {
		sb.WriteString("**Source:** " + html.EscapeString(meta.Source.Reference) + "\n")
	}
	if len(meta.Tags) > 0 {
		sb.WriteString("**Tags:** " + html.EscapeString(strings.Join(meta.Tags, ", ")) + "\n")
	}
	if meta.PRID != "" {
		sb.WriteString("**PR:** " + html.EscapeString(meta.PRID) + "\n")
	}
	sb.WriteString("**Created:** " + html.EscapeString(meta.CreatedAt) + "\n")
	sb.WriteString("**Updated:** " + html.EscapeString(meta.UpdatedAt) + "\n")
	sb.WriteString("**Exported:** " + html.EscapeString(exportedAt) + "\n")

	if meta.Description != "" {
		sb.WriteString("\n## Description\n\n" + html.EscapeString(meta.Description) + "\n")
	}

	if len(specs) > 0 {
		sb.WriteString("\n## Specifications\n\n")
		for _, spec := range specs {
			sb.WriteString("### " + html.EscapeString(spec.Path) + "\n\n")
			sb.WriteString("```\n")
			sb.WriteString(spec.Content)
			if !strings.HasSuffix(spec.Content, "\n") {
				sb.WriteString("\n")
			}
			sb.WriteString("```\n\n")
		}
	}

	if len(files) > 0 {
		sb.WriteString("## File Changes\n\n")
		sb.WriteString("| Status | Path |\n")
		sb.WriteString("|--------|------|\n")
		for _, f := range files {
			sb.WriteString("| " + html.EscapeString(f.Status) + " | " + html.EscapeString(f.Path) + " |\n")
		}
		sb.WriteString("\n")
	}

	if len(checkpoints) > 0 {
		sb.WriteString("## Checkpoints\n\n")
		for _, cp := range checkpoints {
			msg := cp.Message
			if msg == "" {
				msg = "(no message)"
			}
			sb.WriteString("- `" + html.EscapeString(cp.SHA[:min(8, len(cp.SHA))]) + "` " + html.EscapeString(msg) + "\n")
		}
		sb.WriteString("\n")
	}

	if len(reviews) > 0 {
		sb.WriteString("## Reviews\n\n")
		for _, r := range reviews {
			title := r.Title
			if title == "" {
				title = fmt.Sprintf("Review #%d", r.Number)
			}
			sb.WriteString("### " + html.EscapeString(title) + "\n\n")
			if r.Status != "" {
				sb.WriteString("**Status:** " + html.EscapeString(r.Status) + "\n")
			}
			if r.Reviewer != "" {
				sb.WriteString("**Reviewer:** " + html.EscapeString(r.Reviewer) + "\n")
			}
			if r.Content != "" {
				sb.WriteString("\n" + html.EscapeString(r.Content) + "\n")
			}
			sb.WriteString("\n")
		}
	}

	if len(chat) > 0 {
		sb.WriteString("## Chat History\n\n")
		for _, msg := range chat {
			ts := msg.Timestamp
			if ts == "" {
				ts = "?"
			}
			sb.WriteString("**[" + html.EscapeString(ts) + "] " + html.EscapeString(msg.Role) + ":**\n\n" + html.EscapeString(msg.Content) + "\n\n---\n\n")
		}
	}

	return sb.String()
}

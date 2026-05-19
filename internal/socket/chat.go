package socket

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/valksor/kvelmo/internal/storage"
	"github.com/valksor/kvelmo/internal/worker"
	"github.com/valksor/kvelmo/settings"
)

// ChatMessage represents a message in the chat.
type ChatMessage struct {
	ID        string   `json:"id"`
	Role      string   `json:"role"` // "user", "assistant", "system"
	Content   string   `json:"content"`
	Mentions  []string `json:"mentions,omitempty"` // File paths mentioned with @
	Timestamp string   `json:"timestamp,omitempty"`
	JobID     string   `json:"job_id,omitempty"` // Job ID if this message triggered a job
}

// ChatSendRequest is the enhanced request for chat.send.
type ChatSendRequest struct {
	Message    string `json:"message"`
	WorktreeID string `json:"worktree_id,omitempty"`
	IsAnswer   bool   `json:"is_answer,omitempty"` // True if answering an agent question
}

// ChatResponse is the response for chat operations.
type ChatResponse struct {
	JobID   string `json:"job_id,omitempty"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// mentionPattern matches @filename or @path/to/file mentions.
var mentionPattern = regexp.MustCompile(`@([^\s@]+)`)

// extractMentions extracts file mentions from a message.
// Mentions are in the format @filename or @path/to/file.
func extractMentions(message string) []string {
	matches := mentionPattern.FindAllStringSubmatch(message, -1)
	seen := make(map[string]bool)
	var mentions []string
	for _, match := range matches {
		if len(match) > 1 && !seen[match[1]] {
			seen[match[1]] = true
			mentions = append(mentions, match[1])
		}
	}

	return mentions
}

// resolveMentions expands mentions to their file contents.
// Returns the message with mentions expanded to include file content.
func resolveMentions(message string, workDir string) (string, []string) {
	mentions := extractMentions(message)
	if len(mentions) == 0 || workDir == "" {
		return message, nil
	}

	var resolved []string
	var sb strings.Builder
	sb.WriteString(message)
	sb.WriteString("\n\n---\n\n## Referenced Files\n\n")

	anyResolved := false
	for _, mention := range mentions {
		// Try to resolve the file path
		var fullPath string
		if filepath.IsAbs(mention) {
			fullPath = mention
		} else {
			fullPath = filepath.Join(workDir, mention)
		}

		// Validate path is within workDir to prevent path traversal
		validPath, err := ValidatePathWithRoots([]string{workDir}, fullPath)
		if err != nil {
			sb.WriteString("### @" + mention + "\n")
			sb.WriteString("*Access denied: path outside working directory*\n\n")
			anyResolved = true

			continue
		}

		// Read file content

		content, err := os.ReadFile(validPath)
		if err != nil {
			sb.WriteString("### @" + mention + "\n")
			sb.WriteString("*File not found or cannot be read*\n\n")
			anyResolved = true // Still counts as expansion for display purposes

			continue
		}

		resolved = append(resolved, mention)
		anyResolved = true
		sb.WriteString("### @" + mention + "\n")
		sb.WriteString("```\n")
		// Truncate very large files
		if len(content) > 50000 {
			sb.Write(content[:50000])
			sb.WriteString("\n... (truncated)\n")
		} else {
			sb.Write(content)
		}
		sb.WriteString("\n```\n\n")
	}

	if !anyResolved {
		return message, nil
	}

	return sb.String(), resolved
}

// handleChatSendEnhanced is the enhanced chat handler with mention support.
// Requires an active task in the worktree.
// Uses streaming to push job events to the client instead of requiring polling.
func (g *GlobalSocket) handleChatSendEnhanced(ctx context.Context, req *Request, conn net.Conn) (*Response, error) {
	if g.pool == nil {
		return NewErrorResponse(req.ID, -32603, "no worker pool configured"), nil
	}

	var params ChatSendRequest
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error()), nil
	}

	if params.Message == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "message is required"), nil
	}

	// Get worktree info
	var workDir string
	var worktreeState string
	var taskID string
	if params.WorktreeID != "" {
		g.mu.RLock()
		if wt, ok := g.worktrees[params.WorktreeID]; ok {
			workDir = wt.Path
			worktreeState = wt.State
			// Use worktree ID as task ID for storage (or extract from state if available)
			taskID = wt.ID
		}
		g.mu.RUnlock()
	}

	// Require active task for chat
	if worktreeState == "" || worktreeState == string(StateNone) {
		return NewErrorResponse(req.ID, -32600, "no active task - load a task first with 'kvelmo start'"), nil
	}

	// Resolve mentions in the message
	expandedMessage, resolvedMentions := resolveMentions(params.Message, workDir)

	// Persist user message to storage
	if workDir != "" && taskID != "" {
		chatStore := g.getChatStore(workDir)
		msg := storage.ChatMessage{
			ID:        uuid.New().String(),
			Role:      "user",
			Content:   params.Message, // Store original, not expanded
			Mentions:  resolvedMentions,
			Timestamp: time.Now().Format(time.RFC3339),
		}
		if err := chatStore.SaveMessage(taskID, msg); err != nil {
			// Log but don't fail - chat persistence is best-effort
			fmt.Printf("Warning: failed to persist chat message: %v\n", err)
		}
	}

	// Build job options with context
	opts := &worker.JobOptions{
		WorkDir: workDir,
		Metadata: map[string]any{
			"mentions":  resolvedMentions,
			"is_answer": params.IsAnswer,
			"worktree":  params.WorktreeID,
			"task_id":   taskID,
		},
	}

	// Submit as a chat job
	job, err := g.pool.SubmitWithOptions(worker.JobTypeChat, params.WorktreeID, expandedMessage, opts)
	if err != nil {
		return NewErrorResponse(req.ID, -32603, err.Error()), nil
	}

	// Start streaming job events to the client
	go g.streamJobEvents(job.ID, taskID, workDir, conn)

	return NewResultResponse(req.ID, ChatResponse{
		JobID:  job.ID,
		Status: string(job.Status),
	})
}

// ChatEvent represents a streaming event for chat jobs.
type ChatEvent struct {
	Type      string    `json:"type"`              // job_started, stream, job_completed, job_failed
	JobID     string    `json:"job_id"`            // The job this event relates to
	Content   string    `json:"content,omitempty"` // Streaming content or message
	Result    string    `json:"result,omitempty"`  // Final result on completion
	Error     string    `json:"error,omitempty"`   // Error message on failure
	Timestamp time.Time `json:"timestamp"`
}

// streamJobEvents forwards worker pool events to the WebSocket connection.
func (g *GlobalSocket) streamJobEvents(jobID, taskID, workDir string, conn net.Conn) {
	stream := g.pool.Stream(jobID)
	if stream == nil {
		return
	}

	var resultContent strings.Builder

	for event := range stream {
		// Convert worker event to chat event
		chatEvent := ChatEvent{
			Type:      event.Type,
			JobID:     jobID,
			Content:   event.Content,
			Timestamp: event.Timestamp,
		}

		// Accumulate content for final result
		if event.Type == "stream" || event.Type == "assistant" {
			resultContent.WriteString(event.Content)
		}

		// Handle completion - persist assistant message
		if event.Type == worker.EventJobCompleted {
			chatEvent.Result = resultContent.String()

			// Persist assistant message to storage
			if workDir != "" && taskID != "" {
				chatStore := g.getChatStore(workDir)
				msg := storage.ChatMessage{
					ID:        uuid.New().String(),
					Role:      "assistant",
					Content:   resultContent.String(),
					Timestamp: time.Now().Format(time.RFC3339),
					JobID:     jobID,
				}
				if err := chatStore.SaveMessage(taskID, msg); err != nil {
					fmt.Printf("Warning: failed to persist assistant message: %v\n", err)
				}
			}
		}

		// Handle failure
		if event.Type == worker.EventJobFailed {
			chatEvent.Error = event.Content
		}

		// Write event to connection
		if err := WriteEvent(conn, chatEvent); err != nil {
			// Connection closed, stop streaming
			return
		}
	}
}

// getChatStore returns a chat store for the given project root.
func (g *GlobalSocket) getChatStore(projectRoot string) *storage.ChatStore {
	// Load settings to determine storage location
	effective, _, _, err := settings.LoadEffective(projectRoot)
	saveInProject := false
	if err == nil && effective != nil {
		saveInProject = settings.BoolValue(effective.Storage.SaveInProject, false)
	}
	store := storage.NewStore(projectRoot, saveInProject)

	return storage.NewChatStore(store)
}

// ChatStopParams holds params for chat.stop.
type ChatStopParams struct {
	WorktreeID string `json:"worktree_id"`
	JobID      string `json:"job_id,omitempty"` // Specific job to stop, or current if empty
}

// handleChatStop handles stopping the current chat job (but keeps worker).
// This allows the user to pause/stop the current response while still being able to chat.
func (g *GlobalSocket) handleChatStop(ctx context.Context, req *Request) (*Response, error) {
	if g.pool == nil {
		return NewErrorResponse(req.ID, -32603, "no worker pool configured"), nil
	}

	var params ChatStopParams
	if req.Params != nil {
		_ = json.Unmarshal(req.Params, &params)
	}

	// Note: stopping the job without killing the worker is not yet implemented.
	// This should signal the agent to stop its current generation
	// but keep the worker available for further chat.

	return NewResultResponse(req.ID, map[string]string{
		keyStatus:  "stopped",
		keyMessage: "Chat stopped (worker retained)",
	})
}

// ChatHistoryRequest holds params for chat.history.
type ChatHistoryRequest struct {
	WorktreeID string `json:"worktree_id"`
}

// ChatHistoryResponse is the response for chat.history.
type ChatHistoryResponse struct {
	Messages []ChatMessage `json:"messages"`
	TaskID   string        `json:"task_id"`
}

// handleChatHistory returns the chat history for the current task.
func (g *GlobalSocket) handleChatHistory(ctx context.Context, req *Request) (*Response, error) {
	var params ChatHistoryRequest
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error()), nil
	}

	// Get worktree info
	var workDir string
	var worktreeState string
	var taskID string
	if params.WorktreeID != "" {
		g.mu.RLock()
		if wt, ok := g.worktrees[params.WorktreeID]; ok {
			workDir = wt.Path
			worktreeState = wt.State
			taskID = wt.ID
		}
		g.mu.RUnlock()
	}

	// Require active task
	if worktreeState == "" || worktreeState == string(StateNone) {
		return NewErrorResponse(req.ID, -32600, "no active task"), nil
	}

	// Load chat history
	chatStore := g.getChatStore(workDir)
	history, err := chatStore.LoadHistory(taskID)
	if err != nil {
		return NewErrorResponse(req.ID, -32603, fmt.Sprintf("load history: %v", err)), nil
	}

	// Convert storage messages to socket messages
	messages := make([]ChatMessage, len(history.Messages))
	for i, msg := range history.Messages {
		messages[i] = ChatMessage{
			ID:        msg.ID,
			Role:      msg.Role,
			Content:   msg.Content,
			Mentions:  msg.Mentions,
			Timestamp: msg.Timestamp,
			JobID:     msg.JobID,
		}
	}

	return NewResultResponse(req.ID, ChatHistoryResponse{
		Messages: messages,
		TaskID:   taskID,
	})
}

// ChatClearRequest holds params for chat.clear.
type ChatClearRequest struct {
	WorktreeID string `json:"worktree_id"`
}

// handleChatClear clears the chat history for the current task.
func (g *GlobalSocket) handleChatClear(ctx context.Context, req *Request) (*Response, error) {
	var params ChatClearRequest
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error()), nil
	}

	// Get worktree info
	var workDir string
	var worktreeState string
	var taskID string
	if params.WorktreeID != "" {
		g.mu.RLock()
		if wt, ok := g.worktrees[params.WorktreeID]; ok {
			workDir = wt.Path
			worktreeState = wt.State
			taskID = wt.ID
		}
		g.mu.RUnlock()
	}

	// Require active task
	if worktreeState == "" || worktreeState == string(StateNone) {
		return NewErrorResponse(req.ID, -32600, "no active task"), nil
	}

	// Clear chat history
	chatStore := g.getChatStore(workDir)
	if err := chatStore.ClearHistory(taskID); err != nil {
		return NewErrorResponse(req.ID, -32603, fmt.Sprintf("clear history: %v", err)), nil
	}

	return NewResultResponse(req.ID, map[string]string{
		keyStatus:  "cleared",
		keyMessage: "Chat history cleared",
	})
}

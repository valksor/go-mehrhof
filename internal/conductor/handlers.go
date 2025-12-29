package conductor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/valksor/go-mehrhof/internal/agent"
	"github.com/valksor/go-mehrhof/internal/events"
	"github.com/valksor/go-mehrhof/internal/storage"
	"github.com/valksor/go-mehrhof/internal/workflow"
)

// ErrPendingQuestion is returned when the agent asks a question.
// Using errors.New() instead of fmt.Errorf() ensures errors.Is() works reliably.
var ErrPendingQuestion = errors.New("agent has a pending question")

// ensureDirExists creates the directory for the given file path if it doesn't exist.
// This is a helper to avoid code duplication when writing files.
func ensureDirExists(path string) error {
	dir := filepath.Dir(path)
	if dir == "" || dir == "." {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}

// createCheckpointIfNeeded creates a git checkpoint if there are changes
func (c *Conductor) createCheckpointIfNeeded(taskID, message string) *events.Event {
	if c.git == nil || !c.activeTask.UseGit {
		return nil
	}

	hasChanges, err := c.git.HasChanges()
	if err != nil {
		// If we can't determine changes, log but continue (treat as no changes)
		// This allows checkpoint creation to fail gracefully
		c.publishProgress(fmt.Sprintf("Warning: could not check git changes: %v", err), 0)
		return nil
	}
	if !hasChanges {
		return nil
	}

	// Use stored commit prefix, fallback to default [taskID] format
	commitPrefix := ""
	if c.taskWork != nil {
		commitPrefix = c.taskWork.Git.CommitPrefix
	}
	if commitPrefix == "" {
		commitPrefix = fmt.Sprintf("[%s]", taskID)
	}

	checkpoint, err := c.git.CreateCheckpointWithPrefix(taskID, message, commitPrefix)
	if err != nil {
		c.logError(fmt.Errorf("create checkpoint: %w", err))
		return nil
	}

	return &events.Event{
		Type: events.TypeCheckpoint,
		Data: map[string]any{
			"action":     "create",
			"checkpoint": checkpoint.Number,
			"commit":     checkpoint.ID,
		},
	}
}

// RunPlanning executes the planning phase (creates SPEC files)
func (c *Conductor) RunPlanning(ctx context.Context) error {
	c.publishProgress("Starting planning phase...", 0)

	taskID := c.activeTask.ID

	// Get agent for planning step
	planningAgent, err := c.GetAgentForStep(workflow.StepPlanning)
	if err != nil {
		return fmt.Errorf("get planning agent: %w", err)
	}

	// Create session for this planning run
	session, filename, err := c.workspace.CreateSession(taskID, "planning", planningAgent.Name(), c.activeTask.State)
	if err != nil {
		c.logError(fmt.Errorf("create session: %w", err))
	} else {
		c.currentSession = session
		c.currentSessionFile = filename
	}

	// Get source content for the prompt
	sourceContent, err := c.workspace.GetSourceContent(taskID)
	if err != nil {
		return fmt.Errorf("get source content: %w", err)
	}

	// NOTE: Errors from workspace reads below are ignored intentionally.
	// Missing notes/specs are valid states (new task), so empty results are acceptable.
	var notes string
	notes, _ = c.workspace.ReadNotes(taskID)

	// Get existing specifications (for iterative planning)
	existingSpecifications, _ := c.workspace.GatherSpecificationsContent(taskID)
	if existingSpecifications != "" {
		specifications, _ := c.workspace.ListSpecifications(taskID)
		c.publishProgress(fmt.Sprintf("Found %d existing specification(s), including in context...", len(specifications)), 5)
	}

	// Check for pending context from previous planning session
	var pendingContext string
	if c.workspace.HasPendingQuestion(taskID) {
		pq, err := c.workspace.LoadPendingQuestion(taskID)
		if err == nil && pq != nil {
			// Use summary by default, full context if flag is set
			if c.opts.IncludeFullContext && pq.FullContext != "" {
				pendingContext = pq.FullContext
			} else if pq.ContextSummary != "" {
				pendingContext = pq.ContextSummary
			}
		}
		// Clear the pending question (answer should be in notes via chat command)
		_ = c.workspace.ClearPendingQuestion(taskID)
	}

	// Build planning prompt
	prompt := buildPlanningPrompt(c.taskWork.Metadata.Title, sourceContent, notes, existingSpecifications)
	if pendingContext != "" {
		prompt += "\n\n## Previous Analysis (before question)\nThe following is context from your previous planning session. Use this to avoid re-exploring:\n\n" + pendingContext
	}

	// Run agent with streaming
	c.publishProgress("Agent analyzing task...", 20)
	response, err := planningAgent.RunWithCallback(ctx, prompt, func(event agent.Event) error {
		c.eventBus.PublishRaw(events.Event{
			Type: events.TypeAgentMessage,
			Data: map[string]any{"event": event},
		})
		return nil
	})
	if err != nil {
		c.activeTask.State = "idle"
		if err := c.workspace.SaveActiveTask(c.activeTask); err != nil {
			c.logError(fmt.Errorf("save active task after planning error: %w", err))
		}
		_ = c.machine.Dispatch(ctx, workflow.EventError)
		return fmt.Errorf("agent planning: %w", err)
	}

	// If agent asked a question, handle based on mode
	if response.Question != nil {
		if c.opts.SkipAgentQuestions {
			// In auto mode, skip questions and proceed with agent's best guess
			c.publishProgress("Skipping agent question (auto mode)...", 50)
			// Log the skipped question for audit trail
			c.logError(fmt.Errorf("skipped agent question: %s", response.Question.Text))
			// Continue with whatever specs were generated (if any)
		} else {
			// Normal mode: save question and return, waiting for user answer
			pendingQuestion := &storage.PendingQuestion{
				Question: response.Question.Text,
				Phase:    "planning",
				AskedAt:  time.Now(),
				// Save context to avoid re-exploration on next plan call
				ContextSummary: extractContextSummary(response),
				FullContext:    buildFullContext(response),
				ExploredFiles:  extractExploredFiles(response),
			}
			for _, opt := range response.Question.Options {
				pendingQuestion.Options = append(pendingQuestion.Options, storage.QuestionOption{
					Label:       opt.Label,
					Description: opt.Description,
				})
			}
			if err := c.workspace.SavePendingQuestion(taskID, pendingQuestion); err != nil {
				c.logError(fmt.Errorf("save pending question: %w", err))
			}
			// Dispatch EventWait to properly transition FSM to StateWaiting
			_ = c.machine.Dispatch(ctx, workflow.EventWait)
			c.activeTask.State = string(workflow.StateWaiting)
			if err := c.workspace.SaveActiveTask(c.activeTask); err != nil {
				c.logError(fmt.Errorf("save active task after pending question: %w", err))
			}
			return ErrPendingQuestion
		}
	}

	c.publishProgress("Creating specifications...", 70)

	// Create specification from response
	nextNum, err := c.workspace.NextSpecificationNumber(taskID)
	if err != nil {
		return fmt.Errorf("get next specification number: %w", err)
	}

	// Format specification content
	specContent := formatSpecificationContent(nextNum, response)

	if err := c.workspace.SaveSpecification(taskID, nextNum, specContent); err != nil {
		return fmt.Errorf("save specification: %w", err)
	}

	// Create checkpoint if git is available
	c.createCheckpointIfNeeded(taskID, fmt.Sprintf("Add specification-%d for task %s", nextNum, taskID))

	// Update state back to idle
	c.activeTask.State = "idle"
	if err := c.workspace.SaveActiveTask(c.activeTask); err != nil {
		c.logError(fmt.Errorf("save active task: %w", err))
	}

	// Dispatch completion
	_ = c.machine.Dispatch(ctx, workflow.EventPlanDone)

	// Save session with completion time
	c.saveCurrentSession(taskID)

	c.publishProgress("Planning complete", 100)
	return nil
}

// RunImplementation executes the implementation phase
func (c *Conductor) RunImplementation(ctx context.Context) error {
	c.publishProgress("Starting implementation phase...", 0)

	taskID := c.activeTask.ID

	// Get agent for implementing step
	implementingAgent, err := c.GetAgentForStep(workflow.StepImplementing)
	if err != nil {
		return fmt.Errorf("get implementing agent: %w", err)
	}

	// Create session for this implementation run
	session, filename, err := c.workspace.CreateSession(taskID, "implementation", implementingAgent.Name(), c.activeTask.State)
	if err != nil {
		c.logError(fmt.Errorf("create session: %w", err))
	} else {
		c.currentSession = session
		c.currentSessionFile = filename
	}

	// Get latest specification content only (use the most refined version)
	specContent, specNum, err := c.workspace.GetLatestSpecificationContent(taskID)
	if err != nil {
		return fmt.Errorf("get latest specification: %w", err)
	}
	if specContent == "" {
		return fmt.Errorf("no specifications found - run 'task plan' first")
	}

	c.publishProgress(fmt.Sprintf("Using specification-%d for implementation...", specNum), 5)

	// Get source content for context
	sourceContent, err := c.workspace.GetSourceContent(taskID)
	if err != nil {
		return fmt.Errorf("get source content: %w", err)
	}

	// Get notes (missing notes is acceptable, returns empty string)
	notes, _ := c.workspace.ReadNotes(taskID)

	// Build implementation prompt with latest spec
	prompt := buildImplementationPrompt(c.taskWork.Metadata.Title, sourceContent, specContent, notes)

	// Run agent with streaming
	c.publishProgress("Agent implementing...", 20)
	response, err := implementingAgent.RunWithCallback(ctx, prompt, func(event agent.Event) error {
		c.eventBus.PublishRaw(events.Event{
			Type: events.TypeAgentMessage,
			Data: map[string]any{"event": event},
		})
		return nil
	})
	if err != nil {
		c.activeTask.State = "idle"
		if err := c.workspace.SaveActiveTask(c.activeTask); err != nil {
			c.logError(fmt.Errorf("save active task after implementation error: %w", err))
		}
		_ = c.machine.Dispatch(ctx, workflow.EventError)
		return fmt.Errorf("agent implementation: %w", err)
	}

	c.publishProgress("Applying changes...", 70)

	// Apply file changes
	if !c.opts.DryRun && len(response.Files) > 0 {
		if err := applyFiles(ctx, c, response.Files); err != nil {
			return fmt.Errorf("apply files: %w", err)
		}
	}

	// Create checkpoint if git is available
	if event := c.createCheckpointIfNeeded(taskID, fmt.Sprintf("Implement task %s", taskID)); event != nil {
		c.eventBus.PublishRaw(*event)
	}

	// Update state back to idle
	c.activeTask.State = "idle"
	if err := c.workspace.SaveActiveTask(c.activeTask); err != nil {
		c.logError(fmt.Errorf("save active task: %w", err))
	}

	// Dispatch completion
	_ = c.machine.Dispatch(ctx, workflow.EventImplementDone)

	// Save session with completion time
	c.saveCurrentSession(taskID)

	c.publishProgress("Implementation complete", 100)
	return nil
}

// RunReview executes the review phase
func (c *Conductor) RunReview(ctx context.Context) error {
	c.publishProgress("Starting review phase...", 0)

	taskID := c.activeTask.ID

	// Get agent for reviewing step
	reviewAgent, err := c.GetAgentForStep(workflow.StepReviewing)
	if err != nil {
		return fmt.Errorf("get review agent: %w", err)
	}

	// Create session for this review run
	session, filename, err := c.workspace.CreateSession(taskID, "review", reviewAgent.Name(), c.activeTask.State)
	if err != nil {
		c.logError(fmt.Errorf("create session: %w", err))
	} else {
		c.currentSession = session
		c.currentSessionFile = filename
	}

	// Get source content
	sourceContent, err := c.workspace.GetSourceContent(taskID)
	if err != nil {
		return fmt.Errorf("get source content: %w", err)
	}

	// Get latest specification (review against most recent specification)
	specContent, specNum, _ := c.workspace.GetLatestSpecificationContent(taskID)
	if specContent != "" {
		c.publishProgress(fmt.Sprintf("Reviewing against specification-%d...", specNum), 5)
	}

	// Build review prompt
	prompt := buildReviewPrompt(c.taskWork.Metadata.Title, sourceContent, specContent)

	// Run agent
	c.publishProgress("Agent reviewing...", 20)
	response, err := reviewAgent.RunWithCallback(ctx, prompt, func(event agent.Event) error {
		c.eventBus.PublishRaw(events.Event{
			Type: events.TypeAgentMessage,
			Data: map[string]any{"event": event},
		})
		return nil
	})
	if err != nil {
		c.activeTask.State = "idle"
		if err := c.workspace.SaveActiveTask(c.activeTask); err != nil {
			c.logError(fmt.Errorf("save active task after review error: %w", err))
		}
		_ = c.machine.Dispatch(ctx, workflow.EventError)
		return fmt.Errorf("agent review: %w", err)
	}

	c.publishProgress("Processing review...", 70)

	// Save review as note
	reviewContent := response.Summary
	if reviewContent == "" && len(response.Messages) > 0 {
		reviewContent = response.Messages[0]
	}
	if reviewContent != "" {
		if err := c.workspace.AppendNote(taskID, "## Review Results\n\n"+reviewContent, "reviewing"); err != nil {
			c.logError(fmt.Errorf("append review note: %w", err))
		}
	}

	// Apply any suggested fixes if not dry-run
	if !c.opts.DryRun && len(response.Files) > 0 {
		if err := applyFiles(ctx, c, response.Files); err != nil {
			c.logError(fmt.Errorf("apply review fixes: %w", err))
		}

		// Create checkpoint for review fixes
		c.createCheckpointIfNeeded(taskID, fmt.Sprintf("Apply review fixes for task %s", taskID))
	}

	// Update state back to idle
	c.activeTask.State = "idle"
	if err := c.workspace.SaveActiveTask(c.activeTask); err != nil {
		c.logError(fmt.Errorf("save active task: %w", err))
	}

	// Dispatch completion
	_ = c.machine.Dispatch(ctx, workflow.EventReviewDone)

	// Save session with completion time
	c.saveCurrentSession(taskID)

	c.publishProgress("Review complete", 100)
	return nil
}

// DeleteFileSentinel is a special marker that indicates a file should be deleted
// This provides an alternative to setting operation: delete in YAML blocks
const DeleteFileSentinel = "__DELETE_FILE__"

// validatePathInWorkspace ensures that a resolved path is within the workspace root.
// This prevents path traversal attacks when applying file changes from AI agent output.
func validatePathInWorkspace(resolved, root string) error {
	// Get the relative path from root to resolved
	rel, err := filepath.Rel(root, resolved)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}
	// Check if the relative path starts with ".." which would escape the root
	if strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
		return fmt.Errorf("path outside workspace: %s", resolved)
	}
	return nil
}

// applyFiles writes agent file changes to disk
func applyFiles(ctx context.Context, c *Conductor, files []agent.FileChange) error {
	root := c.opts.WorkDir
	if c.git != nil {
		root = c.git.Root()
	}

	// Resolve symlinks in root path for accurate validation (handles macOS /var -> /private/var symlinks)
	resolvedRoot := root
	if res, err := filepath.EvalSymlinks(root); err == nil {
		resolvedRoot = res
	}
	// If root doesn't exist yet, EvalSymlinks will fail, so we use root as-is

	var stats struct {
		created int
		updated int
		deleted int
	}

	for _, fc := range files {
		path := filepath.Join(root, fc.Path)

		// Validate the path is within workspace (prevent path traversal attacks)
		// Resolve symlinks in the target path and validate it stays within root
		resolvedPath := path
		if res, err := filepath.EvalSymlinks(path); err == nil {
			resolvedPath = res
		}
		// Validate against both the original root and resolved root to handle symlinked paths
		if err := validatePathInWorkspace(resolvedPath, root); err != nil {
			if err := validatePathInWorkspace(resolvedPath, resolvedRoot); err != nil {
				return fmt.Errorf("invalid file path %q: %w", fc.Path, err)
			}
		}

		// Check for delete sentinel in content (alternative to operation: delete)
		if fc.Content == DeleteFileSentinel {
			fc.Operation = agent.FileOpDelete
		}

		switch fc.Operation {
		case agent.FileOpCreate:
			// Ensure directory exists
			if err := ensureDirExists(path); err != nil {
				return fmt.Errorf("create directory for %s: %w", path, err)
			}

			// Write file
			if err := os.WriteFile(path, []byte(fc.Content), 0o644); err != nil {
				return fmt.Errorf("write file %s: %w", path, err)
			}
			stats.created++

			c.eventBus.PublishRaw(events.Event{
				Type: events.TypeFileChanged,
				Data: map[string]any{
					"path":      fc.Path,
					"operation": "create",
				},
			})

		case agent.FileOpUpdate:
			// Ensure directory exists
			if err := ensureDirExists(path); err != nil {
				return fmt.Errorf("create directory for %s: %w", path, err)
			}

			// Write file
			if err := os.WriteFile(path, []byte(fc.Content), 0o644); err != nil {
				return fmt.Errorf("write file %s: %w", path, err)
			}
			stats.updated++

			c.eventBus.PublishRaw(events.Event{
				Type: events.TypeFileChanged,
				Data: map[string]any{
					"path":      fc.Path,
					"operation": "update",
				},
			})

		case agent.FileOpDelete:
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("delete file %s: %w", path, err)
			}
			stats.deleted++

			c.eventBus.PublishRaw(events.Event{
				Type: events.TypeFileChanged,
				Data: map[string]any{
					"path":      fc.Path,
					"operation": "delete",
				},
			})
		}
	}

	// Publish summary of file operations
	if stats.created > 0 || stats.updated > 0 || stats.deleted > 0 {
		c.eventBus.PublishRaw(events.Event{
			Type: events.TypeProgress,
			Data: map[string]any{
				"message": fmt.Sprintf("Files: %d created, %d updated, %d deleted",
					stats.created, stats.updated, stats.deleted),
			},
		})
	}

	return nil
}

// formatSpecificationContent formats a specification file from agent response
func formatSpecificationContent(num int, response *agent.Response) string {
	content := fmt.Sprintf("# Specification %d\n\n", num)

	if response.Summary != "" {
		content += "## Summary\n\n" + response.Summary + "\n\n"
	}

	if len(response.Messages) > 0 {
		content += "## Details\n\n"
		for _, msg := range response.Messages {
			content += msg + "\n\n"
		}
	}

	return content
}

// buildPlanningPrompt creates the prompt for specification generation
func buildPlanningPrompt(title, sourceContent, notes, existingSpecs string) string {
	prompt := fmt.Sprintf(`You are a software architect. Analyze this task and create a detailed implementation specification.

## Task
%s

## Source Content
%s
`, title, sourceContent)

	if existingSpecs != "" {
		prompt += fmt.Sprintf(`
## Previous Specifications
IMPORTANT: The following specifications already exist from previous planning iterations.
DO NOT start from scratch. Build upon these, refine them, or address any gaps:

%s

Your new specification should acknowledge what was already planned and either:
1. Refine/improve the existing specification
2. Add missing details
3. Address any gaps or questions that arose
`, existingSpecs)
	}

	if notes != "" {
		prompt += fmt.Sprintf(`
## Additional Notes
%s
`, notes)
	}

	prompt += `
## Instructions
Create a detailed specification that includes:
1. Overview of what needs to be implemented
2. Technical approach and architecture decisions
3. Step-by-step implementation plan
4. Files that need to be created or modified
5. Testing strategy
6. Acceptance criteria

Output your specification in a structured format with clear sections.`

	return prompt
}

// buildImplementationPrompt creates the prompt for implementation
func buildImplementationPrompt(title, sourceContent, specsContent, notes string) string {
	prompt := fmt.Sprintf(`You are a software engineer. Implement the following task according to the specifications.

## Task
%s

## Original Requirements
%s

## Specifications
%s
`, title, sourceContent, specsContent)

	if notes != "" {
		prompt += fmt.Sprintf(`
## Additional Notes
%s
`, notes)
	}

	prompt += `
## Instructions
Implement this task according to the specifications. For each file you create or modify:
1. Use yaml:file blocks with path, operation (create/update/delete), and content
2. Follow existing code style and patterns
3. Include necessary imports
4. Add appropriate error handling
5. Write clean, maintainable code

Output each file change in a yaml:file block.`

	return prompt
}

// buildReviewPrompt creates the prompt for code review
func buildReviewPrompt(title, sourceContent, specsContent string) string {
	return fmt.Sprintf(`You are a senior software engineer conducting a code review.

## Task
%s

## Original Requirements
%s

## Specifications
%s

## Instructions
Review the implementation for:
1. Correctness - Does it meet the specifications?
2. Code quality - Is it clean, readable, and maintainable?
3. Security - Are there any vulnerabilities?
4. Performance - Are there any obvious bottlenecks?
5. Best practices - Does it follow language/framework conventions?

Provide:
1. A summary of your findings
2. Any issues found (critical, major, minor)
3. Suggested improvements
4. If needed, provide corrected code in yaml:file blocks`, title, sourceContent, specsContent)
}

// buildChatPrompt creates a context-aware prompt for dialogue mode.
// This ensures the agent has full task awareness when answering questions or providing feedback.
func buildChatPrompt(title, sourceContent, notes, specs string, pq *storage.PendingQuestion, message string) string {
	var sb strings.Builder

	sb.WriteString("You are helping with a task. Respond to the user's message with context awareness.\n\n")
	sb.WriteString(fmt.Sprintf("## Task: %s\n\n", title))

	if sourceContent != "" {
		sb.WriteString("## Source Content\n")
		sb.WriteString(sourceContent)
		sb.WriteString("\n\n")
	}

	if specs != "" {
		sb.WriteString("## Current Specifications\n")
		sb.WriteString(specs)
		sb.WriteString("\n\n")
	}

	if notes != "" {
		sb.WriteString("## Previous Notes\n")
		sb.WriteString(notes)
		sb.WriteString("\n\n")
	}

	if pq != nil {
		sb.WriteString("## Your Previous Question\n")
		sb.WriteString(pq.Question)
		sb.WriteString("\n\n")
		if pq.ContextSummary != "" {
			sb.WriteString("## Context Before Question\n")
			sb.WriteString(pq.ContextSummary)
			sb.WriteString("\n\n")
		}
	}

	sb.WriteString("## User's Message\n")
	sb.WriteString(message)
	sb.WriteString("\n\n")

	sb.WriteString("## Instructions\n")
	sb.WriteString("Respond helpfully to the user's message. Keep your response focused and concise.\n")
	sb.WriteString("If this is an answer to your previous question, acknowledge it and explain how you'll proceed.\n")

	return sb.String()
}

// saveCurrentSession saves the current session if one exists
func (c *Conductor) saveCurrentSession(taskID string) {
	if c.currentSession == nil || c.currentSessionFile == "" {
		return
	}

	// Set end time
	c.currentSession.Metadata.EndedAt = time.Now()

	// Save session
	if err := c.workspace.SaveSession(taskID, c.currentSessionFile, c.currentSession); err != nil {
		c.logError(fmt.Errorf("save session: %w", err))
	}

	// Clear current session
	c.currentSession = nil
	c.currentSessionFile = ""
}

// extractContextSummary extracts a brief summary from the agent response.
// Uses the Summary field if available, otherwise truncates the first message.
func extractContextSummary(response *agent.Response) string {
	if response.Summary != "" {
		return response.Summary
	}
	if len(response.Messages) > 0 {
		msg := response.Messages[0]
		// Truncate to ~2000 chars for token efficiency
		if len(msg) > 2000 {
			return msg[:2000] + "\n[truncated...]"
		}
		return msg
	}
	return ""
}

// buildFullContext combines all agent output into a single context string.
// This includes the summary and all messages.
func buildFullContext(response *agent.Response) string {
	var parts []string
	if response.Summary != "" {
		parts = append(parts, "## Summary\n"+response.Summary)
	}
	if len(response.Messages) > 0 {
		parts = append(parts, "## Messages\n"+strings.Join(response.Messages, "\n\n"))
	}
	return strings.Join(parts, "\n\n")
}

// extractExploredFiles extracts file paths from the agent response.
// Returns paths from file changes and attempts to find file references in messages.
func extractExploredFiles(response *agent.Response) []string {
	seen := make(map[string]bool)
	var files []string

	// Add files from file changes
	for _, fc := range response.Files {
		if !seen[fc.Path] {
			seen[fc.Path] = true
			files = append(files, fc.Path)
		}
	}

	return files
}

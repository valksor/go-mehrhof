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
	"github.com/valksor/go-mehrhof/internal/progress"
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

	// Create progress tracker for this phase
	var statusLine *progress.StatusLine
	if !c.opts.DryRun {
		statusLine = progress.NewStatusLine("Planning")
		defer statusLine.Done()
	}

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
		// Clear the pending question (answer should be in notes via note command)
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
		// Always publish to event bus
		c.eventBus.PublishRaw(events.Event{
			Type: events.TypeAgentMessage,
			Data: map[string]any{"event": event},
		})
		// Also track progress if not dry-run
		if statusLine != nil {
			_ = statusLine.OnEvent(event)
		}
		return nil
	})
	if err != nil {
		if statusLine != nil {
			statusLine.Done()
		}
		c.activeTask.State = "idle"
		if err := c.workspace.SaveActiveTask(c.activeTask); err != nil {
			c.logError(fmt.Errorf("save active task after planning error: %w", err))
		}
		_ = c.machine.Dispatch(ctx, workflow.EventError)
		return fmt.Errorf("agent planning: %w", err)
	}

	// Record usage stats
	if response.Usage != nil {
		if err := c.workspace.AddUsage(taskID, "planning",
			response.Usage.InputTokens,
			response.Usage.OutputTokens,
			response.Usage.CachedTokens,
			response.Usage.CostUSD,
		); err != nil {
			c.logError(fmt.Errorf("record planning usage: %w", err))
		}
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

	// Create progress tracker for this phase
	var statusLine *progress.StatusLine
	if !c.opts.DryRun {
		statusLine = progress.NewStatusLine("Implementing")
		defer statusLine.Done()
	}

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
		// Always publish to event bus
		c.eventBus.PublishRaw(events.Event{
			Type: events.TypeAgentMessage,
			Data: map[string]any{"event": event},
		})
		// Also track progress if not dry-run
		if statusLine != nil {
			_ = statusLine.OnEvent(event)
		}
		return nil
	})
	if err != nil {
		if statusLine != nil {
			statusLine.Done()
		}
		c.activeTask.State = "idle"
		if err := c.workspace.SaveActiveTask(c.activeTask); err != nil {
			c.logError(fmt.Errorf("save active task after implementation error: %w", err))
		}
		_ = c.machine.Dispatch(ctx, workflow.EventError)
		return fmt.Errorf("agent implementation: %w", err)
	}

	// Record usage stats
	if response.Usage != nil {
		if err := c.workspace.AddUsage(taskID, "implementing",
			response.Usage.InputTokens,
			response.Usage.OutputTokens,
			response.Usage.CachedTokens,
			response.Usage.CostUSD,
		); err != nil {
			c.logError(fmt.Errorf("record implementation usage: %w", err))
		}
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

	// Create progress tracker for this phase
	var statusLine *progress.StatusLine
	if !c.opts.DryRun {
		statusLine = progress.NewStatusLine("Reviewing")
		defer statusLine.Done()
	}

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
		// Always publish to event bus
		c.eventBus.PublishRaw(events.Event{
			Type: events.TypeAgentMessage,
			Data: map[string]any{"event": event},
		})
		// Also track progress if not dry-run
		if statusLine != nil {
			_ = statusLine.OnEvent(event)
		}
		return nil
	})
	if err != nil {
		if statusLine != nil {
			statusLine.Done()
		}
		c.activeTask.State = "idle"
		if err := c.workspace.SaveActiveTask(c.activeTask); err != nil {
			c.logError(fmt.Errorf("save active task after review error: %w", err))
		}
		_ = c.machine.Dispatch(ctx, workflow.EventError)
		return fmt.Errorf("agent review: %w", err)
	}

	// Record usage stats
	if response.Usage != nil {
		if err := c.workspace.AddUsage(taskID, "review",
			response.Usage.InputTokens,
			response.Usage.OutputTokens,
			response.Usage.CachedTokens,
			response.Usage.CostUSD,
		); err != nil {
			c.logError(fmt.Errorf("record review usage: %w", err))
		}
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

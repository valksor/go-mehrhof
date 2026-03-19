package provider

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

// Subtask represents a checklist item within a task.
type Subtask struct {
	ID        string // "{taskID}-task-{index}"
	Text      string
	Completed bool
	Index     int
}

// Task represents a task fetched from a provider.
type Task struct {
	ID          string
	Title       string
	Description string
	URL         string
	Labels      []string
	Source      string // file, github, gitlab, wrike

	// Inference fields - populated by provider from labels
	Priority string // p0-p3 (inferred from labels)
	Type     string // e.g. "enhancement", "feature", "chore", "docs"
	Slug     string // URL-safe title slug

	// Subtasks - checklist items from the task body
	Subtasks []*Subtask

	// Dependencies - tasks that this task depends on
	Dependencies []*Task

	// Hierarchy fields — populated when the provider supports hierarchy fetching
	// and the corresponding settings are enabled.
	ParentTask   *Task   // Parent task, if available (e.g. Wrike parent folder/task)
	SiblingTasks []*Task // Sibling tasks sharing the same parent, if available

	// metadata holds provider-specific key/value pairs that are used internally
	// (e.g. to pass parent IDs between fetch stages) but are not exposed to the
	// AI prompt. Use SetMetadata/Metadata to access them.
	metadata map[string]string
}

// SetMetadata stores a provider-specific key/value pair on the task.
func (t *Task) SetMetadata(key, value string) {
	if t.metadata == nil {
		t.metadata = make(map[string]string)
	}
	t.metadata[key] = value
}

// Metadata returns the value for the given metadata key, or "" if not set.
func (t *Task) Metadata(key string) string {
	if t.metadata == nil {
		return ""
	}

	return t.metadata[key]
}

// Provider is the core interface for fetching tasks from external systems.
type Provider interface {
	Name() string
	FetchTask(ctx context.Context, id string) (*Task, error)
	UpdateStatus(ctx context.Context, id string, status string) error
}

// HierarchyProvider extends Provider for sources that have a hierarchical task
// structure (e.g. Wrike folders/tasks). Providers that support hierarchy
// implement this interface; the conductor uses it to enrich a Task with
// parent and sibling context before building AI prompts.
//
// Both methods are best-effort: they should return a nil result (not an error)
// when the hierarchy information is simply unavailable for a given task, and
// only return errors for genuine API/network failures.
type HierarchyProvider interface {
	Provider
	// FetchParent returns the parent task of the given task, or nil if the
	// task has no parent or hierarchy is not applicable.
	FetchParent(ctx context.Context, task *Task) (*Task, error)
	// FetchSiblings returns sibling tasks (other tasks sharing the same parent).
	// Implementations should cap the result to a reasonable number (e.g. 5)
	// so the AI prompt stays concise.
	FetchSiblings(ctx context.Context, task *Task) ([]*Task, error)
}

// SubtaskProvider extends Provider for sources that support task checklists.
// Providers implement this interface to fetch subtasks (checklist items) from
// issue/task bodies.
type SubtaskProvider interface {
	Provider
	// FetchSubtasks parses and returns subtasks from the task body.
	// Returns an empty slice if no subtasks are found.
	FetchSubtasks(ctx context.Context, task *Task) ([]*Subtask, error)
}

// DependencyProvider extends Provider for sources that support task dependencies.
// Providers implement this interface to fetch and create dependency relationships
// between tasks.
type DependencyProvider interface {
	Provider
	// FetchDependencies returns tasks that the given task depends on.
	// Returns an empty slice if no dependencies are found.
	FetchDependencies(ctx context.Context, task *Task) ([]*Task, error)
	// CreateDependency creates a dependency relationship where taskID depends on dependsOnID.
	CreateDependency(ctx context.Context, taskID, dependsOnID string) error
}

// Comment represents a comment on a task.
type Comment struct {
	ID        string
	Body      string
	Author    string
	CreatedAt string // ISO 8601 timestamp
}

// CommentProvider extends Provider for sources that support fetching comments.
type CommentProvider interface {
	Provider
	// FetchComments returns all comments on a task.
	FetchComments(ctx context.Context, id string) ([]Comment, error)
}

// LabelProvider extends Provider for sources that support label management.
type LabelProvider interface {
	Provider
	// AddLabels adds labels to a task.
	AddLabels(ctx context.Context, id string, labels []string) error
	// RemoveLabels removes labels from a task.
	RemoveLabels(ctx context.Context, id string, labels []string) error
}

// BranchProtection describes the protection rules on a branch.
type BranchProtection struct {
	RequiredChecks      []string `json:"required_checks"`
	RequireReviews      bool     `json:"require_reviews"`
	MinReviewers        int      `json:"min_reviewers"`
	DismissStaleReviews bool     `json:"dismiss_stale_reviews"`
	EnforceAdmins       bool     `json:"enforce_admins"`
}

// BranchProtectionProvider extends Provider for sources that support branch protection rules.
type BranchProtectionProvider interface {
	Provider
	// GetBranchProtection returns protection rules for the given branch.
	// Returns nil, nil if the branch has no protection rules.
	GetBranchProtection(ctx context.Context, owner, repo, branch string) (*BranchProtection, error)
}

// ListOptions configures task listing.
type ListOptions struct {
	Team   string // Team key (e.g., "ENG" for Linear)
	Status string // Filter by status
	Limit  int    // Max results (0 = default)
	Cursor string // Pagination cursor
}

// ListResult contains paginated task results.
type ListResult struct {
	Tasks      []*Task
	NextCursor string
	HasMore    bool
}

// ListProvider extends Provider for sources that support listing tasks.
type ListProvider interface {
	Provider
	// ListTasks returns tasks matching the given options.
	ListTasks(ctx context.Context, opts ListOptions) (*ListResult, error)
}

// CreateTaskOptions configures new task creation.
type CreateTaskOptions struct {
	Title       string
	Description string
	Team        string   // Team key for Linear
	Priority    string   // e.g., "high", "normal"
	Labels      []string // Label names to apply
}

// CreateProvider extends Provider for sources that support creating tasks.
type CreateProvider interface {
	Provider
	// CreateTask creates a new task and returns it.
	CreateTask(ctx context.Context, opts CreateTaskOptions) (*Task, error)
}

// AttachmentProvider extends Provider for sources that support attachments.
type AttachmentProvider interface {
	Provider
	// DownloadAttachment downloads an attachment by URL.
	DownloadAttachment(ctx context.Context, url string) ([]byte, error)
}

// SubmitProvider extends Provider with submission capabilities.
// Not all providers support submission (e.g., file provider).
type SubmitProvider interface {
	Provider
	CreatePR(ctx context.Context, opts PROptions) (*PRResult, error)
	AddComment(ctx context.Context, id string, comment string) error
}

// MergeProvider extends SubmitProvider with PR/MR approval and merge capabilities.
// Providers that support programmatic PR approval and merging implement this interface.
type MergeProvider interface {
	SubmitProvider
	// ApprovePR approves a PR/MR with an optional comment.
	// The taskID format is provider-specific (e.g., "owner/repo#number" for GitHub).
	ApprovePR(ctx context.Context, taskID string, comment string) error
	// MergePR merges a PR/MR using the specified method.
	// Method should be one of: "merge", "squash", "rebase".
	// An empty method uses the provider's default (typically "rebase").
	MergePR(ctx context.Context, taskID string, method string) error
}

// PROptions contains options for creating a pull request.
type PROptions struct {
	Title     string   // PR title
	Body      string   // PR description
	Head      string   // Source branch
	Base      string   // Target branch (default: main)
	Draft     bool     // Create as draft PR
	Labels    []string // Labels to add
	Reviewers []string // Reviewers to request
	TaskID    string   // Original task ID (for linking)
	TaskURL   string   // Original task URL (for linking)
}

// PRResult contains the result of creating a pull request.
type PRResult struct {
	ID     string // PR identifier (e.g., "owner/repo#123")
	Number int    // PR number
	URL    string // PR web URL
	State  string // PR state (open, draft)
}

//nolint:nonamedreturns // Named returns document the return values
func Parse(source string) (provider string, id string, err error) {
	// Check if it's a file path
	if strings.HasPrefix(source, "file:") {
		return "file", strings.TrimPrefix(source, "file:"), nil
	}

	// Check if it's an empty/manual task
	if strings.HasPrefix(source, "empty:") {
		return "empty", strings.TrimPrefix(source, "empty:"), nil
	}

	// Check if it's a URL
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		u, err := url.Parse(source)
		if err != nil {
			return "", "", fmt.Errorf("parse url: %w", err)
		}

		host := u.Host
		path := u.Path

		if strings.Contains(host, "github.com") {
			// GitHub: /owner/repo/issues/123 or /owner/repo/pull/123
			parts := strings.Split(strings.Trim(path, "/"), "/")
			if len(parts) >= 4 {
				return "github", fmt.Sprintf("%s/%s#%s", parts[0], parts[1], parts[3]), nil
			}
		}

		if strings.Contains(host, "gitlab") {
			// GitLab: /owner/repo/-/issues/123 or /owner/repo/-/merge_requests/45
			parts := strings.Split(strings.Trim(path, "/"), "/")
			for i, p := range parts {
				if p == "issues" || p == "merge_requests" {
					if i >= 2 && i+1 < len(parts) {
						owner := strings.Join(parts[:i-1], "/")
						if p == "issues" {
							return "gitlab", fmt.Sprintf("%s#%s", owner, parts[i+1]), nil
						}
						// merge_requests use ! separator
						return "gitlab", fmt.Sprintf("%s!%s", owner, parts[i+1]), nil
					}
				}
			}
		}

		if strings.Contains(host, "wrike.com") {
			// Wrike: extract task ID from URL
			parts := strings.Split(path, "/")
			for _, p := range parts {
				if strings.HasPrefix(p, "task-") || strings.HasPrefix(p, "IEAA") {
					return "wrike", p, nil
				}
			}
		}

		if strings.Contains(host, "atlassian.net") {
			// Jira: /browse/PROJ-123
			parts := strings.Split(strings.Trim(path, "/"), "/")
			for i, p := range parts {
				if p == "browse" && i+1 < len(parts) {
					return "jira", parts[i+1], nil
				}
			}
		}

		if strings.Contains(host, "dev.azure.com") || strings.Contains(host, "visualstudio.com") {
			// Azure DevOps: /org/project/_workitems/edit/12345
			parts := strings.Split(strings.Trim(path, "/"), "/")
			for i, p := range parts {
				if p == "edit" && i+1 < len(parts) {
					return "azuredevops", parts[i+1], nil
				}
			}
		}

		if strings.Contains(host, "linear.app") {
			// Linear: /team/issue/ENG-123-title or /issue/ENG-123/title
			parts := strings.Split(strings.Trim(path, "/"), "/")
			for i, p := range parts {
				if p == "issue" && i+1 < len(parts) {
					// Extract identifier (ENG-123) from "ENG-123-slug" or "ENG-123"
					slug := parts[i+1]
					// Linear identifiers are TEAM-NUMBER format
					// Slug may be "ENG-123" or "ENG-123-some-title"
					// Extract first two dash-separated parts
					idParts := strings.SplitN(slug, "-", 3)
					if len(idParts) >= 2 {
						return "linear", idParts[0] + "-" + idParts[1], nil
					}
				}
			}
		}

		return "", "", fmt.Errorf("unsupported URL: %s", source)
	}

	// Check for shorthand: github:owner/repo#123
	if strings.HasPrefix(source, "github:") {
		return "github", strings.TrimPrefix(source, "github:"), nil
	}
	if strings.HasPrefix(source, "gitlab:") {
		return "gitlab", strings.TrimPrefix(source, "gitlab:"), nil
	}
	if strings.HasPrefix(source, "wrike:") {
		return "wrike", strings.TrimPrefix(source, "wrike:"), nil
	}
	if strings.HasPrefix(source, "linear:") {
		return "linear", strings.TrimPrefix(source, "linear:"), nil
	}
	if strings.HasPrefix(source, "ln:") {
		return "linear", strings.TrimPrefix(source, "ln:"), nil
	}
	if strings.HasPrefix(source, "jira:") {
		return "jira", strings.TrimPrefix(source, "jira:"), nil
	}
	if strings.HasPrefix(source, "azuredevops:") {
		return "azuredevops", strings.TrimPrefix(source, "azuredevops:"), nil
	}
	if strings.HasPrefix(source, "ado:") {
		return "azuredevops", strings.TrimPrefix(source, "ado:"), nil
	}

	return "", "", fmt.Errorf("unknown source format: %s", source)
}

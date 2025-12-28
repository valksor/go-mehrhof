package provider

import (
	"context"
	"io"
)

// Reader fetches work units from a provider
type Reader interface {
	Fetch(ctx context.Context, id string) (*WorkUnit, error)
}

// Identifier parses and validates references
type Identifier interface {
	Parse(input string) (string, error)
	Match(input string) bool
}

// Lister enumerates work units
type Lister interface {
	List(ctx context.Context, opts ListOptions) ([]*WorkUnit, error)
}

// AttachmentDownloader downloads attachments
type AttachmentDownloader interface {
	DownloadAttachment(ctx context.Context, workUnitID, attachmentID string) (io.ReadCloser, error)
}

// CommentFetcher retrieves comments
type CommentFetcher interface {
	FetchComments(ctx context.Context, workUnitID string) ([]Comment, error)
}

// Commenter adds comments to work units
type Commenter interface {
	AddComment(ctx context.Context, workUnitID string, body string) (*Comment, error)
}

// StatusUpdater changes work unit status
type StatusUpdater interface {
	UpdateStatus(ctx context.Context, workUnitID string, status Status) error
}

// LabelManager manages labels on work units
type LabelManager interface {
	AddLabels(ctx context.Context, workUnitID string, labels []string) error
	RemoveLabels(ctx context.Context, workUnitID string, labels []string) error
}

// ReadOnlyProvider is the minimum interface for a provider
type ReadOnlyProvider interface {
	Reader
	Identifier
}

// BidirectionalProvider supports read and write operations
type BidirectionalProvider interface {
	Reader
	Identifier
	Commenter
	StatusUpdater
}

// ListOptions configures list operations
type ListOptions struct {
	Status   Status
	Labels   []string
	Limit    int
	Offset   int
	OrderBy  string
	OrderDir string // asc, desc
}

// Snapshot contains captured source content (read-only copy)
type Snapshot struct {
	Type    string         // directory, file
	Ref     string         // original reference
	Files   []SnapshotFile // for directories
	Content string         // for single files
}

// SnapshotFile represents a single file in a snapshot
type SnapshotFile struct {
	Path    string
	Content string
}

// Snapshotter captures source content for storage
type Snapshotter interface {
	Snapshot(ctx context.Context, id string) (*Snapshot, error)
}

// ──────────────────────────────────────────────────────────────────────────────
// Extended interfaces for bidirectional providers (GitHub, Wrike, etc.)
// ──────────────────────────────────────────────────────────────────────────────

// PRCreator creates pull requests (for GitHub-like providers)
type PRCreator interface {
	CreatePullRequest(ctx context.Context, opts PullRequestOptions) (*PullRequest, error)
}

// PullRequestOptions for creating a PR
type PullRequestOptions struct {
	Title        string
	Body         string
	SourceBranch string
	TargetBranch string
	Labels       []string
	Draft        bool
	Reviewers    []string
}

// PullRequest represents a pull/merge request
type PullRequest struct {
	ID     string
	Number int
	URL    string
	Title  string
	State  string // open, closed, merged
}

// BranchLinker links work units to git branches
type BranchLinker interface {
	LinkBranch(ctx context.Context, workUnitID, branch string) error
	UnlinkBranch(ctx context.Context, workUnitID, branch string) error
	GetLinkedBranch(ctx context.Context, workUnitID string) (string, error)
}

// WorkUnitCreator creates new work units (for Wrike, GitHub issues, etc.)
type WorkUnitCreator interface {
	CreateWorkUnit(ctx context.Context, opts CreateWorkUnitOptions) (*WorkUnit, error)
}

// CreateWorkUnitOptions for creating a work unit
type CreateWorkUnitOptions struct {
	Title        string
	Description  string
	Labels       []string
	Assignees    []string
	Priority     Priority
	ParentID     string // For subtasks
	CustomFields map[string]any
}

// FullProvider is a comprehensive interface combining all capabilities
// This is primarily for documentation; actual providers implement subsets
type FullProvider interface {
	ReadOnlyProvider
	Lister
	Commenter
	StatusUpdater
	LabelManager
	AttachmentDownloader
	CommentFetcher
	PRCreator
	BranchLinker
	WorkUnitCreator
	Snapshotter
}

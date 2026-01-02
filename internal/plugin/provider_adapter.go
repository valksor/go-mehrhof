package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/valksor/go-mehrhof/internal/provider"
)

// ProviderAdapter wraps a plugin process to implement provider interfaces.
// It dynamically implements interfaces based on the plugin's declared capabilities.
type ProviderAdapter struct {
	manifest *Manifest
	proc     *Process
}

// NewProviderAdapter creates a new provider adapter for a plugin.
func NewProviderAdapter(manifest *Manifest, proc *Process) *ProviderAdapter {
	return &ProviderAdapter{
		manifest: manifest,
		proc:     proc,
	}
}

// Manifest returns the plugin manifest.
func (a *ProviderAdapter) Manifest() *Manifest {
	return a.manifest
}

// ─────────────────────────────────────────────────────────────────────────────
// Identifier interface
// ─────────────────────────────────────────────────────────────────────────────

// Match checks if the input matches this provider's scheme.
func (a *ProviderAdapter) Match(input string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := a.proc.Call(ctx, "provider.match", &MatchParams{Input: input})
	if err != nil {
		return false
	}

	var resp MatchResult
	if err := json.Unmarshal(result, &resp); err != nil {
		return false
	}

	return resp.Matches
}

// Parse parses and validates a reference.
func (a *ProviderAdapter) Parse(input string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := a.proc.Call(ctx, "provider.parse", &ParseParams{Input: input})
	if err != nil {
		return "", err
	}

	var resp ParseResult
	if err := json.Unmarshal(result, &resp); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

	if resp.Error != "" {
		return "", fmt.Errorf("parse error: %s", resp.Error)
	}

	return resp.ID, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Reader interface
// ─────────────────────────────────────────────────────────────────────────────

// Fetch retrieves a work unit by ID.
func (a *ProviderAdapter) Fetch(ctx context.Context, id string) (*provider.WorkUnit, error) {
	result, err := a.proc.Call(ctx, "provider.fetch", &FetchParams{ID: id})
	if err != nil {
		return nil, fmt.Errorf("fetch work unit: %w", err)
	}

	var resp WorkUnitResult
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, fmt.Errorf("parse work unit: %w", err)
	}

	return convertWorkUnit(&resp), nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Optional interfaces (based on capabilities)
// ─────────────────────────────────────────────────────────────────────────────

// List enumerates work units.
func (a *ProviderAdapter) List(ctx context.Context, opts provider.ListOptions) ([]*provider.WorkUnit, error) {
	if !a.manifest.HasCapability("list") {
		return nil, errors.New("plugin does not support listing")
	}

	params := &ListParams{
		Status: string(opts.Status),
		Labels: opts.Labels,
		Limit:  opts.Limit,
		Offset: opts.Offset,
	}

	result, err := a.proc.Call(ctx, "provider.list", params)
	if err != nil {
		return nil, fmt.Errorf("list work units: %w", err)
	}

	var resp []WorkUnitResult
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, fmt.Errorf("parse work units: %w", err)
	}

	workUnits := make([]*provider.WorkUnit, len(resp))
	for i, wu := range resp {
		workUnits[i] = convertWorkUnit(&wu)
	}

	return workUnits, nil
}

// AddComment adds a comment to a work unit.
func (a *ProviderAdapter) AddComment(ctx context.Context, workUnitID string, body string) (*provider.Comment, error) {
	if !a.manifest.HasCapability("comment") {
		return nil, errors.New("plugin does not support commenting")
	}

	result, err := a.proc.Call(ctx, "provider.addComment", &AddCommentParams{
		WorkUnitID: workUnitID,
		Body:       body,
	})
	if err != nil {
		return nil, fmt.Errorf("add comment: %w", err)
	}

	var resp CommentResult
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, fmt.Errorf("parse comment: %w", err)
	}

	return &provider.Comment{
		ID:   resp.ID,
		Body: resp.Body,
		Author: provider.Person{
			ID:    resp.Author.ID,
			Name:  resp.Author.Name,
			Email: resp.Author.Email,
		},
		CreatedAt: resp.CreatedAt,
	}, nil
}

// UpdateStatus changes a work unit's status.
func (a *ProviderAdapter) UpdateStatus(ctx context.Context, workUnitID string, status provider.Status) error {
	if !a.manifest.HasCapability("update_status") {
		return errors.New("plugin does not support status updates")
	}

	_, err := a.proc.Call(ctx, "provider.updateStatus", &UpdateStatusParams{
		WorkUnitID: workUnitID,
		Status:     string(status),
	})

	return err
}

// CreatePullRequest creates a pull request.
func (a *ProviderAdapter) CreatePullRequest(ctx context.Context, opts provider.PullRequestOptions) (*provider.PullRequest, error) {
	if !a.manifest.HasCapability("create_pr") {
		return nil, errors.New("plugin does not support PR creation")
	}

	result, err := a.proc.Call(ctx, "provider.createPR", &CreatePRParams{
		Title:        opts.Title,
		Description:  opts.Body,
		SourceBranch: opts.SourceBranch,
		TargetBranch: opts.TargetBranch,
		Draft:        opts.Draft,
	})
	if err != nil {
		return nil, fmt.Errorf("create PR: %w", err)
	}

	var resp PullRequestResult
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, fmt.Errorf("parse PR: %w", err)
	}

	return &provider.PullRequest{
		ID:     resp.ID,
		Number: resp.Number,
		URL:    resp.URL,
		State:  resp.State,
	}, nil
}

// Snapshot captures source content.
func (a *ProviderAdapter) Snapshot(ctx context.Context, id string) (*provider.Snapshot, error) {
	if !a.manifest.HasCapability("snapshot") {
		return nil, errors.New("plugin does not support snapshots")
	}

	result, err := a.proc.Call(ctx, "provider.snapshot", &SnapshotParams{ID: id})
	if err != nil {
		return nil, fmt.Errorf("snapshot: %w", err)
	}

	var resp SnapshotResult
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, fmt.Errorf("parse snapshot: %w", err)
	}

	return &provider.Snapshot{
		Content: resp.Content,
		Ref:     id,
	}, nil
}

// FetchComments retrieves comments for a work unit.
func (a *ProviderAdapter) FetchComments(ctx context.Context, workUnitID string) ([]provider.Comment, error) {
	if !a.manifest.HasCapability("fetch_comments") {
		return nil, errors.New("plugin does not support fetching comments")
	}

	result, err := a.proc.Call(ctx, "provider.fetchComments", map[string]string{"workUnitId": workUnitID})
	if err != nil {
		return nil, fmt.Errorf("fetch comments: %w", err)
	}

	var resp []CommentResult
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, fmt.Errorf("parse comments: %w", err)
	}

	comments := make([]provider.Comment, len(resp))
	for i, c := range resp {
		comments[i] = provider.Comment{
			ID:   c.ID,
			Body: c.Body,
			Author: provider.Person{
				ID:    c.Author.ID,
				Name:  c.Author.Name,
				Email: c.Author.Email,
			},
			CreatedAt: c.CreatedAt,
		}
	}

	return comments, nil
}

// AddLabels adds labels to a work unit.
func (a *ProviderAdapter) AddLabels(ctx context.Context, workUnitID string, labels []string) error {
	if !a.manifest.HasCapability("manage_labels") {
		return errors.New("plugin does not support label management")
	}

	_, err := a.proc.Call(ctx, "provider.addLabels", map[string]any{
		"workUnitId": workUnitID,
		"labels":     labels,
	})

	return err
}

// RemoveLabels removes labels from a work unit.
func (a *ProviderAdapter) RemoveLabels(ctx context.Context, workUnitID string, labels []string) error {
	if !a.manifest.HasCapability("manage_labels") {
		return errors.New("plugin does not support label management")
	}

	_, err := a.proc.Call(ctx, "provider.removeLabels", map[string]any{
		"workUnitId": workUnitID,
		"labels":     labels,
	})

	return err
}

// LinkBranch links a work unit to a git branch.
func (a *ProviderAdapter) LinkBranch(ctx context.Context, workUnitID, branch string) error {
	if !a.manifest.HasCapability("link_branch") {
		return errors.New("plugin does not support branch linking")
	}

	_, err := a.proc.Call(ctx, "provider.linkBranch", map[string]string{
		"workUnitId": workUnitID,
		"branch":     branch,
	})

	return err
}

// UnlinkBranch unlinks a work unit from a git branch.
func (a *ProviderAdapter) UnlinkBranch(ctx context.Context, workUnitID, branch string) error {
	if !a.manifest.HasCapability("link_branch") {
		return errors.New("plugin does not support branch linking")
	}

	_, err := a.proc.Call(ctx, "provider.unlinkBranch", map[string]string{
		"workUnitId": workUnitID,
		"branch":     branch,
	})

	return err
}

// GetLinkedBranch returns the linked branch for a work unit.
func (a *ProviderAdapter) GetLinkedBranch(ctx context.Context, workUnitID string) (string, error) {
	if !a.manifest.HasCapability("link_branch") {
		return "", errors.New("plugin does not support branch linking")
	}

	result, err := a.proc.Call(ctx, "provider.getLinkedBranch", map[string]string{"workUnitId": workUnitID})
	if err != nil {
		return "", err
	}

	var resp struct {
		Branch string `json:"branch"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

	return resp.Branch, nil
}

// DownloadAttachment downloads an attachment.
func (a *ProviderAdapter) DownloadAttachment(ctx context.Context, workUnitID, attachmentID string) (io.ReadCloser, error) {
	if !a.manifest.HasCapability("download_attachment") {
		return nil, errors.New("plugin does not support attachment download")
	}

	result, err := a.proc.Call(ctx, "provider.downloadAttachment", map[string]string{
		"workUnitId":   workUnitID,
		"attachmentId": attachmentID,
	})
	if err != nil {
		return nil, fmt.Errorf("download attachment: %w", err)
	}

	var resp struct {
		Content string `json:"content"` // Base64 encoded
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	// Return content as a ReadCloser
	return io.NopCloser(strings.NewReader(resp.Content)), nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Capability checking
// ─────────────────────────────────────────────────────────────────────────────

// Capabilities returns the provider's capability set.
func (a *ProviderAdapter) Capabilities() provider.CapabilitySet {
	caps := make(provider.CapabilitySet)

	capMap := map[string]provider.Capability{
		"read":                provider.CapRead,
		"list":                provider.CapList,
		"download_attachment": provider.CapDownloadAttachment,
		"fetch_comments":      provider.CapFetchComments,
		"comment":             provider.CapComment,
		"update_status":       provider.CapUpdateStatus,
		"manage_labels":       provider.CapManageLabels,
		"snapshot":            provider.CapSnapshot,
		"create_pr":           provider.CapCreatePR,
		"link_branch":         provider.CapLinkBranch,
		"create_work_unit":    provider.CapCreateWorkUnit,
	}

	// Always has read capability (required for providers)
	caps[provider.CapRead] = true

	for _, c := range a.manifest.Provider.Capabilities {
		if mappedCap, ok := capMap[c]; ok {
			caps[mappedCap] = true
		}
	}

	return caps
}

// ─────────────────────────────────────────────────────────────────────────────
// Conversion helpers
// ─────────────────────────────────────────────────────────────────────────────

func convertWorkUnit(r *WorkUnitResult) *provider.WorkUnit {
	wu := &provider.WorkUnit{
		ID:          r.ID,
		ExternalID:  r.ExternalID,
		Provider:    r.Provider,
		Title:       r.Title,
		Description: r.Description,
		Status:      provider.Status(r.Status),
		Priority:    provider.Priority(r.Priority),
		Labels:      r.Labels,
		Subtasks:    r.Subtasks,
		ExternalKey: r.ExternalKey,
		TaskType:    r.TaskType,
		Slug:        r.Slug,
		Metadata:    r.Metadata,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}

	// Convert source info
	if r.Source.Reference != "" {
		wu.Source = provider.SourceInfo{
			Reference: r.Source.Reference,
		}
	}

	// Convert assignees
	wu.Assignees = make([]provider.Person, len(r.Assignees))
	for i, p := range r.Assignees {
		wu.Assignees[i] = provider.Person{
			ID:    p.ID,
			Name:  p.Name,
			Email: p.Email,
		}
	}

	// Convert comments
	wu.Comments = make([]provider.Comment, len(r.Comments))
	for i, c := range r.Comments {
		wu.Comments[i] = provider.Comment{
			ID:   c.ID,
			Body: c.Body,
			Author: provider.Person{
				ID:    c.Author.ID,
				Name:  c.Author.Name,
				Email: c.Author.Email,
			},
			CreatedAt: c.CreatedAt,
		}
	}

	// Convert attachments
	wu.Attachments = make([]provider.Attachment, len(r.Attachments))
	for i, a := range r.Attachments {
		wu.Attachments[i] = provider.Attachment{
			ID:          a.ID,
			Name:        a.Name,
			URL:         a.URL,
			ContentType: a.MimeType,
			Size:        a.Size,
		}
	}

	return wu
}

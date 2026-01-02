package azuredevops

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/valksor/go-mehrhof/internal/provider"
)

// ProviderName is the canonical name for this provider.
const ProviderName = "azuredevops"

// Provider implements the Azure DevOps work item provider.
type Provider struct {
	client *Client
	config *Config
}

// Config holds Azure DevOps provider configuration.
type Config struct {
	Token         string
	Organization  string
	Project       string
	AreaPath      string // Default area path for filtering
	IterationPath string // Default iteration path for filtering
	RepoName      string // Default repository for PR creation
	TargetBranch  string // Default target branch for PRs
	BranchPattern string
	CommitPrefix  string
}

// Info returns provider metadata.
func Info() provider.ProviderInfo {
	return provider.ProviderInfo{
		Name:        ProviderName,
		Description: "Load work items from Azure DevOps",
		Schemes:     []string{"azdo", "azure"},
		Capabilities: provider.CapabilitySet{
			provider.CapRead:           true,
			provider.CapList:           true,
			provider.CapFetchComments:  true,
			provider.CapComment:        true,
			provider.CapUpdateStatus:   true,
			provider.CapManageLabels:   true,
			provider.CapSnapshot:       true,
			provider.CapCreatePR:       true,
			provider.CapLinkBranch:     true,
			provider.CapCreateWorkUnit: true,
			provider.CapFetchSubtasks:  true,
		},
	}
}

// New creates a new Azure DevOps provider instance.
func New(_ context.Context, cfg provider.Config) (any, error) {
	config := &Config{
		Token:         cfg.GetString("token"),
		Organization:  cfg.GetString("organization"),
		Project:       cfg.GetString("project"),
		AreaPath:      cfg.GetString("area_path"),
		IterationPath: cfg.GetString("iteration_path"),
		RepoName:      cfg.GetString("repo_name"),
		TargetBranch:  cfg.GetString("target_branch"),
		BranchPattern: cfg.GetString("branch_pattern"),
		CommitPrefix:  cfg.GetString("commit_prefix"),
	}

	if config.Organization == "" {
		return nil, ErrOrgRequired
	}
	if config.Project == "" {
		return nil, ErrProjectRequired
	}

	// Resolve token
	token, err := ResolveToken(config.Token)
	if err != nil {
		return nil, err
	}

	client := NewClient(config.Organization, config.Project, token)

	return &Provider{
		client: client,
		config: config,
	}, nil
}

// Match checks if the input looks like an Azure DevOps reference.
func (p *Provider) Match(input string) bool {
	// Check for azdo: or azure: prefix
	if strings.HasPrefix(input, "azdo:") || strings.HasPrefix(input, "azure:") {
		return true
	}

	// Check for Azure DevOps URL patterns
	if strings.Contains(input, "dev.azure.com") || strings.Contains(input, "visualstudio.com") {
		return true
	}

	// Check for bare work item ID or org/project#ID pattern
	_, err := ParseReference(input)
	return err == nil
}

// Parse parses an Azure DevOps reference and returns a canonical ID.
func (p *Provider) Parse(input string) (string, error) {
	ref, err := ParseReference(input)
	if err != nil {
		return "", err
	}
	return strconv.Itoa(ref.WorkItemID), nil
}

// Fetch retrieves a work item by its ID.
func (p *Provider) Fetch(ctx context.Context, id string) (*provider.WorkUnit, error) {
	ref, err := ParseReference(id)
	if err != nil {
		return nil, fmt.Errorf("parse reference: %w", err)
	}

	// Override org/project if specified in reference
	if ref.Organization != "" && ref.Project != "" {
		p.client.SetOrganization(ref.Organization)
		p.client.SetProject(ref.Project)
	}

	workItem, err := p.client.GetWorkItem(ctx, ref.WorkItemID)
	if err != nil {
		return nil, fmt.Errorf("fetch work item %d: %w", ref.WorkItemID, err)
	}

	return p.workItemToWorkUnit(workItem), nil
}

// Snapshot creates a snapshot of the work item's current state.
func (p *Provider) Snapshot(ctx context.Context, id string) (*provider.Snapshot, error) {
	ref, err := ParseReference(id)
	if err != nil {
		return nil, fmt.Errorf("parse reference: %w", err)
	}

	// Override org/project if specified in reference
	if ref.Organization != "" && ref.Project != "" {
		p.client.SetOrganization(ref.Organization)
		p.client.SetProject(ref.Project)
	}

	workItem, err := p.client.GetWorkItem(ctx, ref.WorkItemID)
	if err != nil {
		return nil, fmt.Errorf("snapshot work item %d: %w", ref.WorkItemID, err)
	}

	// Build markdown content
	content := buildSnapshotContent(workItem)

	return &provider.Snapshot{
		Type:    ProviderName,
		Ref:     fmt.Sprintf("azdo:%d", workItem.ID),
		Content: content,
	}, nil
}

// List retrieves work items based on filter criteria.
func (p *Provider) List(ctx context.Context, opts provider.ListOptions) ([]*provider.WorkUnit, error) {
	// Build WIQL query
	wiql := buildWIQLQuery(p.config, opts)

	ids, err := p.client.QueryWorkItems(ctx, wiql)
	if err != nil {
		return nil, fmt.Errorf("query work items: %w", err)
	}

	if len(ids) == 0 {
		return nil, nil
	}

	// Apply limit
	if opts.Limit > 0 && len(ids) > opts.Limit {
		ids = ids[:opts.Limit]
	}

	workItems, err := p.client.GetWorkItems(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("get work items: %w", err)
	}

	var units []*provider.WorkUnit
	for _, wi := range workItems {
		units = append(units, p.workItemToWorkUnit(&wi))
	}

	return units, nil
}

// FetchComments retrieves comments for a work item.
func (p *Provider) FetchComments(ctx context.Context, id string) ([]*provider.Comment, error) {
	ref, err := ParseReference(id)
	if err != nil {
		return nil, fmt.Errorf("parse reference: %w", err)
	}

	comments, err := p.client.GetWorkItemComments(ctx, ref.WorkItemID)
	if err != nil {
		return nil, fmt.Errorf("fetch comments for %d: %w", ref.WorkItemID, err)
	}

	var result []*provider.Comment
	for _, comment := range comments {
		author := provider.Person{}
		if comment.CreatedBy != nil {
			author = provider.Person{
				ID:    comment.CreatedBy.ID,
				Name:  comment.CreatedBy.DisplayName,
				Email: comment.CreatedBy.UniqueName,
			}
		}

		result = append(result, &provider.Comment{
			ID:        strconv.Itoa(comment.ID),
			Author:    author,
			Body:      comment.Text,
			CreatedAt: parseAzureTime(comment.CreatedDate),
		})
	}

	return result, nil
}

// AddComment adds a comment to a work item.
func (p *Provider) AddComment(ctx context.Context, id string, body string) error {
	ref, err := ParseReference(id)
	if err != nil {
		return fmt.Errorf("parse reference: %w", err)
	}

	_, err = p.client.AddWorkItemComment(ctx, ref.WorkItemID, body)
	if err != nil {
		return fmt.Errorf("add comment to %d: %w", ref.WorkItemID, err)
	}
	return nil
}

// UpdateStatus updates the work item state.
func (p *Provider) UpdateStatus(ctx context.Context, id string, status provider.Status) error {
	ref, err := ParseReference(id)
	if err != nil {
		return fmt.Errorf("parse reference: %w", err)
	}

	// Map provider status to Azure DevOps state
	azState := mapToAzureState(status)
	if azState == "" {
		return nil // No mapping for this status
	}

	_, err = p.client.UpdateWorkItemState(ctx, ref.WorkItemID, azState)
	if err != nil {
		return fmt.Errorf("update work item state %d: %w", ref.WorkItemID, err)
	}
	return nil
}

// CreatePullRequest creates a pull request.
// Work items can be linked automatically via AB#123 syntax in title/body.
func (p *Provider) CreatePullRequest(ctx context.Context, opts provider.PullRequestOptions) (*provider.PullRequest, error) {
	repoName := p.config.RepoName
	if repoName == "" {
		// Try to find default repository
		repos, err := p.client.GetRepositories(ctx)
		if err != nil {
			return nil, fmt.Errorf("get repositories: %w", err)
		}
		if len(repos) == 0 {
			return nil, fmt.Errorf("no repositories found in project")
		}
		repoName = repos[0].Name
	}

	targetBranch := opts.TargetBranch
	if targetBranch == "" {
		targetBranch = p.config.TargetBranch
	}
	if targetBranch == "" {
		targetBranch = "main"
	}

	// Extract work item IDs from title/body for auto-linking (AB#123 or #123 format)
	workItemIDs := ExtractWorkItemIDs(opts.Title + " " + opts.Body)

	pr, err := p.client.CreatePullRequest(ctx, repoName, opts.SourceBranch, targetBranch, opts.Title, opts.Body, workItemIDs)
	if err != nil {
		return nil, fmt.Errorf("create pull request: %w", err)
	}

	prURL := fmt.Sprintf("https://dev.azure.com/%s/%s/_git/%s/pullrequest/%d",
		p.config.Organization, p.config.Project, repoName, pr.PullRequestID)

	return &provider.PullRequest{
		ID:    strconv.Itoa(pr.PullRequestID),
		URL:   prURL,
		Title: pr.Title,
		State: pr.Status,
	}, nil
}

// LinkBranch links a branch to the work item.
func (p *Provider) LinkBranch(ctx context.Context, id string, branch string) error {
	ref, err := ParseReference(id)
	if err != nil {
		return fmt.Errorf("parse reference: %w", err)
	}

	// Add branch link relation
	updates := []PatchOperation{
		{
			Op:   "add",
			Path: "/relations/-",
			Value: map[string]any{
				"rel": "ArtifactLink",
				"url": fmt.Sprintf("vstfs:///Git/Ref/%s/%s/GB%s", p.config.Organization, p.config.Project, branch),
				"attributes": map[string]any{
					"name": "Branch",
				},
			},
		},
	}

	_, err = p.client.UpdateWorkItem(ctx, ref.WorkItemID, updates)
	if err != nil {
		return fmt.Errorf("link branch to %d: %w", ref.WorkItemID, err)
	}
	return nil
}

// --- Helper functions ---

func (p *Provider) workItemToWorkUnit(wi *WorkItem) *provider.WorkUnit {
	unit := &provider.WorkUnit{
		ID:          strconv.Itoa(wi.ID),
		ExternalID:  strconv.Itoa(wi.ID),
		ExternalKey: strconv.Itoa(wi.ID),
		Provider:    ProviderName,
		Title:       wi.Fields.Title,
		Description: wi.Fields.Description,
		Status:      mapAzureState(wi.Fields.State),
		Priority:    mapAzurePriority(wi.Fields.Priority),
		TaskType:    mapWorkItemType(wi.Fields.WorkItemType),
		Labels:      parseTags(wi.Fields.Tags),
		CreatedAt:   parseAzureTime(wi.Fields.CreatedDate),
		UpdatedAt:   parseAzureTime(wi.Fields.ChangedDate),
		Source: provider.SourceInfo{
			Type:      ProviderName,
			Reference: fmt.Sprintf("azdo:%d", wi.ID),
			SyncedAt:  time.Now(),
		},
		Metadata: map[string]any{
			"url":            wi.URL,
			"area_path":      wi.Fields.AreaPath,
			"iteration_path": wi.Fields.IterationPath,
			"work_item_type": wi.Fields.WorkItemType,
			"state":          wi.Fields.State,
			"reason":         wi.Fields.Reason,
		},
	}

	// Set assignee
	if wi.Fields.AssignedTo != nil {
		unit.Assignees = []provider.Person{
			{
				ID:    wi.Fields.AssignedTo.ID,
				Name:  wi.Fields.AssignedTo.DisplayName,
				Email: wi.Fields.AssignedTo.UniqueName,
			},
		}
	}

	// Add repro steps for bugs
	if wi.Fields.ReproSteps != "" {
		unit.Metadata["repro_steps"] = wi.Fields.ReproSteps
	}

	// Add acceptance criteria
	if wi.Fields.AcceptanceCriteria != "" {
		unit.Metadata["acceptance_criteria"] = wi.Fields.AcceptanceCriteria
	}

	return unit
}

func mapAzureState(state string) provider.Status {
	stateLower := strings.ToLower(state)
	switch {
	case contains(stateLower, "done") || contains(stateLower, "closed") || contains(stateLower, "resolved"):
		return provider.StatusClosed
	case contains(stateLower, "active") || contains(stateLower, "committed") || contains(stateLower, "in progress"):
		return provider.StatusInProgress
	case contains(stateLower, "review") || contains(stateLower, "pr"):
		return provider.StatusReview
	case contains(stateLower, "new") || contains(stateLower, "to do") || contains(stateLower, "proposed"):
		return provider.StatusOpen
	}
	return provider.StatusOpen
}

func mapToAzureState(status provider.Status) string {
	switch status {
	case provider.StatusClosed, provider.StatusDone:
		return "Done"
	case provider.StatusInProgress:
		return "Active"
	case provider.StatusOpen:
		return "New"
	case provider.StatusReview:
		return "Resolved" // Or could use custom state if available
	default:
		return ""
	}
}

func mapAzurePriority(priority int) provider.Priority {
	switch priority {
	case 1:
		return provider.PriorityCritical
	case 2:
		return provider.PriorityHigh
	case 3:
		return provider.PriorityNormal
	case 4:
		return provider.PriorityLow
	default:
		return provider.PriorityNormal
	}
}

func mapWorkItemType(wiType string) string {
	typeLower := strings.ToLower(wiType)
	switch {
	case contains(typeLower, "bug"):
		return "fix"
	case contains(typeLower, "feature") || contains(typeLower, "user story"):
		return "feature"
	case contains(typeLower, "task"):
		return "task"
	case contains(typeLower, "epic"):
		return "epic"
	case contains(typeLower, "issue"):
		return "issue"
	}
	return "task"
}

func parseTags(tags string) []string {
	if tags == "" {
		return nil
	}
	// Azure DevOps stores tags as semicolon-separated string
	parts := strings.Split(tags, ";")
	var result []string
	for _, part := range parts {
		tag := strings.TrimSpace(part)
		if tag != "" {
			result = append(result, tag)
		}
	}
	return result
}

func parseAzureTime(ts string) time.Time {
	if ts == "" {
		return time.Time{}
	}

	// Azure DevOps uses ISO 8601 format
	if t, err := time.Parse(time.RFC3339, ts); err == nil {
		return t
	}
	if t, err := time.Parse("2006-01-02T15:04:05Z", ts); err == nil {
		return t
	}
	if t, err := time.Parse("2006-01-02T15:04:05.999Z", ts); err == nil {
		return t
	}

	return time.Time{}
}

func buildWIQLQuery(config *Config, opts provider.ListOptions) string {
	var conditions []string

	// Filter by area path
	if config.AreaPath != "" {
		conditions = append(conditions, fmt.Sprintf("[System.AreaPath] UNDER '%s'", config.AreaPath))
	}

	// Filter by iteration path
	if config.IterationPath != "" {
		conditions = append(conditions, fmt.Sprintf("[System.IterationPath] UNDER '%s'", config.IterationPath))
	}

	// Filter by status
	switch opts.Status {
	case provider.StatusOpen:
		conditions = append(conditions, "[System.State] IN ('New', 'To Do', 'Proposed')")
	case provider.StatusInProgress:
		conditions = append(conditions, "[System.State] IN ('Active', 'In Progress', 'Committed')")
	case provider.StatusReview:
		conditions = append(conditions, "[System.State] IN ('Resolved', 'In Review')")
	case provider.StatusClosed, provider.StatusDone:
		conditions = append(conditions, "[System.State] IN ('Done', 'Closed', 'Removed')")
	}

	// Filter by labels (tags)
	for _, label := range opts.Labels {
		conditions = append(conditions, fmt.Sprintf("[System.Tags] CONTAINS '%s'", label))
	}

	query := "SELECT [System.Id] FROM WorkItems"
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY [System.ChangedDate] DESC"

	return query
}

func buildSnapshotContent(wi *WorkItem) string {
	var sb strings.Builder

	// Title
	sb.WriteString("# ")
	sb.WriteString(wi.Fields.Title)
	sb.WriteString("\n\n")

	// Metadata
	sb.WriteString("**ID:** ")
	sb.WriteString(strconv.Itoa(wi.ID))
	sb.WriteString("\n")

	sb.WriteString("**Type:** ")
	sb.WriteString(wi.Fields.WorkItemType)
	sb.WriteString("\n")

	sb.WriteString("**State:** ")
	sb.WriteString(wi.Fields.State)
	sb.WriteString("\n")

	if wi.Fields.AssignedTo != nil {
		sb.WriteString("**Assigned To:** ")
		sb.WriteString(wi.Fields.AssignedTo.DisplayName)
		sb.WriteString("\n")
	}

	if wi.Fields.AreaPath != "" {
		sb.WriteString("**Area:** ")
		sb.WriteString(wi.Fields.AreaPath)
		sb.WriteString("\n")
	}

	if wi.Fields.IterationPath != "" {
		sb.WriteString("**Iteration:** ")
		sb.WriteString(wi.Fields.IterationPath)
		sb.WriteString("\n")
	}

	if wi.Fields.Priority > 0 {
		sb.WriteString("**Priority:** ")
		sb.WriteString(strconv.Itoa(wi.Fields.Priority))
		sb.WriteString("\n")
	}

	if wi.Fields.Tags != "" {
		sb.WriteString("**Tags:** ")
		sb.WriteString(wi.Fields.Tags)
		sb.WriteString("\n")
	}

	sb.WriteString("\n")

	// Description
	if wi.Fields.Description != "" {
		sb.WriteString("## Description\n\n")
		sb.WriteString(wi.Fields.Description)
		sb.WriteString("\n")
	}

	// Repro steps for bugs
	if wi.Fields.ReproSteps != "" {
		sb.WriteString("\n## Repro Steps\n\n")
		sb.WriteString(wi.Fields.ReproSteps)
		sb.WriteString("\n")
	}

	// Acceptance criteria
	if wi.Fields.AcceptanceCriteria != "" {
		sb.WriteString("\n## Acceptance Criteria\n\n")
		sb.WriteString(wi.Fields.AcceptanceCriteria)
		sb.WriteString("\n")
	}

	return sb.String()
}

// GetBranchSuggestion returns a suggested branch name for the work item.
func (p *Provider) GetBranchSuggestion(task *provider.WorkUnit) string {
	if p.config.BranchPattern == "" {
		// Default pattern
		return fmt.Sprintf("ab%s", task.ID)
	}

	// Simple template replacement
	result := p.config.BranchPattern
	result = strings.ReplaceAll(result, "{key}", task.ExternalKey)
	result = strings.ReplaceAll(result, "{id}", task.ID)

	// Slugify title
	slug := slugify(task.Title)
	result = strings.ReplaceAll(result, "{slug}", slug)

	return result
}

func slugify(s string) string {
	// Simple slugification
	s = strings.ToLower(s)
	s = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			return r
		}
		if r == ' ' || r == '-' || r == '_' {
			return '-'
		}
		return -1
	}, s)

	// Remove consecutive hyphens
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	s = strings.Trim(s, "-")

	// Truncate
	if len(s) > 50 {
		s = s[:50]
		s = strings.TrimRight(s, "-")
	}

	return s
}

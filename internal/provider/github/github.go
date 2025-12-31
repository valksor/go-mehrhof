package github

import (
	"context"
	"fmt"
	"strings"
	"time"

	gh "github.com/google/go-github/v67/github"

	"github.com/valksor/go-mehrhof/internal/cache"
	"github.com/valksor/go-mehrhof/internal/naming"
	"github.com/valksor/go-mehrhof/internal/provider"
)

// ProviderName is the registered name for this provider
const ProviderName = "github"

// Provider handles GitHub issue tasks
type Provider struct {
	client *Client
	owner  string
	repo   string
	config *Config
	cache  *cache.Cache
}

// Config holds GitHub provider configuration
type Config struct {
	Token         string
	Owner         string
	Repo          string
	BranchPattern string // Default: "issue/{key}-{slug}"
	CommitPrefix  string // Default: "[#{key}]"
	TargetBranch  string // Default: detected from repo
	DraftPR       bool
	Comments      *CommentsConfig
}

// CommentsConfig controls automated commenting
type CommentsConfig struct {
	Enabled         bool
	OnBranchCreated bool
	OnPlanDone      bool
	OnImplementDone bool
	OnPRCreated     bool
}

// Info returns provider metadata
func Info() provider.ProviderInfo {
	return provider.ProviderInfo{
		Name:        ProviderName,
		Description: "GitHub Issues task source",
		Schemes:     []string{"github", "gh"},
		Priority:    20, // Higher than file/directory
		Capabilities: provider.CapabilitySet{
			provider.CapRead:               true,
			provider.CapList:               true,
			provider.CapFetchComments:      true,
			provider.CapComment:            true,
			provider.CapUpdateStatus:       true,
			provider.CapManageLabels:       true,
			provider.CapCreateWorkUnit:     true,
			provider.CapCreatePR:           true,
			provider.CapDownloadAttachment: true,
			provider.CapSnapshot:           true,
			provider.CapFetchSubtasks:      true,
		},
	}
}

// New creates a GitHub provider
func New(ctx context.Context, cfg provider.Config) (any, error) {
	// Extract config values
	token := cfg.GetString("token")
	owner := cfg.GetString("owner")
	repo := cfg.GetString("repo")
	branchPattern := cfg.GetString("branch_pattern")
	commitPrefix := cfg.GetString("commit_prefix")
	targetBranch := cfg.GetString("target_branch")
	draftPR := cfg.GetBool("draft_pr")

	// Resolve token
	resolvedToken, err := ResolveToken(token)
	if err != nil {
		return nil, err
	}

	// Set defaults
	if branchPattern == "" {
		branchPattern = "issue/{key}-{slug}"
	}
	if commitPrefix == "" {
		commitPrefix = "[#{key}]"
	}

	// Parse comments config
	commentsEnabled := cfg.GetBool("comments.enabled")
	comments := &CommentsConfig{
		Enabled:         commentsEnabled,
		OnBranchCreated: cfg.GetBool("comments.on_branch_created"),
		OnPlanDone:      cfg.GetBool("comments.on_plan_done"),
		OnImplementDone: cfg.GetBool("comments.on_implement_done"),
		OnPRCreated:     cfg.GetBool("comments.on_pr_created"),
	}

	config := &Config{
		Token:         resolvedToken,
		Owner:         owner,
		Repo:          repo,
		BranchPattern: branchPattern,
		CommitPrefix:  commitPrefix,
		TargetBranch:  targetBranch,
		DraftPR:       draftPR,
		Comments:      comments,
	}

	// Create cache (enabled by default, can be disabled via config or SetCache(nil))
	providerCache := cache.New()
	if cfg.GetBool("cache.disabled") {
		providerCache.Disable()
	}

	return &Provider{
		client: NewClientWithCache(resolvedToken, owner, repo, providerCache),
		owner:  owner,
		repo:   repo,
		config: config,
		cache:  providerCache,
	}, nil
}

// Match checks if input has the github: or gh: scheme prefix
func (p *Provider) Match(input string) bool {
	return strings.HasPrefix(input, "github:") || strings.HasPrefix(input, "gh:")
}

// Parse extracts the issue reference from input
func (p *Provider) Parse(input string) (string, error) {
	ref, err := ParseReference(input)
	if err != nil {
		return "", err
	}

	// If explicit owner/repo provided, use it
	if ref.IsExplicit {
		return fmt.Sprintf("%s/%s#%d", ref.Owner, ref.Repo, ref.IssueNumber), nil
	}

	// Otherwise, check if we have owner/repo configured
	owner := p.owner
	repo := p.repo

	if owner == "" || repo == "" {
		return "", fmt.Errorf("%w: use github:owner/repo#N format or configure github.owner and github.repo", ErrRepoNotConfigured)
	}

	return fmt.Sprintf("%s/%s#%d", owner, repo, ref.IssueNumber), nil
}

// Fetch reads a GitHub issue and creates a WorkUnit
func (p *Provider) Fetch(ctx context.Context, id string) (*provider.WorkUnit, error) {
	ref, err := ParseReference(id)
	if err != nil {
		return nil, err
	}

	// Determine owner/repo
	owner := ref.Owner
	repo := ref.Repo
	if owner == "" {
		owner = p.owner
	}
	if repo == "" {
		repo = p.repo
	}
	if owner == "" || repo == "" {
		return nil, ErrRepoNotConfigured
	}

	// Update client with correct owner/repo
	p.client.SetOwnerRepo(owner, repo)

	// Fetch issue
	issue, err := p.client.GetIssue(ctx, ref.IssueNumber)
	if err != nil {
		return nil, err
	}

	// Map to WorkUnit
	wu := &provider.WorkUnit{
		ID:          fmt.Sprintf("%d", issue.GetNumber()),
		ExternalID:  fmt.Sprintf("%s/%s#%d", owner, repo, issue.GetNumber()),
		Provider:    ProviderName,
		Title:       issue.GetTitle(),
		Description: issue.GetBody(),
		Status:      mapGitHubState(issue.GetState()),
		Priority:    inferPriorityFromLabels(issue.Labels),
		Labels:      extractLabelNames(issue.Labels),
		Assignees:   mapAssignees(issue.Assignees),
		CreatedAt:   issue.GetCreatedAt().Time,
		UpdatedAt:   issue.GetUpdatedAt().Time,
		Source: provider.SourceInfo{
			Type:      ProviderName,
			Reference: id,
			SyncedAt:  time.Now(),
		},

		// Naming fields for branch/commit customization
		ExternalKey: fmt.Sprintf("%d", issue.GetNumber()),
		TaskType:    inferTypeFromLabels(issue.Labels),
		Slug:        naming.Slugify(issue.GetTitle(), 50),

		Metadata: map[string]any{
			"html_url":       issue.GetHTMLURL(),
			"owner":          owner,
			"repo":           repo,
			"issue_number":   issue.GetNumber(),
			"labels_raw":     issue.Labels,
			"branch_pattern": p.config.BranchPattern,
			"commit_prefix":  p.config.CommitPrefix,
		},
	}

	// Fetch comments if available
	comments, err := p.client.GetIssueComments(ctx, ref.IssueNumber)
	if err == nil && len(comments) > 0 {
		wu.Comments = mapComments(comments)
	}

	// Extract linked issues
	linkedNums := ExtractLinkedIssues(issue.GetBody())
	if len(linkedNums) > 0 {
		wu.Metadata["linked_issues"] = linkedNums
	}

	// Extract image URLs
	imageURLs := ExtractImageURLs(issue.GetBody())
	if len(imageURLs) > 0 {
		wu.Attachments = make([]provider.Attachment, len(imageURLs))
		for i, url := range imageURLs {
			wu.Attachments[i] = provider.Attachment{
				ID:   fmt.Sprintf("img-%d", i),
				Name: fmt.Sprintf("image-%d", i),
				URL:  url,
			}
		}
	}

	return wu, nil
}

// Snapshot captures the issue content for storage
func (p *Provider) Snapshot(ctx context.Context, id string) (*provider.Snapshot, error) {
	ref, err := ParseReference(id)
	if err != nil {
		return nil, err
	}

	owner := ref.Owner
	repo := ref.Repo
	if owner == "" {
		owner = p.owner
	}
	if repo == "" {
		repo = p.repo
	}
	if owner == "" || repo == "" {
		return nil, ErrRepoNotConfigured
	}

	p.client.SetOwnerRepo(owner, repo)

	// Fetch main issue
	issue, err := p.client.GetIssue(ctx, ref.IssueNumber)
	if err != nil {
		return nil, err
	}

	snapshot := &provider.Snapshot{
		Type: ProviderName,
		Ref:  id,
		Files: []provider.SnapshotFile{
			{
				Path:    "issue.md",
				Content: formatIssueMarkdown(issue),
			},
		},
	}

	// Fetch and include comments
	comments, err := p.client.GetIssueComments(ctx, ref.IssueNumber)
	if err == nil && len(comments) > 0 {
		snapshot.Files = append(snapshot.Files, provider.SnapshotFile{
			Path:    "comments.md",
			Content: formatCommentsMarkdown(comments),
		})
	}

	// Fetch linked issues
	linkedNums := ExtractLinkedIssues(issue.GetBody())
	for _, num := range linkedNums {
		if num == ref.IssueNumber {
			continue // Skip self-reference
		}
		linked, err := p.client.GetIssue(ctx, num)
		if err != nil {
			continue // Skip issues we can't fetch
		}
		snapshot.Files = append(snapshot.Files, provider.SnapshotFile{
			Path:    fmt.Sprintf("linked/issue-%d.md", num),
			Content: formatIssueMarkdown(linked),
		})
	}

	return snapshot, nil
}

// GetConfig returns the provider configuration
func (p *Provider) GetConfig() *Config {
	return p.config
}

// GetClient returns the GitHub API client
func (p *Provider) GetClient() *Client {
	return p.client
}

// SetCache sets or replaces the cache for this provider and its client
func (p *Provider) SetCache(c *cache.Cache) {
	p.cache = c
	p.client.SetCache(c)
}

// GetCache returns the cache for this provider
func (p *Provider) GetCache() *cache.Cache {
	return p.cache
}

// --- Helper functions ---

func mapGitHubState(state string) provider.Status {
	switch state {
	case "open":
		return provider.StatusOpen
	case "closed":
		return provider.StatusClosed
	default:
		return provider.StatusOpen
	}
}

// labelTypeMap maps GitHub labels to task types
var labelTypeMap = map[string]string{
	"bug":           "fix",
	"bugfix":        "fix",
	"fix":           "fix",
	"feature":       "feature",
	"enhancement":   "feature",
	"docs":          "docs",
	"documentation": "docs",
	"refactor":      "refactor",
	"chore":         "chore",
	"test":          "test",
	"ci":            "ci",
}

func inferTypeFromLabels(labels []*gh.Label) string {
	for _, label := range labels {
		name := strings.ToLower(label.GetName())
		if t, ok := labelTypeMap[name]; ok {
			return t
		}
	}
	return "issue"
}

// labelPriorityMap maps GitHub labels to priorities
var labelPriorityMap = map[string]provider.Priority{
	"critical":      provider.PriorityCritical,
	"urgent":        provider.PriorityCritical,
	"priority:high": provider.PriorityHigh,
	"high-priority": provider.PriorityHigh,
	"priority:low":  provider.PriorityLow,
	"low-priority":  provider.PriorityLow,
}

func inferPriorityFromLabels(labels []*gh.Label) provider.Priority {
	for _, label := range labels {
		name := strings.ToLower(label.GetName())
		if p, ok := labelPriorityMap[name]; ok {
			return p
		}
	}
	return provider.PriorityNormal
}

func extractLabelNames(labels []*gh.Label) []string {
	names := make([]string, len(labels))
	for i, label := range labels {
		names[i] = label.GetName()
	}
	return names
}

func mapAssignees(assignees []*gh.User) []provider.Person {
	persons := make([]provider.Person, len(assignees))
	for i, u := range assignees {
		persons[i] = provider.Person{
			ID:    fmt.Sprintf("%d", u.GetID()),
			Name:  u.GetLogin(),
			Email: u.GetEmail(),
		}
	}
	return persons
}

func mapComments(comments []*gh.IssueComment) []provider.Comment {
	result := make([]provider.Comment, len(comments))
	for i, c := range comments {
		result[i] = provider.Comment{
			ID:        fmt.Sprintf("%d", c.GetID()),
			Body:      c.GetBody(),
			CreatedAt: c.GetCreatedAt().Time,
			Author: provider.Person{
				ID:   fmt.Sprintf("%d", c.GetUser().GetID()),
				Name: c.GetUser().GetLogin(),
			},
		}
	}
	return result
}

func formatIssueMarkdown(issue *gh.Issue) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# #%d: %s\n\n", issue.GetNumber(), issue.GetTitle()))

	// Metadata
	sb.WriteString("## Metadata\n\n")
	sb.WriteString(fmt.Sprintf("- **State:** %s\n", issue.GetState()))
	sb.WriteString(fmt.Sprintf("- **Created:** %s\n", issue.GetCreatedAt().Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("- **Updated:** %s\n", issue.GetUpdatedAt().Format(time.RFC3339)))

	if issue.GetUser() != nil {
		sb.WriteString(fmt.Sprintf("- **Author:** @%s\n", issue.GetUser().GetLogin()))
	}

	if len(issue.Labels) > 0 {
		labels := make([]string, len(issue.Labels))
		for i, l := range issue.Labels {
			labels[i] = l.GetName()
		}
		sb.WriteString(fmt.Sprintf("- **Labels:** %s\n", strings.Join(labels, ", ")))
	}

	if len(issue.Assignees) > 0 {
		assignees := make([]string, len(issue.Assignees))
		for i, a := range issue.Assignees {
			assignees[i] = "@" + a.GetLogin()
		}
		sb.WriteString(fmt.Sprintf("- **Assignees:** %s\n", strings.Join(assignees, ", ")))
	}

	sb.WriteString(fmt.Sprintf("- **URL:** %s\n", issue.GetHTMLURL()))

	// Body
	sb.WriteString("\n## Description\n\n")
	sb.WriteString(issue.GetBody())
	sb.WriteString("\n")

	return sb.String()
}

func formatCommentsMarkdown(comments []*gh.IssueComment) string {
	var sb strings.Builder

	sb.WriteString("# Comments\n\n")

	for _, c := range comments {
		sb.WriteString(fmt.Sprintf("## Comment by @%s\n\n", c.GetUser().GetLogin()))
		sb.WriteString(fmt.Sprintf("*%s*\n\n", c.GetCreatedAt().Format(time.RFC3339)))
		sb.WriteString(c.GetBody())
		sb.WriteString("\n\n---\n\n")
	}

	return sb.String()
}

package linear

import (
	"context"
	"strings"
	"time"

	"github.com/valksor/go-mehrhof/internal/naming"
	"github.com/valksor/go-mehrhof/internal/provider"
)

// ProviderName is the registered name for this provider.
const ProviderName = "linear"

// Provider handles Linear issue tasks.
type Provider struct {
	client *Client
	team   string // Default team key
}

// Config holds Linear provider configuration.
type Config struct {
	Token string
	Team  string // Default team key for operations
}

// Info returns provider metadata.
func Info() provider.ProviderInfo {
	return provider.ProviderInfo{
		Name:        ProviderName,
		Description: "Linear issue source",
		Schemes:     []string{"linear", "ln"},
		Priority:    20, // Same as GitHub and Wrike
		Capabilities: provider.CapabilitySet{
			provider.CapRead:           true,
			provider.CapList:           true,
			provider.CapFetchComments:  true,
			provider.CapComment:        true,
			provider.CapUpdateStatus:   true,
			provider.CapManageLabels:   true,
			provider.CapCreateWorkUnit: true,
			provider.CapSnapshot:       true,
			provider.CapFetchSubtasks:  true,
		},
	}
}

// New creates a Linear provider.
func New(_ context.Context, cfg provider.Config) (any, error) {
	token := cfg.GetString("token")
	team := cfg.GetString("team")

	// Try to resolve token from env if not provided
	if token == "" {
		resolvedToken, err := ResolveToken("")
		if err != nil {
			return nil, err
		}
		token = resolvedToken
	}

	return &Provider{
		client: NewClient(token),
		team:   team,
	}, nil
}

// Match checks if input has the linear: or ln: scheme prefix.
func (p *Provider) Match(input string) bool {
	return strings.HasPrefix(input, "linear:") || strings.HasPrefix(input, "ln:")
}

// Parse extracts the issue reference from input.
func (p *Provider) Parse(input string) (string, error) {
	ref, err := ParseReference(input)
	if err != nil {
		return "", err
	}

	return ref.IssueID, nil
}

// Fetch reads a Linear issue and creates a WorkUnit.
func (p *Provider) Fetch(ctx context.Context, id string) (*provider.WorkUnit, error) {
	ref, err := ParseReference(id)
	if err != nil {
		return nil, err
	}

	// Fetch issue from Linear
	issue, err := p.client.GetIssue(ctx, ref.IssueID)
	if err != nil {
		return nil, err
	}

	// Map to WorkUnit
	wu := &provider.WorkUnit{
		ID:          issue.ID,
		ExternalID:  issue.Identifier,
		Provider:    ProviderName,
		Title:       issue.Title,
		Description: issue.Description,
		Status:      mapLinearStatus(issue.State),
		Priority:    mapLinearPriority(issue.Priority),
		Labels:      extractLabelNames(issue.Labels),
		Assignees:   mapAssignees(issue.Assignee),
		CreatedAt:   issue.CreatedAt,
		UpdatedAt:   issue.UpdatedAt,
		Source: provider.SourceInfo{
			Type:      ProviderName,
			Reference: issue.Identifier,
			SyncedAt:  time.Now(),
		},
		// Naming fields for branch/commit customization
		ExternalKey: issue.Identifier,
		TaskType:    "issue",
		Slug:        naming.Slugify(issue.Title, 50),
		Metadata:    buildMetadata(issue),
	}

	// Fetch comments if available
	comments, err := p.client.GetComments(ctx, issue.ID)
	if err == nil && len(comments) > 0 {
		wu.Comments = mapComments(comments)
	}

	return wu, nil
}

// GetConfig returns the provider configuration.
func (p *Provider) GetConfig() *Config {
	return &Config{
		Team: p.team,
	}
}

// GetClient returns the Linear API client.
func (p *Provider) GetClient() *Client {
	return p.client
}

// GetDefaultTeam returns the default team key.
func (p *Provider) GetDefaultTeam() string {
	return p.team
}

// ──────────────────────────────────────────────────────────────────────────────
// Helper functions
// ──────────────────────────────────────────────────────────────────────────────

// mapLinearStatus converts Linear state to provider status.
func mapLinearStatus(state *State) provider.Status {
	if state == nil {
		return provider.StatusOpen
	}

	switch strings.ToLower(state.Name) {
	case "backlog", "todo", "unstarted":
		return provider.StatusOpen
	case "in progress", "started", "in review":
		return provider.StatusInProgress
	case "done", "completed", "closed":
		return provider.StatusDone
	case "canceled", "cancelled":
		return provider.StatusClosed
	default:
		// Check state type as fallback
		switch strings.ToLower(state.Type) {
		case "backlog", "unstarted":
			return provider.StatusOpen
		case "started", "in_progress":
			return provider.StatusInProgress
		case "completed", "canceled":
			return provider.StatusDone
		default:
			return provider.StatusOpen
		}
	}
}

// mapLinearPriority converts Linear priority to provider priority
// Linear priority: 0 = No priority, 1 = Urgent, 2 = High, 3 = Medium, 4 = Low.
func mapLinearPriority(priority int) provider.Priority {
	switch priority {
	case 1: // Urgent
		return provider.PriorityCritical
	case 2: // High
		return provider.PriorityHigh
	case 4: // Low
		return provider.PriorityLow
	case 0, 3: // No priority, Medium
		return provider.PriorityNormal
	default:
		return provider.PriorityNormal
	}
}

// mapProviderPriorityToLinear converts provider priority to Linear priority.
func mapProviderPriorityToLinear(priority provider.Priority) *int {
	var p int
	switch priority {
	case provider.PriorityCritical:
		p = 1 // Urgent
	case provider.PriorityHigh:
		p = 2 // High
	case provider.PriorityNormal:
		p = 3 // Medium
	case provider.PriorityLow:
		p = 4 // Low
	}

	return &p
}

// mapProviderStatusToLinearStateName converts provider status to Linear state name.
func mapProviderStatusToLinearStateName(status provider.Status) string {
	switch status {
	case provider.StatusOpen:
		return "Todo"
	case provider.StatusInProgress:
		return "In Progress"
	case provider.StatusReview:
		return "In Review"
	case provider.StatusDone:
		return "Done"
	case provider.StatusClosed:
		return "Canceled"
	default:
		return "Todo"
	}
}

// extractLabelNames extracts label names from Linear labels.
func extractLabelNames(labels []*Label) []string {
	if labels == nil {
		return []string{}
	}
	names := make([]string, len(labels))
	for i, label := range labels {
		names[i] = label.Name
	}

	return names
}

// mapAssignees converts Linear assignee to provider Person.
func mapAssignees(assignee *User) []provider.Person {
	if assignee == nil {
		return []provider.Person{}
	}

	return []provider.Person{
		{
			ID:    assignee.ID,
			Name:  assignee.Name,
			Email: assignee.Email,
		},
	}
}

// mapComments converts Linear comments to provider comments.
func mapComments(comments []*Comment) []provider.Comment {
	if comments == nil {
		return nil
	}

	result := make([]provider.Comment, 0, len(comments))
	for _, c := range comments {
		var author provider.Person
		if c.User != nil {
			author = provider.Person{
				ID:   c.User.ID,
				Name: c.User.Name,
			}
		}
		result = append(result, provider.Comment{
			ID:        c.ID,
			Body:      c.Body,
			CreatedAt: c.CreatedAt,
			UpdatedAt: c.UpdatedAt,
			Author:    author,
		})
	}

	return result
}

// buildMetadata creates metadata map from issue.
func buildMetadata(issue *Issue) map[string]any {
	metadata := make(map[string]any)

	metadata["url"] = issue.URL
	metadata["state_id"] = issue.State.ID
	metadata["state_name"] = issue.State.Name
	metadata["state_type"] = issue.State.Type
	metadata["identifier"] = issue.Identifier

	if issue.Team != nil {
		metadata["team_key"] = issue.Team.Key
		metadata["team_name"] = issue.Team.Name
	}

	return metadata
}

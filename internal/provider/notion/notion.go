package notion

import (
	"context"
	"strings"
	"time"

	"github.com/valksor/go-mehrhof/internal/naming"
	"github.com/valksor/go-mehrhof/internal/provider"
)

// ProviderName is the registered name for this provider
const ProviderName = "notion"

// Provider handles Notion pages as tasks
type Provider struct {
	client              *Client
	databaseID          string
	statusProperty      string
	descriptionProperty string
	labelsProperty      string
}

// Config holds Notion provider configuration
type Config struct {
	Token               string
	DatabaseID          string
	StatusProperty      string
	DescriptionProperty string
	LabelsProperty      string
}

// Info returns provider metadata
func Info() provider.ProviderInfo {
	return provider.ProviderInfo{
		Name:        ProviderName,
		Description: "Notion page and database source",
		Schemes:     []string{"notion", "nt"},
		Priority:    20, // Same as GitHub, Wrike, Linear
		Capabilities: provider.CapabilitySet{
			provider.CapRead:           true,
			provider.CapList:           true,
			provider.CapFetchComments:  true,
			provider.CapComment:        true,
			provider.CapUpdateStatus:   true,
			provider.CapManageLabels:   true,
			provider.CapCreateWorkUnit: true,
			provider.CapSnapshot:       true,
		},
	}
}

// New creates a Notion provider
func New(_ context.Context, cfg provider.Config) (any, error) {
	token := cfg.GetString("token")
	databaseID := cfg.GetString("database_id")
	statusProperty := cfg.GetString("status_property")
	descriptionProperty := cfg.GetString("description_property")
	labelsProperty := cfg.GetString("labels_property")

	// Set defaults for property names
	if statusProperty == "" {
		statusProperty = "Status"
	}
	if descriptionProperty == "" {
		descriptionProperty = "Description"
	}
	if labelsProperty == "" {
		labelsProperty = "Tags"
	}

	// Try to resolve token from env if not provided
	if token == "" {
		resolvedToken, err := ResolveToken("")
		if err != nil {
			return nil, err
		}
		token = resolvedToken
	}

	return &Provider{
		client:              NewClient(token),
		databaseID:          databaseID,
		statusProperty:      statusProperty,
		descriptionProperty: descriptionProperty,
		labelsProperty:      labelsProperty,
	}, nil
}

// Match checks if input has the notion: or nt: scheme prefix
func (p *Provider) Match(input string) bool {
	return strings.HasPrefix(input, "notion:") || strings.HasPrefix(input, "nt:")
}

// Parse extracts the page reference from input
func (p *Provider) Parse(input string) (string, error) {
	ref, err := ParseReference(input)
	if err != nil {
		return "", err
	}
	return ref.PageID, nil
}

// Fetch reads a Notion page and creates a WorkUnit
func (p *Provider) Fetch(ctx context.Context, id string) (*provider.WorkUnit, error) {
	ref, err := ParseReference(id)
	if err != nil {
		return nil, err
	}

	// Fetch page from Notion
	page, err := p.client.GetPage(ctx, ref.PageID)
	if err != nil {
		return nil, err
	}

	// Fetch page content (blocks)
	blocks, err := p.client.GetPageContent(ctx, ref.PageID)
	if err != nil {
		blocks = []Block{} // Continue without content
	}

	// Map to WorkUnit
	wu := &provider.WorkUnit{
		ID:          page.ID,
		ExternalID:  page.ID,
		Provider:    ProviderName,
		Title:       extractTitle(*page),
		Description: extractDescription(*page, blocks, p.descriptionProperty),
		Status:      extractStatus(*page, p.statusProperty),
		Priority:    provider.PriorityNormal, // Notion doesn't have built-in priority
		Labels:      extractLabelsFromPage(*page, p.labelsProperty),
		Assignees:   extractAssignees(*page),
		CreatedAt:   page.CreatedTime,
		UpdatedAt:   page.LastEditedTime,
		Source: provider.SourceInfo{
			Type:      ProviderName,
			Reference: ref.String(),
			SyncedAt:  time.Now(),
		},
		// Naming fields for branch/commit customization
		ExternalKey: page.ID[:8], // Use first 8 chars of UUID
		TaskType:    "page",
		Slug:        naming.Slugify(extractTitle(*page), 50),
		Metadata:    buildMetadata(*page, ref),
	}

	return wu, nil
}

// GetClient returns the Notion API client
func (p *Provider) GetClient() *Client {
	return p.client
}

// GetDatabaseID returns the default database ID
func (p *Provider) GetDatabaseID() string {
	return p.databaseID
}

// GetStatusProperty returns the configured status property name
func (p *Provider) GetStatusProperty() string {
	return p.statusProperty
}

// GetLabelsProperty returns the configured labels property name
func (p *Provider) GetLabelsProperty() string {
	return p.labelsProperty
}

// ──────────────────────────────────────────────────────────────────────────────
// Helper functions
// ──────────────────────────────────────────────────────────────────────────────

// extractTitle extracts the title from a page
func extractTitle(page Page) string {
	// Look for the title property (usually the first property or named "Name" or "Title")
	for key, prop := range page.Properties {
		if prop.Type == "title" || strings.EqualFold(key, "Name") || strings.EqualFold(key, "Title") {
			return ExtractPlainText(prop)
		}
	}
	// Fallback: use URL as title
	if page.URL != "" {
		return page.URL
	}
	return "Untitled"
}

// extractDescription extracts the description from a page
// First tries the configured description property, then falls back to page content
func extractDescription(page Page, blocks []Block, descriptionProperty string) string {
	// Try the configured description property first
	if prop, ok := GetProperty(page, descriptionProperty); ok {
		if text := ExtractPlainText(prop); text != "" {
			return text
		}
	}

	// Fall back to page block content
	if len(blocks) > 0 {
		return BlocksToMarkdown(blocks)
	}

	return ""
}

// extractStatus extracts the status from a page
func extractStatus(page Page, statusProperty string) provider.Status {
	if prop, ok := GetProperty(page, statusProperty); ok {
		if prop.Status != nil {
			return mapNotionStatus(prop.Status.Name)
		}
		if prop.Select != nil {
			return mapNotionStatus(prop.Select.Name)
		}
	}
	return provider.StatusOpen
}

// mapNotionStatus converts Notion status to provider status
func mapNotionStatus(status string) provider.Status {
	switch strings.ToLower(status) {
	case "not started", "backlog", "todo", "to do":
		return provider.StatusOpen
	case "in progress", "started", "doing":
		return provider.StatusInProgress
	case "in review", "review", "reviewing":
		return provider.StatusReview
	case "done", "completed", "finished", "closed":
		return provider.StatusDone
	case "cancelled", "canceled", "archived":
		return provider.StatusClosed
	default:
		return provider.StatusOpen
	}
}

// mapProviderStatusToNotion converts provider status to Notion status name
func mapProviderStatusToNotion(status provider.Status) string {
	switch status {
	case provider.StatusOpen:
		return "Not Started"
	case provider.StatusInProgress:
		return "In Progress"
	case provider.StatusReview:
		return "In Review"
	case provider.StatusDone:
		return "Done"
	case provider.StatusClosed:
		return "Cancelled"
	default:
		return "Not Started"
	}
}

// extractLabelsFromPage extracts labels from a page's multi-select property
func extractLabelsFromPage(page Page, labelsProperty string) []string {
	if prop, ok := GetProperty(page, labelsProperty); ok {
		return ExtractLabels(prop)
	}
	return []string{}
}

// extractAssignees extracts assignees from a page
func extractAssignees(page Page) []provider.Person {
	// Look for a people property (commonly named "Assignee" or "Owner")
	for key, prop := range page.Properties {
		if (prop.Type == "people" || strings.EqualFold(key, "Assignee") || strings.EqualFold(key, "Owner")) && prop.People != nil {
			assignees := make([]provider.Person, len(prop.People.People))
			for i, user := range prop.People.People {
				assignees[i] = provider.Person{
					ID:    user.ID,
					Name:  user.Name,
					Email: getEmail(user),
				}
			}
			return assignees
		}
	}
	return []provider.Person{}
}

// getEmail extracts email from a user
func getEmail(user User) string {
	if user.Person != nil {
		return user.Person.Email
	}
	return ""
}

// buildMetadata creates metadata map from page
func buildMetadata(page Page, ref *Ref) map[string]any {
	metadata := make(map[string]any)

	metadata["url"] = page.URL
	metadata["created_time"] = page.CreatedTime.Format(time.RFC3339)
	metadata["last_edited_time"] = page.LastEditedTime.Format(time.RFC3339)
	metadata["archived"] = page.Archived

	if ref != nil && ref.URL != "" {
		metadata["source_url"] = ref.URL
	}

	if page.Parent.Type == "database_id" {
		metadata["database_id"] = page.Parent.DatabaseID
	}

	return metadata
}

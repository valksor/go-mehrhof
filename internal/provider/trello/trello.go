package trello

import (
	"context"
	"strings"
	"time"

	"github.com/valksor/go-mehrhof/internal/naming"
	"github.com/valksor/go-mehrhof/internal/provider"
)

// ProviderName is the registered name for this provider
const ProviderName = "trello"

// Provider handles Trello cards as tasks
type Provider struct {
	client  *Client
	boardID string // Default board ID
}

// Config holds Trello provider configuration
type Config struct {
	APIKey  string
	Token   string
	BoardID string
}

// Info returns provider metadata
func Info() provider.ProviderInfo {
	return provider.ProviderInfo{
		Name:        ProviderName,
		Description: "Trello card source",
		Schemes:     []string{"trello", "tr"},
		Priority:    20,
		Capabilities: provider.CapabilitySet{
			provider.CapRead:               true,
			provider.CapList:               true,
			provider.CapFetchComments:      true,
			provider.CapComment:            true,
			provider.CapUpdateStatus:       true,
			provider.CapManageLabels:       true,
			provider.CapDownloadAttachment: true,
			provider.CapSnapshot:           true,
		},
	}
}

// New creates a Trello provider
func New(_ context.Context, cfg provider.Config) (any, error) {
	apiKey := cfg.GetString("api_key")
	token := cfg.GetString("token")
	boardID := cfg.GetString("board")

	// Try to resolve credentials from env if not provided
	if apiKey == "" {
		apiKey = ResolveAPIKey("")
	}
	if token == "" {
		token = ResolveToken("")
	}

	return &Provider{
		client:  NewClient(apiKey, token),
		boardID: boardID,
	}, nil
}

// Match checks if input has the trello: or tr: scheme prefix
func (p *Provider) Match(input string) bool {
	return strings.HasPrefix(input, "trello:") || strings.HasPrefix(input, "tr:")
}

// Parse extracts the card reference from input
func (p *Provider) Parse(input string) (string, error) {
	ref, err := ParseReference(input)
	if err != nil {
		return "", err
	}
	return ref.CardID, nil
}

// Fetch reads a Trello card and creates a WorkUnit
func (p *Provider) Fetch(ctx context.Context, id string) (*provider.WorkUnit, error) {
	ref, err := ParseReference(id)
	if err != nil {
		return nil, err
	}

	// Fetch card from Trello
	card, err := p.client.GetCard(ctx, ref.CardID)
	if err != nil {
		return nil, err
	}

	// Get the list to determine status
	list, err := p.client.GetList(ctx, card.IDList)
	if err != nil {
		// Continue without list info
		list = &List{Name: "Unknown"}
	}

	// Map to WorkUnit
	wu := &provider.WorkUnit{
		ID:          card.ID,
		ExternalID:  card.ID,
		Provider:    ProviderName,
		Title:       card.Name,
		Description: card.Desc,
		Status:      mapTrelloListToStatus(list.Name),
		Priority:    provider.PriorityNormal, // Trello doesn't have built-in priority
		Labels:      extractLabels(card),
		Assignees:   extractMembers(card),
		Attachments: extractAttachments(card),
		CreatedAt:   extractCreatedAt(card.ID),
		UpdatedAt:   card.DateLastActivity,
		Source: provider.SourceInfo{
			Type:      ProviderName,
			Reference: ref.String(),
			SyncedAt:  time.Now(),
		},
		// Naming fields for branch/commit customization
		ExternalKey: card.ShortLink,
		TaskType:    "card",
		Slug:        naming.Slugify(card.Name, 50),
		Metadata:    buildMetadata(card, list, ref),
	}

	return wu, nil
}

// List returns cards from a board
func (p *Provider) List(ctx context.Context, opts provider.ListOptions) ([]*provider.WorkUnit, error) {
	boardID := p.boardID
	if boardID == "" {
		return nil, ErrNoBoardConfigured
	}

	cards, err := p.client.GetBoardCards(ctx, boardID)
	if err != nil {
		return nil, err
	}

	// Cache lists for status mapping
	lists, err := p.client.GetBoardLists(ctx, boardID)
	if err != nil {
		lists = []List{}
	}
	listMap := make(map[string]string)
	for _, l := range lists {
		listMap[l.ID] = l.Name
	}

	result := make([]*provider.WorkUnit, 0, len(cards))
	for _, card := range cards {
		listName := listMap[card.IDList]
		status := mapTrelloListToStatus(listName)

		// Filter by status if specified
		if opts.Status != "" && status != opts.Status {
			continue
		}

		// Filter by labels if specified
		if len(opts.Labels) > 0 && !hasAnyLabel(card, opts.Labels) {
			continue
		}

		wu := &provider.WorkUnit{
			ID:          card.ID,
			ExternalID:  card.ID,
			Provider:    ProviderName,
			Title:       card.Name,
			Description: card.Desc,
			Status:      status,
			Priority:    provider.PriorityNormal,
			Labels:      extractLabels(&card),
			CreatedAt:   extractCreatedAt(card.ID),
			UpdatedAt:   card.DateLastActivity,
			Source: provider.SourceInfo{
				Type:      ProviderName,
				Reference: "trello:" + card.ID,
				SyncedAt:  time.Now(),
			},
			ExternalKey: card.ShortLink,
			TaskType:    "card",
			Slug:        naming.Slugify(card.Name, 50),
		}
		result = append(result, wu)

		// Apply limit
		if opts.Limit > 0 && len(result) >= opts.Limit {
			break
		}
	}

	return result, nil
}

// FetchComments retrieves comments on a card
func (p *Provider) FetchComments(ctx context.Context, workUnitID string) ([]provider.Comment, error) {
	actions, err := p.client.GetCardActions(ctx, workUnitID, "commentCard")
	if err != nil {
		return nil, err
	}

	comments := make([]provider.Comment, len(actions))
	for i, action := range actions {
		comments[i] = provider.Comment{
			ID:        action.ID,
			Body:      action.Data.Text,
			Author:    provider.Person{ID: action.MemberCreator.ID, Name: action.MemberCreator.FullName},
			CreatedAt: action.Date,
		}
	}

	return comments, nil
}

// AddComment adds a comment to a card
func (p *Provider) AddComment(ctx context.Context, workUnitID string, body string) (*provider.Comment, error) {
	action, err := p.client.AddComment(ctx, workUnitID, body)
	if err != nil {
		return nil, err
	}

	return &provider.Comment{
		ID:        action.ID,
		Body:      action.Data.Text,
		Author:    provider.Person{ID: action.MemberCreator.ID, Name: action.MemberCreator.FullName},
		CreatedAt: action.Date,
	}, nil
}

// UpdateStatus moves a card to a different list
func (p *Provider) UpdateStatus(ctx context.Context, workUnitID string, status provider.Status) error {
	// Get the board ID from the card
	card, err := p.client.GetCard(ctx, workUnitID)
	if err != nil {
		return err
	}

	// Find the target list based on status
	listName := mapProviderStatusToListName(status)
	targetList, err := p.client.FindListByName(ctx, card.IDBoard, listName)
	if err != nil {
		return err
	}

	// Move the card
	return p.client.MoveCard(ctx, workUnitID, targetList.ID)
}

// AddLabels adds labels to a card
func (p *Provider) AddLabels(ctx context.Context, workUnitID string, labels []string) error {
	for _, label := range labels {
		if err := p.client.AddLabel(ctx, workUnitID, label); err != nil {
			return err
		}
	}
	return nil
}

// RemoveLabels removes labels from a card
func (p *Provider) RemoveLabels(ctx context.Context, workUnitID string, labels []string) error {
	for _, label := range labels {
		if err := p.client.RemoveLabel(ctx, workUnitID, label); err != nil {
			return err
		}
	}
	return nil
}

// DownloadAttachment downloads an attachment from a card
func (p *Provider) DownloadAttachment(ctx context.Context, workUnitID, attachmentID string) (any, error) {
	return p.client.DownloadAttachment(ctx, workUnitID, attachmentID)
}

// Snapshot captures the card content for storage
func (p *Provider) Snapshot(ctx context.Context, id string) (*provider.Snapshot, error) {
	card, err := p.client.GetCard(ctx, id)
	if err != nil {
		return nil, err
	}

	return &provider.Snapshot{
		Type:    "card",
		Ref:     "trello:" + id,
		Content: buildSnapshotContent(card),
	}, nil
}

// ──────────────────────────────────────────────────────────────────────────────
// Helper functions
// ──────────────────────────────────────────────────────────────────────────────

// mapTrelloListToStatus converts Trello list name to provider status
func mapTrelloListToStatus(listName string) provider.Status {
	switch strings.ToLower(listName) {
	case "to do", "todo", "backlog", "open", "new":
		return provider.StatusOpen
	case "in progress", "doing", "wip", "started", "working":
		return provider.StatusInProgress
	case "in review", "review", "reviewing", "pending review":
		return provider.StatusReview
	case "done", "completed", "finished", "closed", "complete":
		return provider.StatusDone
	case "cancelled", "canceled", "archived":
		return provider.StatusClosed
	default:
		return provider.StatusOpen
	}
}

// mapProviderStatusToListName converts provider status to Trello list name
func mapProviderStatusToListName(status provider.Status) string {
	switch status {
	case provider.StatusOpen:
		return "To Do"
	case provider.StatusInProgress:
		return "Doing"
	case provider.StatusReview:
		return "In Review"
	case provider.StatusDone:
		return "Done"
	case provider.StatusClosed:
		return "Done"
	default:
		return "To Do"
	}
}

// extractLabels extracts label names from a card
func extractLabels(card *Card) []string {
	labels := make([]string, len(card.Labels))
	for i, label := range card.Labels {
		if label.Name != "" {
			labels[i] = label.Name
		} else {
			// Use color as fallback
			labels[i] = label.Color
		}
	}
	return labels
}

// extractMembers extracts members as assignees
func extractMembers(card *Card) []provider.Person {
	members := make([]provider.Person, len(card.Members))
	for i, member := range card.Members {
		members[i] = provider.Person{
			ID:   member.ID,
			Name: member.FullName,
		}
	}
	return members
}

// extractAttachments converts Trello attachments to provider attachments
func extractAttachments(card *Card) []provider.Attachment {
	attachments := make([]provider.Attachment, len(card.Attachments))
	for i, att := range card.Attachments {
		attachments[i] = provider.Attachment{
			ID:          att.ID,
			Name:        att.Name,
			URL:         att.URL,
			ContentType: att.MimeType,
			Size:        att.Bytes,
			CreatedAt:   att.Date,
		}
	}
	return attachments
}

// extractCreatedAt extracts creation time from card ID (first 8 chars are hex timestamp)
func extractCreatedAt(cardID string) time.Time {
	if len(cardID) < 8 {
		return time.Time{}
	}
	// Trello card IDs start with 8 hex chars representing timestamp
	var timestamp int64
	_, err := parseHexTimestamp(cardID[:8], &timestamp)
	if err != nil {
		return time.Time{}
	}
	return time.Unix(timestamp, 0)
}

// parseHexTimestamp parses the first 8 chars of Trello ID as Unix timestamp
func parseHexTimestamp(hex string, result *int64) (int, error) {
	var n int64
	for _, c := range hex {
		n *= 16
		switch {
		case c >= '0' && c <= '9':
			n += int64(c - '0')
		case c >= 'a' && c <= 'f':
			n += int64(c - 'a' + 10)
		case c >= 'A' && c <= 'F':
			n += int64(c - 'A' + 10)
		default:
			return 0, nil
		}
	}
	*result = n
	return 8, nil
}

// hasAnyLabel checks if a card has any of the specified labels
func hasAnyLabel(card Card, targetLabels []string) bool {
	cardLabels := make(map[string]bool)
	for _, label := range card.Labels {
		cardLabels[strings.ToLower(label.Name)] = true
		cardLabels[strings.ToLower(label.Color)] = true
	}
	for _, target := range targetLabels {
		if cardLabels[strings.ToLower(target)] {
			return true
		}
	}
	return false
}

// buildMetadata creates metadata map from card
func buildMetadata(card *Card, list *List, ref *Ref) map[string]any {
	metadata := make(map[string]any)

	metadata["url"] = card.URL
	metadata["short_url"] = card.ShortURL
	metadata["short_link"] = card.ShortLink
	metadata["board_id"] = card.IDBoard
	metadata["list_id"] = card.IDList
	metadata["list_name"] = list.Name
	metadata["due"] = card.Due
	metadata["due_complete"] = card.DueComplete
	metadata["subscribed"] = card.Subscribed

	if ref != nil && ref.URL != "" {
		metadata["source_url"] = ref.URL
	}

	return metadata
}

// buildSnapshotContent creates markdown content from a card
func buildSnapshotContent(card *Card) string {
	var sb strings.Builder
	sb.WriteString("# ")
	sb.WriteString(card.Name)
	sb.WriteString("\n\n")

	if card.Desc != "" {
		sb.WriteString(card.Desc)
		sb.WriteString("\n")
	}

	if len(card.Labels) > 0 {
		sb.WriteString("\n## Labels\n")
		for _, label := range card.Labels {
			name := label.Name
			if name == "" {
				name = label.Color
			}
			sb.WriteString("- ")
			sb.WriteString(name)
			sb.WriteString("\n")
		}
	}

	if len(card.Checklists) > 0 {
		sb.WriteString("\n## Checklists\n")
		for _, cl := range card.Checklists {
			sb.WriteString("### ")
			sb.WriteString(cl.Name)
			sb.WriteString("\n")
			for _, item := range cl.CheckItems {
				if item.State == "complete" {
					sb.WriteString("- [x] ")
				} else {
					sb.WriteString("- [ ] ")
				}
				sb.WriteString(item.Name)
				sb.WriteString("\n")
			}
		}
	}

	return sb.String()
}

package trello

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/valksor/go-mehrhof/internal/provider/httpclient"
)

const (
	baseURL = "https://api.trello.com/1"
)

// Client is a Trello API client
type Client struct {
	http   *http.Client
	apiKey string
	token  string
}

// NewClient creates a new Trello API client
func NewClient(apiKey, token string) *Client {
	return &Client{
		http:   httpclient.NewHTTPClient(),
		apiKey: apiKey,
		token:  token,
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// API Types
// ──────────────────────────────────────────────────────────────────────────────

// Card represents a Trello card
type Card struct {
	ID               string       `json:"id"`
	Name             string       `json:"name"`
	Desc             string       `json:"desc"`
	IDBoard          string       `json:"idBoard"`
	IDList           string       `json:"idList"`
	Due              *time.Time   `json:"due"`
	DueComplete      bool         `json:"dueComplete"`
	Closed           bool         `json:"closed"`
	URL              string       `json:"url"`
	ShortURL         string       `json:"shortUrl"`
	ShortLink        string       `json:"shortLink"`
	Subscribed       bool         `json:"subscribed"`
	DateLastActivity time.Time    `json:"dateLastActivity"`
	Labels           []Label      `json:"labels"`
	Members          []Member     `json:"members"`
	Attachments      []Attachment `json:"attachments"`
	Checklists       []Checklist  `json:"checklists"`
	IDMembers        []string     `json:"idMembers"`
	IDLabels         []string     `json:"idLabels"`
}

// List represents a Trello list
type List struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	IDBoard  string `json:"idBoard"`
	Closed   bool   `json:"closed"`
	Position int    `json:"pos"`
}

// Board represents a Trello board
type Board struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url"`
}

// Label represents a Trello label
type Label struct {
	ID      string `json:"id"`
	IDBoard string `json:"idBoard"`
	Name    string `json:"name"`
	Color   string `json:"color"`
}

// Member represents a Trello member
type Member struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	FullName string `json:"fullName"`
	Email    string `json:"email,omitempty"`
}

// Attachment represents a Trello attachment
type Attachment struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	URL      string    `json:"url"`
	MimeType string    `json:"mimeType"`
	Bytes    int64     `json:"bytes"`
	Date     time.Time `json:"date"`
}

// Checklist represents a Trello checklist
type Checklist struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	IDCard     string      `json:"idCard"`
	CheckItems []CheckItem `json:"checkItems"`
}

// CheckItem represents an item in a checklist
type CheckItem struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	State string `json:"state"` // complete, incomplete
}

// Action represents a Trello action (e.g., comment)
type Action struct {
	ID            string       `json:"id"`
	Type          string       `json:"type"`
	Date          time.Time    `json:"date"`
	Data          ActionData   `json:"data"`
	MemberCreator ActionMember `json:"memberCreator"`
}

// ActionData contains action-specific data
type ActionData struct {
	Text string `json:"text"`
	Card struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"card"`
}

// ActionMember represents the member who created an action
type ActionMember struct {
	ID       string `json:"id"`
	FullName string `json:"fullName"`
	Username string `json:"username"`
}

// ──────────────────────────────────────────────────────────────────────────────
// API Methods
// ──────────────────────────────────────────────────────────────────────────────

// GetCard fetches a card by ID
func (c *Client) GetCard(ctx context.Context, cardID string) (*Card, error) {
	endpoint := fmt.Sprintf("/cards/%s", cardID)
	params := url.Values{
		"fields":      {"all"},
		"members":     {"true"},
		"attachments": {"true"},
		"checklists":  {"all"},
	}

	var card Card
	if err := c.get(ctx, endpoint, params, &card); err != nil {
		return nil, err
	}
	return &card, nil
}

// GetList fetches a list by ID
func (c *Client) GetList(ctx context.Context, listID string) (*List, error) {
	endpoint := fmt.Sprintf("/lists/%s", listID)

	var list List
	if err := c.get(ctx, endpoint, nil, &list); err != nil {
		return nil, err
	}
	return &list, nil
}

// GetBoardCards fetches all cards from a board
func (c *Client) GetBoardCards(ctx context.Context, boardID string) ([]Card, error) {
	endpoint := fmt.Sprintf("/boards/%s/cards", boardID)
	params := url.Values{
		"fields":  {"all"},
		"members": {"true"},
	}

	var cards []Card
	if err := c.get(ctx, endpoint, params, &cards); err != nil {
		return nil, err
	}
	return cards, nil
}

// GetBoardLists fetches all lists from a board
func (c *Client) GetBoardLists(ctx context.Context, boardID string) ([]List, error) {
	endpoint := fmt.Sprintf("/boards/%s/lists", boardID)

	var lists []List
	if err := c.get(ctx, endpoint, nil, &lists); err != nil {
		return nil, err
	}
	return lists, nil
}

// GetCardActions fetches actions (e.g., comments) for a card
func (c *Client) GetCardActions(ctx context.Context, cardID, filter string) ([]Action, error) {
	endpoint := fmt.Sprintf("/cards/%s/actions", cardID)
	params := url.Values{}
	if filter != "" {
		params.Set("filter", filter)
	}

	var actions []Action
	if err := c.get(ctx, endpoint, params, &actions); err != nil {
		return nil, err
	}
	return actions, nil
}

// AddComment adds a comment to a card
func (c *Client) AddComment(ctx context.Context, cardID, text string) (*Action, error) {
	endpoint := fmt.Sprintf("/cards/%s/actions/comments", cardID)
	params := url.Values{
		"text": {text},
	}

	var action Action
	if err := c.post(ctx, endpoint, params, &action); err != nil {
		return nil, err
	}
	return &action, nil
}

// MoveCard moves a card to a different list
func (c *Client) MoveCard(ctx context.Context, cardID, listID string) error {
	endpoint := fmt.Sprintf("/cards/%s", cardID)
	params := url.Values{
		"idList": {listID},
	}

	return c.put(ctx, endpoint, params, nil)
}

// FindListByName finds a list by name on a board
func (c *Client) FindListByName(ctx context.Context, boardID, name string) (*List, error) {
	lists, err := c.GetBoardLists(ctx, boardID)
	if err != nil {
		return nil, err
	}

	nameLower := strings.ToLower(name)
	for _, list := range lists {
		if strings.ToLower(list.Name) == nameLower {
			return &list, nil
		}
	}

	return nil, fmt.Errorf("list %q not found on board", name)
}

// AddLabel adds a label to a card
func (c *Client) AddLabel(ctx context.Context, cardID, labelID string) error {
	endpoint := fmt.Sprintf("/cards/%s/idLabels", cardID)
	params := url.Values{
		"value": {labelID},
	}

	return c.post(ctx, endpoint, params, nil)
}

// RemoveLabel removes a label from a card
func (c *Client) RemoveLabel(ctx context.Context, cardID, labelID string) error {
	endpoint := fmt.Sprintf("/cards/%s/idLabels/%s", cardID, labelID)
	return c.delete(ctx, endpoint)
}

// CreateCard creates a new card in a list
func (c *Client) CreateCard(ctx context.Context, listID, name, desc string) (*Card, error) {
	endpoint := "/cards"
	params := url.Values{
		"idList": {listID},
		"name":   {name},
	}
	if desc != "" {
		params.Set("desc", desc)
	}

	var card Card
	if err := c.post(ctx, endpoint, params, &card); err != nil {
		return nil, err
	}
	return &card, nil
}

// DownloadAttachment downloads an attachment
func (c *Client) DownloadAttachment(ctx context.Context, cardID, attachmentID string) (io.ReadCloser, error) {
	// First get the attachment details to get the URL
	endpoint := fmt.Sprintf("/cards/%s/attachments/%s", cardID, attachmentID)

	var attachment Attachment
	if err := c.get(ctx, endpoint, nil, &attachment); err != nil {
		return nil, err
	}

	// Download from the URL
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, attachment.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download attachment: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, httpclient.NewHTTPError(resp.StatusCode, "failed to download attachment")
	}

	return resp.Body, nil
}

// ──────────────────────────────────────────────────────────────────────────────
// HTTP Helpers
// ──────────────────────────────────────────────────────────────────────────────

func (c *Client) get(ctx context.Context, endpoint string, params url.Values, result any) error {
	return c.request(ctx, http.MethodGet, endpoint, params, result)
}

func (c *Client) post(ctx context.Context, endpoint string, params url.Values, result any) error {
	return c.request(ctx, http.MethodPost, endpoint, params, result)
}

func (c *Client) put(ctx context.Context, endpoint string, params url.Values, result any) error {
	return c.request(ctx, http.MethodPut, endpoint, params, result)
}

func (c *Client) delete(ctx context.Context, endpoint string) error {
	return c.request(ctx, http.MethodDelete, endpoint, nil, nil)
}

func (c *Client) request(ctx context.Context, method, endpoint string, params url.Values, result any) error {
	if params == nil {
		params = url.Values{}
	}

	// Add authentication
	params.Set("key", c.apiKey)
	params.Set("token", c.token)

	u := baseURL + endpoint
	if len(params) > 0 && method == http.MethodGet {
		u += "?" + params.Encode()
	}

	var body io.Reader
	if method != http.MethodGet && len(params) > 0 {
		body = strings.NewReader(params.Encode())
	}

	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	if method != http.MethodGet && body != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	var resp *http.Response
	err = httpclient.WithRetry(ctx, httpclient.DefaultRetryConfig(), func() error {
		var err error
		resp, err = c.http.Do(req) //nolint:bodyclose // closed after WithRetry
		return err
	})
	if err != nil {
		return fmt.Errorf("execute request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return httpclient.NewHTTPError(resp.StatusCode, string(bodyBytes))
	}

	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}

	return nil
}

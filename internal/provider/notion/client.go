package notion

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/valksor/go-mehrhof/internal/provider/token"
)

const (
	defaultBaseURL = "https://api.notion.com"
	defaultVersion = "2022-06-28"
	defaultTimeout = 30 * time.Second
)

// Client wraps the Notion API client
type Client struct {
	httpClient *http.Client
	baseURL    string
	token      string
	version    string
}

// NewClient creates a new Notion API client
func NewClient(token string) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: defaultTimeout},
		baseURL:    defaultBaseURL,
		token:      token,
		version:    defaultVersion,
	}
}

// ResolveToken finds the Notion token from multiple sources.
// Priority order:
//  1. MEHR_NOTION_TOKEN env var
//  2. NOTION_TOKEN env var
//  3. configToken (from config.yaml)
func ResolveToken(configToken string) (string, error) {
	return token.ResolveToken(token.Config("NOTION", configToken).
		WithEnvVars("NOTION_TOKEN"))
}

// doRequest performs an HTTP request to the Notion API
func (c *Client) doRequest(ctx context.Context, method, path string, body, result any) error {
	var bodyReader io.Reader
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode request: %w", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	reqURL := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, reqURL, bodyReader)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Notion-Version", c.version)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return wrapAPIError(err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return wrapAPIError(&httpError{code: resp.StatusCode, message: string(respBody)})
	}

	if result != nil {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}

	return nil
}

// GetPage fetches a page by ID
func (c *Client) GetPage(ctx context.Context, pageID string) (*Page, error) {
	// Normalize page ID (ensure it's 32-char hex)
	normalizedID := NormalizePageID(pageID)

	var page Page
	path := fmt.Sprintf("/v1/pages/%s", normalizedID)

	if err := c.doRequest(ctx, http.MethodGet, path, nil, &page); err != nil {
		return nil, err
	}

	return &page, nil
}

// GetPageContent fetches the block content of a page
func (c *Client) GetPageContent(ctx context.Context, pageID string) ([]Block, error) {
	normalizedID := NormalizePageID(pageID)

	var blocks struct {
		Object     string  `json:"object"`
		NextCursor string  `json:"next_cursor,omitempty"`
		Results    []Block `json:"results"`
		HasMore    bool    `json:"has_more"`
	}

	path := fmt.Sprintf("/v1/blocks/%s/children", normalizedID)
	allBlocks := []Block{}

	for {
		if err := c.doRequest(ctx, http.MethodGet, path, nil, &blocks); err != nil {
			if len(allBlocks) > 0 {
				break // Return what we have
			}
			return nil, err
		}

		allBlocks = append(allBlocks, blocks.Results...)

		if !blocks.HasMore {
			break
		}

		// Continue pagination
		path = fmt.Sprintf("/v1/blocks/%s/children?start_cursor=%s", normalizedID, blocks.NextCursor)
	}

	return allBlocks, nil
}

// QueryDatabase queries a database with optional filters
func (c *Client) QueryDatabase(ctx context.Context, databaseID string, req *DatabaseQueryRequest) (*DatabaseQueryResponse, error) {
	// Normalize database ID
	normalizedID := NormalizePageID(databaseID)

	var response DatabaseQueryResponse
	path := fmt.Sprintf("/v1/databases/%s/query", normalizedID)

	if req == nil {
		req = &DatabaseQueryRequest{}
	}

	// Set default page size if not specified
	if req.PageSize == 0 {
		req.PageSize = 100
	}

	if err := c.doRequest(ctx, http.MethodPost, path, req, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// QueryDatabaseAll queries a database and returns all pages (handles pagination)
func (c *Client) QueryDatabaseAll(ctx context.Context, databaseID string, req *DatabaseQueryRequest) ([]Page, error) {
	var allPages []Page
	startCursor := ""

	for {
		currentReq := *req
		if startCursor != "" {
			currentReq.StartCursor = startCursor
		}

		response, err := c.QueryDatabase(ctx, databaseID, &currentReq)
		if err != nil {
			if len(allPages) > 0 {
				break // Return what we have
			}
			return nil, err
		}

		allPages = append(allPages, response.Results...)

		if !response.HasMore {
			break
		}

		startCursor = response.NextCursor
	}

	return allPages, nil
}

// UpdatePage updates page properties
func (c *Client) UpdatePage(ctx context.Context, pageID string, input *UpdatePageInput) (*Page, error) {
	normalizedID := NormalizePageID(pageID)

	var page Page
	path := fmt.Sprintf("/v1/pages/%s", normalizedID)

	if err := c.doRequest(ctx, http.MethodPatch, path, input, &page); err != nil {
		return nil, err
	}

	return &page, nil
}

// CreatePage creates a new page in a database
func (c *Client) CreatePage(ctx context.Context, input *CreatePageInput) (*Page, error) {
	var page Page
	path := "/v1/pages"

	if err := c.doRequest(ctx, http.MethodPost, path, input, &page); err != nil {
		return nil, err
	}

	return &page, nil
}

// GetComments fetches comments for a page
func (c *Client) GetComments(ctx context.Context, pageID string) ([]Comment, error) {
	normalizedID := NormalizePageID(pageID)

	var response CommentResponse
	path := fmt.Sprintf("/v1/comments?block_id=%s", normalizedID)
	allComments := []Comment{}

	for {
		if err := c.doRequest(ctx, http.MethodGet, path, nil, &response); err != nil {
			if len(allComments) > 0 {
				break // Return what we have
			}
			return nil, err
		}

		allComments = append(allComments, response.Results...)

		if !response.HasMore {
			break
		}

		// Continue pagination
		path = fmt.Sprintf("/v1/comments?block_id=%s&start_cursor=%s", normalizedID, response.NextCursor)
	}

	return allComments, nil
}

// AddComment adds a comment to a page
func (c *Client) AddComment(ctx context.Context, input *AddCommentInput) (*Comment, error) {
	var comment Comment
	path := "/v1/comments"

	if err := c.doRequest(ctx, http.MethodPost, path, input, &comment); err != nil {
		return nil, err
	}

	return &comment, nil
}

// GetDatabase fetches database metadata
func (c *Client) GetDatabase(ctx context.Context, databaseID string) (*Database, error) {
	normalizedID := NormalizePageID(databaseID)

	var database Database
	path := fmt.Sprintf("/v1/databases/%s", normalizedID)

	if err := c.doRequest(ctx, http.MethodGet, path, nil, &database); err != nil {
		return nil, err
	}

	return &database, nil
}

// ──────────────────────────────────────────────────────────────────────────────
// HTTP Error wrapper
// ──────────────────────────────────────────────────────────────────────────────

// httpError wraps an HTTP error for proper error handling
type httpError struct {
	message string
	code    int
}

func (e *httpError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.code, e.message)
}

func (e *httpError) HTTPStatusCode() int {
	return e.code
}

// Helper function to create a title property
func MakeTitleProperty(text string) Property {
	return Property{
		Type: "title",
		Title: &TitleProp{
			Type: "title",
			Title: []RichText{
				{
					Type: "text",
					Text: &TextContent{
						Content: text,
					},
					PlainText: text,
				},
			},
		},
	}
}

// Helper function to create a rich text property
func MakeRichTextProperty(text string) Property {
	return Property{
		Type: "rich_text",
		RichText: &RichTextProp{
			Type: "rich_text",
			RichText: []RichText{
				{
					Type: "text",
					Text: &TextContent{
						Content: text,
					},
					PlainText: text,
				},
			},
		},
	}
}

// Helper function to create a status property
func MakeStatusProperty(status string) Property {
	if status == "" {
		return Property{
			Type: "status",
			Status: &StatusProp{
				Name: "",
			},
		}
	}
	return Property{
		Type: "status",
		Status: &StatusProp{
			Name: status,
		},
	}
}

// Helper function to create a multi-select property
func MakeMultiSelectProperty(labels []string) Property {
	options := make([]SelectProp, len(labels))
	for i, label := range labels {
		options[i] = SelectProp{
			Name: label,
		}
	}
	return Property{
		Type: "multi_select",
		MultiSelect: &MultiSelectProp{
			Options: options,
		},
	}
}

// Helper function to extract plain text from a property
func ExtractPlainText(prop Property) string {
	switch {
	case prop.Title != nil && len(prop.Title.Title) > 0:
		return prop.Title.Title[0].PlainText
	case prop.RichText != nil && len(prop.RichText.RichText) > 0:
		return prop.RichText.RichText[0].PlainText
	case prop.Select != nil:
		return prop.Select.Name
	case prop.Status != nil:
		return prop.Status.Name
	default:
		return ""
	}
}

// Helper function to extract all labels from a multi-select property
func ExtractLabels(prop Property) []string {
	if prop.MultiSelect == nil {
		return []string{}
	}
	labels := make([]string, len(prop.MultiSelect.Options))
	for i, opt := range prop.MultiSelect.Options {
		labels[i] = opt.Name
	}
	return labels
}

// Convert blocks to markdown
func BlocksToMarkdown(blocks []Block) string {
	var md strings.Builder

	for _, block := range blocks {
		switch block.Type {
		case "paragraph":
			if block.Paragraph != nil {
				for _, rt := range block.Paragraph.RichText {
					md.WriteString(rt.PlainText)
				}
				md.WriteString("\n\n")
			}
		case "heading_1":
			if block.Heading1 != nil {
				md.WriteString("# ")
				for _, rt := range block.Heading1.RichText {
					md.WriteString(rt.PlainText)
				}
				md.WriteString("\n\n")
			}
		case "heading_2":
			if block.Heading2 != nil {
				md.WriteString("## ")
				for _, rt := range block.Heading2.RichText {
					md.WriteString(rt.PlainText)
				}
				md.WriteString("\n\n")
			}
		case "heading_3":
			if block.Heading3 != nil {
				md.WriteString("### ")
				for _, rt := range block.Heading3.RichText {
					md.WriteString(rt.PlainText)
				}
				md.WriteString("\n\n")
			}
		case "bulleted_list_item":
			if block.BulletedListItem != nil {
				md.WriteString("- ")
				for _, rt := range block.BulletedListItem.RichText {
					md.WriteString(rt.PlainText)
				}
				md.WriteString("\n")
			}
		case "numbered_list_item":
			if block.NumberedListItem != nil {
				md.WriteString("1. ")
				for _, rt := range block.NumberedListItem.RichText {
					md.WriteString(rt.PlainText)
				}
				md.WriteString("\n")
			}
		case "to_do":
			if block.ToDo != nil {
				checkbox := "[ ]"
				if block.ToDo.Checked {
					checkbox = "[x]"
				}
				md.WriteString(checkbox)
				md.WriteString(" ")
				for _, rt := range block.ToDo.RichText {
					md.WriteString(rt.PlainText)
				}
				md.WriteString("\n")
			}
		case "code":
			if block.Code != nil {
				md.WriteString("```")
				md.WriteString(block.Code.Language)
				md.WriteString("\n")
				for _, rt := range block.Code.RichText {
					md.WriteString(rt.PlainText)
				}
				md.WriteString("\n```\n\n")
			}
		case "quote":
			if block.Quote != nil {
				md.WriteString("> ")
				for _, rt := range block.Quote.RichText {
					md.WriteString(rt.PlainText)
				}
				md.WriteString("\n\n")
			}
		case "divider":
			md.WriteString("---\n\n")
		case "callout":
			if block.Callout != nil {
				md.WriteString("> ")
				for _, rt := range block.Callout.RichText {
					md.WriteString(rt.PlainText)
				}
				md.WriteString("\n\n")
			}
		}
	}

	return md.String()
}

// Helper to get property by name (case-insensitive)
func GetProperty(page Page, name string) (Property, bool) {
	for key, prop := range page.Properties {
		if strings.EqualFold(key, name) {
			return prop, true
		}
	}
	return Property{}, false
}

// Helper to get property ID by name (case-insensitive)
func GetPropertyID(page Page, name string) (string, bool) {
	for key, prop := range page.Properties {
		if strings.EqualFold(key, name) {
			return prop.ID, true
		}
	}
	return "", false
}

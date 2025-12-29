package jira

import (
	"fmt"
	"regexp"
	"strings"
)

// Ref represents a parsed Jira issue reference
type Ref struct {
	IssueKey string // The issue key (e.g., "JIRA-123")
	ProjectKey string // The project key (e.g., "JIRA")
	Number     int    // The issue number (e.g., 123)
	URL        string // The full URL if provided
	BaseURL    string // The base URL extracted from URL
	IsExplicit bool   // true if explicitly formatted
}

// String returns the canonical string representation
func (r *Ref) String() string {
	if r.URL != "" {
		return r.URL
	}
	if r.IssueKey != "" {
		return r.IssueKey
	}
	if r.ProjectKey != "" {
		return fmt.Sprintf("%s-%d", r.ProjectKey, r.Number)
	}
	return ""
}

var (
	// Matches: https://domain.atlassian.net/browse/JIRA-123
	// Also matches: https://domain.atlassian.net/browse/JIRA-123?foo=bar
	// Also handles Jira Server: https://jira.example.com/browse/PROJ-123
	jiraURLPattern = regexp.MustCompile(`^https?://[^/]+/browse/([A-Z0-9]+-[0-9]+)`)
	// Matches: PROJ-123 format (project key uppercase + dash + number)
	// Project key is typically 2-10 uppercase letters/numbers
	issueKeyPattern = regexp.MustCompile(`^([A-Z0-9]{2,10})-([0-9]+)$`)
)

// ParseReference parses various Jira issue reference formats
// Supported formats:
//   - "jira:JIRA-123"          -> issue key with scheme
//   - "j:JIRA-123"             -> short scheme
//   - "jira:https://domain.atlassian.net/browse/JIRA-123" -> URL with scheme
//   - "https://domain.atlassian.net/browse/JIRA-123" -> URL
//   - "JIRA-123"               -> bare issue key (if default provider)
func ParseReference(input string) (*Ref, error) {
	input = strings.TrimSpace(input)

	if input == "" {
		return nil, fmt.Errorf("%w: empty reference", ErrInvalidReference)
	}

	// Strip scheme prefix if present
	schemeStripped := strings.TrimPrefix(input, "jira:")
	schemeStripped = strings.TrimPrefix(schemeStripped, "j:")

	// Check for Jira URL in both original and scheme-stripped input
	if matches := jiraURLPattern.FindStringSubmatch(input); matches != nil {
		issueKey := matches[1]
		ref, err := parseIssueKey(issueKey)
		if err != nil {
			return nil, err
		}
		ref.URL = input
		ref.BaseURL = extractBaseURL(input)
		ref.IsExplicit = true
		return ref, nil
	}

	// Also check scheme-stripped for URLs
	if matches := jiraURLPattern.FindStringSubmatch(schemeStripped); matches != nil {
		issueKey := matches[1]
		ref, err := parseIssueKey(issueKey)
		if err != nil {
			return nil, err
		}
		ref.URL = schemeStripped
		ref.BaseURL = extractBaseURL(schemeStripped)
		ref.IsExplicit = true
		return ref, nil
	}

	// Use scheme-stripped version for remaining checks
	issueKey := schemeStripped

	// Parse issue key format (PROJ-123)
	ref, err := parseIssueKey(issueKey)
	if err != nil {
		return nil, fmt.Errorf("%w: unrecognized format: %s (expected PROJ-123 or jira URL)", ErrInvalidReference, input)
	}

	return ref, nil
}

// parseIssueKey parses an issue key in PROJ-123 format
func parseIssueKey(issueKey string) (*Ref, error) {
	if matches := issueKeyPattern.FindStringSubmatch(issueKey); matches != nil {
		projectKey := matches[1]
		var number int
		if _, err := fmt.Sscanf(matches[2], "%d", &number); err != nil {
			return nil, fmt.Errorf("%w: invalid issue number: %s", ErrInvalidReference, matches[2])
		}
		return &Ref{
			IssueKey:   issueKey,
			ProjectKey: projectKey,
			Number:     number,
			IsExplicit: false,
		}, nil
	}
	return nil, fmt.Errorf("%w: invalid issue key format: %s (expected PROJ-123)", ErrInvalidReference, issueKey)
}

// ExtractIssueKey extracts the issue key from a Jira URL
// Returns empty string if not a valid URL
func ExtractIssueKey(url string) string {
	if matches := jiraURLPattern.FindStringSubmatch(url); matches != nil {
		return matches[1]
	}
	return ""
}

// extractBaseURL extracts the base URL from a Jira URL
// e.g., "https://domain.atlassian.net/browse/JIRA-123" -> "https://domain.atlassian.net"
func extractBaseURL(jiraURL string) string {
	// Find the /browse/ part
	browseIndex := strings.Index(jiraURL, "/browse/")
	if browseIndex == -1 {
		return ""
	}

	// Also remove any query parameters or fragments
	baseURL := jiraURL[:browseIndex]
	if queryIndex := strings.Index(baseURL, "?"); queryIndex != -1 {
		baseURL = baseURL[:queryIndex]
	}
	if fragIndex := strings.Index(baseURL, "#"); fragIndex != -1 {
		baseURL = baseURL[:fragIndex]
	}

	return baseURL
}

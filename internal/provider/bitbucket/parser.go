package bitbucket

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Reference represents a parsed Bitbucket issue reference.
type Reference struct {
	Workspace  string // Bitbucket workspace (username or team)
	RepoSlug   string // Repository slug
	IssueID    int    // Issue number
	IsExplicit bool   // True if workspace/repo was explicitly provided
}

// String returns a canonical string representation.
func (r *Reference) String() string {
	if r.Workspace != "" && r.RepoSlug != "" {
		return fmt.Sprintf("%s/%s#%d", r.Workspace, r.RepoSlug, r.IssueID)
	}
	return fmt.Sprintf("%d", r.IssueID)
}

// Patterns for parsing Bitbucket references.
var (
	// Matches: workspace/repo#123.
	explicitRepoPattern = regexp.MustCompile(`^([a-zA-Z0-9_-]+)/([a-zA-Z0-9_.-]+)#(\d+)$`)

	// Matches: #123 or 123.
	simpleIssuePattern = regexp.MustCompile(`^#?(\d+)$`)

	// Matches: https://bitbucket.org/workspace/repo/issues/123 or bitbucket.org/workspace/repo/issues/123.
	urlPattern = regexp.MustCompile(`(?:https?://)?bitbucket\.org/([a-zA-Z0-9_-]+)/([a-zA-Z0-9_.-]+)/issues/(\d+)(?:/.*)?$`)
)

// ParseReference parses a Bitbucket issue reference
// Supported formats:
//   - bitbucket:123 or bb:123 - issue number (requires config)
//   - bitbucket:#123 or bb:#123 - issue number with hash
//   - bitbucket:workspace/repo#123 - explicit workspace and repo
//   - workspace/repo#123 - explicit without scheme
//   - https://bitbucket.org/workspace/repo/issues/123 - URL format
//   - bitbucket.org/workspace/repo/issues/123 - URL without scheme
func ParseReference(input string) (*Reference, error) {
	if input == "" {
		return nil, ErrInvalidReference
	}

	// Strip scheme prefix
	ref := input
	ref = strings.TrimPrefix(ref, "bitbucket:")
	ref = strings.TrimPrefix(ref, "bb:")
	ref = strings.TrimSpace(ref)

	if ref == "" {
		return nil, ErrInvalidReference
	}

	// Try URL format first
	if matches := urlPattern.FindStringSubmatch(ref); matches != nil {
		issueID, err := strconv.Atoi(matches[3])
		if err != nil || issueID <= 0 {
			return nil, fmt.Errorf("%w: invalid issue number", ErrInvalidReference)
		}
		return &Reference{
			Workspace:  matches[1],
			RepoSlug:   matches[2],
			IssueID:    issueID,
			IsExplicit: true,
		}, nil
	}

	// Try explicit workspace/repo#number format
	if matches := explicitRepoPattern.FindStringSubmatch(ref); matches != nil {
		issueID, err := strconv.Atoi(matches[3])
		if err != nil || issueID <= 0 {
			return nil, fmt.Errorf("%w: invalid issue number", ErrInvalidReference)
		}
		return &Reference{
			Workspace:  matches[1],
			RepoSlug:   matches[2],
			IssueID:    issueID,
			IsExplicit: true,
		}, nil
	}

	// Try simple issue number format
	if matches := simpleIssuePattern.FindStringSubmatch(ref); matches != nil {
		issueID, err := strconv.Atoi(matches[1])
		if err != nil || issueID <= 0 {
			return nil, fmt.Errorf("%w: invalid issue number", ErrInvalidReference)
		}
		return &Reference{
			IssueID:    issueID,
			IsExplicit: false,
		}, nil
	}

	return nil, fmt.Errorf("%w: %s", ErrInvalidReference, input)
}

// ExtractLinkedIssues extracts issue references from text (e.g., "fixes #123").
func ExtractLinkedIssues(text string) []int {
	pattern := regexp.MustCompile(`(?i)(?:fixes?|closes?|resolves?)\s+#(\d+)`)
	matches := pattern.FindAllStringSubmatch(text, -1)

	var issues []int
	seen := make(map[int]bool)

	for _, match := range matches {
		if id, err := strconv.Atoi(match[1]); err == nil && !seen[id] {
			issues = append(issues, id)
			seen[id] = true
		}
	}

	return issues
}

// ExtractImageURLs extracts image URLs from markdown content.
func ExtractImageURLs(text string) []string {
	// Match markdown image syntax: ![alt](url)
	pattern := regexp.MustCompile(`!\[[^\]]*\]\(([^)]+)\)`)
	matches := pattern.FindAllStringSubmatch(text, -1)

	var urls []string
	for _, match := range matches {
		url := match[1]
		// Filter to only include image URLs
		if strings.HasSuffix(strings.ToLower(url), ".png") ||
			strings.HasSuffix(strings.ToLower(url), ".jpg") ||
			strings.HasSuffix(strings.ToLower(url), ".jpeg") ||
			strings.HasSuffix(strings.ToLower(url), ".gif") ||
			strings.HasSuffix(strings.ToLower(url), ".webp") ||
			strings.Contains(url, "bitbucket.org") {
			urls = append(urls, url)
		}
	}

	return urls
}

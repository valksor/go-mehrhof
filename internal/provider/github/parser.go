package github

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Ref represents a parsed GitHub issue reference.
type Ref struct {
	Owner       string
	Repo        string
	IssueNumber int
	IsExplicit  bool // true if owner/repo was explicitly provided
}

// String returns the canonical string representation.
func (r *Ref) String() string {
	if r.Owner != "" && r.Repo != "" {
		return fmt.Sprintf("%s/%s#%d", r.Owner, r.Repo, r.IssueNumber)
	}

	return fmt.Sprintf("#%d", r.IssueNumber)
}

var (
	// Matches: owner/repo#123.
	explicitRefPattern = regexp.MustCompile(`^([a-zA-Z0-9_-]+)/([a-zA-Z0-9._-]+)#(\d+)$`)
	// Matches: #123 or just 123.
	simpleRefPattern = regexp.MustCompile(`^#?(\d+)$`)
)

// ParseReference parses various GitHub issue reference formats
// Supported formats:
//   - "5" or "#5"           -> issue 5 from auto-detected repo
//   - "owner/repo#5"        -> explicit repo
//   - "github:5"            -> scheme prefix (handled by registry, but we handle it too)
//   - "github:owner/repo#5" -> scheme prefix with explicit repo
func ParseReference(input string) (*Ref, error) {
	// Strip scheme prefix if present
	input = strings.TrimPrefix(input, "github:")
	input = strings.TrimPrefix(input, "gh:")
	input = strings.TrimSpace(input)

	if input == "" {
		return nil, fmt.Errorf("%w: empty reference", ErrInvalidReference)
	}

	// Try explicit owner/repo#number format
	if matches := explicitRefPattern.FindStringSubmatch(input); matches != nil {
		num, err := strconv.Atoi(matches[3])
		if err != nil {
			return nil, fmt.Errorf("%w: invalid issue number: %s", ErrInvalidReference, matches[3])
		}

		return &Ref{
			Owner:       matches[1],
			Repo:        matches[2],
			IssueNumber: num,
			IsExplicit:  true,
		}, nil
	}

	// Try simple #number or number format
	if matches := simpleRefPattern.FindStringSubmatch(input); matches != nil {
		num, err := strconv.Atoi(matches[1])
		if err != nil {
			return nil, fmt.Errorf("%w: invalid issue number: %s", ErrInvalidReference, matches[1])
		}

		return &Ref{
			IssueNumber: num,
			IsExplicit:  false,
		}, nil
	}

	return nil, fmt.Errorf("%w: unrecognized format: %s (expected #N, N, or owner/repo#N)", ErrInvalidReference, input)
}

// DetectRepository parses the GitHub owner/repo from a git remote URL
// Supports:
//   - git@github.com:owner/repo.git
//   - https://github.com/owner/repo.git
//   - https://github.com/owner/repo
func DetectRepository(remoteURL string) (string, string, error) {
	remoteURL = strings.TrimSpace(remoteURL)
	if remoteURL == "" {
		return "", "", ErrRepoNotDetected
	}

	// SSH format: git@github.com:owner/repo.git
	if strings.HasPrefix(remoteURL, "git@github.com:") {
		path := strings.TrimPrefix(remoteURL, "git@github.com:")
		path = strings.TrimSuffix(path, ".git")
		parts := strings.Split(path, "/")
		if len(parts) == 2 {
			return parts[0], parts[1], nil
		}
	}

	// HTTPS format: https://github.com/owner/repo.git
	if strings.Contains(remoteURL, "github.com/") {
		// Extract path after github.com/
		idx := strings.Index(remoteURL, "github.com/")
		if idx >= 0 {
			path := remoteURL[idx+len("github.com/"):]
			path = strings.TrimSuffix(path, ".git")
			path = strings.TrimSuffix(path, "/")
			parts := strings.Split(path, "/")
			if len(parts) >= 2 {
				return parts[0], parts[1], nil
			}
		}
	}

	return "", "", fmt.Errorf("%w: not a GitHub URL: %s", ErrRepoNotDetected, remoteURL)
}

// ExtractLinkedIssues finds #123 references in text.
func ExtractLinkedIssues(body string) []int {
	pattern := regexp.MustCompile(`#(\d+)`)
	matches := pattern.FindAllStringSubmatch(body, -1)

	seen := make(map[int]bool)
	var issues []int
	for _, m := range matches {
		num, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		if !seen[num] {
			seen[num] = true
			issues = append(issues, num)
		}
	}

	return issues
}

// ExtractImageURLs finds markdown image URLs in text.
func ExtractImageURLs(body string) []string {
	// Match ![alt](url) patterns
	pattern := regexp.MustCompile(`!\[[^\]]*\]\(([^)]+)\)`)
	matches := pattern.FindAllStringSubmatch(body, -1)

	var urls []string
	seen := make(map[string]bool)
	for _, m := range matches {
		url := m[1]
		if !seen[url] {
			seen[url] = true
			urls = append(urls, url)
		}
	}

	return urls
}

// TaskItem represents a parsed task list item from markdown.
type TaskItem struct {
	Text      string // The task text
	Completed bool   // Whether the checkbox is checked
	Line      int    // Line number in the body (1-based)
}

// taskListPattern matches markdown task list items:
// - [ ] unchecked task
// - [x] checked task
// * [ ] alternative bullet.
var taskListPattern = regexp.MustCompile(`^[\s]*[-*]\s+\[([ xX])\]\s+(.+)$`)

// ParseTaskList extracts task list items from markdown text.
func ParseTaskList(body string) []TaskItem {
	if body == "" {
		return nil
	}

	lines := strings.Split(body, "\n")
	var tasks []TaskItem

	for i, line := range lines {
		matches := taskListPattern.FindStringSubmatch(line)
		if matches == nil {
			continue
		}

		// matches[1] is the checkbox state (space, x, or X)
		// matches[2] is the task text
		completed := matches[1] == "x" || matches[1] == "X"
		text := strings.TrimSpace(matches[2])

		if text != "" {
			tasks = append(tasks, TaskItem{
				Text:      text,
				Completed: completed,
				Line:      i + 1, // 1-based line number
			})
		}
	}

	return tasks
}

package gitlab

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Ref represents a parsed GitLab issue reference
type Ref struct {
	ProjectPath string // e.g., "group/project"
	ProjectID   int64  // Numeric project ID (alternative to path)
	IssueIID    int64  // Issue IID (internal issue number, not global ID)
	IsExplicit  bool   // true if project was explicitly provided
}

// String returns the canonical string representation
func (r *Ref) String() string {
	if r.ProjectPath != "" {
		return fmt.Sprintf("%s#%d", r.ProjectPath, r.IssueIID)
	}
	if r.ProjectID > 0 {
		return fmt.Sprintf("%d#%d", r.ProjectID, r.IssueIID)
	}
	return fmt.Sprintf("#%d", r.IssueIID)
}

var (
	// Matches: group/project#123
	explicitRefPattern = regexp.MustCompile(`^([a-zA-Z0-9_/-]+)/([a-zA-Z0-9_-]+)#(\d+)$`)
	// Matches: #123 or just 123
	simpleRefPattern = regexp.MustCompile(`^#?(\d+)$`)
	// Matches: project ID format like 12345#67890
	projectIDRefPattern = regexp.MustCompile(`^(\d+)#(\d+)$`)
)

// ParseReference parses various GitLab issue reference formats
// Supported formats:
//   - "5" or "#5"           -> issue 5 from auto-detected project
//   - "group/project#5"    -> explicit project path
//   - "12345#5"            -> project ID with issue IID
//   - "gitlab:5"           -> scheme prefix (handled by registry, but we handle it too)
//   - "gitlab:group/project#5" -> scheme prefix with explicit project
func ParseReference(input string) (*Ref, error) {
	// Strip scheme prefix if present
	input = strings.TrimPrefix(input, "gitlab:")
	input = strings.TrimPrefix(input, "gl:")
	input = strings.TrimSpace(input)

	if input == "" {
		return nil, fmt.Errorf("%w: empty reference", ErrInvalidReference)
	}

	// Try explicit group/project#iid format
	if matches := explicitRefPattern.FindStringSubmatch(input); matches != nil {
		iid, err := strconv.ParseInt(matches[3], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%w: invalid issue IID: %s", ErrInvalidReference, matches[3])
		}
		return &Ref{
			ProjectPath: matches[1] + "/" + matches[2],
			IssueIID:    iid,
			IsExplicit:  true,
		}, nil
	}

	// Try project ID#iid format (e.g., 12345#678)
	if matches := projectIDRefPattern.FindStringSubmatch(input); matches != nil {
		pid, err := strconv.ParseInt(matches[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%w: invalid project ID: %s", ErrInvalidReference, matches[1])
		}
		iid, err := strconv.ParseInt(matches[2], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%w: invalid issue IID: %s", ErrInvalidReference, matches[2])
		}
		return &Ref{
			ProjectID:  pid,
			IssueIID:   iid,
			IsExplicit: true,
		}, nil
	}

	// Try simple #number or number format
	if matches := simpleRefPattern.FindStringSubmatch(input); matches != nil {
		iid, err := strconv.ParseInt(matches[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%w: invalid issue IID: %s", ErrInvalidReference, matches[1])
		}
		return &Ref{
			IssueIID:   iid,
			IsExplicit: false,
		}, nil
	}

	return nil, fmt.Errorf("%w: unrecognized format: %s (expected #N, N, group/project#N, or projectID#N)", ErrInvalidReference, input)
}

// DetectProject parses the GitLab group/project from a git remote URL
// Supports:
//   - git@gitlab.com:group/project.git
//   - https://gitlab.com/group/project.git
//   - https://gitlab.com/group/project
//   - https://custom.gitlab.host/group/project.git (self-hosted)
func DetectProject(remoteURL, host string) (projectPath string, err error) {
	remoteURL = strings.TrimSpace(remoteURL)
	if remoteURL == "" {
		return "", ErrProjectNotDetected
	}

	// Default to gitlab.com if no host specified
	if host == "" {
		host = "gitlab.com"
	}

	// SSH format: git@gitlab.com:group/project.git
	sshPrefix := "git@" + host + ":"
	if strings.HasPrefix(remoteURL, sshPrefix) {
		path := strings.TrimPrefix(remoteURL, sshPrefix)
		path = strings.TrimSuffix(path, ".git")
		return path, nil
	}

	// Generic SSH format: git@host:group/project.git
	if strings.HasPrefix(remoteURL, "git@") {
		idx := strings.Index(remoteURL, ":")
		if idx >= 0 {
			path := remoteURL[idx+1:]
			path = strings.TrimSuffix(path, ".git")
			return path, nil
		}
	}

	// HTTPS format: https://gitlab.com/group/project.git or similar
	hostPattern := host + "/"
	if strings.Contains(remoteURL, hostPattern) {
		// Extract path after host/
		idx := strings.Index(remoteURL, hostPattern)
		if idx >= 0 {
			path := remoteURL[idx+len(hostPattern):]
			path = strings.TrimSuffix(path, ".git")
			path = strings.TrimSuffix(path, "/")
			return path, nil
		}
	}

	// Generic HTTPS URL format
	if strings.HasPrefix(remoteURL, "https://") || strings.HasPrefix(remoteURL, "http://") {
		// Parse URL and extract path
		parts := strings.Split(remoteURL, "/")
		if len(parts) >= 2 {
			// Join everything after the scheme and host
			pathParts := parts[3:] // Skip scheme, empty, host
			path := strings.Join(pathParts, "/")
			path = strings.TrimSuffix(path, ".git")
			path = strings.TrimSuffix(path, "/")
			return path, nil
		}
	}

	return "", fmt.Errorf("%w: not a GitLab URL: %s", ErrProjectNotDetected, remoteURL)
}

// ExtractLinkedIssues finds #123 references in text
func ExtractLinkedIssues(body string) []int64 {
	pattern := regexp.MustCompile(`#(\d+)`)
	matches := pattern.FindAllStringSubmatch(body, -1)

	seen := make(map[int64]bool)
	var issues []int64
	for _, m := range matches {
		num, err := strconv.ParseInt(m[1], 10, 64)
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

// ExtractImageURLs finds markdown image URLs in text
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

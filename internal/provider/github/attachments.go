package github

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
)

// DownloadAttachment downloads an attachment by ID.
// For GitHub, attachments are typically image URLs embedded in the issue body.
func (p *Provider) DownloadAttachment(ctx context.Context, workUnitID, attachmentID string) (io.ReadCloser, error) {
	// Parse the work unit to get issue details
	ref, err := ParseReference(workUnitID)
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

	p.client.SetOwnerRepo(owner, repo)

	// Fetch the issue to find the attachment URL
	issue, err := p.client.GetIssue(ctx, ref.IssueNumber)
	if err != nil {
		return nil, err
	}

	// Extract image URLs from issue body
	urls := ExtractImageURLs(issue.GetBody())

	// Find the attachment by ID (format: img-N)
	var targetURL string
	for i, url := range urls {
		if attachmentID == fmt.Sprintf("img-%d", i) {
			targetURL = url

			break
		}
	}

	if targetURL == "" {
		return nil, fmt.Errorf("attachment not found: %s", attachmentID)
	}

	// Download the attachment
	return downloadURL(ctx, targetURL)
}

// downloadURL fetches content from a URL.
func downloadURL(ctx context.Context, url string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()

		return nil, fmt.Errorf("download failed: status %d", resp.StatusCode)
	}

	return resp.Body, nil
}

// FetchLinkedIssueContent fetches the content of linked issues and returns them as markdown.
func (p *Provider) FetchLinkedIssueContent(ctx context.Context, issueNumber int) (map[int]string, error) {
	// Fetch main issue
	issue, err := p.client.GetIssue(ctx, issueNumber)
	if err != nil {
		return nil, err
	}

	// Extract linked issue numbers
	linkedNums := ExtractLinkedIssues(issue.GetBody())
	if len(linkedNums) == 0 {
		return nil, nil //nolint:nilnil // No linked issues found (not an error)
	}

	result := make(map[int]string)
	for _, num := range linkedNums {
		if num == issueNumber {
			continue // Skip self-reference
		}

		linked, err := p.client.GetIssue(ctx, num)
		if err != nil {
			// Skip issues we can't access
			continue
		}

		result[num] = formatIssueMarkdown(linked)
	}

	return result, nil
}

// FetchRepoFile fetches a file from the repository (useful for linked markdown files).
func (p *Provider) FetchRepoFile(ctx context.Context, path, ref string) ([]byte, error) {
	return p.client.DownloadFile(ctx, path, ref)
}

// ExtractRepoFileLinks finds repository file links in markdown text.
// Matches patterns like: [text](./docs/spec.md) or [text](/path/to/file.md).
func ExtractRepoFileLinks(body string) []string {
	// Match markdown links to .md, .txt, .yaml, .yml files
	// Pattern: [any text](path ending with supported extension)
	patterns := []string{
		`\[[^\]]*\]\((\./[^)]+\.md)\)`,    // ./relative/path.md
		`\[[^\]]*\]\((/[^)]+\.md)\)`,      // /absolute/path.md
		`\[[^\]]*\]\(([^)]+\.md)\)`,       // path.md (no leading ./ or /)
		`\[[^\]]*\]\((\./[^)]+\.txt)\)`,   // ./relative/path.txt
		`\[[^\]]*\]\((\./[^)]+\.ya?ml)\)`, // ./relative/path.yaml or .yml
	}

	seen := make(map[string]bool)
	var links []string

	for _, pattern := range patterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			continue
		}

		matches := re.FindAllStringSubmatch(body, -1)
		for _, m := range matches {
			if len(m) > 1 {
				path := m[1]
				// Skip external URLs
				if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
					continue
				}
				if !seen[path] {
					seen[path] = true
					links = append(links, path)
				}
			}
		}
	}

	return links
}

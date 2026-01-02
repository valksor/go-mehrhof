package github

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/valksor/go-mehrhof/internal/provider"
	"github.com/valksor/go-mehrhof/internal/storage"
)

// FetchComments retrieves all comments from a GitHub issue.
func (p *Provider) FetchComments(ctx context.Context, workUnitID string) ([]provider.Comment, error) {
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

	comments, err := p.client.GetIssueComments(ctx, ref.IssueNumber)
	if err != nil {
		return nil, err
	}

	result := make([]provider.Comment, len(comments))
	for i, c := range comments {
		result[i] = provider.Comment{
			ID:        fmt.Sprintf("%d", c.GetID()),
			Body:      c.GetBody(),
			CreatedAt: c.GetCreatedAt().Time,
			Author: provider.Person{
				ID:   fmt.Sprintf("%d", c.GetUser().GetID()),
				Name: c.GetUser().GetLogin(),
			},
		}
	}

	return result, nil
}

// AddComment adds a comment to a GitHub issue.
func (p *Provider) AddComment(ctx context.Context, workUnitID string, body string) (*provider.Comment, error) {
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

	comment, err := p.client.AddComment(ctx, ref.IssueNumber, body)
	if err != nil {
		return nil, err
	}

	return &provider.Comment{
		ID:        fmt.Sprintf("%d", comment.GetID()),
		Body:      comment.GetBody(),
		CreatedAt: comment.GetCreatedAt().Time,
		Author: provider.Person{
			ID:   fmt.Sprintf("%d", comment.GetUser().GetID()),
			Name: comment.GetUser().GetLogin(),
		},
	}, nil
}

// CommentGenerator generates comment content for various events.
type CommentGenerator struct {
	provider *Provider
}

// NewCommentGenerator creates a new comment generator.
func NewCommentGenerator(p *Provider) *CommentGenerator {
	return &CommentGenerator{provider: p}
}

// GenerateBranchCreatedComment generates comment for branch creation.
func (g *CommentGenerator) GenerateBranchCreatedComment(branchName string) string {
	return fmt.Sprintf("Started working on this issue.\nBranch: `%s`", branchName)
}

// GeneratePlanComment generates comment summarizing the implementation plan
// This extracts key information from specification files.
func (g *CommentGenerator) GeneratePlanComment(specs []*storage.Specification) string {
	if len(specs) == 0 {
		return "Planning complete."
	}

	var sb strings.Builder
	sb.WriteString("## Implementation Plan\n\n")

	// Use the latest specification
	spec := specs[len(specs)-1]

	// Extract planned files from specification content
	plannedFiles := extractPlannedFiles(spec.Content)
	if len(plannedFiles) > 0 {
		sb.WriteString("**Files to be created/modified:**\n")
		for _, f := range plannedFiles {
			sb.WriteString(fmt.Sprintf("- `%s`\n", f))
		}
		sb.WriteString("\n")
	}

	// Extract approach summary (first paragraph after "## Approach" or similar)
	approach := extractApproachSummary(spec.Content)
	if approach != "" {
		sb.WriteString("**Approach:**\n")
		sb.WriteString(approach)
		sb.WriteString("\n")
	}

	return sb.String()
}

// GenerateImplementComment generates comment summarizing implementation changes.
func (g *CommentGenerator) GenerateImplementComment(diffStat string, summary string) string {
	var sb strings.Builder
	sb.WriteString("## Implementation Complete\n\n")

	if summary != "" {
		sb.WriteString("**Summary:**\n")
		sb.WriteString(summary)
		sb.WriteString("\n\n")
	}

	if diffStat != "" {
		sb.WriteString("**Files changed:**\n```\n")
		sb.WriteString(diffStat)
		sb.WriteString("\n```\n\n")
	}

	sb.WriteString("Ready for review.")

	return sb.String()
}

// GeneratePRCreatedComment generates comment for PR creation.
func (g *CommentGenerator) GeneratePRCreatedComment(prNumber int, prURL string) string {
	return fmt.Sprintf("Pull request created: #%d\n%s", prNumber, prURL)
}

// --- Helper functions for extracting info from specifications ---

// extractPlannedFiles finds file paths mentioned in specification content.
func extractPlannedFiles(content string) []string {
	var files []string
	seen := make(map[string]bool)

	// Match various patterns for file references
	patterns := []*regexp.Regexp{
		regexp.MustCompile("`([a-zA-Z0-9_/.-]+\\.[a-z]+)`"),                          // `path/to/file.ext`
		regexp.MustCompile("(?:create|modify|update|add|edit).*?`([^`]+\\.[a-z]+)`"), // create/modify `file.ext`
		regexp.MustCompile("- `([^`]+\\.[a-z]+)`"),                                   // - `file.ext` (list items)
	}

	for _, pattern := range patterns {
		matches := pattern.FindAllStringSubmatch(content, -1)
		for _, m := range matches {
			if len(m) > 1 {
				path := m[1]
				// Filter out obvious non-file patterns
				if isLikelyFilePath(path) && !seen[path] {
					seen[path] = true
					files = append(files, path)
				}
			}
		}
	}

	return files
}

// isLikelyFilePath checks if a string looks like a file path.
func isLikelyFilePath(s string) bool {
	// Must have an extension
	if !strings.Contains(s, ".") {
		return false
	}

	// Common code file extensions
	validExts := []string{
		".go", ".py", ".js", ".ts", ".tsx", ".jsx", ".java", ".kt", ".swift",
		".rs", ".rb", ".php", ".c", ".cpp", ".h", ".hpp", ".cs", ".m", ".mm",
		".md", ".yaml", ".yml", ".json", ".toml", ".xml", ".html", ".css",
		".sql", ".sh", ".bash", ".zsh", ".ps1", ".bat",
	}

	for _, ext := range validExts {
		if strings.HasSuffix(strings.ToLower(s), ext) {
			return true
		}
	}

	return false
}

// extractApproachSummary extracts approach/strategy summary from specification.
func extractApproachSummary(content string) string {
	// Look for common approach headings
	headingPatterns := []string{
		`(?i)##\s*approach\s*\n+([^\n#]+)`,
		`(?i)##\s*strategy\s*\n+([^\n#]+)`,
		`(?i)##\s*implementation\s+approach\s*\n+([^\n#]+)`,
		`(?i)##\s*solution\s*\n+([^\n#]+)`,
	}

	for _, pattern := range headingPatterns {
		re := regexp.MustCompile(pattern)
		match := re.FindStringSubmatch(content)
		if len(match) > 1 {
			return strings.TrimSpace(match[1])
		}
	}

	return ""
}

// ParseDiffStat parses git diff --stat output and formats it nicely.
func ParseDiffStat(diffOutput string) string {
	lines := strings.Split(strings.TrimSpace(diffOutput), "\n")
	if len(lines) == 0 {
		return ""
	}

	var sb strings.Builder
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Format: file | count ++++----
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	return sb.String()
}

// GenerateChangeSummary creates a brief summary of changes from session exchanges.
func GenerateChangeSummary(exchanges []storage.Exchange) string {
	var changedFiles []string
	fileOps := make(map[string]string)

	for _, ex := range exchanges {
		for _, fc := range ex.FilesChanged {
			if _, exists := fileOps[fc.Path]; !exists {
				fileOps[fc.Path] = fc.Operation
				changedFiles = append(changedFiles, fc.Path)
			}
		}
	}

	if len(changedFiles) == 0 {
		return ""
	}

	var sb strings.Builder
	for _, f := range changedFiles {
		op := fileOps[f]
		sb.WriteString(fmt.Sprintf("- `%s` (%s)\n", f, op))
	}

	return sb.String()
}

// CommentEvent represents a comment event for the comment service.
type CommentEvent string

const (
	CommentEventBranchCreated CommentEvent = "branch_created"
	CommentEventPlanDone      CommentEvent = "plan_done"
	CommentEventImplementDone CommentEvent = "implement_done"
	CommentEventPRCreated     CommentEvent = "pr_created"
)

// ShouldComment checks if a comment should be posted for the given event.
func (p *Provider) ShouldComment(event CommentEvent) bool {
	if p.config == nil || p.config.Comments == nil || !p.config.Comments.Enabled {
		return false
	}

	switch event {
	case CommentEventBranchCreated:
		return p.config.Comments.OnBranchCreated
	case CommentEventPlanDone:
		return p.config.Comments.OnPlanDone
	case CommentEventImplementDone:
		return p.config.Comments.OnImplementDone
	case CommentEventPRCreated:
		return p.config.Comments.OnPRCreated
	default:
		return false
	}
}

// PostCommentIfEnabled posts a comment if enabled for the given event.
func (p *Provider) PostCommentIfEnabled(ctx context.Context, workUnitID string, event CommentEvent, body string) error {
	if !p.ShouldComment(event) {
		return nil
	}

	_, err := p.AddComment(ctx, workUnitID, body)
	return err
}

// CommentTimestamp returns formatted timestamp for comment.
func CommentTimestamp() string {
	return time.Now().Format("2006-01-02 15:04:05 UTC")
}

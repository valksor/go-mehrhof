package provider

import (
	"context"
	"fmt"
	"strconv"

	"github.com/google/go-github/v67/github"
)

const maxSiblingTasksGitHub = 5

// FetchParent returns nil for GitHub since it doesn't have native parent/child.
func (p *GitHubProvider) FetchParent(ctx context.Context, task *Task) (*Task, error) {
	// GitHub doesn't have native parent relationships
	// Could be implemented via linked issues or "Parent: #123" convention
	return nil, nil //nolint:nilnil // nil means no parent
}

// FetchSiblings returns other issues in the same milestone.
func (p *GitHubProvider) FetchSiblings(ctx context.Context, task *Task) ([]*Task, error) {
	milestoneStr := task.Metadata("github_milestone_number")
	if milestoneStr == "" {
		return nil, nil
	}

	owner := task.Metadata("github_owner")
	repo := task.Metadata("github_repo")
	if owner == "" || repo == "" {
		return nil, nil
	}

	issues, _, err := p.client.Issues.ListByRepo(ctx, owner, repo, &github.IssueListByRepoOptions{
		Milestone: milestoneStr,
		State:     "all",
		ListOptions: github.ListOptions{
			PerPage: maxSiblingTasksGitHub + 1,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("list milestone issues: %w", err)
	}

	siblings := make([]*Task, 0, maxSiblingTasksGitHub)
	for _, issue := range issues {
		issueID := fmt.Sprintf("%s/%s#%d", owner, repo, issue.GetNumber())
		if issueID == task.ID {
			continue
		}
		siblings = append(siblings, p.issueToTask(owner, repo, issue))
		if len(siblings) >= maxSiblingTasksGitHub {
			break
		}
	}

	return siblings, nil
}

// milestoneNumber returns the milestone number as a string for use with the GitHub API.
func milestoneNumber(issue *github.Issue) string {
	if issue.Milestone == nil {
		return ""
	}

	return strconv.Itoa(issue.Milestone.GetNumber())
}

// milestoneNumberFromPR returns the milestone number from a PR as a string.
func milestoneNumberFromPR(pr *github.PullRequest) string {
	if pr.Milestone == nil {
		return ""
	}

	return strconv.Itoa(pr.Milestone.GetNumber())
}

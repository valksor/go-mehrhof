package provider

import (
	"testing"

	"github.com/google/go-github/v67/github"
)

func TestGitHubProvider_ImplementsHierarchyProvider(t *testing.T) {
	var _ HierarchyProvider = (*GitHubProvider)(nil)
}

func TestGitHubProvider_ImplementsMergeProvider(t *testing.T) {
	var _ MergeProvider = (*GitHubProvider)(nil)
}

func TestMilestoneNumber(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		issue *github.Issue
		want  string
	}{
		{
			name:  "issue with nil Milestone",
			issue: &github.Issue{},
			want:  "",
		},
		{
			name: "issue with Milestone number 1",
			issue: &github.Issue{
				Milestone: &github.Milestone{Number: new(1)},
			},
			want: "1",
		},
		{
			name: "issue with Milestone number 42",
			issue: &github.Issue{
				Milestone: &github.Milestone{Number: new(42)},
			},
			want: "42",
		},
		{
			name: "issue with Milestone number 0",
			issue: &github.Issue{
				Milestone: &github.Milestone{Number: new(0)},
			},
			want: "0",
		},
		{
			name: "issue with large Milestone number",
			issue: &github.Issue{
				Milestone: &github.Milestone{Number: new(9999)},
			},
			want: "9999",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := milestoneNumber(tt.issue)
			if got != tt.want {
				t.Errorf("milestoneNumber() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMilestoneNumberFromPR(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		pr   *github.PullRequest
		want string
	}{
		{
			name: "PR with nil Milestone",
			pr:   &github.PullRequest{},
			want: "",
		},
		{
			name: "PR with Milestone number 1",
			pr: &github.PullRequest{
				Milestone: &github.Milestone{Number: new(1)},
			},
			want: "1",
		},
		{
			name: "PR with Milestone number 7",
			pr: &github.PullRequest{
				Milestone: &github.Milestone{Number: new(7)},
			},
			want: "7",
		},
		{
			name: "PR with Milestone number 0",
			pr: &github.PullRequest{
				Milestone: &github.Milestone{Number: new(0)},
			},
			want: "0",
		},
		{
			name: "PR with large Milestone number",
			pr: &github.PullRequest{
				Milestone: &github.Milestone{Number: new(1024)},
			},
			want: "1024",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := milestoneNumberFromPR(tt.pr)
			if got != tt.want {
				t.Errorf("milestoneNumberFromPR() = %q, want %q", got, tt.want)
			}
		})
	}
}

package provider

import "context"

// FetchParent returns nil for GitLab since it doesn't have native parent/child.
// Could be implemented via epics or "Parent: #123" convention in the future.
func (p *GitLabProvider) FetchParent(ctx context.Context, task *Task) (*Task, error) {
	return nil, nil //nolint:nilnil // nil means no parent
}

// FetchSiblings returns nil for GitLab.
// Could be implemented via epic issues or project milestone issues.
func (p *GitLabProvider) FetchSiblings(ctx context.Context, task *Task) ([]*Task, error) {
	return nil, nil
}

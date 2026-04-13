package provider

import "testing"

func TestGitLabProvider_ImplementsHierarchyProvider(t *testing.T) {
	var _ HierarchyProvider = (*GitLabProvider)(nil)
}

func TestGitLabProvider_ImplementsSubmitProvider(t *testing.T) {
	var _ SubmitProvider = (*GitLabProvider)(nil)
}

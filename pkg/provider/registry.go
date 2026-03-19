package provider

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/valksor/kvelmo/pkg/settings"
)

type Registry struct {
	providers map[string]Provider
}

// NewRegistry creates a provider registry with all providers initialized from settings.
// Tokens come from Settings (loaded from local .env files), never global env vars.
// If s is nil, providers are registered with empty tokens.
func NewRegistry(s *settings.Settings) *Registry {
	r := &Registry{
		providers: make(map[string]Provider),
	}

	// Extract tokens safely (s may be nil during testing or initialization)
	var (
		githubToken string
		gitlabToken string
		wrikeToken  string
		linearToken string
		linearTeam  string
		jiraToken   string
		jiraEmail   string
		jiraBaseURL string
		adoToken    string
		adoBaseURL  string
		adoOrg      string
		adoProject  string
		adoRepo     string
	)
	if s != nil {
		githubToken = s.Providers.GitHub.Token
		gitlabToken = s.Providers.GitLab.Token
		wrikeToken = s.Providers.Wrike.Token
		linearToken = s.Providers.Linear.Token
		linearTeam = s.Providers.Linear.Team
		jiraToken = s.Providers.Jira.Token
		jiraEmail = s.Providers.Jira.Email
		jiraBaseURL = s.Providers.Jira.BaseURL
		adoToken = s.Providers.AzureDevOps.Token
		adoBaseURL = s.Providers.AzureDevOps.BaseURL
		adoOrg = s.Providers.AzureDevOps.Organization
		adoProject = s.Providers.AzureDevOps.Project
		adoRepo = s.Providers.AzureDevOps.Repository
	}

	// Register default providers with tokens from settings
	r.Register(NewFileProvider())
	r.Register(NewGitHubProvider(githubToken))
	if glp, err := NewGitLabProvider(gitlabToken); err != nil {
		slog.Error("failed to create gitlab provider (gitlab features unavailable)", "error", err)
	} else {
		r.Register(glp)
	}
	r.Register(NewWrikeProvider(wrikeToken))
	r.Register(NewLinearProvider(linearToken, linearTeam))
	r.Register(NewJiraProvider(jiraBaseURL, jiraEmail, jiraToken))
	r.Register(NewAzureDevOpsProvider(adoBaseURL, adoOrg, adoProject, adoToken, adoRepo))
	r.Register(NewEmptyProvider())

	return r
}

func (r *Registry) Register(p Provider) {
	r.providers[p.Name()] = p
}

func (r *Registry) Get(name string) (Provider, error) {
	p, ok := r.providers[name]
	if !ok {
		return nil, fmt.Errorf("unknown provider: %s", name)
	}

	return p, nil
}

func (r *Registry) FetchTask(ctx context.Context, source string) (*Task, error) {
	providerName, id, err := Parse(source)
	if err != nil {
		return nil, fmt.Errorf("parse source: %w", err)
	}

	provider, err := r.Get(providerName)
	if err != nil {
		return nil, err
	}

	return provider.FetchTask(ctx, id)
}

// Parse parses a task source string and returns the provider name and ID.
//
//nolint:nonamedreturns // Named returns document the return values
func (r *Registry) Parse(source string) (providerName, sourceID string, err error) {
	return Parse(source)
}

// Fetch fetches a task from a specific provider by ID.
func (r *Registry) Fetch(ctx context.Context, providerName, sourceID string) (*Task, error) {
	provider, err := r.Get(providerName)
	if err != nil {
		return nil, err
	}

	return provider.FetchTask(ctx, sourceID)
}

// PRStatus holds the status of a pull request.
type PRStatus struct {
	Number int    `json:"number"`
	State  string `json:"state"` // "open", "closed"
	Merged bool   `json:"merged"`
	URL    string `json:"url"`
}

// GetPRStatus returns the status of a PR for the given task source.
// Returns an error if the provider doesn't support PR status.
func (r *Registry) GetPRStatus(ctx context.Context, source string) (*PRStatus, error) {
	providerName, id, err := Parse(source)
	if err != nil {
		return nil, fmt.Errorf("parse source: %w", err)
	}

	provider, err := r.Get(providerName)
	if err != nil {
		return nil, err
	}

	// Check if provider implements PR status
	type prStatusProvider interface {
		GetPRStatus(ctx context.Context, taskID string) (*PRStatus, error)
	}

	if psp, ok := provider.(prStatusProvider); ok {
		return psp.GetPRStatus(ctx, id)
	}

	return nil, fmt.Errorf("provider %s does not support PR status", providerName)
}

// HierarchyOptions controls which hierarchy fields are populated when calling
// FetchWithHierarchy.
type HierarchyOptions struct {
	// IncludeParent fetches and attaches the parent task to Task.ParentTask.
	IncludeParent bool
	// IncludeSiblings fetches and attaches sibling tasks to Task.SiblingTasks.
	IncludeSiblings bool
}

// FetchWithHierarchy fetches a task and, when the underlying provider implements
// HierarchyProvider, enriches it with parent and sibling context according to
// opts. Best-effort: hierarchy errors are silently swallowed so a lookup
// failure never blocks the main task fetch.
func (r *Registry) FetchWithHierarchy(ctx context.Context, providerName, sourceID string, opts HierarchyOptions) (*Task, error) {
	p, err := r.Get(providerName)
	if err != nil {
		return nil, err
	}

	task, err := p.FetchTask(ctx, sourceID)
	if err != nil {
		return nil, err
	}

	// Attempt hierarchy enrichment only when the provider supports it.
	hp, ok := p.(HierarchyProvider)
	if !ok {
		return task, nil
	}

	if opts.IncludeParent {
		parent, err := hp.FetchParent(ctx, task)
		if err != nil {
			slog.Warn("failed to fetch parent task", "provider", providerName, "source", sourceID, "error", err)
		} else {
			task.ParentTask = parent
		}
	}

	if opts.IncludeSiblings {
		siblings, err := hp.FetchSiblings(ctx, task)
		if err != nil {
			slog.Warn("failed to fetch sibling tasks", "provider", providerName, "source", sourceID, "error", err)
		} else {
			task.SiblingTasks = siblings
		}
	}

	return task, nil
}

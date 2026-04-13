package conductor

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/valksor/kvelmo/internal/provider"
	"github.com/valksor/kvelmo/settings"
)

// setupStatusSync registers a state machine listener that syncs task status
// to the external ticket provider when the workflow state changes.
// The listener runs asynchronously to never block state transitions.
func (c *Conductor) setupStatusSync() {
	c.machine.AddListener(func(from, to State, event Event, wu *WorkUnit) {
		if wu == nil || wu.Source == nil || wu.Source.Provider == "" {
			return
		}

		effectiveSettings := c.getEffectiveSettings()
		providerName := wu.Source.Provider

		if !isStatusSyncEnabled(effectiveSettings, providerName) {
			return
		}

		mappedStatus := getMappedStatus(effectiveSettings, providerName, string(to))
		if mappedStatus == "" {
			return
		}

		externalID := wu.ExternalID
		providers := c.providers
		fromStr := string(from)

		go func() {
			ctx := context.Background()
			p, err := providers.Get(providerName)
			if err != nil {
				slog.Debug("status sync: provider not found", "provider", providerName)

				return
			}

			// Call UpdateStatus on the provider (all providers implement it via the Provider interface)
			if err := p.UpdateStatus(ctx, externalID, mappedStatus); err != nil {
				slog.Warn("status sync: UpdateStatus failed, falling back to comment",
					"provider", providerName, "error", err)

				// Fall back to comment if the provider supports it
				if sp, ok := p.(provider.SubmitProvider); ok {
					comment := fmt.Sprintf("Status: **%s** (was: %s)", mappedStatus, fromStr)
					if err := sp.AddComment(ctx, externalID, comment); err != nil {
						slog.Debug("status sync: comment fallback failed", "error", err)
					}
				}
			}
		}()
	})
}

// isStatusSyncEnabled checks whether status syncing is enabled for the given provider.
func isStatusSyncEnabled(s *settings.Settings, providerName string) bool {
	if s == nil {
		return false
	}

	switch providerName {
	case provider.NameGitHub:
		return s.Providers.GitHub.StatusSync
	case provider.NameGitLab:
		return s.Providers.GitLab.StatusSync
	case provider.NameLinear:
		return s.Providers.Linear.StatusSync
	case provider.NameJira:
		return s.Providers.Jira.StatusSync
	case provider.NameAzureDevOps:
		return s.Providers.AzureDevOps.StatusSync
	default:
		return false
	}
}

// getMappedStatus returns the provider-specific status string for a kvelmo state.
// If a custom mapping is configured, it is used; otherwise the raw kvelmo state is returned.
func getMappedStatus(s *settings.Settings, providerName, kvelmoState string) string {
	if s == nil {
		return kvelmoState
	}

	var mapping map[string]string

	switch providerName {
	case provider.NameGitHub:
		mapping = s.Providers.GitHub.StatusMapping
	case provider.NameGitLab:
		mapping = s.Providers.GitLab.StatusMapping
	case provider.NameLinear:
		mapping = s.Providers.Linear.StatusMapping
	case provider.NameJira:
		mapping = s.Providers.Jira.StatusMapping
	case provider.NameAzureDevOps:
		mapping = s.Providers.AzureDevOps.StatusMapping
	}

	if mapping != nil {
		if mapped, ok := mapping[kvelmoState]; ok {
			return mapped
		}
	}

	return kvelmoState
}

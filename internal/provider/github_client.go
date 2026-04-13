package provider

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/go-github/v67/github"
	"golang.org/x/oauth2"
)

// newGitHubClient creates a GitHub API client with the given token.
// Token should come from Settings, not environment variables.
func newGitHubClient(token, host string) *github.Client {
	var httpClient *http.Client
	if token != "" {
		ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
		httpClient = oauth2.NewClient(context.Background(), ts)
	}

	if host != "" {
		// Normalize: strip scheme and trailing slashes for comparison
		normalized := strings.TrimPrefix(host, "https://")
		normalized = strings.TrimPrefix(normalized, "http://")
		normalized = strings.TrimSuffix(normalized, "/")

		if normalized != "github.com" {
			// Use separate base and upload URLs for GitHub Enterprise Server
			baseURL := "https://" + normalized + "/api/v3/"
			uploadURL := "https://" + normalized + "/api/uploads/"
			client, err := github.NewClient(httpClient).WithEnterpriseURLs(baseURL, uploadURL)
			if err != nil {
				slog.Error("configure enterprise github", "host", normalized, "error", err)

				return github.NewClient(httpClient)
			}

			return client
		}
	}

	return github.NewClient(httpClient)
}

//nolint:nonamedreturns // Named returns document the return values
func parseGitHubIDFull(id string) (owner, repo string, number int, err error) {
	parts := strings.SplitN(id, "#", 2)
	if len(parts) != 2 {
		return "", "", 0, fmt.Errorf("invalid github id: %s", id)
	}
	repoParts := strings.SplitN(parts[0], "/", 2)
	if len(repoParts) != 2 {
		return "", "", 0, fmt.Errorf("invalid github id: %s", id)
	}
	number, err = strconv.Atoi(parts[1])
	if err != nil {
		return "", "", 0, fmt.Errorf("invalid issue number: %s", parts[1])
	}

	return repoParts[0], repoParts[1], number, nil
}

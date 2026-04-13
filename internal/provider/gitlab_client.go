package provider

import (
	"fmt"
	"strconv"
	"strings"

	gitlab "gitlab.com/gitlab-org/api/client-go"
)

// newGitLabClient creates a GitLab API client with the given token.
// Token should come from Settings, not environment variables.
func newGitLabClient(token, host string) (*gitlab.Client, error) {
	// Normalize: trim trailing slashes to prevent double-slash URLs
	baseURL := strings.TrimSuffix(host, "/")
	if baseURL == "" {
		baseURL = "https://gitlab.com"
	}

	var client *gitlab.Client
	var err error
	if token != "" {
		client, err = gitlab.NewClient(token, gitlab.WithBaseURL(baseURL+"/api/v4"))
	} else {
		client, err = gitlab.NewClient("", gitlab.WithBaseURL(baseURL+"/api/v4"))
	}
	if err != nil {
		return nil, fmt.Errorf("create gitlab client: %w", err)
	}

	return client, nil
}

// parseGitLabID parses "group/project#123" or "group/project!456"
// Returns isMR=true for merge requests (!), false for issues (#)
//
//nolint:nonamedreturns // Named returns document the return values
func parseGitLabID(id string) (project string, number int, isMR bool, err error) {
	var separator string
	if strings.Contains(id, "!") {
		separator = "!"
		isMR = true
	} else if strings.Contains(id, "#") {
		separator = "#"
		isMR = false
	} else {
		return "", 0, false, fmt.Errorf("invalid gitlab id: %s", id)
	}

	parts := strings.SplitN(id, separator, 2)
	if len(parts) != 2 {
		return "", 0, false, fmt.Errorf("invalid gitlab id: %s", id)
	}

	number, err = strconv.Atoi(parts[1])
	if err != nil {
		return "", 0, false, fmt.Errorf("invalid number: %s", parts[1])
	}

	return parts[0], number, isMR, nil
}

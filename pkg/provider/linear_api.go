package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
)

// graphql executes a GraphQL query against Linear API.
func (p *LinearProvider) graphql(ctx context.Context, query string, variables map[string]any, result any) error {
	payload := map[string]any{
		"query": query,
	}
	if variables != nil {
		payload["variables"] = variables
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, linearAPIURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(payloadBytes)), nil
	}

	req.Header.Set("Authorization", p.token)
	req.Header.Set("Content-Type", "application/json")

	slog.Debug("linear: graphql request")

	resp, err := DoWithRetry(httpClient, req, DefaultRetryConfig)
	if err != nil {
		slog.Error("linear: graphql request failed", "error", err)

		return fmt.Errorf("linear api: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)

		slog.Error("linear: graphql error response", "status_code", resp.StatusCode)

		return fmt.Errorf("linear api error: %d - %s", resp.StatusCode, string(body))
	}

	// Check for GraphQL errors
	var graphqlResp struct {
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	// Read body into buffer so we can decode twice
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if err := json.Unmarshal(bodyBytes, &graphqlResp); err == nil && len(graphqlResp.Errors) > 0 {
		slog.Error("linear: graphql api error", "error", graphqlResp.Errors[0].Message)

		return fmt.Errorf("linear api: %s", graphqlResp.Errors[0].Message)
	}

	if err := json.Unmarshal(bodyBytes, result); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	slog.Debug("linear: graphql request completed")

	return nil
}

// fetchIssueByIdentifier fetches an issue by its identifier (e.g., "ENG-123").
func (p *LinearProvider) fetchIssueByIdentifier(ctx context.Context, identifier string) (*linearIssue, error) {
	// Clean up identifier (remove any leading "linear:" or "ln:")
	identifier = strings.TrimPrefix(identifier, "linear:")
	identifier = strings.TrimPrefix(identifier, "ln:")
	identifier = strings.ToUpper(identifier)

	query := `
		query IssueByIdentifier($filter: IssueFilter!) {
			issues(filter: $filter, first: 1) {
				nodes {
					id
					identifier
					title
					description
					url
					priority
					state { id name type }
					team { id key }
					parent { id identifier }
					labels { nodes { id name } }
					assignee { id name }
					children(first: 10) {
						nodes {
							id
							identifier
							title
							state { id name type }
						}
					}
				}
			}
		}
	`

	// Parse identifier to extract team key and number
	parts := strings.SplitN(identifier, "-", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid linear identifier: %s", identifier)
	}
	issueNumber, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid issue number in identifier %q: %w", identifier, err)
	}

	filter := map[string]any{
		"and": []map[string]any{
			{"team": map[string]any{"key": map[string]any{"eq": parts[0]}}},
			{"number": map[string]any{"eq": issueNumber}},
		},
	}

	var result struct {
		Data struct {
			Issues struct {
				Nodes []linearIssue `json:"nodes"`
			} `json:"issues"`
		} `json:"data"`
	}

	err = p.graphql(ctx, query, map[string]any{"filter": filter}, &result)
	if err != nil {
		return nil, err
	}

	if len(result.Data.Issues.Nodes) == 0 {
		return nil, fmt.Errorf("issue not found: %s", identifier)
	}

	return &result.Data.Issues.Nodes[0], nil
}

// fetchIssueByID fetches an issue by its internal ID.
func (p *LinearProvider) fetchIssueByID(ctx context.Context, id string) (*linearIssue, error) {
	query := `
		query Issue($id: String!) {
			issue(id: $id) {
				id
				identifier
				title
				description
				url
				priority
				state { id name type }
				team { id key }
				parent { id identifier }
				labels { nodes { id name } }
				assignee { id name }
			}
		}
	`

	var result struct {
		Data struct {
			Issue linearIssue `json:"issue"`
		} `json:"data"`
	}

	err := p.graphql(ctx, query, map[string]any{"id": id}, &result)
	if err != nil {
		return nil, err
	}

	if result.Data.Issue.ID == "" {
		return nil, fmt.Errorf("issue not found: %s", id)
	}

	return &result.Data.Issue, nil
}

// findWorkflowState finds a workflow state matching the status.
func (p *LinearProvider) findWorkflowState(ctx context.Context, teamID, status string) (string, error) {
	// Map status to Linear state types
	var stateTypes []string
	switch strings.ToLower(status) {
	case stateOpen, "pending", "todo", "backlog":
		stateTypes = []string{"backlog", "unstarted"}
	case "in_progress", "started", "doing":
		stateTypes = []string{"started"}
	case "done", "completed", "closed":
		stateTypes = []string{"completed"}
	case "canceled", "cancelled":
		stateTypes = []string{"canceled"}
	default:
		// Try to find by name
		return p.findWorkflowStateByName(ctx, teamID, status)
	}

	query := `
		query WorkflowStates($teamId: String!) {
			team(id: $teamId) {
				states {
					nodes {
						id
						name
						type
					}
				}
			}
		}
	`

	var result struct {
		Data struct {
			Team struct {
				States struct {
					Nodes []linearState `json:"nodes"`
				} `json:"states"`
			} `json:"team"`
		} `json:"data"`
	}

	err := p.graphql(ctx, query, map[string]any{"teamId": teamID}, &result)
	if err != nil {
		return "", err
	}

	// Find first matching state type
	for _, stateType := range stateTypes {
		for _, state := range result.Data.Team.States.Nodes {
			if state.Type == stateType {
				return state.ID, nil
			}
		}
	}

	return "", fmt.Errorf("no matching workflow state for: %s", status)
}

// findWorkflowStateByName finds a workflow state by name.
func (p *LinearProvider) findWorkflowStateByName(ctx context.Context, teamID, name string) (string, error) {
	query := `
		query WorkflowStates($teamId: String!) {
			team(id: $teamId) {
				states {
					nodes {
						id
						name
					}
				}
			}
		}
	`

	var result struct {
		Data struct {
			Team struct {
				States struct {
					Nodes []linearState `json:"nodes"`
				} `json:"states"`
			} `json:"team"`
		} `json:"data"`
	}

	err := p.graphql(ctx, query, map[string]any{"teamId": teamID}, &result)
	if err != nil {
		return "", err
	}

	nameLower := strings.ToLower(name)
	for _, state := range result.Data.Team.States.Nodes {
		if strings.ToLower(state.Name) == nameLower {
			return state.ID, nil
		}
	}

	return "", fmt.Errorf("workflow state not found: %s", name)
}

// getTeamID gets the team ID from a team key.
func (p *LinearProvider) getTeamID(ctx context.Context, key string) (string, error) {
	query := `
		query Team($key: String!) {
			team(key: $key) {
				id
			}
		}
	`

	var result struct {
		Data struct {
			Team struct {
				ID string `json:"id"`
			} `json:"team"`
		} `json:"data"`
	}

	err := p.graphql(ctx, query, map[string]any{"key": key}, &result)
	if err != nil {
		return "", err
	}

	if result.Data.Team.ID == "" {
		return "", fmt.Errorf("team not found: %s", key)
	}

	return result.Data.Team.ID, nil
}

// getLabelIDs gets label IDs by name for a team.
func (p *LinearProvider) getLabelIDs(ctx context.Context, teamID string, names []string) ([]string, error) {
	if len(names) == 0 {
		return nil, nil
	}

	query := `
		query TeamLabels($teamId: String!) {
			team(id: $teamId) {
				labels {
					nodes {
						id
						name
					}
				}
			}
		}
	`

	var result struct {
		Data struct {
			Team struct {
				Labels struct {
					Nodes []linearLabel `json:"nodes"`
				} `json:"labels"`
			} `json:"team"`
		} `json:"data"`
	}

	err := p.graphql(ctx, query, map[string]any{"teamId": teamID}, &result)
	if err != nil {
		return nil, err
	}

	// Build name -> ID map
	labelMap := make(map[string]string)
	for _, l := range result.Data.Team.Labels.Nodes {
		labelMap[strings.ToLower(l.Name)] = l.ID
	}

	ids := make([]string, 0, len(names))
	for _, name := range names {
		if id, ok := labelMap[strings.ToLower(name)]; ok {
			ids = append(ids, id)
		}
		// Skip unknown labels silently
	}

	return ids, nil
}

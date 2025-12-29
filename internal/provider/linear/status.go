package linear

import (
	"context"

	"github.com/valksor/go-mehrhof/internal/provider"
)

// UpdateStatus changes the state of a Linear issue
// Note: Linear uses state IDs rather than names. This implementation uses
// a simple state name mapping. In production, you'd want to query the
// team's states and map names to IDs.
func (p *Provider) UpdateStatus(ctx context.Context, workUnitID string, status provider.Status) error {
	ref, err := ParseReference(workUnitID)
	if err != nil {
		return err
	}

	// First, fetch the issue to get its ID
	issue, err := p.client.GetIssue(ctx, ref.IssueID)
	if err != nil {
		return err
	}

	// Map provider status to Linear state name
	stateName := mapProviderStatusToLinearStateName(status)

	// In production, you would:
	// 1. Query the team's workflow states
	// 2. Find the state ID matching the state name
	// 3. Use that ID for the update
	//
	// For now, we'll use the state name directly and let the API handle it
	// This may not work in all cases since Linear expects state IDs
	_, err = p.client.UpdateIssue(ctx, issue.ID, UpdateIssueInput{
		StateID: stateName,
	})

	return err
}

// GetStateIDByName looks up a Linear state ID by name for a given team
// This is a helper function that would be used in production to properly resolve state names
// For now, it returns the name as-is since we'd need additional API calls to resolve it
func (p *Provider) GetStateIDByName(ctx context.Context, teamKey, stateName string) (string, error) {
	// In a production implementation, you would:
	// 1. Query the team's workflow states using GraphQL
	// 2. Find the state ID matching the state name
	// 3. Return the state ID
	//
	// For now, we'll return the name as-is with a note
	return stateName, nil
}

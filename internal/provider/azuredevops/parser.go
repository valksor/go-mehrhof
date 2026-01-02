package azuredevops

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	// workItemIDPattern matches numeric work item IDs.
	workItemIDPattern = regexp.MustCompile(`^\d+$`)

	// orgProjectIDPattern matches org/project#ID format.
	// e.g., "myorg/myproject#123".
	orgProjectIDPattern = regexp.MustCompile(`^([a-zA-Z0-9_-]+)/([a-zA-Z0-9_-]+)#(\d+)$`)

	// devAzureURLPattern matches dev.azure.com URLs
	// e.g., https://dev.azure.com/org/project/_workitems/edit/123
	devAzureURLPattern = regexp.MustCompile(`(?:https?://)?dev\.azure\.com/([a-zA-Z0-9_-]+)/([a-zA-Z0-9_-]+)/_workitems/edit/(\d+)`)

	// visualStudioURLPattern matches visualstudio.com URLs
	// e.g., https://org.visualstudio.com/project/_workitems/edit/123
	visualStudioURLPattern = regexp.MustCompile(`(?:https?://)?([a-zA-Z0-9_-]+)\.visualstudio\.com/([a-zA-Z0-9_-]+)/_workitems/edit/(\d+)`)

	// extractWorkItemIDsPattern finds work item IDs in text.
	extractWorkItemIDsPattern = regexp.MustCompile(`(?:AB#|#)(\d+)`)
)

// Reference represents a parsed Azure DevOps reference.
type Reference struct {
	Organization string // Azure DevOps organization
	Project      string // Project name
	WorkItemID   int    // Work item ID
	IsExplicit   bool   // True if parsed from explicit URL/reference format
}

// String returns the string representation of the reference.
func (r Reference) String() string {
	if r.Organization != "" && r.Project != "" {
		return r.Organization + "/" + r.Project + "#" + strconv.Itoa(r.WorkItemID)
	}
	return strconv.Itoa(r.WorkItemID)
}

// ParseReference parses an Azure DevOps reference from various formats:
// - Work item ID: "123"
// - Org/Project#ID: "myorg/myproject#123"
// - dev.azure.com URL: "https://dev.azure.com/org/project/_workitems/edit/123"
// - visualstudio.com URL: "https://org.visualstudio.com/project/_workitems/edit/123"
func ParseReference(input string) (*Reference, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, ErrInvalidReference
	}

	// Strip azdo: or azure: prefix if present
	input = strings.TrimPrefix(input, "azdo:")
	input = strings.TrimPrefix(input, "azure:")
	input = strings.TrimSpace(input)

	// Try dev.azure.com URL pattern
	if matches := devAzureURLPattern.FindStringSubmatch(input); matches != nil {
		id, _ := strconv.Atoi(matches[3])
		return &Reference{
			Organization: matches[1],
			Project:      matches[2],
			WorkItemID:   id,
			IsExplicit:   true,
		}, nil
	}

	// Try visualstudio.com URL pattern
	if matches := visualStudioURLPattern.FindStringSubmatch(input); matches != nil {
		id, _ := strconv.Atoi(matches[3])
		return &Reference{
			Organization: matches[1],
			Project:      matches[2],
			WorkItemID:   id,
			IsExplicit:   true,
		}, nil
	}

	// Try org/project#ID format
	if matches := orgProjectIDPattern.FindStringSubmatch(input); matches != nil {
		id, _ := strconv.Atoi(matches[3])
		return &Reference{
			Organization: matches[1],
			Project:      matches[2],
			WorkItemID:   id,
			IsExplicit:   true,
		}, nil
	}

	// Try bare work item ID pattern
	if workItemIDPattern.MatchString(input) {
		id, err := strconv.Atoi(input)
		if err != nil || id <= 0 {
			return nil, ErrInvalidReference
		}
		return &Reference{
			WorkItemID: id,
			IsExplicit: false,
		}, nil
	}

	return nil, ErrInvalidReference
}

// ExtractWorkItemIDs extracts all work item IDs from text (e.g., commit messages)
// Supports AB#123 and #123 formats.
func ExtractWorkItemIDs(text string) []int {
	matches := extractWorkItemIDsPattern.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return nil
	}

	// Deduplicate
	seen := make(map[int]bool)
	var result []int
	for _, match := range matches {
		if len(match) > 1 {
			if id, err := strconv.Atoi(match[1]); err == nil && !seen[id] {
				seen[id] = true
				result = append(result, id)
			}
		}
	}

	return result
}

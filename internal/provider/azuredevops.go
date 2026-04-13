package provider

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// AzureDevOpsProvider implements Provider, SubmitProvider for Azure DevOps work items.
type AzureDevOpsProvider struct {
	client   *azureDevOpsClient
	repoName string
}

// NewAzureDevOpsProvider creates a new Azure DevOps provider.
// repoName is the Git repository name within the project (defaults to project name).
func NewAzureDevOpsProvider(baseURL, org, project, token, repoName string) *AzureDevOpsProvider {
	if repoName == "" {
		repoName = project
	}

	return &AzureDevOpsProvider{
		client:   newAzureDevOpsClient(baseURL, org, project, token),
		repoName: repoName,
	}
}

func (p *AzureDevOpsProvider) Name() string {
	return "azuredevops"
}

// FetchTask fetches a work item by ID (e.g., "12345").
func (p *AzureDevOpsProvider) FetchTask(ctx context.Context, ref string) (*Task, error) {
	if p.client.token == "" {
		return nil, errors.New("AZURE_DEVOPS_TOKEN not set")
	}

	id := normalizeAzureDevOpsID(ref)

	wi, err := p.client.GetWorkItem(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("azuredevops: fetch task: %w", err)
	}

	return p.workItemToTask(wi), nil
}

// UpdateStatus updates the state of a work item.
func (p *AzureDevOpsProvider) UpdateStatus(ctx context.Context, ref string, status string) error {
	if p.client.token == "" {
		return errors.New("AZURE_DEVOPS_TOKEN not set")
	}

	id := normalizeAzureDevOpsID(ref)

	return p.client.UpdateWorkItemState(ctx, id, status)
}

// AddComment posts a comment on a work item.
func (p *AzureDevOpsProvider) AddComment(ctx context.Context, ref string, body string) error {
	if p.client.token == "" {
		return errors.New("AZURE_DEVOPS_TOKEN not set")
	}

	id := normalizeAzureDevOpsID(ref)

	return p.client.AddWorkItemComment(ctx, id, body)
}

// CreatePR creates a pull request in the Azure DevOps repository.
func (p *AzureDevOpsProvider) CreatePR(ctx context.Context, opts PROptions) (*PRResult, error) {
	if p.client.token == "" {
		return nil, errors.New("AZURE_DEVOPS_TOKEN not set")
	}

	pr, err := p.client.CreatePullRequest(ctx, p.repoName, opts)
	if err != nil {
		return nil, err
	}

	return &PRResult{
		ID:     strconv.Itoa(pr.PullRequestID),
		Number: pr.PullRequestID,
		URL:    pr.URL,
		State:  pr.Status,
	}, nil
}

// workItemToTask converts an Azure DevOps work item to a Task.
func (p *AzureDevOpsProvider) workItemToTask(wi *azureWorkItem) *Task {
	fields := wi.Fields

	title, _ := fields["System.Title"].(string)
	description, _ := fields["System.Description"].(string)
	workItemType, _ := fields["System.WorkItemType"].(string)
	state, _ := fields["System.State"].(string)
	areaPath, _ := fields["System.AreaPath"].(string)

	// Strip HTML tags from description (Azure DevOps uses HTML)
	description = stripHTMLTags(description)

	var labels []string
	if tags, ok := fields["System.Tags"].(string); ok && tags != "" {
		for tag := range strings.SplitSeq(tags, ";") {
			tag = strings.TrimSpace(tag)
			if tag != "" {
				labels = append(labels, tag)
			}
		}
	}
	if state != "" {
		labels = append(labels, state)
	}

	task := &Task{
		ID:          strconv.Itoa(wi.ID),
		Title:       title,
		Description: description,
		URL:         p.client.workItemWebURL(wi.ID),
		Labels:      labels,
		Source:      "azuredevops",
	}

	task.Priority, task.Type, task.Slug = InferAll(task.Title, labels)

	// Override type from work item type
	if workItemType != "" {
		task.Type = strings.ToLower(workItemType)
	}

	// Metadata
	task.SetMetadata("azuredevops_id", strconv.Itoa(wi.ID))
	task.SetMetadata("azuredevops_state", state)
	task.SetMetadata("azuredevops_type", workItemType)
	task.SetMetadata("azuredevops_area", areaPath)

	return task
}

// stripHTMLTags removes HTML tags from a string (simple implementation).
func stripHTMLTags(s string) string {
	var b strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			b.WriteRune(r)
		}
	}

	return strings.TrimSpace(b.String())
}

// normalizeAzureDevOpsID cleans up an Azure DevOps work item ID reference.
func normalizeAzureDevOpsID(ref string) string {
	ref = strings.TrimPrefix(ref, "azuredevops:")

	return strings.TrimSpace(ref)
}

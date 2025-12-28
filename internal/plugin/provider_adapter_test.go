package plugin

import (
	"testing"
	"time"

	"github.com/valksor/go-mehrhof/internal/provider"
)

func TestNewProviderAdapter(t *testing.T) {
	manifest := &Manifest{
		Name:        "test-provider",
		Description: "Test provider",
		Type:        "provider",
		Provider: &ProviderConfig{
			Schemes: []string{"test://"},
		},
	}

	adapter := NewProviderAdapter(manifest, nil)

	if adapter == nil {
		t.Fatal("NewProviderAdapter returned nil")
	}
	if adapter.manifest != manifest {
		t.Error("manifest not set correctly")
	}
}

func TestProviderAdapter_Manifest(t *testing.T) {
	manifest := &Manifest{Name: "test"}
	adapter := NewProviderAdapter(manifest, nil)

	if adapter.Manifest() != manifest {
		t.Error("Manifest() should return the original manifest")
	}
}

func TestProviderAdapter_Capabilities(t *testing.T) {
	tests := []struct {
		name        string
		manifest    *Manifest
		wantCaps    []provider.Capability
		wantNotCaps []provider.Capability
	}{
		{
			name: "no capabilities",
			manifest: &Manifest{
				Name: "basic",
				Provider: &ProviderConfig{
					Capabilities: []string{},
				},
			},
			wantCaps:    []provider.Capability{provider.CapRead}, // Always has read
			wantNotCaps: []provider.Capability{provider.CapList, provider.CapComment},
		},
		{
			name: "with list capability",
			manifest: &Manifest{
				Name: "list-provider",
				Provider: &ProviderConfig{
					Capabilities: []string{"list"},
				},
			},
			wantCaps: []provider.Capability{provider.CapRead, provider.CapList},
		},
		{
			name: "with comment capability",
			manifest: &Manifest{
				Name: "comment-provider",
				Provider: &ProviderConfig{
					Capabilities: []string{"comment"},
				},
			},
			wantCaps: []provider.Capability{provider.CapRead, provider.CapComment},
		},
		{
			name: "multiple capabilities",
			manifest: &Manifest{
				Name: "full-provider",
				Provider: &ProviderConfig{
					Capabilities: []string{
						"list",
						"comment",
						"update_status",
						"manage_labels",
						"snapshot",
						"create_pr",
						"link_branch",
						"fetch_comments",
						"download_attachment",
					},
				},
			},
			wantCaps: []provider.Capability{
				provider.CapRead,
				provider.CapList,
				provider.CapComment,
				provider.CapUpdateStatus,
				provider.CapManageLabels,
				provider.CapSnapshot,
				provider.CapCreatePR,
				provider.CapLinkBranch,
				provider.CapFetchComments,
				provider.CapDownloadAttachment,
			},
		},
		{
			name: "unknown capabilities ignored",
			manifest: &Manifest{
				Name: "unknown-caps",
				Provider: &ProviderConfig{
					Capabilities: []string{"unknown_cap", "list"},
				},
			},
			wantCaps:    []provider.Capability{provider.CapRead, provider.CapList},
			wantNotCaps: []provider.Capability{provider.CapComment},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := NewProviderAdapter(tt.manifest, nil)
			caps := adapter.Capabilities()

			for _, wantCap := range tt.wantCaps {
				if !caps[wantCap] {
					t.Errorf("should have capability %v", wantCap)
				}
			}

			for _, notWantCap := range tt.wantNotCaps {
				if caps[notWantCap] {
					t.Errorf("should not have capability %v", notWantCap)
				}
			}
		})
	}
}

func TestConvertWorkUnit(t *testing.T) {
	now := time.Now()
	input := &WorkUnitResult{
		ID:          "123",
		ExternalID:  "EXT-123",
		Provider:    "test-provider",
		Title:       "Test Work Unit",
		Description: "Description text",
		Status:      "open",
		Priority:    2, // PriorityHigh
		Labels:      []string{"bug", "urgent"},
		Subtasks:    []string{"task-1", "task-2"},
		ExternalKey: "KEY-123",
		TaskType:    "bug",
		Slug:        "test-work-unit",
		Metadata:    map[string]any{"custom": "value"},
		CreatedAt:   now,
		UpdatedAt:   now,
		Source: SourceInfoResult{
			Reference: "test://123",
		},
		Assignees: []PersonResult{
			{ID: "u1", Name: "User One", Email: "one@test.com"},
			{ID: "u2", Name: "User Two", Email: "two@test.com"},
		},
		Comments: []CommentResult{
			{
				ID:        "c1",
				Body:      "First comment",
				Author:    PersonResult{ID: "u1", Name: "User One"},
				CreatedAt: now,
			},
		},
		Attachments: []AttachmentResult{
			{
				ID:       "a1",
				Name:     "file.txt",
				URL:      "https://example.com/file.txt",
				MimeType: "text/plain",
				Size:     1024,
			},
		},
	}

	result := convertWorkUnit(input)

	if result.ID != "123" {
		t.Errorf("ID = %q, want %q", result.ID, "123")
	}
	if result.ExternalID != "EXT-123" {
		t.Errorf("ExternalID = %q, want %q", result.ExternalID, "EXT-123")
	}
	if result.Provider != "test-provider" {
		t.Errorf("Provider = %q, want %q", result.Provider, "test-provider")
	}
	if result.Title != "Test Work Unit" {
		t.Errorf("Title = %q, want %q", result.Title, "Test Work Unit")
	}
	if result.Description != "Description text" {
		t.Errorf("Description = %q, want %q", result.Description, "Description text")
	}
	if result.Status != provider.Status("open") {
		t.Errorf("Status = %v, want %v", result.Status, provider.Status("open"))
	}
	if result.Priority != provider.Priority(2) {
		t.Errorf("Priority = %v, want %v", result.Priority, provider.Priority(2))
	}
	if len(result.Labels) != 2 {
		t.Errorf("Labels count = %d, want 2", len(result.Labels))
	}
	if result.ExternalKey != "KEY-123" {
		t.Errorf("ExternalKey = %q, want %q", result.ExternalKey, "KEY-123")
	}
	if result.TaskType != "bug" {
		t.Errorf("TaskType = %q, want %q", result.TaskType, "bug")
	}
	if result.Slug != "test-work-unit" {
		t.Errorf("Slug = %q, want %q", result.Slug, "test-work-unit")
	}

	// Check source
	if result.Source.Reference != "test://123" {
		t.Errorf("Source.Reference = %q, want %q", result.Source.Reference, "test://123")
	}

	// Check assignees
	if len(result.Assignees) != 2 {
		t.Errorf("Assignees count = %d, want 2", len(result.Assignees))
	} else {
		if result.Assignees[0].Name != "User One" {
			t.Errorf("Assignees[0].Name = %q, want %q", result.Assignees[0].Name, "User One")
		}
	}

	// Check comments
	if len(result.Comments) != 1 {
		t.Errorf("Comments count = %d, want 1", len(result.Comments))
	} else {
		if result.Comments[0].Body != "First comment" {
			t.Errorf("Comments[0].Body = %q, want %q", result.Comments[0].Body, "First comment")
		}
	}

	// Check attachments
	if len(result.Attachments) != 1 {
		t.Errorf("Attachments count = %d, want 1", len(result.Attachments))
	} else {
		if result.Attachments[0].Name != "file.txt" {
			t.Errorf("Attachments[0].Name = %q, want %q", result.Attachments[0].Name, "file.txt")
		}
		if result.Attachments[0].ContentType != "text/plain" {
			t.Errorf("Attachments[0].ContentType = %q, want %q", result.Attachments[0].ContentType, "text/plain")
		}
	}
}

func TestConvertWorkUnit_EmptyArrays(t *testing.T) {
	input := &WorkUnitResult{
		ID:          "empty",
		Title:       "Empty",
		Assignees:   nil,
		Comments:    nil,
		Attachments: nil,
	}

	result := convertWorkUnit(input)

	// Should create empty slices, not nil
	if result.Assignees == nil {
		t.Error("Assignees should be empty slice, not nil")
	}
	if result.Comments == nil {
		t.Error("Comments should be empty slice, not nil")
	}
	if result.Attachments == nil {
		t.Error("Attachments should be empty slice, not nil")
	}
}

func TestConvertWorkUnit_NoSource(t *testing.T) {
	input := &WorkUnitResult{
		ID:     "no-source",
		Title:  "No Source",
		Source: SourceInfoResult{}, // Empty
	}

	result := convertWorkUnit(input)

	// Source should have empty reference
	if result.Source.Reference != "" {
		t.Errorf("Source.Reference = %q, want empty", result.Source.Reference)
	}
}

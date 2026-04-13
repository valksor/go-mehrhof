package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestJiraProvider_Name(t *testing.T) {
	p := NewJiraProvider("https://test.atlassian.net", "user@test.com", "token")
	if got := p.Name(); got != "jira" {
		t.Errorf("Name() = %q, want %q", got, "jira")
	}
}

func TestJiraProvider_FetchTask(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify auth header is present (Basic auth)
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jiraIssue{
			ID:  "10001",
			Key: "PROJ-123",
			Fields: jiraIssueFields{
				Summary: "Implement user authentication",
				Description: map[string]any{
					"type":    "doc",
					"version": float64(1),
					"content": []any{
						map[string]any{
							"type": "paragraph",
							"content": []any{
								map[string]any{
									"type": "text",
									"text": "Add OAuth2 login flow",
								},
							},
						},
					},
				},
				Status:    &jiraStatus{Name: "In Progress"},
				Priority:  &jiraPriority{Name: "High"},
				IssueType: &jiraIssueType{Name: "Story"},
				Labels:    []string{"backend", "auth"},
				Parent: &jiraParentField{
					ID:  "10000",
					Key: "PROJ-100",
				},
				Subtasks: []jiraIssue{
					{
						Key: "PROJ-124",
						Fields: jiraIssueFields{
							Summary: "Add login button",
							Status:  &jiraStatus{Name: "Done"},
						},
					},
					{
						Key: "PROJ-125",
						Fields: jiraIssueFields{
							Summary: "Add OAuth callback handler",
							Status:  &jiraStatus{Name: "To Do"},
						},
					},
				},
			},
		})
	}))
	defer srv.Close()

	origTransport := httpClient.Transport
	httpClient.Transport = &rewriteTransport{
		base:      http.DefaultTransport,
		targetURL: srv.URL,
	}
	defer func() { httpClient.Transport = origTransport }()

	p := NewJiraProvider("https://test.atlassian.net", "user@test.com", "test-token")
	task, err := p.FetchTask(context.Background(), "PROJ-123")
	if err != nil {
		t.Fatalf("FetchTask() error = %v", err)
	}

	if task.ID != "PROJ-123" {
		t.Errorf("ID = %q, want PROJ-123", task.ID)
	}
	if task.Title != "Implement user authentication" {
		t.Errorf("Title = %q, want %q", task.Title, "Implement user authentication")
	}
	if task.Description != "Add OAuth2 login flow" {
		t.Errorf("Description = %q, want %q", task.Description, "Add OAuth2 login flow")
	}
	if task.Source != "jira" {
		t.Errorf("Source = %q, want jira", task.Source)
	}
	if task.Priority != "high" {
		t.Errorf("Priority = %q, want high", task.Priority)
	}
	if task.Metadata("jira_parent_key") != "PROJ-100" {
		t.Errorf("jira_parent_key = %q, want PROJ-100", task.Metadata("jira_parent_key"))
	}
	if task.Metadata("jira_issue_type") != "Story" {
		t.Errorf("jira_issue_type = %q, want Story", task.Metadata("jira_issue_type"))
	}

	// Verify subtasks
	if len(task.Subtasks) != 2 {
		t.Fatalf("len(Subtasks) = %d, want 2", len(task.Subtasks))
	}
	if task.Subtasks[0].ID != "PROJ-124" {
		t.Errorf("Subtasks[0].ID = %q, want PROJ-124", task.Subtasks[0].ID)
	}
	if !task.Subtasks[0].Completed {
		t.Error("Subtasks[0].Completed = false, want true (status is Done)")
	}
	if task.Subtasks[1].Completed {
		t.Error("Subtasks[1].Completed = true, want false (status is To Do)")
	}
}

func TestJiraProvider_FetchTask_NoToken(t *testing.T) {
	p := NewJiraProvider("https://test.atlassian.net", "user@test.com", "")

	_, err := p.FetchTask(context.Background(), "PROJ-123")
	if err == nil {
		t.Error("FetchTask() should return error when token is empty")
	}
}

func TestJiraProvider_FetchTask_PlainStringDescription(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Simulate a Jira instance that returns plain string descriptions
		_, _ = w.Write([]byte(`{
			"id": "10002",
			"key": "PROJ-456",
			"fields": {
				"summary": "Simple task",
				"description": "Plain text description",
				"status": {"name": "To Do"},
				"labels": [],
				"subtasks": []
			}
		}`))
	}))
	defer srv.Close()

	origTransport := httpClient.Transport
	httpClient.Transport = &rewriteTransport{
		base:      http.DefaultTransport,
		targetURL: srv.URL,
	}
	defer func() { httpClient.Transport = origTransport }()

	p := NewJiraProvider("https://test.atlassian.net", "user@test.com", "test-token")
	task, err := p.FetchTask(context.Background(), "PROJ-456")
	if err != nil {
		t.Fatalf("FetchTask() error = %v", err)
	}

	if task.Description != "Plain text description" {
		t.Errorf("Description = %q, want %q", task.Description, "Plain text description")
	}
}

func TestJiraProvider_ParseRef(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		wantProv string
		wantID   string
		wantErr  bool
	}{
		{
			name:     "shorthand prefix",
			source:   "jira:PROJ-123",
			wantProv: "jira",
			wantID:   "PROJ-123",
		},
		{
			name:     "atlassian URL",
			source:   "https://mysite.atlassian.net/browse/PROJ-456",
			wantProv: "jira",
			wantID:   "PROJ-456",
		},
		{
			name:     "atlassian URL with subdomain",
			source:   "https://company.atlassian.net/browse/ENG-789",
			wantProv: "jira",
			wantID:   "ENG-789",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prov, id, err := Parse(tt.source)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Parse(%q) error = %v, wantErr %v", tt.source, err, tt.wantErr)
			}
			if prov != tt.wantProv {
				t.Errorf("Parse(%q) provider = %q, want %q", tt.source, prov, tt.wantProv)
			}
			if id != tt.wantID {
				t.Errorf("Parse(%q) id = %q, want %q", tt.source, id, tt.wantID)
			}
		})
	}
}

func TestJiraProvider_AddComment(t *testing.T) {
	var capturedBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			_ = json.NewDecoder(r.Body).Decode(&capturedBody)
			w.WriteHeader(http.StatusCreated)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	origTransport := httpClient.Transport
	httpClient.Transport = &rewriteTransport{
		base:      http.DefaultTransport,
		targetURL: srv.URL,
	}
	defer func() { httpClient.Transport = origTransport }()

	p := NewJiraProvider("https://test.atlassian.net", "user@test.com", "test-token")
	err := p.AddComment(context.Background(), "PROJ-123", "Great progress!")
	if err != nil {
		t.Fatalf("AddComment() error = %v", err)
	}

	if capturedBody == nil {
		t.Fatal("no request body captured")
	}
	// Verify it sent an ADF body
	if _, ok := capturedBody["body"]; !ok {
		t.Error("comment payload missing 'body' field")
	}
}

func TestJiraProvider_AddComment_NoToken(t *testing.T) {
	p := NewJiraProvider("https://test.atlassian.net", "user@test.com", "")

	err := p.AddComment(context.Background(), "PROJ-123", "comment")
	if err == nil {
		t.Error("AddComment() should return error when token is empty")
	}
}

func TestExtractADFText(t *testing.T) {
	tests := []struct {
		name string
		desc any
		want string
	}{
		{
			name: "nil description",
			desc: nil,
			want: "",
		},
		{
			name: "plain string",
			desc: "simple text",
			want: "simple text",
		},
		{
			name: "ADF with paragraphs",
			desc: map[string]any{
				"type":    "doc",
				"version": float64(1),
				"content": []any{
					map[string]any{
						"type": "paragraph",
						"content": []any{
							map[string]any{
								"type": "text",
								"text": "First paragraph",
							},
						},
					},
					map[string]any{
						"type": "paragraph",
						"content": []any{
							map[string]any{
								"type": "text",
								"text": "Second paragraph",
							},
						},
					},
				},
			},
			want: "First paragraph\nSecond paragraph",
		},
		{
			name: "ADF with heading",
			desc: map[string]any{
				"type":    "doc",
				"version": float64(1),
				"content": []any{
					map[string]any{
						"type": "heading",
						"content": []any{
							map[string]any{
								"type": "text",
								"text": "Title",
							},
						},
					},
					map[string]any{
						"type": "paragraph",
						"content": []any{
							map[string]any{
								"type": "text",
								"text": "Body text",
							},
						},
					},
				},
			},
			want: "Title\nBody text",
		},
		{
			name: "unsupported type",
			desc: 42,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractADFText(tt.desc)
			if got != tt.want {
				t.Errorf("extractADFText() = %q, want %q", got, tt.want)
			}
		})
	}
}

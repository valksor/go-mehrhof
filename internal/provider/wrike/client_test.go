package wrike

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/valksor/go-mehrhof/internal/provider/token"
)

// ──────────────────────────────────────────────────────────────────────────────
// NewClient tests
// ──────────────────────────────────────────────────────────────────────────────

func TestNewClient(t *testing.T) {
	c := NewClient("test-token", "")

	if c == nil {
		t.Fatal("NewClient returned nil")
	}
	if c.httpClient == nil {
		t.Error("httpClient is nil")
	}
	if c.token != "test-token" {
		t.Errorf("token = %q, want %q", c.token, "test-token")
	}
	if c.baseURL != defaultBaseURL {
		t.Errorf("baseURL = %q, want %q", c.baseURL, defaultBaseURL)
	}
}

func TestNewClientWithCustomHost(t *testing.T) {
	customHost := "https://custom.wrike.com/api/v4"
	c := NewClient("test-token", customHost)

	if c.baseURL != customHost {
		t.Errorf("baseURL = %q, want %q", c.baseURL, customHost)
	}
}

func TestNewClientWithTrailingSlashHost(t *testing.T) {
	customHost := "https://custom.wrike.com/api/v4/"
	c := NewClient("test-token", customHost)

	// Trailing slash should be removed
	if c.baseURL != "https://custom.wrike.com/api/v4" {
		t.Errorf("baseURL = %q, want %q (trailing slash removed)", c.baseURL, "https://custom.wrike.com/api/v4")
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// ResolveToken tests
// ──────────────────────────────────────────────────────────────────────────────

func TestResolveToken(t *testing.T) {
	// Clear any existing env vars for clean test
	originalMehr := os.Getenv("MEHR_WRIKE_TOKEN")
	originalWrike := os.Getenv("WRIKE_TOKEN")
	defer func() {
		_ = os.Setenv("MEHR_WRIKE_TOKEN", originalMehr)
		_ = os.Setenv("WRIKE_TOKEN", originalWrike)
	}()

	t.Run("MEHR_WRIKE_TOKEN priority", func(t *testing.T) {
		_ = os.Setenv("MEHR_WRIKE_TOKEN", "mehr-token")
		_ = os.Setenv("WRIKE_TOKEN", "wrike-token")

		token, err := ResolveToken("config-token")
		if err != nil {
			t.Fatalf("ResolveToken error = %v", err)
		}
		if token != "mehr-token" {
			t.Errorf("token = %q, want %q", token, "mehr-token")
		}

		_ = os.Unsetenv("MEHR_WRIKE_TOKEN")
	})

	t.Run("WRIKE_TOKEN fallback", func(t *testing.T) {
		_ = os.Unsetenv("MEHR_WRIKE_TOKEN")
		_ = os.Setenv("WRIKE_TOKEN", "wrike-token")

		token, err := ResolveToken("config-token")
		if err != nil {
			t.Fatalf("ResolveToken error = %v", err)
		}
		if token != "wrike-token" {
			t.Errorf("token = %q, want %q", token, "wrike-token")
		}

		_ = os.Unsetenv("WRIKE_TOKEN")
	})

	t.Run("config token fallback", func(t *testing.T) {
		_ = os.Unsetenv("MEHR_WRIKE_TOKEN")
		_ = os.Unsetenv("WRIKE_TOKEN")

		token, err := ResolveToken("config-token")
		if err != nil {
			t.Fatalf("ResolveToken error = %v", err)
		}
		if token != "config-token" {
			t.Errorf("token = %q, want %q", token, "config-token")
		}
	})

	t.Run("no token available", func(t *testing.T) {
		_ = os.Unsetenv("MEHR_WRIKE_TOKEN")
		_ = os.Unsetenv("WRIKE_TOKEN")

		_, err := ResolveToken("")
		if err != token.ErrNoToken {
			t.Errorf("error = %v, want %v", err, token.ErrNoToken)
		}
	})
}

// ──────────────────────────────────────────────────────────────────────────────
// HTTP Mock tests for API calls
// ──────────────────────────────────────────────────────────────────────────────

// setupMockServer creates a test server with a custom handler
func setupMockServer(t *testing.T, handler http.HandlerFunc) (*Client, func()) {
	t.Helper()

	server := httptest.NewServer(handler)

	c := &Client{
		httpClient: &http.Client{Timeout: 5 * time.Second},
		baseURL:    server.URL,
		token:      "test-token",
	}

	return c, func() { server.Close() }
}

func TestGetTask(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check request
			if r.Method != http.MethodGet {
				t.Errorf("expected GET, got %s", r.Method)
			}
			if r.URL.Path != "/tasks/IEAAJTASKID" {
				t.Errorf("unexpected path: %s", r.URL.Path)
			}
			if r.Header.Get("Authorization") != "Bearer test-token" {
				t.Errorf("missing or incorrect Authorization header")
			}

			// Send response
			response := taskResponse{
				Data: []Task{
					{
						ID:          "IEAAJTASKID",
						Title:       "Test Task",
						Description: "Test description",
						Status:      "Active",
						Priority:    "High",
						Permalink:   "https://www.wrike.com/open.htm?id=1234567890",
						CreatedDate: time.Now(),
						UpdatedDate: time.Now(),
					},
				},
			}
			_ = json.NewEncoder(w).Encode(response)
		})

		client, cleanup := setupMockServer(t, handler)
		defer cleanup()

		task, err := client.GetTask(context.Background(), "IEAAJTASKID")
		if err != nil {
			t.Fatalf("GetTask error = %v", err)
		}
		if task.ID != "IEAAJTASKID" {
			t.Errorf("task.ID = %q, want %q", task.ID, "IEAAJTASKID")
		}
		if task.Title != "Test Task" {
			t.Errorf("task.Title = %q, want %q", task.Title, "Test Task")
		}
	})

	t.Run("not found", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "Not Found"})
		})

		client, cleanup := setupMockServer(t, handler)
		defer cleanup()

		_, err := client.GetTask(context.Background(), "INVALID")
		if err == nil {
			t.Error("expected error for not found task")
		}
	})

	t.Run("unauthorized", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
		})

		client, cleanup := setupMockServer(t, handler)
		defer cleanup()

		_, err := client.GetTask(context.Background(), "IEAAJTASKID")
		if err == nil {
			t.Error("expected error for unauthorized")
		}
	})

	t.Run("empty response data", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := taskResponse{
				Data: []Task{},
			}
			_ = json.NewEncoder(w).Encode(response)
		})

		client, cleanup := setupMockServer(t, handler)
		defer cleanup()

		_, err := client.GetTask(context.Background(), "IEAAJTASKID")
		if err != ErrTaskNotFound {
			t.Errorf("error = %v, want %v", err, ErrTaskNotFound)
		}
	})
}

func TestGetTaskByPermalink(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check that the endpoint is the standard task endpoint with extracted numeric ID
			if r.URL.Path != "/tasks/1234567890" {
				t.Errorf("unexpected path: %s, want /tasks/1234567890", r.URL.Path)
			}

			response := taskResponse{
				Data: []Task{
					{
						ID:          "IEAAJTASKID",
						Title:       "Task by Permalink",
						Description: "Description",
						Status:      "Active",
						Priority:    "Normal",
						Permalink:   "https://www.wrike.com/open.htm?id=1234567890",
					},
				},
			}
			_ = json.NewEncoder(w).Encode(response)
		})

		client, cleanup := setupMockServer(t, handler)
		defer cleanup()

		task, err := client.GetTaskByPermalink(context.Background(), "https://www.wrike.com/open.htm?id=1234567890")
		if err != nil {
			t.Fatalf("GetTaskByPermalink error = %v", err)
		}
		if task.Title != "Task by Permalink" {
			t.Errorf("task.Title = %q, want %q", task.Title, "Task by Permalink")
		}
	})

	t.Run("invalid permalink format", func(t *testing.T) {
		client := &Client{token: "test"}
		_, err := client.GetTaskByPermalink(context.Background(), "not-a-permalink")
		if err == nil {
			t.Error("expected error for invalid permalink format")
		}
	})
}

func TestGetComments(t *testing.T) {
	t.Run("success with comments", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := commentsResponse{
				Data: []Comment{
					{
						ID:          "IEAAJCOMMENT1",
						Text:        "First comment",
						AuthorID:    "USER1",
						AuthorName:  "Author One",
						CreatedDate: time.Now(),
						UpdatedDate: time.Now(),
					},
					{
						ID:          "IEAAJCOMMENT2",
						Text:        "Second comment",
						AuthorID:    "USER2",
						AuthorName:  "Author Two",
						CreatedDate: time.Now(),
						UpdatedDate: time.Now(),
					},
				},
			}
			_ = json.NewEncoder(w).Encode(response)
		})

		client, cleanup := setupMockServer(t, handler)
		defer cleanup()

		comments, err := client.GetComments(context.Background(), "IEAAJTASKID")
		if err != nil {
			t.Fatalf("GetComments error = %v", err)
		}
		if len(comments) != 2 {
			t.Errorf("len(comments) = %d, want 2", len(comments))
		}
	})

	t.Run("empty comments", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := commentsResponse{
				Data: []Comment{},
			}
			_ = json.NewEncoder(w).Encode(response)
		})

		client, cleanup := setupMockServer(t, handler)
		defer cleanup()

		comments, err := client.GetComments(context.Background(), "IEAAJTASKID")
		if err != nil {
			t.Fatalf("GetComments error = %v", err)
		}
		if len(comments) != 0 {
			t.Errorf("len(comments) = %d, want 0", len(comments))
		}
	})

	t.Run("API error", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		})

		client, cleanup := setupMockServer(t, handler)
		defer cleanup()

		_, err := client.GetComments(context.Background(), "IEAAJTASKID")
		if err == nil {
			t.Error("expected error for API failure")
		}
	})
}

func TestGetAttachments(t *testing.T) {
	t.Run("success with attachments", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := attachmentsResponse{
				Data: []Attachment{
					{
						ID:          "IEAAJATTACH1",
						Name:        "document.pdf",
						CreatedDate: time.Now(),
						Size:        1024,
					},
					{
						ID:          "IEAAJATTACH2",
						Name:        "image.png",
						CreatedDate: time.Now(),
						Size:        2048,
					},
				},
			}
			_ = json.NewEncoder(w).Encode(response)
		})

		client, cleanup := setupMockServer(t, handler)
		defer cleanup()

		attachments, err := client.GetAttachments(context.Background(), "IEAAJTASKID")
		if err != nil {
			t.Fatalf("GetAttachments error = %v", err)
		}
		if len(attachments) != 2 {
			t.Errorf("len(attachments) = %d, want 2", len(attachments))
		}
	})

	t.Run("empty attachments", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := attachmentsResponse{
				Data: []Attachment{},
			}
			_ = json.NewEncoder(w).Encode(response)
		})

		client, cleanup := setupMockServer(t, handler)
		defer cleanup()

		attachments, err := client.GetAttachments(context.Background(), "IEAAJTASKID")
		if err != nil {
			t.Fatalf("GetAttachments error = %v", err)
		}
		if len(attachments) != 0 {
			t.Errorf("len(attachments) = %d, want 0", len(attachments))
		}
	})
}

func TestDownloadAttachment(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check path
			if r.URL.Path != "/attachments/IEAAJATTACH1/download" {
				t.Errorf("unexpected path: %s", r.URL.Path)
			}

			// Set Content-Disposition header
			w.Header().Set("Content-Disposition", "attachment; filename=\"test.pdf\"")
			_, _ = w.Write([]byte("test file content"))
		})

		client, cleanup := setupMockServer(t, handler)
		defer cleanup()

		rc, disposition, err := client.DownloadAttachment(context.Background(), "IEAAJATTACH1")
		if err != nil {
			t.Fatalf("DownloadAttachment error = %v", err)
		}
		defer func() { _ = rc.Close() }()

		if disposition == "" {
			t.Error("Content-Disposition header is empty")
		}

		// Read content to verify
		content := make([]byte, 100)
		n, _ := rc.Read(content)
		if n == 0 {
			t.Error("received empty content")
		}
	})

	t.Run("not found", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})

		client, cleanup := setupMockServer(t, handler)
		defer cleanup()

		rc, _, err := client.DownloadAttachment(context.Background(), "INVALID")
		if err == nil {
			_ = rc.Close()
			t.Error("expected error for not found attachment")
		}
	})
}

func TestPostComment(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("expected POST, got %s", r.Method)
			}

			// Parse request body
			var req map[string]string
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Errorf("failed to decode request: %v", err)
			}
			if req["text"] != "Test comment" {
				t.Errorf("request text = %q, want %q", req["text"], "Test comment")
			}

			response := commentResponse{
				Data: []Comment{
					{
						ID:          "IEAAJNEWCOMMENT",
						Text:        "Test comment",
						AuthorID:    "USER1",
						AuthorName:  "Test User",
						CreatedDate: time.Now(),
						UpdatedDate: time.Now(),
					},
				},
			}
			_ = json.NewEncoder(w).Encode(response)
		})

		client, cleanup := setupMockServer(t, handler)
		defer cleanup()

		comment, err := client.PostComment(context.Background(), "IEAAJTASKID", "Test comment")
		if err != nil {
			t.Fatalf("PostComment error = %v", err)
		}
		if comment.ID != "IEAAJNEWCOMMENT" {
			t.Errorf("comment.ID = %q, want %q", comment.ID, "IEAAJNEWCOMMENT")
		}
		if comment.Text != "Test comment" {
			t.Errorf("comment.Text = %q, want %q", comment.Text, "Test comment")
		}
	})

	t.Run("unauthorized", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		})

		client, cleanup := setupMockServer(t, handler)
		defer cleanup()

		_, err := client.PostComment(context.Background(), "IEAAJTASKID", "Test")
		if err == nil {
			t.Error("expected error for unauthorized")
		}
	})

	t.Run("empty response data", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := commentResponse{
				Data: []Comment{},
			}
			_ = json.NewEncoder(w).Encode(response)
		})

		client, cleanup := setupMockServer(t, handler)
		defer cleanup()

		_, err := client.PostComment(context.Background(), "IEAAJTASKID", "Test")
		if err == nil {
			t.Error("expected error for empty response")
		}
	})
}

// ──────────────────────────────────────────────────────────────────────────────
// doRequestWithRetry tests
// ──────────────────────────────────────────────────────────────────────────────

func TestDoRequestWithRetry(t *testing.T) {
	t.Run("retry on rate limit with eventual success", func(t *testing.T) {
		attempts := 0
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attempts++
			if attempts < 2 {
				// First attempt: rate limited
				w.WriteHeader(http.StatusTooManyRequests)
				return
			}
			// Second attempt: success
			response := taskResponse{
				Data: []Task{
					{
						ID:          "IEAAJTASKID",
						Title:       "Test Task",
						Description: "Description",
						Status:      "Active",
						Priority:    "Normal",
						Permalink:   "https://www.wrike.com/open.htm?id=1234567890",
					},
				},
			}
			_ = json.NewEncoder(w).Encode(response)
		})

		client, cleanup := setupMockServer(t, handler)
		defer cleanup()

		// Use a short timeout for testing
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		var response taskResponse
		err := client.doRequestWithRetry(ctx, http.MethodGet, "/tasks/IEAAJTASKID", nil, &response)
		if err != nil {
			t.Fatalf("doRequestWithRetry error = %v", err)
		}

		if attempts != 2 {
			t.Errorf("expected 2 attempts, got %d", attempts)
		}

		if len(response.Data) != 1 {
			t.Errorf("expected 1 task, got %d", len(response.Data))
		}
	})

	t.Run("non-retryable error fails immediately", func(t *testing.T) {
		attempts := 0
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attempts++
			w.WriteHeader(http.StatusUnauthorized)
		})

		client, cleanup := setupMockServer(t, handler)
		defer cleanup()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		var response taskResponse
		err := client.doRequestWithRetry(ctx, http.MethodGet, "/tasks/IEAAJTASKID", nil, &response)
		if err == nil {
			t.Error("expected error for unauthorized")
		}

		if attempts != 1 {
			t.Errorf("expected 1 attempt for non-retryable error, got %d", attempts)
		}
	})

	t.Run("max retries exceeded", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTooManyRequests)
		})

		client, cleanup := setupMockServer(t, handler)
		defer cleanup()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		var response taskResponse
		err := client.doRequestWithRetry(ctx, http.MethodGet, "/tasks/IEAAJTASKID", nil, &response)
		if err == nil {
			t.Error("expected error after max retries")
		}
	})
}

// ──────────────────────────────────────────────────────────────────────────────
// GetTasksInFolder tests
// ──────────────────────────────────────────────────────────────────────────────

func TestGetTasksInFolder(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/folders/IEAAJFOLDERID/tasks" {
				t.Errorf("unexpected path: %s", r.URL.Path)
			}

			response := taskResponse{
				Data: []Task{
					{
						ID:          "IEAAJTASK1",
						Title:       "Task 1",
						Description: "Description 1",
						Status:      "Active",
						Priority:    "Normal",
						Permalink:   "https://www.wrike.com/open.htm?id=1234567890",
					},
					{
						ID:          "IEAAJTASK2",
						Title:       "Task 2",
						Description: "Description 2",
						Status:      "Completed",
						Priority:    "High",
						Permalink:   "https://www.wrike.com/open.htm?id=1234567891",
					},
				},
			}
			_ = json.NewEncoder(w).Encode(response)
		})

		client, cleanup := setupMockServer(t, handler)
		defer cleanup()

		tasks, err := client.GetTasksInFolder(context.Background(), "IEAAJFOLDERID")
		if err != nil {
			t.Fatalf("GetTasksInFolder error = %v", err)
		}

		if len(tasks) != 2 {
			t.Errorf("got %d tasks, want 2", len(tasks))
		}

		if tasks[0].Title != "Task 1" {
			t.Errorf("tasks[0].Title = %q, want %q", tasks[0].Title, "Task 1")
		}
	})

	t.Run("API error", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})

		client, cleanup := setupMockServer(t, handler)
		defer cleanup()

		_, err := client.GetTasksInFolder(context.Background(), "IEAAJFOLDERID")
		if err == nil {
			t.Error("expected error for not found")
		}
	})
}

// ──────────────────────────────────────────────────────────────────────────────
// GetTasksInSpace tests
// ──────────────────────────────────────────────────────────────────────────────

func TestGetTasksInSpace(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/spaces/IEAAJSPACEID/tasks" {
				t.Errorf("unexpected path: %s", r.URL.Path)
			}

			response := taskResponse{
				Data: []Task{
					{
						ID:          "IEAAJTASK1",
						Title:       "Space Task 1",
						Description: "Description 1",
						Status:      "Active",
						Priority:    "Normal",
						Permalink:   "https://www.wrike.com/open.htm?id=1234567890",
					},
				},
			}
			_ = json.NewEncoder(w).Encode(response)
		})

		client, cleanup := setupMockServer(t, handler)
		defer cleanup()

		tasks, err := client.GetTasksInSpace(context.Background(), "IEAAJSPACEID")
		if err != nil {
			t.Fatalf("GetTasksInSpace error = %v", err)
		}

		if len(tasks) != 1 {
			t.Errorf("got %d tasks, want 1", len(tasks))
		}

		if tasks[0].Title != "Space Task 1" {
			t.Errorf("tasks[0].Title = %q, want %q", tasks[0].Title, "Space Task 1")
		}
	})
}

// ──────────────────────────────────────────────────────────────────────────────
// GetComments with pagination tests
// ──────────────────────────────────────────────────────────────────────────────

func TestGetCommentsPagination(t *testing.T) {
	t.Run("paginated comments", func(t *testing.T) {
		callCount := 0
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++
			if callCount == 1 {
				// First page with nextPage link
				response := commentsResponse{
					Data: []Comment{
						{
							ID:          "IEAAJCOMMENT1",
							Text:        "First comment",
							AuthorID:    "USER1",
							AuthorName:  "Author One",
							CreatedDate: time.Now(),
							UpdatedDate: time.Now(),
						},
					},
					NextPage: "/tasks/IEAAJTASKID/comments?page=2",
				}
				_ = json.NewEncoder(w).Encode(response)
			} else {
				// Second page without nextPage
				response := commentsResponse{
					Data: []Comment{
						{
							ID:          "IEAAJCOMMENT2",
							Text:        "Second comment",
							AuthorID:    "USER2",
							AuthorName:  "Author Two",
							CreatedDate: time.Now(),
							UpdatedDate: time.Now(),
						},
					},
				}
				_ = json.NewEncoder(w).Encode(response)
			}
		})

		client, cleanup := setupMockServer(t, handler)
		defer cleanup()

		comments, err := client.GetComments(context.Background(), "IEAAJTASKID")
		if err != nil {
			t.Fatalf("GetComments error = %v", err)
		}

		if callCount != 2 {
			t.Errorf("expected 2 API calls for pagination, got %d", callCount)
		}

		if len(comments) != 2 {
			t.Errorf("got %d comments, want 2", len(comments))
		}

		if comments[0].ID != "IEAAJCOMMENT1" {
			t.Errorf("comments[0].ID = %q, want %q", comments[0].ID, "IEAAJCOMMENT1")
		}

		if comments[1].ID != "IEAAJCOMMENT2" {
			t.Errorf("comments[1].ID = %q, want %q", comments[1].ID, "IEAAJCOMMENT2")
		}
	})

	t.Run("single page comments", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := commentsResponse{
				Data: []Comment{
					{
						ID:          "IEAAJCOMMENT1",
						Text:        "Only comment",
						AuthorID:    "USER1",
						AuthorName:  "Author One",
						CreatedDate: time.Now(),
						UpdatedDate: time.Now(),
					},
				},
				// No nextPage - this is the last page
			}
			_ = json.NewEncoder(w).Encode(response)
		})

		client, cleanup := setupMockServer(t, handler)
		defer cleanup()

		comments, err := client.GetComments(context.Background(), "IEAAJTASKID")
		if err != nil {
			t.Fatalf("GetComments error = %v", err)
		}

		if len(comments) != 1 {
			t.Errorf("got %d comments, want 1", len(comments))
		}
	})
}

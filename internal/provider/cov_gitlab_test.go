package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestGitLabProvider_CreatePR_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/merge_requests"):
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			_ = json.NewEncoder(w).Encode(map[string]any{ //nolint:errchkjson // test helper
				"iid":     7,
				"state":   "opened",
				"web_url": "https://gitlab.example.com/group/repo/-/merge_requests/7",
				"title":   body["title"],
				"draft":   false,
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	p, srv := newGitLabTestProvider(t, handler)
	defer srv.Close()

	res, err := p.CreatePR(context.Background(), PROptions{
		Head:    "group/repo:feature-branch",
		Base:    "main",
		Title:   "Add feature",
		Body:    "the body",
		TaskURL: "https://gitlab.example.com/group/repo/-/issues/3",
	})
	if err != nil {
		t.Fatalf("CreatePR() error = %v", err)
	}
	if res.Number != 7 {
		t.Errorf("Number = %d, want 7", res.Number)
	}
	if res.ID != "group/repo!7" {
		t.Errorf("ID = %q, want group/repo!7", res.ID)
	}
	if res.State != "opened" {
		t.Errorf("State = %q, want opened", res.State)
	}
}

func TestGitLabProvider_CreatePR_Draft_DetectsDefaultBranch(t *testing.T) {
	var createdTarget string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/projects/") &&
			!strings.Contains(r.URL.Path, "/merge_requests"):
			// getDefaultBranch -> GetProject
			_ = json.NewEncoder(w).Encode(map[string]any{ //nolint:errchkjson // test helper
				"id":             1,
				"default_branch": "develop",
			})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/merge_requests"):
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if tb, ok := body["target_branch"].(string); ok {
				createdTarget = tb
			}
			_ = json.NewEncoder(w).Encode(map[string]any{ //nolint:errchkjson // test helper
				"iid":     8,
				"state":   "opened",
				"web_url": "https://gitlab.example.com/group/repo/-/merge_requests/8",
				"title":   body["title"],
				"draft":   true,
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	p, srv := newGitLabTestProvider(t, handler)
	defer srv.Close()

	// No Base => CreatePR calls getDefaultBranch, which returns "develop".
	res, err := p.CreatePR(context.Background(), PROptions{
		TaskID: "group/repo#3",
		Head:   "feature",
		Title:  "Work in progress",
		Draft:  true,
	})
	if err != nil {
		t.Fatalf("CreatePR() error = %v", err)
	}
	if createdTarget != "develop" {
		t.Errorf("target_branch = %q, want develop (detected default)", createdTarget)
	}
	if res.State != "draft" {
		t.Errorf("State = %q, want draft", res.State)
	}
}

func TestGitLabProvider_CreatePR_DefaultBranchFallback(t *testing.T) {
	var createdTarget string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/projects/") &&
			!strings.Contains(r.URL.Path, "/merge_requests"):
			// getDefaultBranch fails -> CreatePR should fall back to "main".
			w.WriteHeader(http.StatusInternalServerError)
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/merge_requests"):
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if tb, ok := body["target_branch"].(string); ok {
				createdTarget = tb
			}
			_ = json.NewEncoder(w).Encode(map[string]any{ //nolint:errchkjson // test helper
				"iid":   9,
				"state": "opened",
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	p, srv := newGitLabTestProvider(t, handler)
	defer srv.Close()

	if _, err := p.CreatePR(context.Background(), PROptions{
		TaskID: "group/repo#3",
		Head:   "feature",
		Title:  "Fallback test",
	}); err != nil {
		t.Fatalf("CreatePR() error = %v", err)
	}
	if createdTarget != "main" {
		t.Errorf("target_branch = %q, want main (fallback)", createdTarget)
	}
}

func TestGitLabProvider_CreatePR_CreateError(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/merge_requests") {
			w.WriteHeader(http.StatusUnprocessableEntity)

			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	p, srv := newGitLabTestProvider(t, handler)
	defer srv.Close()

	if _, err := p.CreatePR(context.Background(), PROptions{
		Head:  "group/repo:feature",
		Base:  "main",
		Title: "Will fail",
	}); err == nil {
		t.Error("CreatePR() should error when the MR creation fails")
	}
}

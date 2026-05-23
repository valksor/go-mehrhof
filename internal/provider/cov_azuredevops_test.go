package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAzureDevOpsProvider_CreatePR_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || !strings.Contains(r.URL.Path, "/pullrequests") {
			http.Error(w, "unexpected request", http.StatusBadRequest)

			return
		}
		var payload map[string]any
		_ = json.NewDecoder(r.Body).Decode(&payload)
		if payload["sourceRefName"] != "refs/heads/feature" {
			t.Errorf("sourceRefName = %v, want refs/heads/feature", payload["sourceRefName"])
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(azurePR{
			PullRequestID: 77,
			Status:        "active",
			URL:           "https://dev.azure.com/org/proj/_git/myrepo/pullrequest/77",
			Title:         "My PR",
		})
	}))
	defer srv.Close()

	p := NewAzureDevOpsProvider(srv.URL, "org", "proj", "secret", "myrepo")
	res, err := p.CreatePR(context.Background(), PROptions{
		Head:  "feature",
		Base:  "main",
		Title: "My PR",
		Body:  "description",
	})
	if err != nil {
		t.Fatalf("CreatePR() error = %v", err)
	}
	if res.Number != 77 {
		t.Errorf("Number = %d, want 77", res.Number)
	}
	if res.ID != "77" {
		t.Errorf("ID = %q, want 77", res.ID)
	}
	if res.State != "active" {
		t.Errorf("State = %q, want active", res.State)
	}
}

func TestAzureDevOpsProvider_CreatePR_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "conflict", http.StatusConflict)
	}))
	defer srv.Close()

	p := NewAzureDevOpsProvider(srv.URL, "org", "proj", "secret", "myrepo")
	if _, err := p.CreatePR(context.Background(), PROptions{Head: "f", Base: "main"}); err == nil {
		t.Error("CreatePR() should error on non-2xx response")
	}
}

func TestAzureDevOpsClient_UpdateWorkItemState_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "forbidden", http.StatusForbidden)
	}))
	defer srv.Close()

	c := newAzureDevOpsClient(srv.URL, "org", "proj", "tok")
	if err := c.UpdateWorkItemState(context.Background(), "42", "Closed"); err == nil {
		t.Error("UpdateWorkItemState() should error on non-200")
	}
}

func TestAzureDevOpsClient_AddWorkItemComment(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost || !strings.Contains(r.URL.Path, "/comments") {
				http.Error(w, "unexpected", http.StatusBadRequest)

				return
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte("{}"))
		}))
		defer srv.Close()

		c := newAzureDevOpsClient(srv.URL, "org", "proj", "tok")
		if err := c.AddWorkItemComment(context.Background(), "42", "a comment"); err != nil {
			t.Fatalf("AddWorkItemComment() error = %v", err)
		}
	})

	t.Run("http error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "boom", http.StatusInternalServerError)
		}))
		defer srv.Close()

		c := newAzureDevOpsClient(srv.URL, "org", "proj", "tok")
		if err := c.AddWorkItemComment(context.Background(), "42", "a comment"); err == nil {
			t.Error("AddWorkItemComment() should error on 500")
		}
	})
}

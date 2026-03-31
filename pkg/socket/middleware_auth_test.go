package socket

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/valksor/kvelmo/pkg/access"
)

func TestAuthMiddleware_NilStore_PassesThrough(t *testing.T) {
	mw := AuthMiddleware(nil)
	called := false
	handler := mw(func(_ context.Context, req *Request) *Response {
		called = true

		return &Response{ID: req.ID}
	})

	resp := handler(context.Background(), &Request{ID: "1", Method: "test"})
	if !called {
		t.Error("handler should have been called when store is nil")
	}
	if resp.Error != nil {
		t.Errorf("unexpected error: %v", resp.Error)
	}
}

func TestAuthMiddleware_MissingToken_Rejects(t *testing.T) {
	store := access.New(filepath.Join(t.TempDir(), "tokens.json"))

	mw := AuthMiddleware(store)
	handler := mw(func(_ context.Context, _ *Request) *Response {
		t.Error("handler should not be called without token")

		return nil
	})

	resp := handler(context.Background(), &Request{
		ID:     "1",
		Method: "test",
		Params: json.RawMessage(`{}`),
	})
	if resp.Error == nil {
		t.Fatal("expected error response")
	}
	if resp.Error.Code != ErrCodeUnauthorized {
		t.Errorf("error code = %d, want %d", resp.Error.Code, ErrCodeUnauthorized)
	}
}

func TestAuthMiddleware_InvalidToken_Rejects(t *testing.T) {
	store := access.New(filepath.Join(t.TempDir(), "tokens.json"))

	mw := AuthMiddleware(store)
	handler := mw(func(_ context.Context, _ *Request) *Response {
		t.Error("handler should not be called with invalid token")

		return nil
	})

	resp := handler(context.Background(), &Request{
		ID:     "1",
		Method: "test",
		Params: json.RawMessage(`{"_token":"bad-token"}`),
	})
	if resp.Error == nil {
		t.Fatal("expected error response")
	}
	if resp.Error.Code != ErrCodeUnauthorized {
		t.Errorf("error code = %d, want %d", resp.Error.Code, ErrCodeUnauthorized)
	}
}

func TestAuthMiddleware_ValidToken_Passes(t *testing.T) {
	store := access.New(filepath.Join(t.TempDir(), "tokens.json"))
	token, err := store.Create(access.RoleOperator, "test", nil)
	if err != nil {
		t.Fatalf("create token: %v", err)
	}

	mw := AuthMiddleware(store)
	called := false
	handler := mw(func(ctx context.Context, req *Request) *Response {
		called = true

		role := RoleFromContext(ctx)
		if role != access.RoleOperator {
			t.Errorf("role = %q, want %q", role, access.RoleOperator)
		}

		// Verify _token was stripped from params
		if req.Params != nil {
			var remaining map[string]json.RawMessage
			if err := json.Unmarshal(req.Params, &remaining); err == nil {
				if _, hasToken := remaining["_token"]; hasToken {
					t.Error("_token should have been stripped from params")
				}
			}
		}

		return &Response{ID: req.ID}
	})

	params := json.RawMessage(`{"_token":"` + token + `"}`)
	resp := handler(context.Background(), &Request{
		ID:     "1",
		Method: "test",
		Params: params,
	})
	if !called {
		t.Error("handler should have been called with valid token")
	}
	if resp.Error != nil {
		t.Errorf("unexpected error: %v", resp.Error)
	}
}

func TestExtractToken_NilParams(t *testing.T) {
	token := extractToken(&Request{})
	if token != "" {
		t.Errorf("expected empty token, got %q", token)
	}
}

func TestExtractToken_NoTokenField(t *testing.T) {
	token := extractToken(&Request{
		Params: json.RawMessage(`{"method":"test"}`),
	})
	if token != "" {
		t.Errorf("expected empty token, got %q", token)
	}
}

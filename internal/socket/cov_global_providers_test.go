package socket

import (
	"context"
	"encoding/json"
	"testing"
)

// --- global_providers.go ---

func TestGlobalHandleProvidersTest(t *testing.T) {
	t.Run("missing provider", func(t *testing.T) {
		g := newTestGlobalSocket(t)
		params, _ := json.Marshal(map[string]string{"provider": ""})
		resp, err := g.handleProvidersTest(context.Background(), &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleProvidersTest() error = %v", err)
		}
		var result map[string]any
		_ = json.Unmarshal(resp.Result, &result)
		if ok, _ := result["ok"].(bool); ok {
			t.Error("ok = true, want false for missing provider")
		}
	})

	t.Run("no token configured", func(t *testing.T) {
		g := newTestGlobalSocket(t)
		// An obscure provider with no configured token resolves to empty.
		params, _ := json.Marshal(map[string]string{"provider": "github", "token": ""})
		resp, err := g.handleProvidersTest(context.Background(), &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleProvidersTest() error = %v", err)
		}
		// Result depends on whether GITHUB_TOKEN is present in the environment.
		// We only assert a well-formed result (no panic, has ok key).
		var result map[string]any
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if _, ok := result["ok"]; !ok {
			t.Error("expected ok key in result")
		}
	})

	t.Run("explicit token under cancelled context fails connection", func(t *testing.T) {
		g := newTestGlobalSocket(t)
		ctx := cancelledCtx(t)
		params, _ := json.Marshal(map[string]string{"provider": "github", "token": "ghp_faketoken"})
		resp, err := g.handleProvidersTest(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleProvidersTest() error = %v", err)
		}
		var result map[string]any
		_ = json.Unmarshal(resp.Result, &result)
		// The HTTP probe is cancelled, so authentication cannot succeed.
		if ok, _ := result["ok"].(bool); ok {
			t.Error("ok = true, want false under cancelled context")
		}
	})

	t.Run("invalid params", func(t *testing.T) {
		g := newTestGlobalSocket(t)
		_, err := g.handleProvidersTest(context.Background(), &Request{ID: "1", Params: json.RawMessage(`bad`)})
		if err == nil {
			t.Fatal("expected Go error for unparseable params")
		}
	})
}

func TestGlobalHandleProviderLogin(t *testing.T) {
	t.Run("invalid params", func(t *testing.T) {
		g := newTestGlobalSocket(t)
		_, err := g.handleProviderLogin(context.Background(), &Request{ID: "1", Params: json.RawMessage(`bad`)})
		if err == nil {
			t.Fatal("expected Go error for unparseable params")
		}
	})

	t.Run("missing provider", func(t *testing.T) {
		g := newTestGlobalSocket(t)
		params, _ := json.Marshal(map[string]string{"provider": "", "token": "x"})
		resp, err := g.handleProviderLogin(context.Background(), &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleProviderLogin() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for missing provider")
		}
	})

	t.Run("missing token", func(t *testing.T) {
		g := newTestGlobalSocket(t)
		params, _ := json.Marshal(map[string]string{"provider": "github", "token": ""})
		resp, err := g.handleProviderLogin(context.Background(), &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleProviderLogin() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for missing token")
		}
	})

	t.Run("unknown provider", func(t *testing.T) {
		g := newTestGlobalSocket(t)
		params, _ := json.Marshal(map[string]string{"provider": "bogusprovider", "token": "x"})
		resp, err := g.handleProviderLogin(context.Background(), &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleProviderLogin() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for unknown provider")
		}
	})
}

func TestResolveProviderToken(t *testing.T) {
	// Unknown provider has no env var mapping → empty token.
	if got := resolveProviderToken("bogusprovider"); got != "" {
		t.Errorf("resolveProviderToken(bogus) = %q, want empty", got)
	}
}

func TestTestProviderToken_UnknownProvider(t *testing.T) {
	ok, detail := testProviderToken(context.Background(), "bogus", "tok")
	if ok {
		t.Error("ok = true, want false for unknown provider")
	}
	if detail == "" {
		t.Error("expected a detail message for unknown provider")
	}
}

func TestTestHTTPToken_CancelledContext(t *testing.T) {
	ctx := cancelledCtx(t)
	ok, detail := testHTTPToken(ctx, "https://api.github.com/user", "tok", "token")
	if ok {
		t.Error("ok = true, want false under cancelled context")
	}
	if detail == "" {
		t.Error("expected a detail message")
	}
}

func TestTestLinearToken_CancelledContext(t *testing.T) {
	ctx := cancelledCtx(t)
	ok, _ := testLinearToken(ctx, "tok")
	if ok {
		t.Error("ok = true, want false under cancelled context")
	}
}

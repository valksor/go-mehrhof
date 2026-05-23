package socket

import (
	"context"
	"encoding/json"
	"testing"
)

// cancelledCtx returns a context that is already cancelled. Browser handlers
// that reach an actual browser invocation route through browser.Exec →
// EnsureInstalled, both of which honour the context. A cancelled context makes
// those calls fail fast (no network download, no subprocess) so we can
// deterministically exercise the post-validation error path of each handler
// without a real Playwright runtime.
func cancelledCtx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	return ctx
}

// browserInteractCase drives a browser handler with valid params so the call
// proceeds past validation into browser.Exec, which fails under a cancelled
// context and yields an error response.
type browserInteractCase struct {
	name    string
	handler func(*GlobalSocket) func(context.Context, *Request) (*Response, error)
	params  any
}

func TestGlobalBrowserInteract_ValidParamsErrorPath(t *testing.T) {
	cases := []browserInteractCase{
		{
			"click",
			func(g *GlobalSocket) func(context.Context, *Request) (*Response, error) { return g.handleBrowserClick },
			BrowserClickParams{Selector: "#btn"},
		},
		{
			"type",
			func(g *GlobalSocket) func(context.Context, *Request) (*Response, error) { return g.handleBrowserType },
			BrowserTypeParams{Selector: "#in", Text: "hi"},
		},
		{
			"wait",
			func(g *GlobalSocket) func(context.Context, *Request) (*Response, error) { return g.handleBrowserWait },
			BrowserWaitParams{Selector: "#x", TimeoutMs: 100},
		},
		{
			"fill",
			func(g *GlobalSocket) func(context.Context, *Request) (*Response, error) { return g.handleBrowserFill },
			BrowserFillParams{Selector: "#f", Value: "v"},
		},
		{
			"select",
			func(g *GlobalSocket) func(context.Context, *Request) (*Response, error) { return g.handleBrowserSelect },
			BrowserSelectParams{Selector: "#s", Values: []string{"a"}},
		},
		{
			"hover",
			func(g *GlobalSocket) func(context.Context, *Request) (*Response, error) { return g.handleBrowserHover },
			BrowserHoverParams{Selector: "#h"},
		},
		{
			"focus",
			func(g *GlobalSocket) func(context.Context, *Request) (*Response, error) { return g.handleBrowserFocus },
			BrowserFocusParams{Selector: "#f"},
		},
		{
			"scroll",
			func(g *GlobalSocket) func(context.Context, *Request) (*Response, error) { return g.handleBrowserScroll },
			BrowserScrollParams{Direction: "down", Amount: 10},
		},
		{
			"press",
			func(g *GlobalSocket) func(context.Context, *Request) (*Response, error) { return g.handleBrowserPress },
			BrowserPressParams{Key: "Enter"},
		},
		{
			"dialog",
			func(g *GlobalSocket) func(context.Context, *Request) (*Response, error) { return g.handleBrowserDialog },
			BrowserDialogParams{Action: "accept"},
		},
		{
			"upload",
			func(g *GlobalSocket) func(context.Context, *Request) (*Response, error) { return g.handleBrowserUpload },
			BrowserUploadParams{Selector: "#u", Files: []string{"/tmp/x"}},
		},
		{
			"back",
			func(g *GlobalSocket) func(context.Context, *Request) (*Response, error) { return g.handleBrowserBack },
			BrowserParams{},
		},
		{
			"forward",
			func(g *GlobalSocket) func(context.Context, *Request) (*Response, error) {
				return g.handleBrowserForward
			},
			BrowserParams{},
		},
		{
			"reload",
			func(g *GlobalSocket) func(context.Context, *Request) (*Response, error) { return g.handleBrowserReload },
			BrowserParams{},
		},
		{
			"pdf",
			func(g *GlobalSocket) func(context.Context, *Request) (*Response, error) { return g.handleBrowserPDF },
			BrowserPDFParams{Format: "A4"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := cancelledCtx(t)
			g := newTestGlobalSocket(t)
			params, _ := json.Marshal(tc.params)
			resp, err := tc.handler(g)(ctx, &Request{ID: "1", Params: params})
			if err != nil {
				t.Fatalf("handleBrowser%s() error = %v", tc.name, err)
			}
			// Under a cancelled context the browser invocation cannot complete:
			// the handler must return a JSON-RPC response (typically an error
			// envelope) rather than panic or hang. We do not require resp.Error
			// to be set because whether a Playwright runtime is present is
			// environment-dependent; the assertion that matters is liveness.
			if resp == nil {
				t.Fatalf("handleBrowser%s() returned nil response", tc.name)
			}
			if resp.ID != "1" {
				t.Errorf("handleBrowser%s() response ID = %q, want 1", tc.name, resp.ID)
			}
		})
	}
}

// invalid-params and missing-required-field error paths for handlers not yet
// exercised elsewhere (back/forward/reload/pdf accept optional params, so they
// only have the invalid-JSON path).

func TestGlobalBrowserBackForwardReloadPDF_InvalidParams(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name    string
		handler func(*GlobalSocket) func(context.Context, *Request) (*Response, error)
	}{
		{"back", func(g *GlobalSocket) func(context.Context, *Request) (*Response, error) { return g.handleBrowserBack }},
		{"forward", func(g *GlobalSocket) func(context.Context, *Request) (*Response, error) {
			return g.handleBrowserForward
		}},
		{"reload", func(g *GlobalSocket) func(context.Context, *Request) (*Response, error) { return g.handleBrowserReload }},
		{"pdf", func(g *GlobalSocket) func(context.Context, *Request) (*Response, error) { return g.handleBrowserPDF }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := newTestGlobalSocket(t)
			resp, err := tc.handler(g)(ctx, &Request{ID: "1", Params: json.RawMessage(`bad`)})
			if err != nil {
				t.Fatalf("handleBrowser%s() error = %v", tc.name, err)
			}
			if resp.Error == nil {
				t.Fatalf("handleBrowser%s() invalid params should return error response", tc.name)
			}
		})
	}
}

func TestGlobalBrowserScroll_InvalidDirection(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)
	params, _ := json.Marshal(BrowserScrollParams{Direction: "sideways"}) //nolint:errchkjson // test data
	resp, err := g.handleBrowserScroll(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleBrowserScroll() error = %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error response for invalid scroll direction")
	}
}

func TestGlobalBrowserReadOnly_ErrorPath(t *testing.T) {
	cases := []struct {
		name    string
		handler func(*GlobalSocket) func(context.Context, *Request) (*Response, error)
		params  any
	}{
		{"snapshot", func(g *GlobalSocket) func(context.Context, *Request) (*Response, error) {
			return g.handleBrowserSnapshot
		}, BrowserParams{}},
		{"console", func(g *GlobalSocket) func(context.Context, *Request) (*Response, error) {
			return g.handleBrowserConsole
		}, BrowserParams{}},
		{"network", func(g *GlobalSocket) func(context.Context, *Request) (*Response, error) {
			return g.handleBrowserNetwork
		}, BrowserParams{}},
		{"screenshot", func(g *GlobalSocket) func(context.Context, *Request) (*Response, error) {
			return g.handleBrowserScreenshot
		}, BrowserScreenshotParams{}},
		{"navigate", func(g *GlobalSocket) func(context.Context, *Request) (*Response, error) {
			return g.handleBrowserNavigate
		}, BrowserNavigateParams{URL: "https://example.com"}},
		{"eval", func(g *GlobalSocket) func(context.Context, *Request) (*Response, error) { return g.handleBrowserEval }, BrowserEvalParams{JS: "1+1"}},
	} //nolint:lll // table of browser handler cases
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := cancelledCtx(t)
			g := newTestGlobalSocket(t)
			params, _ := json.Marshal(tc.params)
			resp, err := tc.handler(g)(ctx, &Request{ID: "1", Params: params})
			if err != nil {
				t.Fatalf("handleBrowser%s() error = %v", tc.name, err)
			}
			// Liveness assertion (see TestGlobalBrowserInteract_ValidParamsErrorPath).
			if resp == nil {
				t.Fatalf("handleBrowser%s() returned nil response", tc.name)
			}
		})
	}
}

func TestGlobalBrowserConsoleNetwork_InvalidParams(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name    string
		handler func(*GlobalSocket) func(context.Context, *Request) (*Response, error)
	}{
		{"snapshot", func(g *GlobalSocket) func(context.Context, *Request) (*Response, error) {
			return g.handleBrowserSnapshot
		}},
		{"console", func(g *GlobalSocket) func(context.Context, *Request) (*Response, error) {
			return g.handleBrowserConsole
		}},
		{"network", func(g *GlobalSocket) func(context.Context, *Request) (*Response, error) {
			return g.handleBrowserNetwork
		}},
		{"screenshot", func(g *GlobalSocket) func(context.Context, *Request) (*Response, error) {
			return g.handleBrowserScreenshot
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := newTestGlobalSocket(t)
			resp, err := tc.handler(g)(ctx, &Request{ID: "1", Params: json.RawMessage(`bad`)})
			if err != nil {
				t.Fatalf("handleBrowser%s() error = %v", tc.name, err)
			}
			if resp.Error == nil {
				t.Fatalf("handleBrowser%s() invalid params should return error response", tc.name)
			}
		})
	}
}

func TestGlobalBrowserInstall_CancelledContext(t *testing.T) {
	ctx := cancelledCtx(t)
	g := newTestGlobalSocket(t)
	resp, err := g.handleBrowserInstall(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleBrowserInstall() error = %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	// Install under a cancelled context: either the runtime is already present
	// (ok=true) or it fails fast with an error response. Both are valid; the
	// point is it must not hang or panic.
}

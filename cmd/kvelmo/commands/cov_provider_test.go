package commands

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/valksor/kvelmo/internal/provider"
	"github.com/valksor/kvelmo/settings"
)

// withProviderURL temporarily points a provider's validation endpoint at a test
// server and restores it afterwards.
func withProviderURL(t *testing.T, name, url string) {
	t.Helper()
	orig, had := providerValidateURLs[name]
	t.Cleanup(func() {
		if had {
			providerValidateURLs[name] = orig
		} else {
			delete(providerValidateURLs, name)
		}
	})
	providerValidateURLs[name] = url
}

// TestTestProviderToken_GitHubSuccess validates the GitHub arm against a stub
// server returning 200.
func TestTestProviderToken_GitHubSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)

			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"login":"octocat"}`))
	}))
	defer srv.Close()

	withProviderURL(t, provider.NameGitHub, srv.URL)

	if err := testProviderToken(provider.NameGitHub, "ghp_valid"); err != nil {
		t.Errorf("expected success for valid github token, got %v", err)
	}
}

// TestTestProviderToken_Unauthorized covers the 401 branch.
func TestTestProviderToken_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	withProviderURL(t, provider.NameGitLab, srv.URL)

	err := testProviderToken(provider.NameGitLab, "bad")
	if err == nil {
		t.Fatal("expected auth error for 401")
	}
}

// TestTestProviderToken_ServerError covers the >= 400 (non-auth) branch.
func TestTestProviderToken_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	withProviderURL(t, provider.NameWrike, srv.URL)

	if err := testProviderToken(provider.NameWrike, "x"); err == nil {
		t.Fatal("expected API error for 500")
	}
}

// TestTestProviderToken_LinearGraphQLError covers the Linear GraphQL-error path.
func TestTestProviderToken_LinearGraphQLError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"errors":[{"message":"invalid token"}]}`))
	}))
	defer srv.Close()

	withProviderURL(t, provider.NameLinear, srv.URL)

	err := testProviderToken(provider.NameLinear, "bad")
	if err == nil {
		t.Fatal("expected GraphQL error from Linear")
	}
}

// TestTestProviderToken_LinearSuccess covers the Linear OK path.
func TestTestProviderToken_LinearSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":{"viewer":{"id":"u1"}}}`))
	}))
	defer srv.Close()

	withProviderURL(t, provider.NameLinear, srv.URL)

	if err := testProviderToken(provider.NameLinear, "good"); err != nil {
		t.Errorf("expected success for valid linear token, got %v", err)
	}
}

// TestTestProviderToken_ConnectionFailure covers the client.Do error branch.
func TestTestProviderToken_ConnectionFailure(t *testing.T) {
	// Point at a closed server to force a connection error.
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close() // close immediately so the connection is refused

	withProviderURL(t, provider.NameGitHub, url)

	if err := testProviderToken(provider.NameGitHub, "x"); err == nil {
		t.Fatal("expected connection error against closed server")
	}
}

// TestRunProviderLogin_ValidationSuccess drives the full login flow with a
// token that validates successfully against a stub endpoint, covering the
// validation-success arm and the save-token path.
func TestRunProviderLogin_ValidationSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"login":"octocat"}`))
	}))
	defer srv.Close()
	withProviderURL(t, provider.NameGitHub, srv.URL)

	shortKvelmoHome(t)

	// Pipe a token via non-terminal stdin.
	r, w, _ := os.Pipe()
	_, _ = w.WriteString("ghp_validtoken99999\n")
	_ = w.Close()
	origStdin := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = origStdin; _ = r.Close() })

	loginCmd := findProviderLogin(GitHubCmd)
	var buf strings.Builder
	loginCmd.SetOut(&buf)

	if err := runProviderLogin("github")(loginCmd, nil); err != nil {
		t.Errorf("runProviderLogin validation success: %v", err)
	}
	if !strings.Contains(buf.String(), "Token saved") {
		t.Errorf("login output missing save confirmation:\n%s", buf.String())
	}
	// Verify the token persisted by detecting it back through the same loader.
	if detectExistingToken("GITHUB_TOKEN", settings.ScopeGlobal, "") == nil {
		t.Error("expected token to be detectable after save")
	}
}

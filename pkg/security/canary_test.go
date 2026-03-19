package security

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCanaryHarnessCreatesFiles(t *testing.T) {
	h, err := NewCanaryHarness()
	if err != nil {
		t.Fatalf("NewCanaryHarness: %v", err)
	}
	defer h.Cleanup()

	if h.Dir == "" {
		t.Fatal("Dir should not be empty")
	}

	expectedPaths := []string{
		".ssh/id_rsa",
		".aws/credentials",
		".env",
		".gnupg/private-keys-v1.d/canary.key",
	}

	for _, rel := range expectedPaths {
		full := filepath.Join(h.Dir, rel)
		if _, err := os.Stat(full); os.IsNotExist(err) {
			t.Errorf("expected canary file %s to exist", rel)
		}

		token, ok := h.Tokens[rel]
		if !ok {
			t.Errorf("no token for %s", rel)

			continue
		}

		content, err := os.ReadFile(full)
		if err != nil {
			t.Errorf("read %s: %v", rel, err)

			continue
		}

		if len(token) == 0 {
			t.Errorf("empty token for %s", rel)
		}

		if len(content) == 0 {
			t.Errorf("empty content for %s", rel)
		}
	}

	if len(h.Tokens) != len(expectedPaths) {
		t.Errorf("expected %d tokens, got %d", len(expectedPaths), len(h.Tokens))
	}
}

func TestCanaryHarnessUniqueTokens(t *testing.T) {
	h, err := NewCanaryHarness()
	if err != nil {
		t.Fatalf("NewCanaryHarness: %v", err)
	}
	defer h.Cleanup()

	seen := make(map[string]bool)
	for path, token := range h.Tokens {
		if seen[token] {
			t.Errorf("duplicate token for %s", path)
		}
		seen[token] = true
	}
}

func TestCanaryHarnessEnv(t *testing.T) {
	h, err := NewCanaryHarness()
	if err != nil {
		t.Fatalf("NewCanaryHarness: %v", err)
	}
	defer h.Cleanup()

	env := h.Env()
	if env["HOME"] != h.Dir {
		t.Errorf("HOME = %q, want %q", env["HOME"], h.Dir)
	}
}

func TestCanaryCheckTokenLeak(t *testing.T) {
	h, err := NewCanaryHarness()
	if err != nil {
		t.Fatalf("NewCanaryHarness: %v", err)
	}
	defer h.Cleanup()

	// Simulate agent output that leaks one token.
	var leakedToken string
	var leakedPath string

	for path, token := range h.Tokens {
		leakedToken = token
		leakedPath = path

		break
	}

	output := "some agent output containing " + leakedToken + " in the middle"
	violations := h.Check(output)

	found := false
	for _, v := range violations {
		if v.Method == "token_leak" && v.Token == leakedToken && v.File == leakedPath {
			found = true
		}
	}

	if !found {
		t.Error("expected token_leak violation for leaked token")
	}
}

func TestCanaryCheckNoViolationsOnCleanRun(t *testing.T) {
	h, err := NewCanaryHarness()
	if err != nil {
		t.Fatalf("NewCanaryHarness: %v", err)
	}
	defer h.Cleanup()

	// Clean output with no tokens.
	violations := h.Check("agent completed successfully, no issues found")

	// Filter to only token_leak violations (atime may trigger on some filesystems).
	var tokenLeaks []CanaryViolation
	for _, v := range violations {
		if v.Method == "token_leak" {
			tokenLeaks = append(tokenLeaks, v)
		}
	}

	if len(tokenLeaks) != 0 {
		t.Errorf("expected 0 token_leak violations, got %d", len(tokenLeaks))
	}
}

func TestCanaryCheckAtimeDetection(t *testing.T) {
	h, err := NewCanaryHarness()
	if err != nil {
		t.Fatalf("NewCanaryHarness: %v", err)
	}
	defer h.Cleanup()

	// Read a canary file to trigger atime update.
	sshKeyPath := filepath.Join(h.Dir, ".ssh/id_rsa")
	_, err = os.ReadFile(sshKeyPath)
	if err != nil {
		t.Fatalf("read canary file: %v", err)
	}

	violations := h.Check("")

	// atime detection is best-effort (noatime mounts won't trigger).
	// We just verify the check runs without error.
	for _, v := range violations {
		if v.Method == "atime" && v.File == ".ssh/id_rsa" {
			// Great — atime detection works on this filesystem.
			return
		}
	}

	t.Log("atime detection did not trigger (filesystem may mount with noatime/relatime)")
}

func TestCanaryCleanup(t *testing.T) {
	h, err := NewCanaryHarness()
	if err != nil {
		t.Fatalf("NewCanaryHarness: %v", err)
	}

	dir := h.Dir
	h.Cleanup()

	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("canary dir should be removed after Cleanup")
	}
}

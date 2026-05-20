package commands

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/valksor/kvelmo/internal/socket"
)

func TestFileExists(t *testing.T) {
	if !fileExists("/") {
		t.Error("expected / to exist")
	}
	if fileExists(filepath.Join(t.TempDir(), "no-such-file")) {
		t.Error("expected missing path to not exist")
	}
}

func TestReplaceOlderSocket_NoSocket(t *testing.T) {
	shortKvelmoHome(t)

	err := replaceOlderSocket(context.Background(), filepath.Join(t.TempDir(), "missing.sock"))
	if err == nil {
		t.Error("expected error when socket missing")
	}
}

func TestReplaceOlderSocket_RunningSocket(t *testing.T) {
	shortKvelmoHome(t)
	_ = startStubGlobalSocket(t)

	// The stub responds to "ping" with {"ok": true} which has no version/commit
	// fields → parsed as empty → matches "" version/commit, so no replacement
	// occurs and the function returns nil. This still exercises the connect,
	// ping, parse, version-compare branches.
	if err := replaceOlderSocket(context.Background(), socket.GlobalSocketPath()); err != nil {
		t.Errorf("replaceOlderSocket: %v", err)
	}
}

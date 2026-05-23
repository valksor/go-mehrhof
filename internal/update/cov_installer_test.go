package update

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestMain intercepts re-exec'd copies of the test binary used to drive the
// installer against a throwaway executable. When KVELMO_INSTALL_HELPER is set
// it runs the helper action and exits; otherwise it runs the normal test suite.
func TestMain(m *testing.M) {
	if v := os.Getenv(installHelperEnv); v != "" {
		mode, arg, _ := strings.Cut(v, ":")
		runInstallHelper(mode, arg)

		return
	}
	os.Exit(m.Run())
}

// The installer's Install/DownloadDir resolve os.Executable() and then
// atomically rename over it. To exercise that for real without clobbering the
// running test binary, we copy the test binary to a temp dir and re-exec the
// copy with KVELMO_INSTALL_HELPER set. In that mode TestMain runs the helper
// logic against the *copy* as the executable.

const installHelperEnv = "KVELMO_INSTALL_HELPER"

// runInstallHelper performs the in-process helper action. It is invoked by the
// re-exec'd copy of the test binary (see TestMain).
func runInstallHelper(mode, arg string) {
	inst := NewInstaller()

	switch mode {
	case "install":
		if err := inst.Install(arg); err != nil {
			_, _ = io.WriteString(os.Stderr, "install error: "+err.Error())
			os.Exit(1)
		}
		_, _ = io.WriteString(os.Stdout, "INSTALL_OK")
	case "downloaddir":
		dir, err := inst.DownloadDir()
		if err != nil {
			_, _ = io.WriteString(os.Stderr, "downloaddir error: "+err.Error())
			os.Exit(1)
		}
		_, _ = io.WriteString(os.Stdout, "DIR:"+dir)
	}
	os.Exit(0)
}

// copyTestBinary copies the current test executable into dir and returns the
// copy's path with executable permissions.
func copyTestBinary(t *testing.T, dir string) string {
	t.Helper()

	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	src, err := os.Open(self)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = src.Close() }()

	dst := filepath.Join(dir, "kvelmo-test-copy")
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(out, src); err != nil {
		_ = out.Close()
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}

	return dst
}

func TestInstaller_Install_AtomicReplace(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("atomic rename install test is POSIX-specific")
	}

	tmpDir := t.TempDir()
	copyBin := copyTestBinary(t, tmpDir)

	newBin := filepath.Join(tmpDir, "new-binary")
	newContent := []byte("REPLACEMENT-BINARY-CONTENT")
	if err := os.WriteFile(newBin, newContent, 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := exec.CommandContext(t.Context(), copyBin)
	cmd.Env = append(os.Environ(), installHelperEnv+"=install:"+newBin)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("install helper failed: %v\n%s", err, out)
	}
	if string(out) != "INSTALL_OK" {
		t.Fatalf("unexpected helper output: %q", out)
	}

	// The copied binary's bytes should now equal newContent (atomic rename).
	got, err := os.ReadFile(copyBin)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(newContent) {
		t.Errorf("binary not replaced; content len = %d", len(got))
	}

	// Source binary should have been moved away.
	if _, err := os.Stat(newBin); !os.IsNotExist(err) {
		t.Errorf("source binary should be moved, stat err = %v", err)
	}

	info, err := os.Stat(copyBin)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Errorf("installed binary not executable, mode = %v", info.Mode())
	}
}

func TestInstaller_Install_SourceMissing(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX-specific")
	}

	tmpDir := t.TempDir()
	copyBin := copyTestBinary(t, tmpDir)

	cmd := exec.CommandContext(t.Context(), copyBin)
	cmd.Env = append(os.Environ(), installHelperEnv+"=install:"+filepath.Join(tmpDir, "nope"))
	if err := cmd.Run(); err == nil {
		t.Error("expected install to fail when source binary missing")
	}
}

func TestInstaller_DownloadDir_ReturnsBinaryDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX-specific")
	}

	tmpDir := t.TempDir()
	copyBin := copyTestBinary(t, tmpDir)

	cmd := exec.CommandContext(t.Context(), copyBin)
	cmd.Env = append(os.Environ(), installHelperEnv+"=downloaddir:")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("downloaddir helper failed: %v\n%s", err, out)
	}

	resolved, _ := filepath.EvalSymlinks(tmpDir)
	want := "DIR:" + resolved
	if string(out) != want {
		t.Errorf("DownloadDir output = %q, want %q", out, want)
	}
}

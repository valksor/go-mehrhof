// Package paths provides centralized path resolution for kvelmo.
// All path computations are encapsulated in PathResolver, which supports
// explicit injection for testing and default resolution for production.
package paths

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"sync"

	"github.com/valksor/kvelmo/pkg/meta"
)

// PathResolver encapsulates all kvelmo path computations.
// Create with NewPathResolver for explicit injection or use DefaultPathResolver
// for standard resolution.
type PathResolver struct {
	baseDir string
}

// NewPathResolver creates a resolver with an explicit base directory.
// Use this in tests for pure injection.
func NewPathResolver(baseDir string) *PathResolver {
	return &PathResolver{baseDir: baseDir}
}

// DefaultPathResolver creates a resolver using standard path resolution:
// 1. KVELMO_HOME env var (if set)
// 2. ~/.valksor/kvelmo (default).
func DefaultPathResolver() *PathResolver {
	if home := os.Getenv(meta.EnvPrefix + "_HOME"); home != "" {
		return &PathResolver{baseDir: home}
	}

	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}

	return &PathResolver{baseDir: filepath.Join(home, meta.GlobalDir)}
}

// BaseDir returns the base directory for all kvelmo data.
func (p *PathResolver) BaseDir() string {
	return p.baseDir
}

// GlobalSocketPath returns the path to the global socket.
func (p *PathResolver) GlobalSocketPath() string {
	return filepath.Join(p.baseDir, "global.sock")
}

// GlobalLockPath returns the path to the global lock file.
func (p *PathResolver) GlobalLockPath() string {
	return filepath.Join(p.baseDir, "global.lock")
}

// WorktreeSocketPath returns the socket path for a worktree directory.
// The path is based on a hash of the absolute worktree path.
func (p *PathResolver) WorktreeSocketPath(worktreeDir string) string {
	absPath, err := filepath.Abs(worktreeDir)
	if err != nil {
		absPath = worktreeDir
	}

	hash := sha256.Sum256([]byte(absPath))
	hashStr := hex.EncodeToString(hash[:8])

	return filepath.Join(p.baseDir, "worktrees", hashStr+".sock")
}

// MemoryDir returns the directory for memory storage.
func (p *PathResolver) MemoryDir() string {
	return filepath.Join(p.baseDir, "memory")
}

// ConfigPath returns the path to the global config file.
func (p *PathResolver) ConfigPath() string {
	return filepath.Join(p.baseDir, meta.ConfigFile)
}

// EnvPath returns the path to the global .env file.
func (p *PathResolver) EnvPath() string {
	return filepath.Join(p.baseDir, ".env")
}

// EnsureDir creates the required directories.
func (p *PathResolver) EnsureDir() error {
	dirs := []string{
		p.baseDir,
		filepath.Join(p.baseDir, "worktrees"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}

	return nil
}

// Package-level default resolver.
// When injectedPaths is set via SetPaths(), it takes precedence.
// Otherwise, DefaultPathResolver() is called each time to respect env var changes.
var (
	injectedPaths *PathResolver
	pathsMu       sync.RWMutex
)

// Paths returns the PathResolver to use.
// If SetPaths() was called, returns the injected resolver.
// Otherwise, returns DefaultPathResolver() (respects current KVELMO_HOME env var).
func Paths() *PathResolver {
	pathsMu.RLock()
	if injectedPaths != nil {
		defer pathsMu.RUnlock()

		return injectedPaths
	}
	pathsMu.RUnlock()

	// No injection - use default resolver (checks env var each time for test isolation)
	return DefaultPathResolver()
}

// SetPaths sets the PathResolver for injection.
// When set, Paths() returns this resolver instead of DefaultPathResolver().
// Call ResetForTesting() to clear the injection.
func SetPaths(p *PathResolver) {
	pathsMu.Lock()
	defer pathsMu.Unlock()

	injectedPaths = p
}

// ResetForTesting clears any injected PathResolver.
// After this call, Paths() will return DefaultPathResolver().
func ResetForTesting() {
	pathsMu.Lock()
	defer pathsMu.Unlock()

	injectedPaths = nil
}

// Backwards-compatible package-level functions.
// These delegate to the default PathResolver.

// BaseDir returns the base directory using the default resolver.
func BaseDir() string {
	return Paths().BaseDir()
}

// GlobalSocketPath returns the global socket path using the default resolver.
func GlobalSocketPath() string {
	return Paths().GlobalSocketPath()
}

// GlobalLockPath returns the global lock path using the default resolver.
func GlobalLockPath() string {
	return Paths().GlobalLockPath()
}

// WorktreeSocketPath returns the worktree socket path using the default resolver.
func WorktreeSocketPath(worktreeDir string) string {
	return Paths().WorktreeSocketPath(worktreeDir)
}

// MemoryDir returns the memory directory using the default resolver.
func MemoryDir() string {
	return Paths().MemoryDir()
}

// ConfigPath returns the config path using the default resolver.
func ConfigPath() string {
	return Paths().ConfigPath()
}

// EnvPath returns the env path using the default resolver.
func EnvPath() string {
	return Paths().EnvPath()
}

// EnsureDir creates directories using the default resolver.
func EnsureDir() error {
	return Paths().EnsureDir()
}

// WorktreeIDFromPath returns a hash-based ID for a worktree directory.
// This is a pure computation that doesn't depend on the base directory.
func WorktreeIDFromPath(worktreeDir string) string {
	absPath, err := filepath.Abs(worktreeDir)
	if err != nil {
		absPath = worktreeDir
	}

	hash := sha256.Sum256([]byte(absPath))

	return hex.EncodeToString(hash[:8])
}

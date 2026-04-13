package varpool

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// Save writes the pool to a JSON file atomically.
func (p *Pool) Save(path string) error {
	p.mu.RLock()
	data, err := json.MarshalIndent(p.vars, "", "  ")
	p.mu.RUnlock()

	if err != nil {
		return fmt.Errorf("varpool: marshal: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("varpool: mkdir: %w", err)
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("varpool: write tmp: %w", err)
	}

	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp) //nolint:errcheck // best-effort cleanup

		return fmt.Errorf("varpool: rename: %w", err)
	}

	return nil
}

// Load reads the pool from a JSON file.
func (p *Pool) Load(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("varpool: read: %w", err)
	}

	vars := make(map[string]Variable)
	if err := json.Unmarshal(data, &vars); err != nil {
		return fmt.Errorf("varpool: unmarshal: %w", err)
	}

	p.mu.Lock()
	p.vars = vars
	p.mu.Unlock()

	return nil
}

// MarshalJSON serializes the pool's variables.
func (p *Pool) MarshalJSON() ([]byte, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return json.Marshal(p.vars)
}

// UnmarshalJSON deserializes variables into the pool.
func (p *Pool) UnmarshalJSON(data []byte) error {
	vars := make(map[string]Variable)
	if err := json.Unmarshal(data, &vars); err != nil {
		return err
	}

	p.mu.Lock()
	p.vars = vars
	p.mu.Unlock()

	return nil
}

// Clone returns a deep copy of the pool.
// JSON-typed variables are deep-copied via JSON round-trip to prevent
// shared mutation of slices/maps between original and clone.
func (p *Pool) Clone() *Pool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	clone := &Pool{vars: make(map[string]Variable, len(p.vars))}
	for k, v := range p.vars {
		if v.Type == TypeJSON {
			if data, err := json.Marshal(v.Value); err == nil {
				var deep any
				if err := json.Unmarshal(data, &deep); err == nil {
					v.Value = deep
				}
			}
		}
		clone.vars[k] = v
	}

	return clone
}

// Scope constants for namespaced variable addressing.
const (
	ScopeSystem    = "sys"
	ScopePlan      = "plan"
	ScopeImplement = "implement"
	ScopeSimplify  = "simplify"
	ScopeOptimize  = "optimize"
	ScopeReview    = "review"
)

// scopedKey creates a namespaced key: scopedKey("plan", "spec") → "plan.spec".
func scopedKey(scope, name string) string {
	return scope + "." + name
}

// SetScoped stores a variable with an explicit namespace scope.
func (p *Pool) SetScoped(scope, name string, value any, setBy string) {
	p.Set(scopedKey(scope, name), value, setBy)
}

// GetScoped retrieves a variable by scope and name.
func (p *Pool) GetScoped(scope, name string) (Variable, bool) {
	return p.Get(scopedKey(scope, name))
}

// GetScopedString returns the string value of a scoped variable, or empty string.
func (p *Pool) GetScopedString(scope, name string) string {
	return p.GetString(scopedKey(scope, name))
}

// ClearScope removes all variables in the given scope.
func (p *Pool) ClearScope(scope string) {
	prefix := scope + "."
	p.mu.Lock()
	defer p.mu.Unlock()

	for k := range p.vars {
		if len(k) > len(prefix) && k[:len(prefix)] == prefix {
			delete(p.vars, k)
		}
	}
}

// ListScope returns all variables in the given scope, sorted by name.
func (p *Pool) ListScope(scope string) []Variable {
	prefix := scope + "."
	p.mu.RLock()
	defer p.mu.RUnlock()

	var result []Variable
	for k, v := range p.vars {
		if len(k) > len(prefix) && k[:len(prefix)] == prefix {
			result = append(result, v)
		}
	}

	slices.SortFunc(result, func(a, b Variable) int {
		if a.Name < b.Name {
			return -1
		}
		if a.Name > b.Name {
			return 1
		}

		return 0
	})

	return result
}

// SummaryOption configures Summary() output.
type SummaryOption func(*summaryConfig)

type summaryConfig struct {
	includeSystem bool
	maxBytes      int
	maxValueLen   int
}

// WithSystemVars includes sys.* variables in the summary (excluded by default).
func WithSystemVars() SummaryOption {
	return func(c *summaryConfig) { c.includeSystem = true }
}

// WithMaxBytes limits total summary size in bytes.
func WithMaxBytes(n int) SummaryOption {
	return func(c *summaryConfig) { c.maxBytes = n }
}

// WithMaxValueLen limits individual value display length.
func WithMaxValueLen(n int) SummaryOption {
	return func(c *summaryConfig) { c.maxValueLen = n }
}

// Summary generates a concise markdown digest of pool state, grouped by scope.
// System variables (sys.*) and internal keys (starting with _) are excluded by default.
// Expired variables are skipped.
func (p *Pool) Summary(opts ...SummaryOption) string {
	cfg := summaryConfig{maxValueLen: 200, maxBytes: 4096}
	for _, o := range opts {
		o(&cfg)
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	now := time.Now()

	// Group variables by scope prefix.
	groups := make(map[string][]Variable)

	for _, v := range p.vars {
		if !v.ExpiresAt.IsZero() && v.ExpiresAt.Before(now) {
			continue
		}

		scope, _ := splitScope(v.Name)

		if scope == ScopeSystem && !cfg.includeSystem {
			continue
		}

		if len(v.Name) > 0 && v.Name[0] == '_' {
			continue
		}

		groups[scope] = append(groups[scope], v)
	}

	if len(groups) == 0 {
		return ""
	}

	// Sort scope names for deterministic output.
	scopes := make([]string, 0, len(groups))
	for s := range groups {
		scopes = append(scopes, s)
	}

	slices.Sort(scopes)

	var b strings.Builder
	b.WriteString("\n## Shared Context\n")

	for _, scope := range scopes {
		vars := groups[scope]

		slices.SortFunc(vars, func(a, bv Variable) int {
			return strings.Compare(a.Name, bv.Name)
		})

		header := scope
		if header == "" {
			header = "general"
		}

		b.WriteString("\n### ")
		b.WriteString(header)
		b.WriteString("\n")

		for _, v := range vars {
			_, key := splitScope(v.Name)
			val := formatValue(v.Value, cfg.maxValueLen)

			entry := "- **" + key + "**: " + val + "\n"

			if cfg.maxBytes > 0 && b.Len()+len(entry) > cfg.maxBytes {
				b.WriteString("\n_(truncated)_\n")

				return b.String()
			}

			b.WriteString(entry)
		}
	}

	return b.String()
}

// splitScope separates "scope.key" into its parts. Returns ("", name) for unscoped keys.
// For multi-dot names like "plan.sub.detail", returns ("plan", "sub.detail") — only the
// first dot is used as the scope boundary.
func splitScope(name string) (string, string) {
	if before, after, ok := strings.Cut(name, "."); ok {
		return before, after
	}

	return "", name
}

// formatValue converts a variable value to a display string, truncating if needed.
// Values are sanitized: leading/trailing newlines are stripped to prevent markdown injection.
// Truncation respects UTF-8 rune boundaries.
func formatValue(value any, maxLen int) string {
	var s string

	switch v := value.(type) {
	case string:
		s = v
	default:
		data, err := json.Marshal(v)
		if err != nil {
			s = fmt.Sprintf("%v", v)
		} else {
			s = string(data)
		}
	}

	s = strings.TrimSpace(s)

	if maxLen > 0 {
		runes := []rune(s)
		if len(runes) > maxLen {
			return string(runes[:maxLen]) + "…"
		}
	}

	return s
}

// Package varpool provides a typed variable pool for sharing context between
// graph nodes during kvelmo's task lifecycle. Variables are persisted to disk
// and serializable for socket transport.
package varpool

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"
)

// VarType identifies the type of a stored variable.
type VarType string

const (
	TypeString VarType = "string"
	TypeNumber VarType = "number"
	TypeBool   VarType = "bool"
	TypeJSON   VarType = "json"
)

// Variable is a single typed entry in the pool.
type Variable struct {
	Name      string    `json:"name"`
	Type      VarType   `json:"type"`
	Value     any       `json:"value"`
	SetBy     string    `json:"set_by"`
	Timestamp time.Time `json:"timestamp"`
}

// Pool is a thread-safe typed key-value store.
type Pool struct {
	mu   sync.RWMutex
	vars map[string]Variable
}

// New creates an empty pool.
func New() *Pool {
	return &Pool{vars: make(map[string]Variable)}
}

// Set stores a variable, auto-detecting its type.
func (p *Pool) Set(name string, value any, setBy string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.vars[name] = Variable{
		Name:      name,
		Type:      detectType(value),
		Value:     value,
		SetBy:     setBy,
		Timestamp: time.Now(),
	}
}

// Get returns a variable and whether it exists.
func (p *Pool) Get(name string) (Variable, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	v, ok := p.vars[name]

	return v, ok
}

// GetString returns the string value of a variable, or empty string.
func (p *Pool) GetString(name string) string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	v, ok := p.vars[name]
	if !ok {
		return ""
	}

	s, _ := v.Value.(string)

	return s
}

// GetNumber returns the numeric value of a variable, or 0.
func (p *Pool) GetNumber(name string) float64 {
	p.mu.RLock()
	defer p.mu.RUnlock()

	v, ok := p.vars[name]
	if !ok {
		return 0
	}

	switch n := v.Value.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case int64:
		return float64(n)
	default:
		return 0
	}
}

// GetBool returns the bool value of a variable, or false.
func (p *Pool) GetBool(name string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	v, ok := p.vars[name]
	if !ok {
		return false
	}

	b, _ := v.Value.(bool)

	return b
}

// Append atomically appends a value to a slice variable.
// If the variable does not exist, a new slice is created.
// If the variable exists but is not a slice, an error is returned.
func (p *Pool) Append(name string, value any, setBy string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	v, ok := p.vars[name]
	if !ok {
		p.vars[name] = Variable{
			Name:      name,
			Type:      TypeJSON,
			Value:     []any{value},
			SetBy:     setBy,
			Timestamp: time.Now(),
		}

		return nil
	}

	slice, ok := v.Value.([]any)
	if !ok {
		return fmt.Errorf("varpool: cannot append to %s (type %s)", name, v.Type)
	}

	v.Value = append(slice, value)
	v.SetBy = setBy
	v.Timestamp = time.Now()
	p.vars[name] = v

	return nil
}

// Increment atomically adds delta to a numeric variable.
// If the variable does not exist, it is created with the delta as its value.
// If the variable exists but is not numeric, an error is returned.
func (p *Pool) Increment(name string, delta float64, setBy string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	v, ok := p.vars[name]
	if !ok {
		p.vars[name] = Variable{
			Name:      name,
			Type:      TypeNumber,
			Value:     delta,
			SetBy:     setBy,
			Timestamp: time.Now(),
		}

		return nil
	}

	if v.Type != TypeNumber {
		return fmt.Errorf("varpool: cannot increment %s (type %s)", name, v.Type)
	}

	current := toFloat64(v.Value)
	v.Value = current + delta
	v.SetBy = setBy
	v.Timestamp = time.Now()
	p.vars[name] = v

	return nil
}

// Decrement atomically subtracts delta from a numeric variable.
// If the variable does not exist, it is created with -delta as its value.
// If the variable exists but is not numeric, an error is returned.
func (p *Pool) Decrement(name string, delta float64, setBy string) error {
	return p.Increment(name, -delta, setBy)
}

// toFloat64 converts a numeric value to float64.
func toFloat64(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case float32:
		return float64(n)
	default:
		return 0
	}
}

// Delete removes a variable.
func (p *Pool) Delete(name string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	delete(p.vars, name)
}

// List returns all variables sorted by name.
func (p *Pool) List() []Variable {
	p.mu.RLock()
	defer p.mu.RUnlock()

	vars := make([]Variable, 0, len(p.vars))
	for _, v := range p.vars {
		vars = append(vars, v)
	}

	slices.SortFunc(vars, func(a, b Variable) int {
		if a.Name < b.Name {
			return -1
		}
		if a.Name > b.Name {
			return 1
		}

		return 0
	})

	return vars
}

// Len returns the number of variables.
func (p *Pool) Len() int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return len(p.vars)
}

// Clear removes all variables.
func (p *Pool) Clear() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.vars = make(map[string]Variable)
}

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

func detectType(value any) VarType {
	switch value.(type) {
	case string:
		return TypeString
	case float64, int, int64, float32:
		return TypeNumber
	case bool:
		return TypeBool
	default:
		return TypeJSON
	}
}

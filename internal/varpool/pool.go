// Package varpool provides a typed variable pool for sharing context between
// graph nodes during kvelmo's task lifecycle. Variables are persisted to disk
// and serializable for socket transport.
package varpool

import (
	"fmt"
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
	Name        string    `json:"name"`
	Type        VarType   `json:"type"`
	Value       any       `json:"value"`
	SetBy       string    `json:"set_by"`
	Timestamp   time.Time `json:"timestamp"`
	ExpiresAt   time.Time `json:"expires_at,omitzero"`    // Zero means no expiry
	ExecutionID string    `json:"execution_id,omitempty"` // Scoped to a specific execution
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
// Expired variables are treated as nonexistent.
func (p *Pool) Get(name string) (Variable, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	v, ok := p.vars[name]
	if !ok {
		return v, false
	}

	if !v.ExpiresAt.IsZero() && time.Now().After(v.ExpiresAt) {
		return Variable{}, false
	}

	return v, true
}

// GetString returns the string value of a variable, or empty string.
// Expired variables return empty string.
func (p *Pool) GetString(name string) string {
	v, ok := p.Get(name)
	if !ok {
		return ""
	}

	s, _ := v.Value.(string)

	return s
}

// GetNumber returns the numeric value of a variable, or 0.
// Expired variables return 0.
func (p *Pool) GetNumber(name string) float64 {
	v, ok := p.Get(name)
	if !ok {
		return 0
	}

	return toFloat64(v.Value)
}

// GetBool returns the bool value of a variable, or false.
// Expired variables return false.
func (p *Pool) GetBool(name string) bool {
	v, ok := p.Get(name)
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

// SetWithTTL stores a variable that expires after the given duration.
// A zero or negative TTL is treated as no expiry.
func (p *Pool) SetWithTTL(name string, value any, setBy string, ttl time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	v := Variable{
		Name:      name,
		Type:      detectType(value),
		Value:     value,
		SetBy:     setBy,
		Timestamp: now,
	}

	if ttl > 0 {
		v.ExpiresAt = now.Add(ttl)
	}

	p.vars[name] = v
}

// SetExecutionScoped stores a variable scoped to a specific execution ID.
// Use ClearExecution to remove all variables for a completed execution.
func (p *Pool) SetExecutionScoped(executionID, name string, value any, setBy string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.vars[name] = Variable{
		Name:        name,
		Type:        detectType(value),
		Value:       value,
		SetBy:       setBy,
		Timestamp:   time.Now(),
		ExecutionID: executionID,
	}
}

// ClearExecution removes all variables scoped to the given execution ID.
func (p *Pool) ClearExecution(executionID string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for k, v := range p.vars {
		if v.ExecutionID == executionID {
			delete(p.vars, k)
		}
	}
}

// CleanExpired removes all variables whose TTL has elapsed.
// Returns the number of variables removed.
func (p *Pool) CleanExpired() int {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	removed := 0

	for k, v := range p.vars {
		if !v.ExpiresAt.IsZero() && now.After(v.ExpiresAt) {
			delete(p.vars, k)
			removed++
		}
	}

	return removed
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

package filter

import (
	"slices"
	"strings"
	"time"

	"github.com/valksor/kvelmo/pkg/page"
)

// FieldAccessor extracts a field value from an item.
type FieldAccessor[T any] func(T) any

// Predicate tests a single item.
type Predicate[T any] func(T) bool

// Filter chains predicates for type-safe in-memory filtering.
type Filter[T any] struct {
	predicates []Predicate[T]
}

// New creates an empty filter.
func New[T any]() *Filter[T] {
	return &Filter[T]{}
}

// Where adds a custom predicate.
func (f *Filter[T]) Where(p Predicate[T]) *Filter[T] {
	f.predicates = append(f.predicates, p)

	return f
}

// Eq adds an equality check on a field.
func (f *Filter[T]) Eq(accessor FieldAccessor[T], value any) *Filter[T] {
	f.predicates = append(f.predicates, func(item T) bool {
		return accessor(item) == value
	})

	return f
}

// Neq adds a not-equal check.
func (f *Filter[T]) Neq(accessor FieldAccessor[T], value any) *Filter[T] {
	f.predicates = append(f.predicates, func(item T) bool {
		return accessor(item) != value
	})

	return f
}

// Contains checks if a string field contains a substring (case-insensitive).
func (f *Filter[T]) Contains(accessor FieldAccessor[T], substr string) *Filter[T] {
	lower := strings.ToLower(substr)
	f.predicates = append(f.predicates, func(item T) bool {
		val, ok := accessor(item).(string)
		if !ok {
			return false
		}

		return strings.Contains(strings.ToLower(val), lower)
	})

	return f
}

// In checks if a field value is in a set.
func (f *Filter[T]) In(accessor FieldAccessor[T], values ...any) *Filter[T] {
	f.predicates = append(f.predicates, func(item T) bool {
		val := accessor(item)
		for _, v := range values {
			if val == v {
				return true
			}
		}

		return false
	})

	return f
}

// Since checks if a time field is after a given time.
func (f *Filter[T]) Since(accessor func(T) time.Time, t time.Time) *Filter[T] {
	f.predicates = append(f.predicates, func(item T) bool {
		return accessor(item).After(t)
	})

	return f
}

// Before checks if a time field is before a given time.
func (f *Filter[T]) Before(accessor func(T) time.Time, t time.Time) *Filter[T] {
	f.predicates = append(f.predicates, func(item T) bool {
		return accessor(item).Before(t)
	})

	return f
}

// HasTag checks if a []string field contains a specific tag.
func (f *Filter[T]) HasTag(accessor func(T) []string, tag string) *Filter[T] {
	f.predicates = append(f.predicates, func(item T) bool {
		return slices.Contains(accessor(item), tag)
	})

	return f
}

// Match tests whether an item passes all predicates (AND logic).
func (f *Filter[T]) Match(item T) bool {
	for _, p := range f.predicates {
		if !p(item) {
			return false
		}
	}

	return true
}

// Apply filters a slice, returning matching items.
func (f *Filter[T]) Apply(items []T) []T {
	result := make([]T, 0, len(items))

	for _, item := range items {
		if f.Match(item) {
			result = append(result, item)
		}
	}

	return result
}

// ApplyPaginated filters and paginates.
func (f *Filter[T]) ApplyPaginated(items []T, p page.Pagination) page.Page[T] {
	return page.NewPage(f.Apply(items), p)
}

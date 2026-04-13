package filter

import (
	"cmp"
	"slices"
	"time"
)

// SortFunc defines a comparison for sorting.
type SortFunc[T any] func(a, b T) int

// Sort sorts items in place using the given comparison and returns the slice.
func Sort[T any](items []T, sf SortFunc[T]) []T {
	slices.SortStableFunc(items, sf)

	return items
}

// SortByString creates a sort function from a string field accessor.
func SortByString[T any](accessor func(T) string, desc bool) SortFunc[T] {
	return func(a, b T) int {
		result := cmp.Compare(accessor(a), accessor(b))
		if desc {
			return -result
		}

		return result
	}
}

// SortByTime creates a sort function from a time field accessor.
func SortByTime[T any](accessor func(T) time.Time, desc bool) SortFunc[T] {
	return func(a, b T) int {
		at, bt := accessor(a), accessor(b)

		result := at.Compare(bt)
		if desc {
			return -result
		}

		return result
	}
}

// SortByInt creates a sort function from an int field accessor.
func SortByInt[T any](accessor func(T) int, desc bool) SortFunc[T] {
	return func(a, b T) int {
		result := cmp.Compare(accessor(a), accessor(b))
		if desc {
			return -result
		}

		return result
	}
}

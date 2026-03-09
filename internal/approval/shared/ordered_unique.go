package shared

import collections "github.com/coldsmirk/go-collections"

// OrderedUnique stores unique values while preserving first-seen order.
type OrderedUnique[T comparable] struct {
	seen  collections.Set[T]
	items []T
}

// NewOrderedUnique creates an ordered-unique container with optional capacity.
func NewOrderedUnique[T comparable](capacity int) *OrderedUnique[T] {
	if capacity < 0 {
		capacity = 0
	}

	return &OrderedUnique[T]{
		seen:  collections.NewHashSetWithCapacity[T](capacity),
		items: make([]T, 0, capacity),
	}
}

// Add inserts value only if it does not already exist, preserving insertion order.
func (o *OrderedUnique[T]) Add(value T) bool {
	if !o.seen.Add(value) {
		return false
	}

	o.items = append(o.items, value)

	return true
}

// AddAll inserts multiple values and returns how many were newly added.
func (o *OrderedUnique[T]) AddAll(values ...T) int {
	added := 0

	for _, value := range values {
		if o.Add(value) {
			added++
		}
	}

	return added
}

// ToSlice returns a copy of ordered unique values.
func (o *OrderedUnique[T]) ToSlice() []T {
	return append([]T(nil), o.items...)
}

// Len returns the number of unique values.
func (o *OrderedUnique[T]) Len() int { return len(o.items) }

// Contains reports whether value already exists in the set.
func (o *OrderedUnique[T]) Contains(value T) bool {
	return o.seen.Contains(value)
}

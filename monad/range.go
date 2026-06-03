package monad

import "cmp"

// Range represents an inclusive range of ordered values [Start, End].
type Range[T cmp.Ordered] struct {
	Start T // Start is the inclusive start of the range
	End   T // End is the inclusive end of the range
}

// NewRange creates a new range with the given start and end values.
// The range is inclusive on both ends: [start, end].
func NewRange[T cmp.Ordered](start, end T) Range[T] {
	return Range[T]{
		Start: start,
		End:   end,
	}
}

// Contains checks if the range contains the given value (inclusive).
// Returns true if start <= value <= end.
func (r Range[T]) Contains(value T) bool {
	return r.Start <= value && value <= r.End
}

// IsValid checks if the range is valid (start <= end).
func (r Range[T]) IsValid() bool {
	return r.Start <= r.End
}

// IsEmpty returns true if the range contains no values (start > end).
func (r Range[T]) IsEmpty() bool {
	return r.Start > r.End
}

// IsNotEmpty returns true if the range contains at least one value (start <= end).
func (r Range[T]) IsNotEmpty() bool {
	return r.Start <= r.End
}

// Overlaps checks if this range overlaps with another range.
func (r Range[T]) Overlaps(other Range[T]) bool {
	return r.Start <= other.End && other.Start <= r.End
}

// Intersection returns the intersection of this range with another range.
// The boolean result reports whether the ranges overlap; when it is false the
// returned range is the zero value and its bounds carry no meaning.
func (r Range[T]) Intersection(other Range[T]) (Range[T], bool) {
	if !r.Overlaps(other) {
		return Range[T]{}, false
	}

	return Range[T]{
		Start: max(r.Start, other.Start),
		End:   min(r.End, other.End),
	}, true
}

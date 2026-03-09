package monad

// Comparable is a type constraint for types that support comparison operations.
// It includes all numeric types and strings.
type Comparable interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 |
		~float32 | ~float64 |
		~string
}

// Range represents an inclusive range of comparable values [Start, End].
type Range[T Comparable] struct {
	Start T // Start is the inclusive start of the range
	End   T // End is the inclusive end of the range
}

// NewRange creates a new range with the given start and end values.
// The range is inclusive on both ends: [start, end].
func NewRange[T Comparable](start, end T) Range[T] {
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

// Overlaps checks if this range overlaps with another range.
func (r Range[T]) Overlaps(other Range[T]) bool {
	return r.Start <= other.End && other.Start <= r.End
}

// Intersection returns the intersection of this range with another range.
// Returns an empty range if there is no intersection.
func (r Range[T]) Intersection(other Range[T]) Range[T] {
	if !r.Overlaps(other) {
		return Range[T]{Start: r.End, End: r.Start}
	}

	return Range[T]{
		Start: max(r.Start, other.Start),
		End:   min(r.End, other.End),
	}
}

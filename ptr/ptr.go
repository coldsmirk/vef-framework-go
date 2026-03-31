package ptr

// Of returns a pointer to v, or nil if v is the zero value.
func Of[T comparable](v T) *T {
	var zero T
	if v == zero {
		return nil
	}

	return new(v)
}

// Zero returns the zero value of type T.
func Zero[T any]() T {
	var zero T

	return zero
}

// Value dereferences a pointer, returning its value.
// If p is nil, it tries each fallback pointer in order.
// If all are nil, the zero value of T is returned.
func Value[T any](p *T, fallbacks ...*T) T {
	if p != nil {
		return *p
	}

	for _, fb := range fallbacks {
		if fb != nil {
			return *fb
		}
	}

	var zero T

	return zero
}

// ValueOrElse dereferences a pointer, calling fn to produce a fallback if nil.
func ValueOrElse[T any](p *T, fn func() T) T {
	if p != nil {
		return *p
	}

	return fn()
}

// Equal compares two pointers by value. Both nil returns true, one nil returns false.
func Equal[T comparable](a, b *T) bool {
	if a == nil && b == nil {
		return true
	}

	if a == nil || b == nil {
		return false
	}

	return *a == *b
}

// Coalesce returns the first non-nil pointer, or nil if all are nil.
func Coalesce[T any](ptrs ...*T) *T {
	for _, p := range ptrs {
		if p != nil {
			return p
		}
	}

	return nil
}

package shared

import (
	"iter"
	"math"

	"github.com/hbollon/go-edlib"
)

// Closest finds the item whose projected key is nearest to target by
// Levenshtein distance. The returned distance is the minimum found.
// ok is false when items is empty or when two items tie for the minimum
// distance (an ambiguous match must not be suggested). Callers needing a
// proximity threshold should apply it to the returned distance themselves.
func Closest[T any](target string, items iter.Seq[T], key func(T) string) (match T, distance int, ok bool) {
	minDistance := math.MaxInt

	var ambiguous bool

	for item := range items {
		d := edlib.LevenshteinDistance(target, key(item))
		switch {
		case d < minDistance:
			minDistance = d
			match = item
			ambiguous = false
		case d == minDistance:
			ambiguous = true
		}
	}

	if minDistance == math.MaxInt || ambiguous {
		var zero T

		return zero, 0, false
	}

	return match, minDistance, true
}

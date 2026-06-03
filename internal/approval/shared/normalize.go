package shared

import "strings"

// NormalizeUniqueIDs trims, deduplicates, and filters empty IDs while
// preserving first-seen order.
func NormalizeUniqueIDs(ids []string) []string {
	if len(ids) == 0 {
		return nil
	}

	unique := NewOrderedUnique[string](len(ids))
	for _, rawID := range ids {
		id := strings.TrimSpace(rawID)
		if id == "" {
			continue
		}

		unique.Add(id)
	}

	return unique.ToSlice()
}

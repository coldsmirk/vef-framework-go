package orm

import (
	"maps"
	"slices"
)

// LabelsEqual builds a condition that ANDs one equality predicate per label
// pair, comparing the JSON-extracted value at the pair's key inside the given
// JSON column with the pair's value. JSONExtract keeps the predicate portable
// across dialects; rows whose label column is NULL never match because
// extraction over NULL yields NULL. Keys are sorted so the generated SQL is
// stable across runs. Callers own key hygiene: a key containing a dot would
// be read as a nested JSON path, so writers should restrict keys at save time.
func LabelsEqual(column string, labels map[string]string) ApplyFunc[ConditionBuilder] {
	return func(cb ConditionBuilder) {
		for _, key := range slices.Sorted(maps.Keys(labels)) {
			cb.Expr(func(eb ExprBuilder) any {
				return eb.Equals(eb.JSONUnquote(eb.JSONExtract(eb.Column(column), key)), labels[key])
			})
		}
	}
}

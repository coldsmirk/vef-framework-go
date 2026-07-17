package orm

import (
	"errors"
	"maps"
	"regexp"
	"slices"
	"unicode/utf8"
)

// ErrInvalidLabel reports a label entry that violates ValidateLabels' rules.
// Modules translate it into their own API error sentinels.
var ErrInvalidLabel = errors.New("orm: invalid label")

// labelKeyPattern is the JSON-path-safe charset for label keys. LabelsEqual
// feeds keys into the cross-dialect JSON path builder, where a dot is a
// nesting separator — a dotted key would be stored fine but never match the
// filter. Restricting keys at save time keeps every stored label filterable.
// 63 chars mirrors the Kubernetes label bound.
var labelKeyPattern = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9_-]*[A-Za-z0-9])?$`)

// maxLabelKeyLength and maxLabelValueLength bound a single label entry.
// Values carry no charset restriction because they are only ever compared as
// bind parameters, so their bound counts runes — a Chinese value gets the
// same budget as an ASCII one (keys are ASCII by pattern, where bytes and
// runes coincide).
const (
	maxLabelKeyLength   = 63
	maxLabelValueLength = 256
)

// ValidateLabels rejects label entries whose key would silently escape a
// LabelsEqual filter (see labelKeyPattern) or whose key or value exceeds the
// size bounds. Empty values are valid — presence-style flags ("mobile": "")
// are a legitimate labeling scheme. Every writer of a LabelsEqual-filtered
// column should run this at save time.
func ValidateLabels(labels map[string]string) error {
	for key, value := range labels {
		if len(key) > maxLabelKeyLength || !labelKeyPattern.MatchString(key) {
			return ErrInvalidLabel
		}

		if utf8.RuneCountInString(value) > maxLabelValueLength {
			return ErrInvalidLabel
		}
	}

	return nil
}

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

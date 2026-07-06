package approval_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coldsmirk/vef-framework-go/approval"
)

func TestAggregateKindIsValid(t *testing.T) {
	valid := []approval.AggregateKind{approval.AggregateSum, approval.AggregateCount, approval.AggregateAvg}
	for _, kind := range valid {
		assert.True(t, kind.IsValid(), "built-in aggregate %q should be valid", kind)
	}

	assert.False(t, approval.AggregateKind("median").IsValid(), "IsValid covers the built-in vocabulary only")
	assert.False(t, approval.AggregateKind("").IsValid(), "the empty kind means no aggregate")
}

func TestAggregateKindFoldsColumn(t *testing.T) {
	assert.True(t, approval.AggregateSum.FoldsColumn(), "sum folds a column")
	assert.True(t, approval.AggregateAvg.FoldsColumn(), "avg folds a column")
	assert.False(t, approval.AggregateCount.FoldsColumn(), "count folds rows")
	assert.True(t, approval.AggregateKind("median").FoldsColumn(),
		"unknown kinds default to column-folding — the safe shape for host aggregates")
}

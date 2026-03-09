package orm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReturningColumnsOrderAndUniqueness(t *testing.T) {
	cols := newReturningColumns()
	cols.AddAll("id", "name", "id", "age", "name")

	assert.Equal(t, []string{"id", "name", "age"}, cols.Values(),
		"returning columns should keep first-seen order and remove duplicates")
	assert.False(t, cols.IsEmpty(), "columns should not be empty after AddAll")

	cols.Clear()
	assert.True(t, cols.IsEmpty(), "columns should be empty after Clear")
	assert.Empty(t, cols.Values(), "values should be empty after Clear")
}

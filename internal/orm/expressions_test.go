package orm

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/schema"
)

func newTestQueryGen() schema.QueryGen {
	return schema.NewQueryGen(sqlitedialect.New())
}

// TestExpressionsAppendQuery verifies expressions AppendQuery with various separator and element types.
func TestExpressionsAppendQuery(t *testing.T) {
	gen := newTestQueryGen()

	t.Run("StringSeparator", func(t *testing.T) {
		expr := newExpressions(", ", "hello", "world")

		b, err := expr.AppendQuery(gen, nil)
		assert.NoError(t, err, "Should append expressions with string separator")

		result := string(b)
		assert.Contains(t, result, "hello", "Should contain first expression")
		assert.Contains(t, result, "world", "Should contain second expression")
		assert.Contains(t, result, ", ", "Should contain separator between expressions")
	})

	t.Run("EmptyExpressions", func(t *testing.T) {
		expr := newExpressions(", ")

		b, err := expr.AppendQuery(gen, nil)
		assert.NoError(t, err, "Should handle empty expression list")
		assert.Equal(t, "NULL", string(b), "Should return NULL for empty expressions")
	})

	t.Run("QueryAppenderSeparator", func(t *testing.T) {
		sep := bun.Safe(" AND ")
		expr := newExpressions(sep, "a", "b")

		b, err := expr.AppendQuery(gen, nil)
		assert.NoError(t, err, "Should append with QueryAppender separator")

		result := string(b)
		assert.Contains(t, result, " AND ", "Should use QueryAppender as separator")
	})

	t.Run("DefaultSeparator", func(t *testing.T) {
		// Use an int as separator -- falls through to default branch
		expr := newExpressions(42, "x", "y")

		b, err := expr.AppendQuery(gen, nil)
		assert.NoError(t, err, "Should handle non-standard separator type")
		assert.NotEmpty(t, b, "Should produce non-empty output")
	})

	t.Run("SliceElement", func(t *testing.T) {
		inner := []any{"a", "b"}
		expr := newExpressions(", ", inner)

		b, err := expr.AppendQuery(gen, nil)
		assert.NoError(t, err, "Should handle slice element")

		result := string(b)
		assert.Contains(t, result, "a", "Should contain first slice element")
		assert.Contains(t, result, "b", "Should contain second slice element")
	})

	t.Run("QueryAppenderElement", func(t *testing.T) {
		appender := bun.Safe("NOW()")
		expr := newExpressions(", ", appender)

		b, err := expr.AppendQuery(gen, nil)
		assert.NoError(t, err, "Should handle QueryAppender element")

		result := string(b)
		assert.Contains(t, result, "NOW()", "Should contain appended SQL expression")
	})

	t.Run("SeparatorError", func(t *testing.T) {
		sep := ErrorAppender{}
		expr := newExpressions(sep, "a", "b")

		_, err := expr.AppendQuery(gen, nil)
		assert.Error(t, err, "Should propagate separator AppendQuery error")
	})

	t.Run("MultipleElements", func(t *testing.T) {
		expr := newExpressions(", ", bun.Safe("a"), bun.Safe("b"), bun.Safe("c"))

		b, err := expr.AppendQuery(gen, nil)
		assert.NoError(t, err, "Should append multiple elements")

		result := string(b)
		assert.Contains(t, result, "a", "Should contain first element")
		assert.Contains(t, result, "b", "Should contain second element")
		assert.Contains(t, result, "c", "Should contain third element")
		assert.Contains(t, result, ", ", "Should separate elements with comma")
	})

	t.Run("BytesSliceNotNested", func(t *testing.T) {
		// []byte should NOT be treated as a nested slice
		data := []byte("hello")
		expr := newExpressions(", ", data)

		b, err := expr.AppendQuery(gen, nil)
		assert.NoError(t, err, "Should handle []byte element")

		result := string(b)
		assert.NotContains(t, result, "(", "Should not wrap []byte in parentheses")
	})
}

// ErrorAppender is a QueryAppender that always returns an error.
type ErrorAppender struct{}

func (ErrorAppender) AppendQuery(schema.QueryGen, []byte) ([]byte, error) {
	return nil, errors.New("test error")
}

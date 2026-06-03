package orm

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun/schema"
)

// StubAppender is a schema.QueryAppender whose output and error are fully controlled
// by the test, so filterClause can be exercised without a real dialect or query.
type StubAppender struct {
	out []byte
	err error
}

func (s StubAppender) AppendQuery(_ schema.QueryGen, b []byte) ([]byte, error) {
	if s.err != nil {
		return nil, s.err
	}

	return append(b, s.out...), nil
}

func TestFilterClause(t *testing.T) {
	t.Run("WrapsConditionInFilterWhere", func(t *testing.T) {
		got, err := newFilterClause(StubAppender{out: []byte("amount > 0")}).AppendQuery(newTestQueryGen(), nil)

		require.NoError(t, err, "rendering a valid condition should not error")
		assert.Equal(t, " FILTER (WHERE amount > 0)", string(got),
			"FILTER clause should wrap the condition in FILTER (WHERE ...)")
	})

	t.Run("PreservesExistingBuffer", func(t *testing.T) {
		got, err := newFilterClause(StubAppender{out: []byte("x")}).AppendQuery(newTestQueryGen(), []byte("COUNT(*)"))

		require.NoError(t, err, "rendering onto an existing buffer should not error")
		assert.Equal(t, "COUNT(*) FILTER (WHERE x)", string(got),
			"FILTER clause should append to the supplied buffer rather than replace it")
	})

	t.Run("PropagatesConditionError", func(t *testing.T) {
		wantErr := errors.New("condition render failed")

		got, err := newFilterClause(StubAppender{err: wantErr}).AppendQuery(newTestQueryGen(), []byte("COUNT(*)"))

		require.ErrorIs(t, err, wantErr, "condition rendering error should be propagated unchanged")
		assert.Nil(t, got, "no buffer should be returned when the condition fails to render")
	})
}

package orm

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWithQuietSQLLog(t *testing.T) {
	ctx := context.Background()
	assert.False(t, IsQuietSQLLog(ctx), "An unmarked context must not read as quiet")

	quiet := WithQuietSQLLog(ctx)
	assert.True(t, IsQuietSQLLog(quiet), "A marked context must read as quiet")
	assert.False(t, IsQuietSQLLog(ctx), "Marking must not mutate the original context")

	type nestedKey struct{}

	nested := context.WithValue(quiet, nestedKey{}, "value")
	assert.True(t, IsQuietSQLLog(nested), "The mark must survive further context derivation")
}

func TestWithoutQuietSQLLog(t *testing.T) {
	t.Run("LiftsTheMark", func(t *testing.T) {
		quiet := WithQuietSQLLog(context.Background())

		lifted := WithoutQuietSQLLog(quiet)
		assert.False(t, IsQuietSQLLog(lifted), "Lifting must clear the quiet mark")
		assert.True(t, IsQuietSQLLog(quiet), "Lifting must not mutate the original context")
	})

	t.Run("LiftsThroughFurtherDerivation", func(t *testing.T) {
		type nestedKey struct{}

		nested := context.WithValue(WithQuietSQLLog(context.Background()), nestedKey{}, "value")

		lifted := WithoutQuietSQLLog(nested)
		assert.False(t, IsQuietSQLLog(lifted), "Lifting must clear a mark inherited from an outer context")
		assert.Equal(t, "value", lifted.Value(nestedKey{}), "Lifting must preserve unrelated context values")
	})

	t.Run("UnmarkedContextIsReturnedUnchanged", func(t *testing.T) {
		ctx := context.Background()

		lifted := WithoutQuietSQLLog(ctx)
		assert.False(t, IsQuietSQLLog(lifted), "An unmarked context stays unmarked")
		assert.Equal(t, ctx, lifted, "Lifting an unmarked context must not wrap it")
	})

	t.Run("MarkCanBeReapplied", func(t *testing.T) {
		remarked := WithQuietSQLLog(WithoutQuietSQLLog(WithQuietSQLLog(context.Background())))
		assert.True(t, IsQuietSQLLog(remarked), "Marking a lifted context must make it quiet again")
	})
}

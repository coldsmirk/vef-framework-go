package expression_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/expression"
	"github.com/coldsmirk/vef-framework-go/result"
)

func TestErrEvaluationFailed(t *testing.T) {
	t.Run("Code", func(t *testing.T) {
		assert.Equal(t, expression.ErrCodeEvaluationFailed, expression.ErrEvaluationFailed.Code, "sentinel should carry its code")
	})

	t.Run("WrappedIdentity", func(t *testing.T) {
		cause := errors.New("zen: bad expression")
		err := fmt.Errorf("%w: %w", expression.ErrEvaluationFailed, cause)

		assert.ErrorIs(t, err, expression.ErrEvaluationFailed, "wrapped error should match the sentinel by code")
		assert.ErrorIs(t, err, cause, "wrapped error should preserve the cause")

		got, ok := result.AsErr(err)
		require.True(t, ok, "result.AsErr should extract the business error")
		assert.Equal(t, expression.ErrCodeEvaluationFailed, got.Code, "extracted error should carry the code")
	})
}

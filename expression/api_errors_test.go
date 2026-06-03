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
		assert.Equal(t, expression.ErrCodeEvaluationFailed, expression.ErrEvaluationFailed.Code, "Sentinel should carry its code")
	})

	t.Run("WrappedIdentity", func(t *testing.T) {
		cause := errors.New("zen: bad expression")
		err := fmt.Errorf("%w: %w", expression.ErrEvaluationFailed, cause)

		assert.ErrorIs(t, err, expression.ErrEvaluationFailed, "Wrapped error should match the sentinel by code")
		assert.ErrorIs(t, err, cause, "Wrapped error should preserve the cause")

		got, ok := result.AsErr(err)
		require.True(t, ok, "Business error should be extracted via result.AsErr")
		assert.Equal(t, expression.ErrCodeEvaluationFailed, got.Code, "Extracted error should carry the code")
	})
}

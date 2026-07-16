package exec

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coldsmirk/vef-framework-go/integration"
)

// TestClassify pins the failure classification, in particular the split of
// caller cancellation from a run timeout — conflating the two would distort
// the per-node health statistics.
func TestClassify(t *testing.T) {
	t.Run("CallerCancellationClassifiesCanceled", func(t *testing.T) {
		kind, apiErr := classify(context.Background(), context.Canceled)
		assert.Equal(t, integration.FailureCanceled, kind, "A caller cancellation is canceled, not a timeout")
		assert.ErrorIs(t, apiErr, integration.ErrInvocationCanceled, "The API error is the canceled sentinel")
	})

	t.Run("RunDeadlineClassifiesTimeout", func(t *testing.T) {
		kind, apiErr := classify(context.Background(), context.DeadlineExceeded)
		assert.Equal(t, integration.FailureTimeout, kind, "An exceeded deadline is a timeout")
		assert.ErrorIs(t, apiErr, integration.ErrInvocationTimeout, "The API error is the timeout sentinel")
	})

	t.Run("CanceledContextWinsOverDeadlineError", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		// Even if the surfaced error looks like a deadline, a canceled context
		// means the caller walked away — classify checks cancellation first.
		kind, _ := classify(ctx, context.DeadlineExceeded)
		assert.Equal(t, integration.FailureCanceled, kind, "A canceled context takes precedence over a deadline error")
	})

	t.Run("UncategorizedErrorClassifiesScript", func(t *testing.T) {
		kind, _ := classify(context.Background(), errors.New("boom"))
		assert.Equal(t, integration.FailureScript, kind, "An uncategorized error is a script failure")
	})
}

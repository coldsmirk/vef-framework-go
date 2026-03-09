package shared

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestComputeTaskDeadline(t *testing.T) {
	t.Run("ZeroTimeout", func(t *testing.T) {
		assert.Nil(t, ComputeTaskDeadline(0), "Should return nil when timeout is disabled")
	})

	t.Run("NegativeTimeout", func(t *testing.T) {
		assert.Nil(t, ComputeTaskDeadline(-1), "Should return nil when timeout is negative")
	})

	t.Run("PositiveTimeout", func(t *testing.T) {
		before := time.Now()
		deadline := ComputeTaskDeadline(4)
		after := time.Now()

		require.NotNil(t, deadline, "Should return non-nil deadline for positive timeout")
		assert.True(
			t,
			deadline.Unwrap().After(before.Add(3*time.Hour+59*time.Minute)),
			"Deadline should be near now plus configured timeout hours",
		)
		assert.True(
			t,
			deadline.Unwrap().Before(after.Add(4*time.Hour+time.Minute)),
			"Deadline should be near now plus configured timeout hours",
		)
	})
}

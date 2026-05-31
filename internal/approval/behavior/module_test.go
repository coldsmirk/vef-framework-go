package behavior

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
)

func TestAssertCollectorBehaviorsRegistered(t *testing.T) {
	t.Run("PassesWhenBothCollectorBehaviorsArePresent", func(t *testing.T) {
		behaviors := []cqrs.Behavior{
			&collectorBehavior[*approval.ActionLog]{},
			&collectorBehavior[approval.DomainEvent]{},
		}

		require.NoError(t, assertCollectorBehaviorsRegistered(behaviors),
			"Both collector behaviors should satisfy startup check")
	})

	t.Run("FailsWhenBothCollectorBehaviorsAreMissing", func(t *testing.T) {
		err := assertCollectorBehaviorsRegistered(nil)

		require.Error(t, err, "Missing collector behaviors should return an error")
		assert.ErrorIs(t, err, ErrMissingCollectorBehavior, "Error should wrap ErrMissingCollectorBehavior")
		assert.Contains(t, err.Error(), "ActionLogBehavior", "Error should name missing ActionLogBehavior")
		assert.Contains(t, err.Error(), "EventPublishBehavior", "Error should name missing EventPublishBehavior")
	})

	t.Run("FailsWhenActionLogBehaviorIsMissing", func(t *testing.T) {
		behaviors := []cqrs.Behavior{
			&collectorBehavior[approval.DomainEvent]{},
		}

		err := assertCollectorBehaviorsRegistered(behaviors)

		require.Error(t, err, "Missing ActionLogBehavior should return an error")
		assert.ErrorIs(t, err, ErrMissingCollectorBehavior, "Error should wrap ErrMissingCollectorBehavior")
		assert.Contains(t, err.Error(), "ActionLogBehavior", "Error should name missing ActionLogBehavior")
		assert.NotContains(t, err.Error(), "EventPublishBehavior", "Error should not name present EventPublishBehavior")
	})

	t.Run("FailsWhenEventPublishBehaviorIsMissing", func(t *testing.T) {
		behaviors := []cqrs.Behavior{
			&collectorBehavior[*approval.ActionLog]{},
		}

		err := assertCollectorBehaviorsRegistered(behaviors)

		require.Error(t, err, "Missing EventPublishBehavior should return an error")
		assert.ErrorIs(t, err, ErrMissingCollectorBehavior, "Error should wrap ErrMissingCollectorBehavior")
		assert.Contains(t, err.Error(), "EventPublishBehavior", "Error should name missing EventPublishBehavior")
		assert.NotContains(t, err.Error(), "ActionLogBehavior", "Error should not name present ActionLogBehavior")
	})
}

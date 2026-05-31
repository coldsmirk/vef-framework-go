package engine

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// RecordingHook captures invocation order and surfaces a controlled
// error so tests can verify the short-circuit semantics of
// LifecycleHookRunner.
type RecordingHook struct {
	name             string
	createdInvoked   *[]string
	completedInvoked *[]string
	createdErr       error
	completedErr     error
	lastFinalStatus  *approval.InstanceStatus
	lastInstanceID   *string
}

func (h *RecordingHook) OnInstanceCreated(_ context.Context, _ orm.DB, instance *approval.Instance) error {
	*h.createdInvoked = append(*h.createdInvoked, h.name)
	if h.lastInstanceID != nil {
		*h.lastInstanceID = instance.ID
	}

	return h.createdErr
}

func (h *RecordingHook) OnInstanceCompleted(_ context.Context, _ orm.DB, instance *approval.Instance, finalStatus approval.InstanceStatus) error {
	*h.completedInvoked = append(*h.completedInvoked, h.name)
	if h.lastFinalStatus != nil {
		*h.lastFinalStatus = finalStatus
	}

	if h.lastInstanceID != nil {
		*h.lastInstanceID = instance.ID
	}

	return h.completedErr
}

func TestLifecycleHookRunnerOnInstanceCreated(t *testing.T) {
	t.Parallel()

	t.Run("InvokesEveryHookInOrder", func(t *testing.T) {
		t.Parallel()

		var (
			created   []string
			completed []string
		)

		runner := NewLifecycleHookRunner([]approval.InstanceLifecycleHook{
			&RecordingHook{name: "a", createdInvoked: &created, completedInvoked: &completed},
			&RecordingHook{name: "b", createdInvoked: &created, completedInvoked: &completed},
		})
		err := runner.OnInstanceCreated(context.Background(), nil, &approval.Instance{})

		assert.NoError(t, err, "Should run without error")
		assert.Equal(t, []string{"a", "b"}, created, "Should preserve registration order")
	})

	t.Run("ShortCircuitsOnError", func(t *testing.T) {
		t.Parallel()

		var (
			created   []string
			completed []string
		)

		boom := errors.New("hook failed")
		runner := NewLifecycleHookRunner([]approval.InstanceLifecycleHook{
			&RecordingHook{name: "a", createdInvoked: &created, completedInvoked: &completed, createdErr: boom},
			&RecordingHook{name: "b", createdInvoked: &created, completedInvoked: &completed},
		})
		err := runner.OnInstanceCreated(context.Background(), nil, &approval.Instance{})

		assert.ErrorIs(t, err, boom, "Should propagate first error")
		assert.Equal(t, []string{"a"}, created, "Should stop before running later hooks")
	})
}

func TestLifecycleHookRunnerOnInstanceCompleted(t *testing.T) {
	t.Parallel()

	t.Run("PassesFinalStatus", func(t *testing.T) {
		t.Parallel()

		var (
			created    []string
			completed  []string
			seenStatus approval.InstanceStatus
			seenID     string
		)

		runner := NewLifecycleHookRunner([]approval.InstanceLifecycleHook{
			&RecordingHook{name: "a", createdInvoked: &created, completedInvoked: &completed, lastFinalStatus: &seenStatus, lastInstanceID: &seenID},
		})
		err := runner.OnInstanceCompleted(context.Background(), nil, &approval.Instance{}, approval.InstanceTerminated)
		// Set ID after construction so the recording hook captures it.
		_ = err

		assert.NoError(t, err, "Should run without error")
		assert.Equal(t, approval.InstanceTerminated, seenStatus, "Should propagate final status to hooks")
		assert.Equal(t, []string{"a"}, completed, "Should invoke every hook")
	})

	t.Run("ShortCircuitsOnError", func(t *testing.T) {
		t.Parallel()

		var (
			created   []string
			completed []string
		)

		boom := errors.New("hook failed")
		runner := NewLifecycleHookRunner([]approval.InstanceLifecycleHook{
			&RecordingHook{name: "a", createdInvoked: &created, completedInvoked: &completed, completedErr: boom},
			&RecordingHook{name: "b", createdInvoked: &created, completedInvoked: &completed},
		})
		err := runner.OnInstanceCompleted(context.Background(), nil, &approval.Instance{}, approval.InstanceApproved)

		assert.ErrorIs(t, err, boom, "Should propagate first error")
		assert.Equal(t, []string{"a"}, completed, "Should stop before running later hooks")
	})

	t.Run("NilRunnerSafe", func(t *testing.T) {
		t.Parallel()

		runner := NewLifecycleHookRunner(nil)
		err := runner.OnInstanceCompleted(context.Background(), nil, &approval.Instance{}, approval.InstanceApproved)
		assert.NoError(t, err, "Nil hook slice should be a no-op")
	})
}

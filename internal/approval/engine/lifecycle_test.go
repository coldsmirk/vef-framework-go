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

type RecordingProjector struct {
	instanceIDs *[]string
	err         error
}

func (p *RecordingProjector) Project(_ context.Context, _ orm.DB, instance *approval.Instance) error {
	*p.instanceIDs = append(*p.instanceIDs, instance.ID)

	return p.err
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

		runner := NewLifecycleHookRunner(nil, []approval.InstanceLifecycleHook{
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
		runner := NewLifecycleHookRunner(nil, []approval.InstanceLifecycleHook{
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

		runner := NewLifecycleHookRunner(nil, []approval.InstanceLifecycleHook{
			&RecordingHook{name: "a", createdInvoked: &created, completedInvoked: &completed, lastFinalStatus: &seenStatus, lastInstanceID: &seenID},
		})

		instance := &approval.Instance{}
		instance.ID = "inst-1"

		err := runner.OnInstanceCompleted(context.Background(), nil, instance, approval.InstanceTerminated)
		assert.NoError(t, err, "Should run without error")
		assert.Equal(t, approval.InstanceTerminated, seenStatus, "Should propagate final status to hooks")
		assert.Equal(t, "inst-1", seenID, "Should propagate the instance ID to hooks")
		assert.Equal(t, []string{"a"}, completed, "Should invoke every hook")
	})

	t.Run("ShortCircuitsOnError", func(t *testing.T) {
		t.Parallel()

		var (
			created   []string
			completed []string
		)

		boom := errors.New("hook failed")
		runner := NewLifecycleHookRunner(nil, []approval.InstanceLifecycleHook{
			&RecordingHook{name: "a", createdInvoked: &created, completedInvoked: &completed, completedErr: boom},
			&RecordingHook{name: "b", createdInvoked: &created, completedInvoked: &completed},
		})
		err := runner.OnInstanceCompleted(context.Background(), nil, &approval.Instance{}, approval.InstanceApproved)

		assert.ErrorIs(t, err, boom, "Should propagate first error")
		assert.Equal(t, []string{"a"}, completed, "Should stop before running later hooks")
	})

	t.Run("NilRunnerSafe", func(t *testing.T) {
		t.Parallel()

		runner := NewLifecycleHookRunner(nil, nil)
		err := runner.OnInstanceCompleted(context.Background(), nil, &approval.Instance{}, approval.InstanceApproved)
		assert.NoError(t, err, "Nil hook slice should be a no-op")
	})
}

func TestLifecycleHookRunnerOnInstanceTransitioned(t *testing.T) {
	t.Parallel()

	t.Run("ProjectsTransitionedInstance", func(t *testing.T) {
		t.Parallel()

		var instanceIDs []string

		runner := NewLifecycleHookRunner(&RecordingProjector{instanceIDs: &instanceIDs}, nil)
		instance := new(approval.Instance)
		instance.ID = "instance-1"

		err := runner.OnInstanceTransitioned(context.Background(), nil, instance)
		assert.NoError(t, err, "Transition projector should run without error")
		assert.Equal(t, []string{"instance-1"}, instanceIDs,
			"Transition projector should receive the transitioned instance")
	})

	t.Run("PropagatesProjectorError", func(t *testing.T) {
		t.Parallel()

		var instanceIDs []string

		boom := errors.New("projection failed")
		runner := NewLifecycleHookRunner(&RecordingProjector{instanceIDs: &instanceIDs, err: boom}, nil)

		err := runner.OnInstanceTransitioned(context.Background(), nil, new(approval.Instance))
		assert.ErrorIs(t, err, boom, "Transition projector error should abort the caller transaction")
		assert.Len(t, instanceIDs, 1, "Transition projector should be invoked exactly once")
	})

	t.Run("NilProjectorIsNoOp", func(t *testing.T) {
		t.Parallel()

		runner := NewLifecycleHookRunner(nil, nil)
		err := runner.OnInstanceTransitioned(context.Background(), nil, new(approval.Instance))
		assert.NoError(t, err, "Missing projector should be a no-op for non-binding test fixtures")
	})
}

package engine

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// RecordingHook appends its invocations to a shared log and surfaces
// controlled errors so tests can verify the ordering and short-circuit
// semantics of LifecycleHookRunner.
type RecordingHook struct {
	name           string
	log            *[]string
	createdErr     error
	transitionErr  error
	lastFrom       *approval.InstanceStatus
	lastTo         *approval.InstanceStatus
	lastInstanceID *string
}

// RecordingProjector appends its invocations to the same shared log so the
// projector-before-hooks ordering is observable.
type RecordingProjector struct {
	log *[]string
	err error
}

func (p *RecordingProjector) Project(_ context.Context, _ orm.DB, instance *approval.Instance) error {
	*p.log = append(*p.log, "projector:"+instance.ID)

	return p.err
}

func (h *RecordingHook) OnInstanceCreated(_ context.Context, _ orm.DB, instance *approval.Instance) error {
	*h.log = append(*h.log, h.name+":created")
	if h.lastInstanceID != nil {
		*h.lastInstanceID = instance.ID
	}

	return h.createdErr
}

func (h *RecordingHook) OnInstanceTransition(_ context.Context, _ orm.DB, instance *approval.Instance, from, to approval.InstanceStatus) error {
	*h.log = append(*h.log, h.name+":transition")
	if h.lastFrom != nil {
		*h.lastFrom = from
	}

	if h.lastTo != nil {
		*h.lastTo = to
	}

	if h.lastInstanceID != nil {
		*h.lastInstanceID = instance.ID
	}

	return h.transitionErr
}

func TestLifecycleHookRunnerOnInstanceCreated(t *testing.T) {
	t.Parallel()

	t.Run("InvokesEveryHookInOrder", func(t *testing.T) {
		t.Parallel()

		var log []string

		runner := NewLifecycleHookRunner(nil, []approval.InstanceLifecycleHook{
			&RecordingHook{name: "a", log: &log},
			&RecordingHook{name: "b", log: &log},
		})
		err := runner.OnInstanceCreated(context.Background(), nil, &approval.Instance{})

		assert.NoError(t, err, "Should run without error")
		assert.Equal(t, []string{"a:created", "b:created"}, log, "Should preserve registration order")
	})

	t.Run("ShortCircuitsOnError", func(t *testing.T) {
		t.Parallel()

		var log []string

		boom := errors.New("hook failed")
		runner := NewLifecycleHookRunner(nil, []approval.InstanceLifecycleHook{
			&RecordingHook{name: "a", log: &log, createdErr: boom},
			&RecordingHook{name: "b", log: &log},
		})
		err := runner.OnInstanceCreated(context.Background(), nil, &approval.Instance{})

		assert.ErrorIs(t, err, boom, "Should propagate first error")
		assert.Equal(t, []string{"a:created"}, log, "Should stop before running later hooks")
	})
}

func TestLifecycleHookRunnerOnInstanceTransition(t *testing.T) {
	t.Parallel()

	t.Run("ProjectsBeforeHooks", func(t *testing.T) {
		t.Parallel()

		var (
			log      []string
			seenFrom approval.InstanceStatus
			seenTo   approval.InstanceStatus
			seenID   string
		)

		runner := NewLifecycleHookRunner(&RecordingProjector{log: &log}, []approval.InstanceLifecycleHook{
			&RecordingHook{name: "a", log: &log, lastFrom: &seenFrom, lastTo: &seenTo, lastInstanceID: &seenID},
			&RecordingHook{name: "b", log: &log},
		})

		instance := new(approval.Instance)
		instance.ID = "inst-1"

		err := runner.OnInstanceTransition(context.Background(), nil, instance, approval.InstanceRunning, approval.InstanceTerminated)
		assert.NoError(t, err, "Should run without error")
		assert.Equal(t, []string{"projector:inst-1", "a:transition", "b:transition"}, log,
			"Projection must complete before any host hook observes the transition")
		assert.Equal(t, approval.InstanceRunning, seenFrom, "Should pass the pre-transition status to hooks")
		assert.Equal(t, approval.InstanceTerminated, seenTo, "Should pass the post-transition status to hooks")
		assert.Equal(t, "inst-1", seenID, "Should pass the transitioned instance to hooks")
	})

	t.Run("ProjectorErrorSkipsHooks", func(t *testing.T) {
		t.Parallel()

		var log []string

		boom := errors.New("projection failed")
		runner := NewLifecycleHookRunner(&RecordingProjector{log: &log, err: boom}, []approval.InstanceLifecycleHook{
			&RecordingHook{name: "a", log: &log},
		})

		err := runner.OnInstanceTransition(context.Background(), nil, new(approval.Instance), approval.InstanceRunning, approval.InstanceApproved)
		assert.ErrorIs(t, err, boom, "Projection failure should abort the caller transaction")
		assert.Equal(t, []string{"projector:"}, log, "No host hook may run after a projection failure")
	})

	t.Run("HookShortCircuitsOnError", func(t *testing.T) {
		t.Parallel()

		var log []string

		boom := errors.New("hook failed")
		runner := NewLifecycleHookRunner(nil, []approval.InstanceLifecycleHook{
			&RecordingHook{name: "a", log: &log, transitionErr: boom},
			&RecordingHook{name: "b", log: &log},
		})

		err := runner.OnInstanceTransition(context.Background(), nil, new(approval.Instance), approval.InstanceRunning, approval.InstanceReturned)
		assert.ErrorIs(t, err, boom, "Should propagate first error")
		assert.Equal(t, []string{"a:transition"}, log, "Should stop before running later hooks")
	})

	t.Run("NilProjectorRunsHooks", func(t *testing.T) {
		t.Parallel()

		var log []string

		runner := NewLifecycleHookRunner(nil, []approval.InstanceLifecycleHook{
			&RecordingHook{name: "a", log: &log},
		})

		err := runner.OnInstanceTransition(context.Background(), nil, new(approval.Instance), approval.InstanceReturned, approval.InstanceRunning)
		assert.NoError(t, err, "Missing projector should be a no-op for non-binding test fixtures")
		assert.Equal(t, []string{"a:transition"}, log, "Hooks should still run without a projector")
	})

	t.Run("NilRunnerSafe", func(t *testing.T) {
		t.Parallel()

		var runner *LifecycleHookRunner

		err := runner.OnInstanceTransition(context.Background(), nil, new(approval.Instance), approval.InstanceRunning, approval.InstanceApproved)
		assert.NoError(t, err, "A nil runner must be callable so fixtures can pass hooks=nil")
	})
}

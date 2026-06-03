package command

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
)

// TestRegisterHandlers verifies that every command type provided in the module
// is also registered with the bus. A handler that is provided via fx.Provide
// but omitted from registerHandlers would compile and wire cleanly yet leave
// the command silently unhandled at runtime; this test catches that drift.
func TestRegisterHandlers(t *testing.T) {
	bus := cqrs.NewBus(nil)

	// Zero-value handler instances — safe for registration; we never dispatch
	// to them, only verify that cqrs.Register was called for each command type.
	registerHandlers(
		bus,
		new(CreateFlowHandler),
		new(DeployFlowHandler),
		new(PublishVersionHandler),
		new(UpdateFlowHandler),
		new(ToggleFlowActiveHandler),
		new(ApproveTaskHandler),
		new(RejectTaskHandler),
		new(TransferTaskHandler),
		new(RollbackTaskHandler),
		new(StartInstanceHandler),
		new(WithdrawHandler),
		new(ResubmitHandler),
		new(AddCCHandler),
		new(MarkCCReadHandler),
		new(AddAssigneeHandler),
		new(RemoveAssigneeHandler),
		new(UrgeTaskHandler),
		new(TerminateInstanceHandler),
		new(ReassignTaskHandler),
	)

	// For each expected command type, send a zero-value command and confirm the
	// bus did not return ErrHandlerNotFound. A nil-receiver panic means the
	// handler was registered but its deps are nil — that still proves registration.
	cases := []struct {
		name string
		send func() error
	}{
		{"CreateFlow", func() (err error) {
			defer recoverDispatch(&err)

			_, err = cqrs.Send[CreateFlowCmd, *approval.Flow](context.Background(), bus, CreateFlowCmd{})

			return err
		}},
		{"DeployFlow", func() (err error) {
			defer recoverDispatch(&err)

			_, err = cqrs.Send[DeployFlowCmd, *approval.FlowVersion](context.Background(), bus, DeployFlowCmd{})

			return err
		}},
		{"PublishVersion", func() (err error) {
			defer recoverDispatch(&err)

			_, err = cqrs.Send[PublishVersionCmd, cqrs.Unit](context.Background(), bus, PublishVersionCmd{})

			return err
		}},
		{"UpdateFlow", func() (err error) {
			defer recoverDispatch(&err)

			_, err = cqrs.Send[UpdateFlowCmd, *approval.Flow](context.Background(), bus, UpdateFlowCmd{})

			return err
		}},
		{"ToggleFlowActive", func() (err error) {
			defer recoverDispatch(&err)

			_, err = cqrs.Send[ToggleFlowActiveCmd, cqrs.Unit](context.Background(), bus, ToggleFlowActiveCmd{})

			return err
		}},
		{"ApproveTask", func() (err error) {
			defer recoverDispatch(&err)

			_, err = cqrs.Send[ApproveTaskCmd, cqrs.Unit](context.Background(), bus, ApproveTaskCmd{})

			return err
		}},
		{"RejectTask", func() (err error) {
			defer recoverDispatch(&err)

			_, err = cqrs.Send[RejectTaskCmd, cqrs.Unit](context.Background(), bus, RejectTaskCmd{})

			return err
		}},
		{"TransferTask", func() (err error) {
			defer recoverDispatch(&err)

			_, err = cqrs.Send[TransferTaskCmd, cqrs.Unit](context.Background(), bus, TransferTaskCmd{})

			return err
		}},
		{"RollbackTask", func() (err error) {
			defer recoverDispatch(&err)

			_, err = cqrs.Send[RollbackTaskCmd, cqrs.Unit](context.Background(), bus, RollbackTaskCmd{})

			return err
		}},
		{"StartInstance", func() (err error) {
			defer recoverDispatch(&err)

			_, err = cqrs.Send[StartInstanceCmd, *approval.Instance](context.Background(), bus, StartInstanceCmd{})

			return err
		}},
		{"Withdraw", func() (err error) {
			defer recoverDispatch(&err)

			_, err = cqrs.Send[WithdrawCmd, cqrs.Unit](context.Background(), bus, WithdrawCmd{})

			return err
		}},
		{"Resubmit", func() (err error) {
			defer recoverDispatch(&err)

			_, err = cqrs.Send[ResubmitCmd, cqrs.Unit](context.Background(), bus, ResubmitCmd{})

			return err
		}},
		{"AddCC", func() (err error) {
			defer recoverDispatch(&err)

			_, err = cqrs.Send[AddCCCmd, cqrs.Unit](context.Background(), bus, AddCCCmd{})

			return err
		}},
		{"MarkCCRead", func() (err error) {
			defer recoverDispatch(&err)

			_, err = cqrs.Send[MarkCCReadCmd, cqrs.Unit](context.Background(), bus, MarkCCReadCmd{})

			return err
		}},
		{"AddAssignee", func() (err error) {
			defer recoverDispatch(&err)

			_, err = cqrs.Send[AddAssigneeCmd, cqrs.Unit](context.Background(), bus, AddAssigneeCmd{})

			return err
		}},
		{"RemoveAssignee", func() (err error) {
			defer recoverDispatch(&err)

			_, err = cqrs.Send[RemoveAssigneeCmd, cqrs.Unit](context.Background(), bus, RemoveAssigneeCmd{})

			return err
		}},
		{"UrgeTask", func() (err error) {
			defer recoverDispatch(&err)

			_, err = cqrs.Send[UrgeTaskCmd, cqrs.Unit](context.Background(), bus, UrgeTaskCmd{})

			return err
		}},
		{"TerminateInstance", func() (err error) {
			defer recoverDispatch(&err)

			_, err = cqrs.Send[TerminateInstanceCmd, cqrs.Unit](context.Background(), bus, TerminateInstanceCmd{})

			return err
		}},
		{"ReassignTask", func() (err error) {
			defer recoverDispatch(&err)

			_, err = cqrs.Send[ReassignTaskCmd, cqrs.Unit](context.Background(), bus, ReassignTaskCmd{})

			return err
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.send()
			assert.False(t, errors.Is(err, cqrs.ErrHandlerNotFound),
				"Handler for %s must be registered with the bus", tc.name)
		})
	}
}

// recoverDispatch converts a nil-receiver dispatch panic into a nil error so
// the registration check treats a panic as "handler was registered and found".
// Only cqrs.ErrHandlerNotFound means the handler is missing from the bus.
func recoverDispatch(errp *error) {
	//nolint:revive // recoverDispatch is only ever invoked via defer, so recover() catches the dispatch panic correctly.
	if recover() != nil {
		// Panic means the handler was found but its nil deps panicked on dispatch.
		// This proves registration succeeded.
		*errp = nil
	}
}

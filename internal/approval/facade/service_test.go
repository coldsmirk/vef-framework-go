package facade_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/approval/behavior"
	"github.com/coldsmirk/vef-framework-go/internal/approval/command"
	"github.com/coldsmirk/vef-framework-go/internal/approval/facade"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// Captured is what a stub command handler records: the command it received
// and the DB handle bound to its context, which is how the facade hands the
// caller's transaction boundary to the pipeline.
type Captured[TCmd any] struct {
	cmd TCmd
	db  orm.DB
}

// capture registers a stub handler for TCmd on bus that records its input and
// returns the zero TResult.
func capture[TCmd cqrs.Action, TResult any](bus cqrs.Bus) *Captured[TCmd] {
	c := new(Captured[TCmd])

	cqrs.Register(bus, cqrs.HandlerFunc[TCmd, TResult](func(ctx context.Context, cmd TCmd) (TResult, error) {
		c.cmd = cmd
		c.db = contextx.DB(ctx)

		var zero TResult

		return zero, nil
	}))

	return c
}

// requireNoZeroField fails when any exported field reachable from cmd is left
// at its zero value. The forwarding assertions compare a captured command with
// the expected one, so a field the test forgot to populate would pass
// trivially — this makes such an omission fail instead. Embedded structs are
// walked rather than skipped because a command carries its whole payload that
// way (command.ApproveTaskCmd embeds approval.ApproveTaskInput).
func requireNoZeroField(t *testing.T, cmd any) {
	t.Helper()
	requireNoZeroFieldValue(t, reflect.ValueOf(cmd))
}

func requireNoZeroFieldValue(t *testing.T, v reflect.Value) {
	t.Helper()

	for i := range v.NumField() {
		field := v.Type().Field(i)

		switch {
		case field.Anonymous && field.Type.Kind() == reflect.Struct:
			requireNoZeroFieldValue(t, v.Field(i))
		case !field.IsExported():
			continue
		default:
			require.False(t, v.Field(i).IsZero(), "test input must populate %s.%s so forwarding is actually checked", v.Type().Name(), field.Name)
		}
	}
}

// assertForwards runs call against a bus whose TCmd handler is a capturing
// stub and asserts that the handler received exactly want, on a handle that
// is inside a transaction.
func assertForwards[TCmd cqrs.Action, TResult any](t *testing.T, bus cqrs.Bus, db orm.DB, want TCmd, call func(context.Context, orm.DB) error) {
	t.Helper()
	requireNoZeroField(t, want)

	got := capture[TCmd, TResult](bus)

	require.NoError(t, call(context.Background(), db), "facade call should dispatch without error")
	assert.Equal(t, want, got.cmd, "facade must forward every input field onto %T", want)
	require.NotNil(t, got.db, "handler must see a DB handle bound to its context")
	assert.True(t, got.db.InTx(), "handler must run inside a transaction")
}

var (
	testUser   = approval.UserInfo{ID: "u-1", Name: "User One", DepartmentID: new("d-1"), DepartmentName: new("Dept One")}
	testCaller = approval.CallerContext{TenantID: "t-1"}
	testForm   = map[string]any{"amount": 42}
	testFiles  = []string{"file-1", "file-2"}
)

// newBus builds a bus carrying the real TransactionBehavior over db so the
// forwarding tests exercise the actual transaction-boundary contract rather
// than a stub of it.
func newBus(db orm.DB) cqrs.Bus {
	return cqrs.NewBus([]cqrs.Behavior{behavior.NewTransactionBehavior(db)})
}

func TestServiceForwardsInputs(t *testing.T) {
	db := testx.NewTestDB(t)
	bus := newBus(db)
	svc := facade.NewService(bus)

	t.Run("StartInstance", func(t *testing.T) {
		in := approval.StartInstanceInput{
			TenantID:    "t-1",
			FlowCode:    "leave",
			Applicant:   testUser,
			BusinessRef: new("order-1"),
			FormData:    testForm,
			Globals:     map[string]any{"region": "cn"},
			Caller:      testCaller,
		}

		assertForwards[command.StartInstanceCmd, *approval.Instance](t, bus, db,
			command.StartInstanceCmd{StartInstanceInput: in},
			func(ctx context.Context, db orm.DB) error {
				_, err := svc.StartInstance(ctx, db, in)

				return err
			})
	})

	t.Run("WithdrawInstance", func(t *testing.T) {
		in := approval.WithdrawInstanceInput{InstanceID: "i-1", Operator: testUser, Reason: "changed my mind", Caller: testCaller}

		assertForwards[command.WithdrawInstanceCmd, cqrs.Unit](t, bus, db,
			command.WithdrawInstanceCmd{WithdrawInstanceInput: in},
			func(ctx context.Context, db orm.DB) error {
				return svc.WithdrawInstance(ctx, db, in)
			})
	})

	t.Run("ResubmitInstance", func(t *testing.T) {
		in := approval.ResubmitInstanceInput{InstanceID: "i-1", Operator: testUser, FormData: testForm, Caller: testCaller}

		assertForwards[command.ResubmitInstanceCmd, cqrs.Unit](t, bus, db,
			command.ResubmitInstanceCmd{ResubmitInstanceInput: in},
			func(ctx context.Context, db orm.DB) error {
				return svc.ResubmitInstance(ctx, db, in)
			})
	})

	t.Run("TerminateInstance", func(t *testing.T) {
		in := approval.TerminateInstanceInput{InstanceID: "i-1", Operator: testUser, Reason: "order canceled", Caller: testCaller}

		assertForwards[command.TerminateInstanceCmd, cqrs.Unit](t, bus, db,
			command.TerminateInstanceCmd{TerminateInstanceInput: in},
			func(ctx context.Context, db orm.DB) error {
				return svc.TerminateInstance(ctx, db, in)
			})
	})

	t.Run("ApproveTask", func(t *testing.T) {
		in := approval.ApproveTaskInput{TaskID: "k-1", Operator: testUser, Opinion: "ok", FormData: testForm, Attachments: testFiles, Caller: testCaller}

		assertForwards[command.ApproveTaskCmd, cqrs.Unit](t, bus, db,
			command.ApproveTaskCmd{ApproveTaskInput: in},
			func(ctx context.Context, db orm.DB) error {
				return svc.ApproveTask(ctx, db, in)
			})
	})

	t.Run("RejectTask", func(t *testing.T) {
		in := approval.RejectTaskInput{TaskID: "k-1", Operator: testUser, Opinion: "no", FormData: testForm, Attachments: testFiles, Caller: testCaller}

		assertForwards[command.RejectTaskCmd, cqrs.Unit](t, bus, db,
			command.RejectTaskCmd{RejectTaskInput: in},
			func(ctx context.Context, db orm.DB) error {
				return svc.RejectTask(ctx, db, in)
			})
	})

	t.Run("TransferTask", func(t *testing.T) {
		in := approval.TransferTaskInput{
			TaskID:       "k-1",
			Operator:     testUser,
			Opinion:      "yours",
			FormData:     testForm,
			TransferToID: "u-2",
			Attachments:  testFiles,
			Caller:       testCaller,
		}

		assertForwards[command.TransferTaskCmd, cqrs.Unit](t, bus, db,
			command.TransferTaskCmd{TransferTaskInput: in},
			func(ctx context.Context, db orm.DB) error {
				return svc.TransferTask(ctx, db, in)
			})
	})

	t.Run("RollbackTask", func(t *testing.T) {
		in := approval.RollbackTaskInput{
			TaskID:       "k-1",
			Operator:     testUser,
			Opinion:      "redo",
			FormData:     testForm,
			TargetNodeID: "n-1",
			Attachments:  testFiles,
			Caller:       testCaller,
		}

		assertForwards[command.RollbackTaskCmd, cqrs.Unit](t, bus, db,
			command.RollbackTaskCmd{RollbackTaskInput: in},
			func(ctx context.Context, db orm.DB) error {
				return svc.RollbackTask(ctx, db, in)
			})
	})

	t.Run("ReassignTask", func(t *testing.T) {
		in := approval.ReassignTaskInput{TaskID: "k-1", NewAssigneeID: "u-2", Operator: testUser, Reason: "left the company", Caller: testCaller}

		assertForwards[command.ReassignTaskCmd, cqrs.Unit](t, bus, db,
			command.ReassignTaskCmd{ReassignTaskInput: in},
			func(ctx context.Context, db orm.DB) error {
				return svc.ReassignTask(ctx, db, in)
			})
	})

	t.Run("AddAssignee", func(t *testing.T) {
		in := approval.AddAssigneeInput{TaskID: "k-1", UserIDs: []string{"u-2"}, AddType: approval.AddAssigneeAfter, Operator: testUser, Caller: testCaller}

		assertForwards[command.AddAssigneeCmd, cqrs.Unit](t, bus, db,
			command.AddAssigneeCmd{AddAssigneeInput: in},
			func(ctx context.Context, db orm.DB) error {
				return svc.AddAssignee(ctx, db, in)
			})
	})

	t.Run("RemoveAssignee", func(t *testing.T) {
		in := approval.RemoveAssigneeInput{TaskID: "k-1", Operator: testUser, Caller: testCaller}

		assertForwards[command.RemoveAssigneeCmd, cqrs.Unit](t, bus, db,
			command.RemoveAssigneeCmd{RemoveAssigneeInput: in},
			func(ctx context.Context, db orm.DB) error {
				return svc.RemoveAssignee(ctx, db, in)
			})
	})

	t.Run("AddCC", func(t *testing.T) {
		in := approval.AddCCInput{InstanceID: "i-1", CCUserIDs: []string{"u-3"}, Operator: testUser, Caller: testCaller}

		assertForwards[command.AddCCCmd, cqrs.Unit](t, bus, db,
			command.AddCCCmd{AddCCInput: in},
			func(ctx context.Context, db orm.DB) error {
				return svc.AddCC(ctx, db, in)
			})
	})

	t.Run("MarkCCRead", func(t *testing.T) {
		in := approval.MarkCCReadInput{InstanceID: "i-1", UserID: "u-3", Caller: testCaller}

		assertForwards[command.MarkCCReadCmd, cqrs.Unit](t, bus, db,
			command.MarkCCReadCmd{MarkCCReadInput: in},
			func(ctx context.Context, db orm.DB) error {
				return svc.MarkCCRead(ctx, db, in)
			})
	})

	t.Run("UrgeTask", func(t *testing.T) {
		in := approval.UrgeTaskInput{TaskID: "k-1", UrgerID: "u-1", Message: "please", Caller: testCaller}

		assertForwards[command.UrgeTaskCmd, cqrs.Unit](t, bus, db,
			command.UrgeTaskCmd{UrgeTaskInput: in},
			func(ctx context.Context, db orm.DB) error {
				return svc.UrgeTask(ctx, db, in)
			})
	})
}

// TestServiceTransactionBoundary pins the contract the explicit db parameter
// exists for: the handle the caller passes is the one the pipeline runs on.
func TestServiceTransactionBoundary(t *testing.T) {
	db := testx.NewTestDB(t)
	in := approval.WithdrawInstanceInput{InstanceID: "i-1", Operator: testUser, Caller: testCaller}

	// Each subtest builds its own bus and captures afresh: a shared recorder
	// would let a subtest pass on the handle its predecessor left behind,
	// which is exactly the case where the handler was never reached at all.
	t.Run("JoinsCallerTransaction", func(t *testing.T) {
		bus := newBus(db)
		svc := facade.NewService(bus)
		got := capture[command.WithdrawInstanceCmd, cqrs.Unit](bus)

		var callerTx orm.DB

		err := db.RunInTx(context.Background(), func(ctx context.Context, tx orm.DB) error {
			callerTx = tx

			return svc.WithdrawInstance(ctx, tx, in)
		})
		require.NoError(t, err, "operation inside the caller's RunInTx should succeed")
		assert.Same(t, callerTx, got.db, "handler must run on the caller's own transaction handle, not a new one")
	})

	t.Run("OpensOwnTransactionOnPlainHandle", func(t *testing.T) {
		bus := newBus(db)
		svc := facade.NewService(bus)
		got := capture[command.WithdrawInstanceCmd, cqrs.Unit](bus)

		require.NoError(t, svc.WithdrawInstance(context.Background(), db, in), "operation on a plain handle should succeed")
		require.NotNil(t, got.db, "handler must see a DB handle bound to its context")
		assert.NotSame(t, db, got.db, "pipeline must open a transaction rather than run on the pool handle")
		assert.True(t, got.db.InTx(), "handler must run inside the transaction the pipeline opened")
	})

	t.Run("CallerRollbackDiscardsTheOperation", func(t *testing.T) {
		bus := newBus(db)
		svc := facade.NewService(bus)
		errAbort := errors.New("abort")
		joined := false

		cqrs.Register(bus, cqrs.HandlerFunc[command.UrgeTaskCmd, cqrs.Unit](func(ctx context.Context, _ command.UrgeTaskCmd) (cqrs.Unit, error) {
			_, err := contextx.DB(ctx).NewRaw("CREATE TABLE facade_probe (id INTEGER)").Exec(ctx)
			joined = err == nil

			return cqrs.Unit{}, err
		}))

		err := db.RunInTx(context.Background(), func(ctx context.Context, tx orm.DB) error {
			if err := svc.UrgeTask(ctx, tx, approval.UrgeTaskInput{TaskID: "k-1", UrgerID: "u-1", Caller: testCaller}); err != nil {
				return err
			}

			return errAbort
		})
		require.ErrorIs(t, err, errAbort, "the caller's error should surface unchanged")
		require.True(t, joined, "handler DDL should have run on the joined transaction")

		var count int

		err = db.NewRaw("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'facade_probe'").Scan(context.Background(), &count)
		require.NoError(t, err, "probing sqlite_master should succeed")
		assert.Zero(t, count, "work done through the facade must roll back with the caller's transaction")
	})
}

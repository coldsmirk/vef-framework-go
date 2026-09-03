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

// captured is what a stub command handler records: the command it received
// and the DB handle bound to its context, which is how the facade hands the
// caller's transaction boundary to the pipeline.
type captured[TCmd any] struct {
	cmd TCmd
	db  orm.DB
}

// capture registers a stub handler for TCmd on bus that records its input and
// returns the zero TResult.
func capture[TCmd cqrs.Action, TResult any](bus cqrs.Bus) *captured[TCmd] {
	c := new(captured[TCmd])

	cqrs.Register(bus, cqrs.HandlerFunc[TCmd, TResult](func(ctx context.Context, cmd TCmd) (TResult, error) {
		c.cmd = cmd
		c.db = contextx.DB(ctx)

		var zero TResult

		return zero, nil
	}))

	return c
}

// requireNoZeroField fails when any field of cmd other than the embedded
// BaseCommand is left at its zero value. The forwarding assertions compare a
// captured command with the expected one, so a field the test forgot to
// populate would pass trivially — this makes such an omission fail instead.
func requireNoZeroField(t *testing.T, cmd any) {
	t.Helper()

	v := reflect.ValueOf(cmd)
	for i := range v.NumField() {
		field := v.Type().Field(i)
		if field.Anonymous {
			continue
		}

		require.False(t, v.Field(i).IsZero(), "test input must populate %s.%s so forwarding is actually checked", v.Type().Name(), field.Name)
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
		got := capture[command.StartInstanceCmd, *approval.Instance](bus)
		want := command.StartInstanceCmd{
			TenantID:    "t-1",
			FlowCode:    "leave",
			Applicant:   testUser,
			BusinessRef: new("order-1"),
			FormData:    testForm,
			Globals:     map[string]any{"region": "cn"},
			Caller:      testCaller,
		}
		requireNoZeroField(t, want)

		_, err := svc.StartInstance(context.Background(), db, approval.StartInstanceInput{
			TenantID:    "t-1",
			FlowCode:    "leave",
			Applicant:   testUser,
			BusinessRef: new("order-1"),
			FormData:    testForm,
			Globals:     map[string]any{"region": "cn"},
			Caller:      testCaller,
		})
		require.NoError(t, err, "StartInstance should dispatch without error")
		assert.Equal(t, want, got.cmd, "StartInstance must forward every input field")
		assert.True(t, got.db.InTx(), "handler must run inside a transaction")
	})

	t.Run("WithdrawInstance", func(t *testing.T) {
		assertForwards[command.WithdrawInstanceCmd, cqrs.Unit](t, bus, db,
			command.WithdrawInstanceCmd{InstanceID: "i-1", Operator: testUser, Reason: "changed my mind", Caller: testCaller},
			func(ctx context.Context, db orm.DB) error {
				return svc.WithdrawInstance(ctx, db, approval.WithdrawInstanceInput{InstanceID: "i-1", Operator: testUser, Reason: "changed my mind", Caller: testCaller})
			})
	})

	t.Run("ResubmitInstance", func(t *testing.T) {
		assertForwards[command.ResubmitInstanceCmd, cqrs.Unit](t, bus, db,
			command.ResubmitInstanceCmd{InstanceID: "i-1", Operator: testUser, FormData: testForm, Caller: testCaller},
			func(ctx context.Context, db orm.DB) error {
				return svc.ResubmitInstance(ctx, db, approval.ResubmitInstanceInput{InstanceID: "i-1", Operator: testUser, FormData: testForm, Caller: testCaller})
			})
	})

	t.Run("TerminateInstance", func(t *testing.T) {
		assertForwards[command.TerminateInstanceCmd, cqrs.Unit](t, bus, db,
			command.TerminateInstanceCmd{InstanceID: "i-1", Operator: testUser, Reason: "order canceled", Caller: testCaller},
			func(ctx context.Context, db orm.DB) error {
				return svc.TerminateInstance(ctx, db, approval.TerminateInstanceInput{InstanceID: "i-1", Operator: testUser, Reason: "order canceled", Caller: testCaller})
			})
	})

	t.Run("ApproveTask", func(t *testing.T) {
		assertForwards[command.ApproveTaskCmd, cqrs.Unit](t, bus, db,
			command.ApproveTaskCmd{TaskID: "k-1", Operator: testUser, Opinion: "ok", FormData: testForm, Attachments: testFiles, Caller: testCaller},
			func(ctx context.Context, db orm.DB) error {
				return svc.ApproveTask(
					ctx,
					db,
					approval.ApproveTaskInput{TaskID: "k-1", Operator: testUser, Opinion: "ok", FormData: testForm, Attachments: testFiles, Caller: testCaller},
				)
			})
	})

	t.Run("RejectTask", func(t *testing.T) {
		assertForwards[command.RejectTaskCmd, cqrs.Unit](t, bus, db,
			command.RejectTaskCmd{TaskID: "k-1", Operator: testUser, Opinion: "no", FormData: testForm, Attachments: testFiles, Caller: testCaller},
			func(ctx context.Context, db orm.DB) error {
				return svc.RejectTask(
					ctx,
					db,
					approval.RejectTaskInput{TaskID: "k-1", Operator: testUser, Opinion: "no", FormData: testForm, Attachments: testFiles, Caller: testCaller},
				)
			})
	})

	t.Run("TransferTask", func(t *testing.T) {
		assertForwards[command.TransferTaskCmd, cqrs.Unit](t, bus, db,
			command.TransferTaskCmd{TaskID: "k-1", Operator: testUser, Opinion: "yours", FormData: testForm, TransferToID: "u-2", Attachments: testFiles, Caller: testCaller},
			func(ctx context.Context, db orm.DB) error {
				return svc.TransferTask(
					ctx,
					db,
					approval.TransferTaskInput{
						TaskID:       "k-1",
						Operator:     testUser,
						Opinion:      "yours",
						FormData:     testForm,
						TransferToID: "u-2",
						Attachments:  testFiles,
						Caller:       testCaller,
					},
				)
			})
	})

	t.Run("RollbackTask", func(t *testing.T) {
		assertForwards[command.RollbackTaskCmd, cqrs.Unit](t, bus, db,
			command.RollbackTaskCmd{TaskID: "k-1", Operator: testUser, Opinion: "redo", FormData: testForm, TargetNodeID: "n-1", Attachments: testFiles, Caller: testCaller},
			func(ctx context.Context, db orm.DB) error {
				return svc.RollbackTask(
					ctx,
					db,
					approval.RollbackTaskInput{
						TaskID:       "k-1",
						Operator:     testUser,
						Opinion:      "redo",
						FormData:     testForm,
						TargetNodeID: "n-1",
						Attachments:  testFiles,
						Caller:       testCaller,
					},
				)
			})
	})

	t.Run("ReassignTask", func(t *testing.T) {
		assertForwards[command.ReassignTaskCmd, cqrs.Unit](t, bus, db,
			command.ReassignTaskCmd{TaskID: "k-1", NewAssigneeID: "u-2", Operator: testUser, Reason: "left the company", Caller: testCaller},
			func(ctx context.Context, db orm.DB) error {
				return svc.ReassignTask(ctx, db, approval.ReassignTaskInput{TaskID: "k-1", NewAssigneeID: "u-2", Operator: testUser, Reason: "left the company", Caller: testCaller})
			})
	})

	t.Run("AddAssignee", func(t *testing.T) {
		assertForwards[command.AddAssigneeCmd, cqrs.Unit](t, bus, db,
			command.AddAssigneeCmd{TaskID: "k-1", UserIDs: []string{"u-2"}, AddType: approval.AddAssigneeAfter, Operator: testUser, Caller: testCaller},
			func(ctx context.Context, db orm.DB) error {
				return svc.AddAssignee(
					ctx,
					db,
					approval.AddAssigneeInput{TaskID: "k-1", UserIDs: []string{"u-2"}, AddType: approval.AddAssigneeAfter, Operator: testUser, Caller: testCaller},
				)
			})
	})

	t.Run("RemoveAssignee", func(t *testing.T) {
		assertForwards[command.RemoveAssigneeCmd, cqrs.Unit](t, bus, db,
			command.RemoveAssigneeCmd{TaskID: "k-1", Operator: testUser, Caller: testCaller},
			func(ctx context.Context, db orm.DB) error {
				return svc.RemoveAssignee(ctx, db, approval.RemoveAssigneeInput{TaskID: "k-1", Operator: testUser, Caller: testCaller})
			})
	})

	t.Run("AddCC", func(t *testing.T) {
		assertForwards[command.AddCCCmd, cqrs.Unit](t, bus, db,
			command.AddCCCmd{InstanceID: "i-1", CCUserIDs: []string{"u-3"}, Operator: testUser, Caller: testCaller},
			func(ctx context.Context, db orm.DB) error {
				return svc.AddCC(ctx, db, approval.AddCCInput{InstanceID: "i-1", CCUserIDs: []string{"u-3"}, Operator: testUser, Caller: testCaller})
			})
	})

	t.Run("MarkCCRead", func(t *testing.T) {
		assertForwards[command.MarkCCReadCmd, cqrs.Unit](t, bus, db,
			command.MarkCCReadCmd{InstanceID: "i-1", UserID: "u-3", Caller: testCaller},
			func(ctx context.Context, db orm.DB) error {
				return svc.MarkCCRead(ctx, db, approval.MarkCCReadInput{InstanceID: "i-1", UserID: "u-3", Caller: testCaller})
			})
	})

	t.Run("UrgeTask", func(t *testing.T) {
		assertForwards[command.UrgeTaskCmd, cqrs.Unit](t, bus, db,
			command.UrgeTaskCmd{TaskID: "k-1", UrgerID: "u-1", Message: "please", Caller: testCaller},
			func(ctx context.Context, db orm.DB) error {
				return svc.UrgeTask(ctx, db, approval.UrgeTaskInput{TaskID: "k-1", UrgerID: "u-1", Message: "please", Caller: testCaller})
			})
	})
}

// TestServiceTransactionBoundary pins the contract the explicit db parameter
// exists for: the handle the caller passes is the one the pipeline runs on.
func TestServiceTransactionBoundary(t *testing.T) {
	db := testx.NewTestDB(t)
	bus := newBus(db)
	svc := facade.NewService(bus)
	got := capture[command.WithdrawInstanceCmd, cqrs.Unit](bus)
	in := approval.WithdrawInstanceInput{InstanceID: "i-1", Operator: testUser, Caller: testCaller}

	t.Run("JoinsCallerTransaction", func(t *testing.T) {
		var callerTx orm.DB

		err := db.RunInTx(context.Background(), func(ctx context.Context, tx orm.DB) error {
			callerTx = tx

			return svc.WithdrawInstance(ctx, tx, in)
		})
		require.NoError(t, err, "operation inside the caller's RunInTx should succeed")
		assert.Same(t, callerTx, got.db, "handler must run on the caller's own transaction handle, not a new one")
	})

	t.Run("OpensOwnTransactionOnPlainHandle", func(t *testing.T) {
		require.NoError(t, svc.WithdrawInstance(context.Background(), db, in), "operation on a plain handle should succeed")
		assert.NotSame(t, db, got.db, "pipeline must open a transaction rather than run on the pool handle")
		assert.True(t, got.db.InTx(), "handler must run inside the transaction the pipeline opened")
	})

	t.Run("CallerRollbackDiscardsTheOperation", func(t *testing.T) {
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

// TestInputCommandParity guards the translation layer against drift: every
// exported field of an internal command must exist on its public input with
// the same type, and vice versa, so a field added to one side cannot be
// silently dropped by the facade.
func TestInputCommandParity(t *testing.T) {
	pairs := []struct {
		input reflect.Type
		cmd   reflect.Type
	}{
		{reflect.TypeFor[approval.StartInstanceInput](), reflect.TypeFor[command.StartInstanceCmd]()},
		{reflect.TypeFor[approval.WithdrawInstanceInput](), reflect.TypeFor[command.WithdrawInstanceCmd]()},
		{reflect.TypeFor[approval.ResubmitInstanceInput](), reflect.TypeFor[command.ResubmitInstanceCmd]()},
		{reflect.TypeFor[approval.TerminateInstanceInput](), reflect.TypeFor[command.TerminateInstanceCmd]()},
		{reflect.TypeFor[approval.ApproveTaskInput](), reflect.TypeFor[command.ApproveTaskCmd]()},
		{reflect.TypeFor[approval.RejectTaskInput](), reflect.TypeFor[command.RejectTaskCmd]()},
		{reflect.TypeFor[approval.TransferTaskInput](), reflect.TypeFor[command.TransferTaskCmd]()},
		{reflect.TypeFor[approval.RollbackTaskInput](), reflect.TypeFor[command.RollbackTaskCmd]()},
		{reflect.TypeFor[approval.ReassignTaskInput](), reflect.TypeFor[command.ReassignTaskCmd]()},
		{reflect.TypeFor[approval.AddAssigneeInput](), reflect.TypeFor[command.AddAssigneeCmd]()},
		{reflect.TypeFor[approval.RemoveAssigneeInput](), reflect.TypeFor[command.RemoveAssigneeCmd]()},
		{reflect.TypeFor[approval.AddCCInput](), reflect.TypeFor[command.AddCCCmd]()},
		{reflect.TypeFor[approval.MarkCCReadInput](), reflect.TypeFor[command.MarkCCReadCmd]()},
		{reflect.TypeFor[approval.UrgeTaskInput](), reflect.TypeFor[command.UrgeTaskCmd]()},
	}

	fields := func(typ reflect.Type) map[string]reflect.Type {
		out := make(map[string]reflect.Type, typ.NumField())

		for f := range typ.Fields() {
			if f.Anonymous || !f.IsExported() {
				continue
			}

			out[f.Name] = f.Type
		}

		return out
	}

	for _, pair := range pairs {
		t.Run(pair.input.Name(), func(t *testing.T) {
			assert.Equal(t, fields(pair.cmd), fields(pair.input), "%s must mirror %s field for field", pair.input.Name(), pair.cmd.Name())
		})
	}
}

package facade

import (
	"context"

	"github.com/gofiber/fiber/v3"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/approval/command"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// Service implements approval.Service over the CQRS bus. Each method wraps the
// public input in its internal command and dispatches once, so the command
// pipeline — transaction, action-log and event collectors, the handler itself
// — stays the single place where an operation is defined; the API resources
// call this type too, which is what keeps the request path and the
// programmatic path one implementation.
//
// The commands embed the public inputs (command.ApproveTaskCmd embeds
// approval.ApproveTaskInput), so the wrapping is one compiler-checked
// assignment rather than a field-by-field copy a new field could be forgotten
// from.
type Service struct {
	bus cqrs.Bus
}

// NewService creates the approval.Service the module exposes in DI.
func NewService(bus cqrs.Bus) approval.Service {
	return &Service{bus: bus}
}

// bindDB validates the caller's handle and binds it to the dispatch context.
// Binding is what hands the caller's transaction boundary to the pipeline:
// TransactionBehavior joins the handle when it is an open transaction and
// opens one on it otherwise, and every handler reads contextx.DB(ctx) rather
// than its injected primary handle.
//
// Two caller mistakes are neutralized here because neither is a compile
// error and both fail silently rather than loudly:
//
//   - A nil handle would leave contextx.DB(ctx) nil and let
//     TransactionBehavior fall back to its own injected primary handle, so the
//     operation would commit outside the caller's transaction and attribute
//     itself to orm.OperatorSystem. It is rejected instead.
//   - A fiber.Ctx satisfies context.Context, so a host API handler can pass
//     its request context straight in. contextx.SetDB mutates a fiber.Ctx's
//     Locals in place instead of deriving a child context, which would leave
//     the whole request bound to a transaction handle that dies at the
//     caller's commit; unwrapping first makes every call site safe by
//     construction.
func bindDB(ctx context.Context, db orm.DB) (context.Context, error) {
	if db == nil {
		return nil, approval.ErrDBRequired
	}

	if c, ok := ctx.(fiber.Ctx); ok {
		ctx = c.Context()
	}

	return contextx.SetDB(ctx, db), nil
}

// send binds db and dispatches cmd, returning the handler's result.
func (s *Service) send[TCmd cqrs.Action, TResult any](ctx context.Context, db orm.DB, cmd TCmd) (TResult, error) {
	ctx, err := bindDB(ctx, db)
	if err != nil {
		var zero TResult

		return zero, err
	}

	return cqrs.Send[TCmd, TResult](ctx, s.bus, cmd)
}

// sendUnit is send for the operations whose handler returns no value. TResult
// is fixed rather than spelled at each call site, so a copy-pasted result type
// cannot reach cqrs.Send — a mismatch there is only detected after the
// pipeline has already committed the operation.
func (s *Service) sendUnit[TCmd cqrs.Action](ctx context.Context, db orm.DB, cmd TCmd) error {
	_, err := s.send[TCmd, cqrs.Unit](ctx, db, cmd)

	return err
}

func (s *Service) StartInstance(ctx context.Context, db orm.DB, in approval.StartInstanceInput) (*approval.Instance, error) {
	return s.send[command.StartInstanceCmd, *approval.Instance](ctx, db, command.StartInstanceCmd{StartInstanceInput: in})
}

func (s *Service) WithdrawInstance(ctx context.Context, db orm.DB, in approval.WithdrawInstanceInput) error {
	return s.sendUnit(ctx, db, command.WithdrawInstanceCmd{WithdrawInstanceInput: in})
}

func (s *Service) ResubmitInstance(ctx context.Context, db orm.DB, in approval.ResubmitInstanceInput) error {
	return s.sendUnit(ctx, db, command.ResubmitInstanceCmd{ResubmitInstanceInput: in})
}

func (s *Service) TerminateInstance(ctx context.Context, db orm.DB, in approval.TerminateInstanceInput) error {
	return s.sendUnit(ctx, db, command.TerminateInstanceCmd{TerminateInstanceInput: in})
}

func (s *Service) ApproveTask(ctx context.Context, db orm.DB, in approval.ApproveTaskInput) error {
	return s.sendUnit(ctx, db, command.ApproveTaskCmd{ApproveTaskInput: in})
}

func (s *Service) RejectTask(ctx context.Context, db orm.DB, in approval.RejectTaskInput) error {
	return s.sendUnit(ctx, db, command.RejectTaskCmd{RejectTaskInput: in})
}

func (s *Service) TransferTask(ctx context.Context, db orm.DB, in approval.TransferTaskInput) error {
	return s.sendUnit(ctx, db, command.TransferTaskCmd{TransferTaskInput: in})
}

func (s *Service) RollbackTask(ctx context.Context, db orm.DB, in approval.RollbackTaskInput) error {
	return s.sendUnit(ctx, db, command.RollbackTaskCmd{RollbackTaskInput: in})
}

func (s *Service) ReassignTask(ctx context.Context, db orm.DB, in approval.ReassignTaskInput) error {
	return s.sendUnit(ctx, db, command.ReassignTaskCmd{ReassignTaskInput: in})
}

func (s *Service) AddAssignee(ctx context.Context, db orm.DB, in approval.AddAssigneeInput) error {
	return s.sendUnit(ctx, db, command.AddAssigneeCmd{AddAssigneeInput: in})
}

func (s *Service) RemoveAssignee(ctx context.Context, db orm.DB, in approval.RemoveAssigneeInput) error {
	return s.sendUnit(ctx, db, command.RemoveAssigneeCmd{RemoveAssigneeInput: in})
}

func (s *Service) AddCC(ctx context.Context, db orm.DB, in approval.AddCCInput) error {
	return s.sendUnit(ctx, db, command.AddCCCmd{AddCCInput: in})
}

func (s *Service) MarkCCRead(ctx context.Context, db orm.DB, in approval.MarkCCReadInput) error {
	return s.sendUnit(ctx, db, command.MarkCCReadCmd{MarkCCReadInput: in})
}

func (s *Service) UrgeTask(ctx context.Context, db orm.DB, in approval.UrgeTaskInput) error {
	return s.sendUnit(ctx, db, command.UrgeTaskCmd{UrgeTaskInput: in})
}

func (s *Service) RetryBusinessProjection(ctx context.Context, db orm.DB, in approval.RetryBusinessProjectionInput) error {
	return s.sendUnit(ctx, db, command.RetryBusinessProjectionCmd{RetryBusinessProjectionInput: in})
}

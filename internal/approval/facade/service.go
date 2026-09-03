package facade

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/approval/command"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// Service implements approval.Service over the CQRS bus. Each method is a pure
// translation of the public input into the internal command followed by one
// dispatch, so the command pipeline — transaction, action-log and event
// collectors, the handler itself — stays the single place where an operation
// is defined; the API resources call this type too, which is what keeps the
// request path and the programmatic path one implementation.
type Service struct {
	bus cqrs.Bus
}

// NewService creates the approval.Service the module exposes in DI.
func NewService(bus cqrs.Bus) approval.Service {
	return &Service{bus: bus}
}

// send binds db to the context and dispatches the command. Binding is what
// hands the caller's transaction boundary to the pipeline: TransactionBehavior
// joins the handle when it is an open transaction and opens one on it
// otherwise, and every handler reads contextx.DB(ctx) rather than its injected
// primary handle.
func send[TCmd cqrs.Action, TResult any](ctx context.Context, bus cqrs.Bus, db orm.DB, cmd TCmd) (TResult, error) {
	return cqrs.Send[TCmd, TResult](contextx.SetDB(ctx, db), bus, cmd)
}

func (s *Service) StartInstance(ctx context.Context, db orm.DB, in approval.StartInstanceInput) (*approval.Instance, error) {
	return send[command.StartInstanceCmd, *approval.Instance](ctx, s.bus, db, command.StartInstanceCmd{
		TenantID:    in.TenantID,
		FlowCode:    in.FlowCode,
		Applicant:   in.Applicant,
		BusinessRef: in.BusinessRef,
		FormData:    in.FormData,
		Globals:     in.Globals,
		Caller:      in.Caller,
	})
}

func (s *Service) WithdrawInstance(ctx context.Context, db orm.DB, in approval.WithdrawInstanceInput) error {
	_, err := send[command.WithdrawInstanceCmd, cqrs.Unit](ctx, s.bus, db, command.WithdrawInstanceCmd{
		InstanceID: in.InstanceID,
		Operator:   in.Operator,
		Reason:     in.Reason,
		Caller:     in.Caller,
	})

	return err
}

func (s *Service) ResubmitInstance(ctx context.Context, db orm.DB, in approval.ResubmitInstanceInput) error {
	_, err := send[command.ResubmitInstanceCmd, cqrs.Unit](ctx, s.bus, db, command.ResubmitInstanceCmd{
		InstanceID: in.InstanceID,
		Operator:   in.Operator,
		FormData:   in.FormData,
		Caller:     in.Caller,
	})

	return err
}

func (s *Service) TerminateInstance(ctx context.Context, db orm.DB, in approval.TerminateInstanceInput) error {
	_, err := send[command.TerminateInstanceCmd, cqrs.Unit](ctx, s.bus, db, command.TerminateInstanceCmd{
		InstanceID: in.InstanceID,
		Operator:   in.Operator,
		Reason:     in.Reason,
		Caller:     in.Caller,
	})

	return err
}

func (s *Service) ApproveTask(ctx context.Context, db orm.DB, in approval.ApproveTaskInput) error {
	_, err := send[command.ApproveTaskCmd, cqrs.Unit](ctx, s.bus, db, command.ApproveTaskCmd{
		TaskID:      in.TaskID,
		Operator:    in.Operator,
		Opinion:     in.Opinion,
		FormData:    in.FormData,
		Attachments: in.Attachments,
		Caller:      in.Caller,
	})

	return err
}

func (s *Service) RejectTask(ctx context.Context, db orm.DB, in approval.RejectTaskInput) error {
	_, err := send[command.RejectTaskCmd, cqrs.Unit](ctx, s.bus, db, command.RejectTaskCmd{
		TaskID:      in.TaskID,
		Operator:    in.Operator,
		Opinion:     in.Opinion,
		FormData:    in.FormData,
		Attachments: in.Attachments,
		Caller:      in.Caller,
	})

	return err
}

func (s *Service) TransferTask(ctx context.Context, db orm.DB, in approval.TransferTaskInput) error {
	_, err := send[command.TransferTaskCmd, cqrs.Unit](ctx, s.bus, db, command.TransferTaskCmd{
		TaskID:       in.TaskID,
		Operator:     in.Operator,
		Opinion:      in.Opinion,
		FormData:     in.FormData,
		TransferToID: in.TransferToID,
		Attachments:  in.Attachments,
		Caller:       in.Caller,
	})

	return err
}

func (s *Service) RollbackTask(ctx context.Context, db orm.DB, in approval.RollbackTaskInput) error {
	_, err := send[command.RollbackTaskCmd, cqrs.Unit](ctx, s.bus, db, command.RollbackTaskCmd{
		TaskID:       in.TaskID,
		Operator:     in.Operator,
		Opinion:      in.Opinion,
		FormData:     in.FormData,
		TargetNodeID: in.TargetNodeID,
		Attachments:  in.Attachments,
		Caller:       in.Caller,
	})

	return err
}

func (s *Service) ReassignTask(ctx context.Context, db orm.DB, in approval.ReassignTaskInput) error {
	_, err := send[command.ReassignTaskCmd, cqrs.Unit](ctx, s.bus, db, command.ReassignTaskCmd{
		TaskID:        in.TaskID,
		NewAssigneeID: in.NewAssigneeID,
		Operator:      in.Operator,
		Reason:        in.Reason,
		Caller:        in.Caller,
	})

	return err
}

func (s *Service) AddAssignee(ctx context.Context, db orm.DB, in approval.AddAssigneeInput) error {
	_, err := send[command.AddAssigneeCmd, cqrs.Unit](ctx, s.bus, db, command.AddAssigneeCmd{
		TaskID:   in.TaskID,
		UserIDs:  in.UserIDs,
		AddType:  in.AddType,
		Operator: in.Operator,
		Caller:   in.Caller,
	})

	return err
}

func (s *Service) RemoveAssignee(ctx context.Context, db orm.DB, in approval.RemoveAssigneeInput) error {
	_, err := send[command.RemoveAssigneeCmd, cqrs.Unit](ctx, s.bus, db, command.RemoveAssigneeCmd{
		TaskID:   in.TaskID,
		Operator: in.Operator,
		Caller:   in.Caller,
	})

	return err
}

func (s *Service) AddCC(ctx context.Context, db orm.DB, in approval.AddCCInput) error {
	_, err := send[command.AddCCCmd, cqrs.Unit](ctx, s.bus, db, command.AddCCCmd{
		InstanceID: in.InstanceID,
		CCUserIDs:  in.CCUserIDs,
		Operator:   in.Operator,
		Caller:     in.Caller,
	})

	return err
}

func (s *Service) MarkCCRead(ctx context.Context, db orm.DB, in approval.MarkCCReadInput) error {
	_, err := send[command.MarkCCReadCmd, cqrs.Unit](ctx, s.bus, db, command.MarkCCReadCmd{
		InstanceID: in.InstanceID,
		UserID:     in.UserID,
		Caller:     in.Caller,
	})

	return err
}

func (s *Service) UrgeTask(ctx context.Context, db orm.DB, in approval.UrgeTaskInput) error {
	_, err := send[command.UrgeTaskCmd, cqrs.Unit](ctx, s.bus, db, command.UrgeTaskCmd{
		TaskID:  in.TaskID,
		UrgerID: in.UrgerID,
		Message: in.Message,
		Caller:  in.Caller,
	})

	return err
}

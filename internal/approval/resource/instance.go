package resource

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/command"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/security"
)

var errUnsupportedAction = errors.New("unsupported action")

// resolveOperator builds an OperatorInfo from the authenticated principal.
func resolveOperator(ctx context.Context, resolver approval.PrincipalDepartmentResolver, principal *security.Principal) (approval.OperatorInfo, error) {
	departmentID, departmentName, err := resolver.Resolve(ctx, principal)
	if err != nil {
		return approval.OperatorInfo{}, fmt.Errorf("resolve operator department: %w", err)
	}

	return approval.OperatorInfo{
		ID:             principal.ID,
		Name:           principal.Name,
		DepartmentID:   departmentID,
		DepartmentName: departmentName,
	}, nil
}

// resolveCaller bundles tenant authority from the principal. Used by
// resource handlers to gate cross-tenant access on entity-scoped commands
// and queries (terminate / reassign / detail / flow mutations).
func resolveCaller(ctx context.Context, resolver approval.PrincipalTenantResolver, principal *security.Principal) (approval.CallerContext, error) {
	tenantID, err := resolver.Resolve(ctx, principal)
	if err != nil {
		return approval.CallerContext{}, fmt.Errorf("resolve caller tenant: %w", err)
	}

	return approval.CallerContext{
		TenantID:     tenantID,
		IsSuperAdmin: approval.IsSuperAdmin(principal),
	}, nil
}

// resolveActor walks the principal once and returns both the operator
// identity (for audit logging) and the caller's tenant authority. Resource
// handlers that hit a command always need both, so threading them together
// avoids two near-identical pairs of resolve / nil-check / wrap-error per
// endpoint. Returning the resolved values by value keeps the call site a
// single assignment — `actor, err := r.resolveActor(...)` — which is the
// shape most handlers want.
type resolvedActor struct {
	Operator approval.OperatorInfo
	Caller   approval.CallerContext
}

func resolveActor(
	ctx context.Context,
	deptResolver approval.PrincipalDepartmentResolver,
	tenantResolver approval.PrincipalTenantResolver,
	principal *security.Principal,
) (resolvedActor, error) {
	operator, err := resolveOperator(ctx, deptResolver, principal)
	if err != nil {
		return resolvedActor{}, err
	}

	caller, err := resolveCaller(ctx, tenantResolver, principal)
	if err != nil {
		return resolvedActor{}, err
	}

	return resolvedActor{Operator: operator, Caller: caller}, nil
}

// InstanceResource handles instance lifecycle and queries.
type InstanceResource struct {
	api.Resource

	bus                cqrs.Bus
	departmentResolver approval.PrincipalDepartmentResolver
	tenantResolver     approval.PrincipalTenantResolver
}

// NewInstanceResource creates a new instance resource.
func NewInstanceResource(
	bus cqrs.Bus,
	departmentResolver approval.PrincipalDepartmentResolver,
	tenantResolver approval.PrincipalTenantResolver,
) api.Resource {
	return &InstanceResource{
		bus:                bus,
		departmentResolver: departmentResolver,
		tenantResolver:     tenantResolver,
		Resource: api.NewRPCResource(
			"approval/instance",
			api.WithOperations(
				// Submission and the per-task decision are the two highest-value
				// state changes — keep audit on so framework-level IP/UA/RequestID
				// land beside the business action_log.
				api.OperationSpec{Action: "start", RequiredPermission: "approval:instance:start", EnableAudit: true},
				api.OperationSpec{Action: "process_task", RequiredPermission: "approval:task:process", EnableAudit: true},
				api.OperationSpec{Action: "withdraw", RequiredPermission: "approval:instance:withdraw", EnableAudit: true},
				api.OperationSpec{Action: "resubmit", RequiredPermission: "approval:instance:resubmit", EnableAudit: true},
				api.OperationSpec{Action: "add_cc", RequiredPermission: "approval:instance:cc", EnableAudit: true},
				// mark_cc_read is a self-service read receipt — low impact, no audit.
				api.OperationSpec{Action: "mark_cc_read", RequiredPermission: "approval:instance:cc"},
				api.OperationSpec{Action: "add_assignee", RequiredPermission: "approval:task:add_assignee", EnableAudit: true},
				api.OperationSpec{Action: "remove_assignee", RequiredPermission: "approval:task:remove_assignee", EnableAudit: true},
				// urge_task additionally rate-limited to deter notification
				// flooding even when the per-task cooldown is satisfied.
				api.OperationSpec{
					Action:             "urge_task",
					RequiredPermission: "approval:task:urge",
					RateLimit:          &api.RateLimitConfig{Max: 10, Period: time.Minute},
				},
			),
		),
	}
}

// StartInstanceParams contains the parameters for starting a new instance.
type StartInstanceParams struct {
	api.P

	TenantID         string         `json:"tenantId" validate:"required"`
	FlowCode         string         `json:"flowCode" validate:"required"`
	BusinessRecordID *string        `json:"businessRecordId"`
	FormData         map[string]any `json:"formData"`
}

// Start creates a new flow instance.
func (r *InstanceResource) Start(ctx fiber.Ctx, principal *security.Principal, params StartInstanceParams) error {
	actor, err := resolveActor(ctx.Context(), r.departmentResolver, r.tenantResolver, principal)
	if err != nil {
		return err
	}

	instance, err := cqrs.Send[command.StartInstanceCmd, *approval.Instance](ctx.Context(), r.bus, command.StartInstanceCmd{
		TenantID:         params.TenantID,
		FlowCode:         params.FlowCode,
		Applicant:        actor.Operator,
		BusinessRecordID: params.BusinessRecordID,
		FormData:         params.FormData,
		Caller:           actor.Caller,
	})
	if err != nil {
		return err
	}

	return result.Ok(instance).Response(ctx)
}

// ProcessTaskParams contains the parameters for processing a task.
type ProcessTaskParams struct {
	api.P

	TaskID       string         `json:"taskId" validate:"required"`
	Action       string         `json:"action" validate:"required,oneof=approve reject transfer rollback handle"`
	Opinion      string         `json:"opinion"`
	FormData     map[string]any `json:"formData"`
	TransferToID string         `json:"transferToId"`
	TargetNodeID string         `json:"targetNodeId"`
}

// ProcessTask handles task actions (approve/reject/transfer/rollback/handle).
func (r *InstanceResource) ProcessTask(ctx fiber.Ctx, principal *security.Principal, params ProcessTaskParams) error {
	actor, err := resolveActor(ctx.Context(), r.departmentResolver, r.tenantResolver, principal)
	if err != nil {
		return err
	}

	switch params.Action {
	case "approve", "handle":
		_, err = cqrs.Send[command.ApproveTaskCmd, cqrs.Unit](ctx.Context(), r.bus, command.ApproveTaskCmd{
			TaskID:   params.TaskID,
			Operator: actor.Operator,
			Opinion:  params.Opinion,
			FormData: params.FormData,
			Caller:   actor.Caller,
		})

	case "reject":
		_, err = cqrs.Send[command.RejectTaskCmd, cqrs.Unit](ctx.Context(), r.bus, command.RejectTaskCmd{
			TaskID:   params.TaskID,
			Operator: actor.Operator,
			Opinion:  params.Opinion,
			FormData: params.FormData,
			Caller:   actor.Caller,
		})

	case "transfer":
		_, err = cqrs.Send[command.TransferTaskCmd, cqrs.Unit](ctx.Context(), r.bus, command.TransferTaskCmd{
			TaskID:       params.TaskID,
			Operator:     actor.Operator,
			Opinion:      params.Opinion,
			FormData:     params.FormData,
			TransferToID: params.TransferToID,
			Caller:       actor.Caller,
		})

	case "rollback":
		_, err = cqrs.Send[command.RollbackTaskCmd, cqrs.Unit](ctx.Context(), r.bus, command.RollbackTaskCmd{
			TaskID:       params.TaskID,
			Operator:     actor.Operator,
			Opinion:      params.Opinion,
			FormData:     params.FormData,
			TargetNodeID: params.TargetNodeID,
			Caller:       actor.Caller,
		})

	default:
		return fmt.Errorf("%w: %s", errUnsupportedAction, params.Action)
	}

	if err != nil {
		return err
	}

	return result.Ok().Response(ctx)
}

// WithdrawParams contains the parameters for withdrawing an instance.
type WithdrawParams struct {
	api.P

	InstanceID string `json:"instanceId" validate:"required"`
	Reason     string `json:"reason"`
}

// Withdraw withdraws an instance.
func (r *InstanceResource) Withdraw(ctx fiber.Ctx, principal *security.Principal, params WithdrawParams) error {
	actor, err := resolveActor(ctx.Context(), r.departmentResolver, r.tenantResolver, principal)
	if err != nil {
		return err
	}

	if _, err := cqrs.Send[command.WithdrawCmd, cqrs.Unit](ctx.Context(), r.bus, command.WithdrawCmd{
		InstanceID: params.InstanceID,
		Operator:   actor.Operator,
		Reason:     params.Reason,
		Caller:     actor.Caller,
	}); err != nil {
		return err
	}

	return result.Ok().Response(ctx)
}

// ResubmitParams contains the parameters for resubmitting a returned instance.
type ResubmitParams struct {
	api.P

	InstanceID string         `json:"instanceId" validate:"required"`
	FormData   map[string]any `json:"formData"`
}

// Resubmit resubmits a returned instance.
func (r *InstanceResource) Resubmit(ctx fiber.Ctx, principal *security.Principal, params ResubmitParams) error {
	actor, err := resolveActor(ctx.Context(), r.departmentResolver, r.tenantResolver, principal)
	if err != nil {
		return err
	}

	if _, err := cqrs.Send[command.ResubmitCmd, cqrs.Unit](ctx.Context(), r.bus, command.ResubmitCmd{
		InstanceID: params.InstanceID,
		Operator:   actor.Operator,
		FormData:   params.FormData,
		Caller:     actor.Caller,
	}); err != nil {
		return err
	}

	return result.Ok().Response(ctx)
}

// AddCCParams contains the parameters for adding CC records.
type AddCCParams struct {
	api.P

	InstanceID string   `json:"instanceId" validate:"required"`
	CCUserIDs  []string `json:"ccUserIds" validate:"required,min=1"`
}

// AddCC adds CC records for an instance.
func (r *InstanceResource) AddCC(ctx fiber.Ctx, principal *security.Principal, params AddCCParams) error {
	actor, err := resolveActor(ctx.Context(), r.departmentResolver, r.tenantResolver, principal)
	if err != nil {
		return err
	}

	if _, err := cqrs.Send[command.AddCCCmd, cqrs.Unit](ctx.Context(), r.bus, command.AddCCCmd{
		InstanceID: params.InstanceID,
		CCUserIDs:  params.CCUserIDs,
		Operator:   actor.Operator,
		Caller:     actor.Caller,
	}); err != nil {
		return err
	}

	return result.Ok().Response(ctx)
}

// MarkCCReadParams contains the parameters for marking CC records as read.
type MarkCCReadParams struct {
	api.P

	InstanceID string `json:"instanceId" validate:"required"`
}

// MarkCCRead marks CC records as read for the user.
func (r *InstanceResource) MarkCCRead(ctx fiber.Ctx, principal *security.Principal, params MarkCCReadParams) error {
	caller, err := resolveCaller(ctx.Context(), r.tenantResolver, principal)
	if err != nil {
		return err
	}

	if _, err := cqrs.Send[command.MarkCCReadCmd, cqrs.Unit](ctx.Context(), r.bus, command.MarkCCReadCmd{
		InstanceID: params.InstanceID,
		UserID:     principal.ID,
		Caller:     caller,
	}); err != nil {
		return err
	}

	return result.Ok().Response(ctx)
}

// AddAssigneeParams contains the parameters for adding assignees.
type AddAssigneeParams struct {
	api.P

	TaskID  string   `json:"taskId" validate:"required"`
	UserIDs []string `json:"userIds" validate:"required,min=1,max=50"`
	AddType string   `json:"addType" validate:"required,oneof=before after parallel"`
}

// AddAssignee dynamically adds assignees to a task.
func (r *InstanceResource) AddAssignee(ctx fiber.Ctx, principal *security.Principal, params AddAssigneeParams) error {
	actor, err := resolveActor(ctx.Context(), r.departmentResolver, r.tenantResolver, principal)
	if err != nil {
		return err
	}

	if _, err := cqrs.Send[command.AddAssigneeCmd, cqrs.Unit](ctx.Context(), r.bus, command.AddAssigneeCmd{
		TaskID:   params.TaskID,
		UserIDs:  params.UserIDs,
		AddType:  approval.AddAssigneeType(params.AddType),
		Operator: actor.Operator,
		Caller:   actor.Caller,
	}); err != nil {
		return err
	}

	return result.Ok().Response(ctx)
}

// RemoveAssigneeParams contains the parameters for removing an assignee.
type RemoveAssigneeParams struct {
	api.P

	TaskID string `json:"taskId" validate:"required"`
}

// RemoveAssignee removes an assignee by canceling their task.
func (r *InstanceResource) RemoveAssignee(ctx fiber.Ctx, principal *security.Principal, params RemoveAssigneeParams) error {
	actor, err := resolveActor(ctx.Context(), r.departmentResolver, r.tenantResolver, principal)
	if err != nil {
		return err
	}

	if _, err := cqrs.Send[command.RemoveAssigneeCmd, cqrs.Unit](ctx.Context(), r.bus, command.RemoveAssigneeCmd{
		TaskID:   params.TaskID,
		Operator: actor.Operator,
		Caller:   actor.Caller,
	}); err != nil {
		return err
	}

	return result.Ok().Response(ctx)
}

// UrgeTaskParams contains the parameters for urging a task.
type UrgeTaskParams struct {
	api.P

	TaskID  string `json:"taskId" validate:"required"`
	Message string `json:"message"`
}

// UrgeTask sends an urge notification for a pending task.
func (r *InstanceResource) UrgeTask(ctx fiber.Ctx, principal *security.Principal, params UrgeTaskParams) error {
	caller, err := resolveCaller(ctx.Context(), r.tenantResolver, principal)
	if err != nil {
		return err
	}

	if _, err := cqrs.Send[command.UrgeTaskCmd, cqrs.Unit](ctx.Context(), r.bus, command.UrgeTaskCmd{
		TaskID:  params.TaskID,
		UrgerID: principal.ID,
		Message: params.Message,
		Caller:  caller,
	}); err != nil {
		return err
	}

	return result.Ok().Response(ctx)
}

package resource

import (
	"context"
	"fmt"

	"github.com/gofiber/fiber/v3"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/approval/admin"
	"github.com/coldsmirk/vef-framework-go/internal/approval/command"
	"github.com/coldsmirk/vef-framework-go/internal/approval/query"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/page"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/security"
)

// AdminResource exposes admin-level approval management endpoints.
type AdminResource struct {
	api.Resource

	bus          cqrs.Bus
	deptResolver approval.PrincipalDeptResolver
}

// NewAdminResource creates a new admin resource.
func NewAdminResource(bus cqrs.Bus, deptResolver approval.PrincipalDeptResolver) api.Resource {
	return &AdminResource{
		bus:          bus,
		deptResolver: deptResolver,
		Resource: api.NewRPCResource(
			"approval/admin",
			api.WithOperations(
				api.OperationSpec{Action: "find_instances", PermToken: "approval:instance:query"},
				api.OperationSpec{Action: "find_tasks", PermToken: "approval:task:query"},
				api.OperationSpec{Action: "get_instance_detail", PermToken: "approval:instance:detail"},
				api.OperationSpec{Action: "find_action_logs", PermToken: "approval:log:query"},
				api.OperationSpec{Action: "terminate_instance", PermToken: "approval:instance:terminate"},
				api.OperationSpec{Action: "reassign_task", PermToken: "approval:task:reassign"},
			),
		),
	}
}

// AdminFindInstancesParams contains the query parameters for admin instance listing.
type AdminFindInstancesParams struct {
	api.P

	TenantID    *string                  `json:"tenantId"`
	ApplicantID *string                  `json:"applicantId"`
	Status      *approval.InstanceStatus `json:"status"`
	FlowID      *string                  `json:"flowId"`
	Keyword     *string                  `json:"keyword"`
	Page        int                      `json:"page"`
	PageSize    int                      `json:"pageSize"`
}

// FindInstances queries instances for admin management.
func (r *AdminResource) FindInstances(ctx fiber.Ctx, _ *security.Principal, params AdminFindInstancesParams) error {
	res, err := cqrs.Send[query.FindAdminInstancesQuery, *page.Page[admin.Instance]](ctx.Context(), r.bus, query.FindAdminInstancesQuery{
		TenantID:    params.TenantID,
		ApplicantID: params.ApplicantID,
		Status:      params.Status,
		FlowID:      params.FlowID,
		Keyword:     params.Keyword,
		Pageable:    page.Pageable{Page: params.Page, Size: params.PageSize},
	})
	if err != nil {
		return err
	}

	return result.Ok(res).Response(ctx)
}

// AdminFindTasksParams contains the query parameters for admin task listing.
type AdminFindTasksParams struct {
	api.P

	TenantID   *string              `json:"tenantId"`
	AssigneeID *string              `json:"assigneeId"`
	InstanceID *string              `json:"instanceId"`
	Status     *approval.TaskStatus `json:"status"`
	Page       int                  `json:"page"`
	PageSize   int                  `json:"pageSize"`
}

// FindTasks queries tasks for admin management.
func (r *AdminResource) FindTasks(ctx fiber.Ctx, _ *security.Principal, params AdminFindTasksParams) error {
	res, err := cqrs.Send[query.FindAdminTasksQuery, *page.Page[admin.Task]](ctx.Context(), r.bus, query.FindAdminTasksQuery{
		TenantID:   params.TenantID,
		AssigneeID: params.AssigneeID,
		InstanceID: params.InstanceID,
		Status:     params.Status,
		Pageable:   page.Pageable{Page: params.Page, Size: params.PageSize},
	})
	if err != nil {
		return err
	}

	return result.Ok(res).Response(ctx)
}

// AdminGetInstanceDetailParams contains the parameters for getting admin instance detail.
type AdminGetInstanceDetailParams struct {
	api.P

	InstanceID string `json:"instanceId" validate:"required"`
}

// GetInstanceDetail returns the full admin detail of an instance.
func (r *AdminResource) GetInstanceDetail(ctx fiber.Ctx, _ *security.Principal, params AdminGetInstanceDetailParams) error {
	detail, err := cqrs.Send[query.GetAdminInstanceDetailQuery, *admin.InstanceDetail](ctx.Context(), r.bus, query.GetAdminInstanceDetailQuery{
		InstanceID: params.InstanceID,
	})
	if err != nil {
		return err
	}

	return result.Ok(detail).Response(ctx)
}

// AdminFindActionLogsParams contains the parameters for querying admin action logs.
type AdminFindActionLogsParams struct {
	api.P

	InstanceID string  `json:"instanceId" validate:"required"`
	TenantID   *string `json:"tenantId"`
	Page       int     `json:"page"`
	PageSize   int     `json:"pageSize"`
}

// FindActionLogs queries action logs for an instance with pagination.
func (r *AdminResource) FindActionLogs(ctx fiber.Ctx, _ *security.Principal, params AdminFindActionLogsParams) error {
	res, err := cqrs.Send[query.FindAdminActionLogsQuery, *page.Page[admin.ActionLog]](ctx.Context(), r.bus, query.FindAdminActionLogsQuery{
		InstanceID: params.InstanceID,
		TenantID:   params.TenantID,
		Pageable:   page.Pageable{Page: params.Page, Size: params.PageSize},
	})
	if err != nil {
		return err
	}

	return result.Ok(res).Response(ctx)
}

// AdminTerminateInstanceParams contains the parameters for terminating an instance.
type AdminTerminateInstanceParams struct {
	api.P

	InstanceID string `json:"instanceId" validate:"required"`
	Reason     string `json:"reason"`
}

// TerminateInstance terminates a running approval instance.
func (r *AdminResource) TerminateInstance(ctx fiber.Ctx, principal *security.Principal, params AdminTerminateInstanceParams) error {
	operator, err := r.resolveOperator(ctx.Context(), principal)
	if err != nil {
		return err
	}

	if _, err := cqrs.Send[command.TerminateInstanceCmd, cqrs.Unit](ctx.Context(), r.bus, command.TerminateInstanceCmd{
		InstanceID: params.InstanceID,
		Operator:   operator,
		Reason:     params.Reason,
	}); err != nil {
		return err
	}

	return result.Ok().Response(ctx)
}

// AdminReassignTaskParams contains the parameters for reassigning a task.
type AdminReassignTaskParams struct {
	api.P

	TaskID        string `json:"taskId" validate:"required"`
	NewAssigneeID string `json:"newAssigneeId" validate:"required"`
	Reason        string `json:"reason"`
}

// ReassignTask reassigns a pending task to a different user.
func (r *AdminResource) ReassignTask(ctx fiber.Ctx, principal *security.Principal, params AdminReassignTaskParams) error {
	operator, err := r.resolveOperator(ctx.Context(), principal)
	if err != nil {
		return err
	}

	if _, err := cqrs.Send[command.ReassignTaskCmd, cqrs.Unit](ctx.Context(), r.bus, command.ReassignTaskCmd{
		TaskID:        params.TaskID,
		NewAssigneeID: params.NewAssigneeID,
		Operator:      operator,
		Reason:        params.Reason,
	}); err != nil {
		return err
	}

	return result.Ok().Response(ctx)
}

// resolveOperator builds an OperatorInfo from the authenticated principal.
func (r *AdminResource) resolveOperator(ctx context.Context, principal *security.Principal) (approval.OperatorInfo, error) {
	deptID, deptName, err := r.deptResolver.Resolve(ctx, principal)
	if err != nil {
		return approval.OperatorInfo{}, fmt.Errorf("resolve operator dept: %w", err)
	}

	return approval.OperatorInfo{
		ID:       principal.ID,
		Name:     principal.Name,
		DeptID:   deptID,
		DeptName: deptName,
	}, nil
}

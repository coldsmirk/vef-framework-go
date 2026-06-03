package resource

import (
	"strings"

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

	bus                cqrs.Bus
	departmentResolver approval.PrincipalDepartmentResolver
	tenantResolver     approval.PrincipalTenantResolver
}

// NewAdminResource creates a new admin resource.
func NewAdminResource(
	bus cqrs.Bus,
	departmentResolver approval.PrincipalDepartmentResolver,
	tenantResolver approval.PrincipalTenantResolver,
) api.Resource {
	return &AdminResource{
		bus:                bus,
		departmentResolver: departmentResolver,
		tenantResolver:     tenantResolver,
		Resource: api.NewRPCResource(
			"approval/admin",
			api.WithOperations(
				api.OperationSpec{Action: "find_instances", RequiredPermission: "approval:instance:query"},
				api.OperationSpec{Action: "find_tasks", RequiredPermission: "approval:task:query"},
				api.OperationSpec{Action: "get_instance_detail", RequiredPermission: "approval:instance:detail"},
				api.OperationSpec{Action: "find_action_logs", RequiredPermission: "approval:log:query"},
				api.OperationSpec{Action: "get_metrics", RequiredPermission: "approval:metrics:query"},
				// Admin write actions: framework-level audit captures who/when/IP
				// in addition to the business-table action_log.
				api.OperationSpec{Action: "terminate_instance", RequiredPermission: "approval:instance:terminate", EnableAudit: true},
				api.OperationSpec{Action: "reassign_task", RequiredPermission: "approval:task:reassign", EnableAudit: true},
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
func (r *AdminResource) FindInstances(ctx fiber.Ctx, principal *security.Principal, params AdminFindInstancesParams) error {
	tenantFilter, err := r.resolveTenantFilter(ctx, principal, params.TenantID)
	if err != nil {
		return err
	}

	res, err := cqrs.Send[query.FindAdminInstancesQuery, *page.Page[admin.Instance]](ctx.Context(), r.bus, query.FindAdminInstancesQuery{
		TenantID:    tenantFilter,
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

// resolveTenantFilter derives the tenant filter for an admin query: a
// non-super-admin caller always filters by their own tenant (override is
// ignored); a super-admin may pass an explicit override or leave it empty
// for cross-tenant visibility. Returns nil when the resolved filter is
// empty so query handlers treat it as "no tenant filter".
func (r *AdminResource) resolveTenantFilter(ctx fiber.Ctx, principal *security.Principal, override *string) (*string, error) {
	caller, err := resolveCaller(ctx.Context(), r.tenantResolver, principal)
	if err != nil {
		return nil, err
	}

	overrideValue := ""
	if override != nil {
		overrideValue = strings.TrimSpace(*override)
	}

	effective := caller.EffectiveTenantID(overrideValue)
	if effective == "" {
		return nil, nil
	}

	return &effective, nil
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
func (r *AdminResource) FindTasks(ctx fiber.Ctx, principal *security.Principal, params AdminFindTasksParams) error {
	tenantFilter, err := r.resolveTenantFilter(ctx, principal, params.TenantID)
	if err != nil {
		return err
	}

	res, err := cqrs.Send[query.FindAdminTasksQuery, *page.Page[admin.Task]](ctx.Context(), r.bus, query.FindAdminTasksQuery{
		TenantID:   tenantFilter,
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
func (r *AdminResource) GetInstanceDetail(ctx fiber.Ctx, principal *security.Principal, params AdminGetInstanceDetailParams) error {
	caller, err := resolveCaller(ctx.Context(), r.tenantResolver, principal)
	if err != nil {
		return err
	}

	detail, err := cqrs.Send[query.GetAdminInstanceDetailQuery, *admin.InstanceDetail](ctx.Context(), r.bus, query.GetAdminInstanceDetailQuery{
		InstanceID: params.InstanceID,
		Caller:     caller,
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
func (r *AdminResource) FindActionLogs(ctx fiber.Ctx, principal *security.Principal, params AdminFindActionLogsParams) error {
	tenantFilter, err := r.resolveTenantFilter(ctx, principal, params.TenantID)
	if err != nil {
		return err
	}

	res, err := cqrs.Send[query.FindAdminActionLogsQuery, *page.Page[admin.ActionLog]](ctx.Context(), r.bus, query.FindAdminActionLogsQuery{
		InstanceID: params.InstanceID,
		TenantID:   tenantFilter,
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
	Reason     string `json:"reason" validate:"max=2000"`
}

// TerminateInstance terminates a running approval instance.
func (r *AdminResource) TerminateInstance(ctx fiber.Ctx, principal *security.Principal, params AdminTerminateInstanceParams) error {
	actor, err := resolveActor(ctx.Context(), r.departmentResolver, r.tenantResolver, principal)
	if err != nil {
		return err
	}

	if _, err := cqrs.Send[command.TerminateInstanceCmd, cqrs.Unit](ctx.Context(), r.bus, command.TerminateInstanceCmd{
		InstanceID: params.InstanceID,
		Operator:   actor.Operator,
		Reason:     params.Reason,
		Caller:     actor.Caller,
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
	Reason        string `json:"reason" validate:"max=2000"`
}

// ReassignTask reassigns a pending task to a different user.
func (r *AdminResource) ReassignTask(ctx fiber.Ctx, principal *security.Principal, params AdminReassignTaskParams) error {
	actor, err := resolveActor(ctx.Context(), r.departmentResolver, r.tenantResolver, principal)
	if err != nil {
		return err
	}

	if _, err := cqrs.Send[command.ReassignTaskCmd, cqrs.Unit](ctx.Context(), r.bus, command.ReassignTaskCmd{
		TaskID:        params.TaskID,
		NewAssigneeID: params.NewAssigneeID,
		Operator:      actor.Operator,
		Reason:        params.Reason,
		Caller:        actor.Caller,
	}); err != nil {
		return err
	}

	return result.Ok().Response(ctx)
}

// AdminGetMetricsParams contains the parameters for the metrics dashboard query.
type AdminGetMetricsParams struct {
	api.P

	TenantID *string `json:"tenantId"`
}

// GetMetrics returns aggregated approval engine metrics for the admin dashboard.
func (r *AdminResource) GetMetrics(ctx fiber.Ctx, principal *security.Principal, params AdminGetMetricsParams) error {
	tenantFilter, err := r.resolveTenantFilter(ctx, principal, params.TenantID)
	if err != nil {
		return err
	}

	tenantID := ""
	if tenantFilter != nil {
		tenantID = *tenantFilter
	}

	metrics, err := cqrs.Send[query.GetMetricsQuery, *admin.Metrics](ctx.Context(), r.bus, query.GetMetricsQuery{
		TenantID: tenantID,
	})
	if err != nil {
		return err
	}

	return result.Ok(metrics).Response(ctx)
}

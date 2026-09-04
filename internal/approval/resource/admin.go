package resource

import (
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/approval/admin"
	"github.com/coldsmirk/vef-framework-go/internal/approval/query"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/page"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/security"
)

// AdminResource exposes admin-level approval management endpoints. Queries
// dispatch on the bus directly; every runtime operation goes through
// approval.Service, so the request path and a programmatic caller stay one
// code path.
type AdminResource struct {
	api.Resource

	bus                cqrs.Bus
	svc                approval.Service
	departmentResolver approval.PrincipalDepartmentResolver
	tenantResolver     approval.PrincipalTenantResolver
}

// NewAdminResource creates a new admin resource.
func NewAdminResource(
	bus cqrs.Bus,
	svc approval.Service,
	departmentResolver approval.PrincipalDepartmentResolver,
	tenantResolver approval.PrincipalTenantResolver,
) api.Resource {
	return &AdminResource{
		bus:                bus,
		svc:                svc,
		departmentResolver: departmentResolver,
		tenantResolver:     tenantResolver,
		Resource: api.NewRPCResource(
			"approval/admin",
			api.WithOperations(
				api.OperationSpec{Action: "find_instances", RequiredPermission: "approval.instance.query"},
				api.OperationSpec{Action: "find_tasks", RequiredPermission: "approval.task.query"},
				api.OperationSpec{Action: "get_instance_detail", RequiredPermission: "approval.instance.detail"},
				api.OperationSpec{Action: "find_action_logs", RequiredPermission: "approval.action_log.query"},
				api.OperationSpec{Action: "get_metrics", RequiredPermission: "approval.metrics.query"},
				api.OperationSpec{Action: "find_business_projections", RequiredPermission: "approval.binding.query"},
				// Admin write actions: framework-level audit captures who/when/IP
				// in addition to the business-table action_log.
				api.OperationSpec{Action: "terminate_instance", RequiredPermission: "approval.instance.terminate", EnableAudit: true},
				api.OperationSpec{Action: "reassign_task", RequiredPermission: "approval.task.reassign", EnableAudit: true},
				api.OperationSpec{Action: "retry_business_projection", RequiredPermission: "approval.binding.retry", EnableAudit: true},
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
		Page:        params.Page,
		Size:        params.PageSize,
	})
	if err != nil {
		return err
	}

	return result.Ok(res).Response(ctx)
}

// resolveTenantFilter derives the tenant filter for an admin query: a
// non-super-admin caller always filters by their own tenant (override is
// ignored) and fails closed if it has no tenant; a super-admin may pass an
// explicit override or leave it empty for cross-tenant visibility. A nil result
// means "no tenant filter" and is only ever produced for a privileged caller.
func (r *AdminResource) resolveTenantFilter(ctx fiber.Ctx, principal *security.Principal, override *string) (*string, error) {
	caller, err := resolveCaller(ctx.Context(), r.tenantResolver, principal)
	if err != nil {
		return nil, err
	}

	overrideValue := ""
	if override != nil {
		overrideValue = strings.TrimSpace(*override)
	}

	return caller.TenantScopeFilter(overrideValue)
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
		Page:       params.Page,
		Size:       params.PageSize,
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
		Page:       params.Page,
		Size:       params.PageSize,
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
func (r *AdminResource) TerminateInstance(ctx fiber.Ctx, db orm.DB, principal *security.Principal, params AdminTerminateInstanceParams) error {
	actor, err := resolveActor(ctx.Context(), r.departmentResolver, r.tenantResolver, principal)
	if err != nil {
		return err
	}

	if err := r.svc.TerminateInstance(ctx.Context(), db, approval.TerminateInstanceInput{
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
func (r *AdminResource) ReassignTask(ctx fiber.Ctx, db orm.DB, principal *security.Principal, params AdminReassignTaskParams) error {
	actor, err := resolveActor(ctx.Context(), r.departmentResolver, r.tenantResolver, principal)
	if err != nil {
		return err
	}

	if err := r.svc.ReassignTask(ctx.Context(), db, approval.ReassignTaskInput{
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

// AdminFindBusinessProjectionsParams contains projection admin filters.
type AdminFindBusinessProjectionsParams struct {
	api.P

	TenantID *string                           `json:"tenantId"`
	Status   *approval.BindingProjectionStatus `json:"status"`
	Page     int                               `json:"page"`
	PageSize int                               `json:"pageSize"`
}

// FindBusinessProjections lists durable binding convergence state.
func (r *AdminResource) FindBusinessProjections(
	ctx fiber.Ctx,
	principal *security.Principal,
	params AdminFindBusinessProjectionsParams,
) error {
	tenantFilter, err := r.resolveTenantFilter(ctx, principal, params.TenantID)
	if err != nil {
		return err
	}

	projections, err := cqrs.Send[query.FindAdminBusinessProjectionsQuery, *page.Page[admin.BusinessProjection]](
		ctx.Context(),
		r.bus,
		query.FindAdminBusinessProjectionsQuery{
			TenantID: tenantFilter,
			Status:   params.Status,
			Page:     params.Page,
			Size:     params.PageSize,
		},
	)
	if err != nil {
		return err
	}

	return result.Ok(projections).Response(ctx)
}

// AdminRetryBusinessProjectionParams identifies a projection to retry now.
type AdminRetryBusinessProjectionParams struct {
	api.P

	ProjectionID string `json:"projectionId" validate:"required"`
}

// RetryBusinessProjection immediately retries one eventual projection.
func (r *AdminResource) RetryBusinessProjection(
	ctx fiber.Ctx,
	db orm.DB,
	principal *security.Principal,
	params AdminRetryBusinessProjectionParams,
) error {
	caller, err := resolveCaller(ctx.Context(), r.tenantResolver, principal)
	if err != nil {
		return err
	}

	if err := r.svc.RetryBusinessProjection(ctx.Context(), db, approval.RetryBusinessProjectionInput{
		ProjectionID: params.ProjectionID,
		Caller:       caller,
	}); err != nil {
		return err
	}

	return result.Ok().Response(ctx)
}

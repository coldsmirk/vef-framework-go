package resource

import (
	"github.com/gofiber/fiber/v3"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/approval/my"
	"github.com/coldsmirk/vef-framework-go/internal/approval/query"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/page"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/security"
)

// MyResource exposes self-service approval queries for the current user.
type MyResource struct {
	api.Resource

	bus          cqrs.Bus
	deptResolver approval.PrincipalDeptResolver
}

// NewMyResource creates a new self-service resource.
func NewMyResource(bus cqrs.Bus, deptResolver approval.PrincipalDeptResolver) api.Resource {
	return &MyResource{
		bus:          bus,
		deptResolver: deptResolver,
		Resource: api.NewRPCResource(
			"approval/my",
			api.WithOperations(
				api.OperationSpec{Action: "find_available_flows"},
				api.OperationSpec{Action: "find_initiated"},
				api.OperationSpec{Action: "find_pending_tasks"},
				api.OperationSpec{Action: "find_completed_tasks"},
				api.OperationSpec{Action: "find_cc_records"},
				api.OperationSpec{Action: "get_pending_counts"},
				api.OperationSpec{Action: "get_instance_detail"},
			),
		),
	}
}

// FindAvailableFlowsParams contains the parameters for querying available flows.
type FindAvailableFlowsParams struct {
	api.P

	TenantID *string `json:"tenantId"`
	Keyword  *string `json:"keyword"`
	Page     int     `json:"page"`
	PageSize int     `json:"pageSize"`
}

// FindAvailableFlows queries flows the current user is allowed to initiate.
func (r *MyResource) FindAvailableFlows(ctx fiber.Ctx, principal *security.Principal, params FindAvailableFlowsParams) error {
	deptID, _, err := r.deptResolver.Resolve(ctx.Context(), principal)
	if err != nil {
		return err
	}

	res, err := cqrs.Send[query.FindAvailableFlowsQuery, *page.Page[my.AvailableFlow]](ctx.Context(), r.bus, query.FindAvailableFlowsQuery{
		UserID:          principal.ID,
		TenantID:        params.TenantID,
		ApplicantDeptID: deptID,
		Keyword:         params.Keyword,
		Pageable:        page.Pageable{Page: params.Page, Size: params.PageSize},
	})
	if err != nil {
		return err
	}

	return result.Ok(res).Response(ctx)
}

// FindInitiatedParams contains the parameters for querying initiated instances.
type FindInitiatedParams struct {
	api.P

	TenantID *string                  `json:"tenantId"`
	Status   *approval.InstanceStatus `json:"status"`
	Keyword  *string                  `json:"keyword"`
	Page     int                      `json:"page"`
	PageSize int                      `json:"pageSize"`
}

// FindInitiated queries instances initiated by the current user.
func (r *MyResource) FindInitiated(ctx fiber.Ctx, principal *security.Principal, params FindInitiatedParams) error {
	res, err := cqrs.Send[query.FindMyInitiatedQuery, *page.Page[my.InitiatedInstance]](ctx.Context(), r.bus, query.FindMyInitiatedQuery{
		UserID:   principal.ID,
		TenantID: params.TenantID,
		Status:   params.Status,
		Keyword:  params.Keyword,
		Pageable: page.Pageable{Page: params.Page, Size: params.PageSize},
	})
	if err != nil {
		return err
	}

	return result.Ok(res).Response(ctx)
}

// FindPendingTasksParams contains the parameters for querying pending tasks.
type FindPendingTasksParams struct {
	api.P

	TenantID *string `json:"tenantId"`
	Page     int     `json:"page"`
	PageSize int     `json:"pageSize"`
}

// FindPendingTasks queries pending tasks assigned to the current user.
func (r *MyResource) FindPendingTasks(ctx fiber.Ctx, principal *security.Principal, params FindPendingTasksParams) error {
	res, err := cqrs.Send[query.FindMyPendingTasksQuery, *page.Page[my.PendingTask]](ctx.Context(), r.bus, query.FindMyPendingTasksQuery{
		UserID:   principal.ID,
		TenantID: params.TenantID,
		Pageable: page.Pageable{Page: params.Page, Size: params.PageSize},
	})
	if err != nil {
		return err
	}

	return result.Ok(res).Response(ctx)
}

// FindCompletedTasksParams contains the parameters for querying completed tasks.
type FindCompletedTasksParams struct {
	api.P

	TenantID *string `json:"tenantId"`
	Page     int     `json:"page"`
	PageSize int     `json:"pageSize"`
}

// FindCompletedTasks queries tasks already processed by the current user.
func (r *MyResource) FindCompletedTasks(ctx fiber.Ctx, principal *security.Principal, params FindCompletedTasksParams) error {
	res, err := cqrs.Send[query.FindMyCompletedTasksQuery, *page.Page[my.CompletedTask]](ctx.Context(), r.bus, query.FindMyCompletedTasksQuery{
		UserID:   principal.ID,
		TenantID: params.TenantID,
		Pageable: page.Pageable{Page: params.Page, Size: params.PageSize},
	})
	if err != nil {
		return err
	}

	return result.Ok(res).Response(ctx)
}

// FindCCRecordsParams contains the parameters for querying CC records.
type FindCCRecordsParams struct {
	api.P

	TenantID *string `json:"tenantId"`
	IsRead   *bool   `json:"isRead"`
	Page     int     `json:"page"`
	PageSize int     `json:"pageSize"`
}

// FindCCRecords queries CC records addressed to the current user.
func (r *MyResource) FindCCRecords(ctx fiber.Ctx, principal *security.Principal, params FindCCRecordsParams) error {
	res, err := cqrs.Send[query.FindMyCCRecordsQuery, *page.Page[my.CCRecord]](ctx.Context(), r.bus, query.FindMyCCRecordsQuery{
		UserID:   principal.ID,
		TenantID: params.TenantID,
		IsRead:   params.IsRead,
		Pageable: page.Pageable{Page: params.Page, Size: params.PageSize},
	})
	if err != nil {
		return err
	}

	return result.Ok(res).Response(ctx)
}

// GetPendingCounts retrieves pending task and unread CC counts for the current user.
func (r *MyResource) GetPendingCounts(ctx fiber.Ctx, principal *security.Principal) error {
	res, err := cqrs.Send[query.GetMyPendingCountsQuery, *my.PendingCounts](ctx.Context(), r.bus, query.GetMyPendingCountsQuery{
		UserID: principal.ID,
	})
	if err != nil {
		return err
	}

	return result.Ok(res).Response(ctx)
}

// GetInstanceDetailParams contains the parameters for getting instance detail.
type GetInstanceDetailParams struct {
	api.P

	InstanceID string `json:"instanceId" validate:"required"`
}

// GetInstanceDetail retrieves instance detail with access control for the current user.
func (r *MyResource) GetInstanceDetail(ctx fiber.Ctx, principal *security.Principal, params GetInstanceDetailParams) error {
	detail, err := cqrs.Send[query.GetMyInstanceDetailQuery, *my.InstanceDetail](ctx.Context(), r.bus, query.GetMyInstanceDetailQuery{
		InstanceID: params.InstanceID,
		UserID:     principal.ID,
	})
	if err != nil {
		return err
	}

	return result.Ok(detail).Response(ctx)
}

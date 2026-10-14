package resource

import (
	"github.com/gofiber/fiber/v3"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/approval/my"
	"github.com/coldsmirk/vef-framework-go/internal/approval/query"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/page"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/security"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// MyResource exposes self-service approval queries for the current user.
type MyResource struct {
	api.Resource

	bus                cqrs.Bus
	departmentResolver approval.PrincipalDepartmentResolver
	tenantResolver     approval.PrincipalTenantResolver
	formGlobals        *service.FormGlobalsService
}

// NewMyResource creates a new self-service resource.
func NewMyResource(
	bus cqrs.Bus,
	departmentResolver approval.PrincipalDepartmentResolver,
	tenantResolver approval.PrincipalTenantResolver,
	formGlobals *service.FormGlobalsService,
) api.Resource {
	return &MyResource{
		bus:                bus,
		departmentResolver: departmentResolver,
		tenantResolver:     tenantResolver,
		formGlobals:        formGlobals,
		Resource: api.NewRPCResource(
			"approval/my",
			api.WithOperations(
				api.OperationSpec{Action: "find_available_flows"},
				api.OperationSpec{Action: "get_start_form"},
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

	Keyword  *string           `json:"keyword"`
	Labels   map[string]string `json:"labels"`
	Page     int               `json:"page"`
	PageSize int               `json:"pageSize"`
}

// FindAvailableFlows queries the flows the current user may initiate in the
// tenant the user acts in: its own flows plus the shared ones.
func (r *MyResource) FindAvailableFlows(ctx fiber.Ctx, principal *security.Principal, params FindAvailableFlowsParams) error {
	departmentID, _, err := r.departmentResolver.Resolve(ctx.Context(), principal)
	if err != nil {
		return err
	}

	caller, err := resolveCaller(ctx.Context(), r.tenantResolver, principal)
	if err != nil {
		return err
	}

	res, err := cqrs.Send[query.FindAvailableFlowsQuery, *page.Page[my.AvailableFlow]](ctx.Context(), r.bus, query.FindAvailableFlowsQuery{
		UserID:                principal.ID,
		ApplicantDepartmentID: departmentID,
		Keyword:               params.Keyword,
		Labels:                params.Labels,
		Caller:                caller,
		Page:                  params.Page,
		Size:                  params.PageSize,
	})
	if err != nil {
		return err
	}

	return result.Ok(res).Response(ctx)
}

// GetStartFormParams contains the parameters for loading a flow's start form.
type GetStartFormParams struct {
	api.P

	FlowCode string `json:"flowCode" validate:"required"`
}

// GetStartForm loads the published form document for initiating a flow,
// gated exactly like starting the instance. The applicant snapshot, the
// tenant, and the form-visible globals are resolved the way Start resolves
// them, so the form is evaluated against what the instance will record.
func (r *MyResource) GetStartForm(ctx fiber.Ctx, principal *security.Principal, params GetStartFormParams) error {
	actor, err := resolveActor(ctx.Context(), r.departmentResolver, r.tenantResolver, principal)
	if err != nil {
		return err
	}

	form, err := cqrs.Send[query.GetStartFormQuery, *my.StartForm](ctx.Context(), r.bus, query.GetStartFormQuery{
		FlowCode:  params.FlowCode,
		Applicant: actor.Operator,
		Caller:    actor.Caller,
	})
	if err != nil {
		return err
	}

	// Resolved only past the gate: a caller who may not start the flow never
	// reaches the host resolver.
	if form.Globals, err = r.formGlobals.Resolve(ctx.Context(), principal, form.FlowCode); err != nil {
		return err
	}

	return result.Ok(form).Response(ctx)
}

// FindInitiatedParams contains the parameters for querying initiated instances.
type FindInitiatedParams struct {
	api.P

	Status   *approval.InstanceStatus `json:"status"`
	Keyword  *string                  `json:"keyword"`
	Page     int                      `json:"page"`
	PageSize int                      `json:"pageSize"`
}

// FindInitiated queries instances initiated by the current user.
func (r *MyResource) FindInitiated(ctx fiber.Ctx, principal *security.Principal, params FindInitiatedParams) error {
	res, err := cqrs.Send[query.FindMyInitiatedQuery, *page.Page[my.InitiatedInstance]](ctx.Context(), r.bus, query.FindMyInitiatedQuery{
		UserID:  principal.ID,
		Status:  params.Status,
		Keyword: params.Keyword,
		Page:    params.Page,
		Size:    params.PageSize,
	})
	if err != nil {
		return err
	}

	return result.Ok(res).Response(ctx)
}

// FindPendingTasksParams contains the parameters for querying pending tasks.
// The time bounds are inclusive.
type FindPendingTasksParams struct {
	api.P

	Keyword       *string         `json:"keyword"`
	ApplicantID   *string         `json:"applicantId"`
	ApplicantName *string         `json:"applicantName"`
	FlowID        *string         `json:"flowId"`
	IsTimeout     *bool           `json:"isTimeout"`
	CreatedAtFrom *timex.DateTime `json:"createdAtFrom"`
	CreatedAtTo   *timex.DateTime `json:"createdAtTo"`
	Page          int             `json:"page"`
	PageSize      int             `json:"pageSize"`
}

// FindPendingTasks queries pending tasks assigned to the current user.
func (r *MyResource) FindPendingTasks(ctx fiber.Ctx, principal *security.Principal, params FindPendingTasksParams) error {
	res, err := cqrs.Send[query.FindMyPendingTasksQuery, *page.Page[my.PendingTask]](ctx.Context(), r.bus, query.FindMyPendingTasksQuery{
		UserID:        principal.ID,
		Keyword:       params.Keyword,
		ApplicantID:   params.ApplicantID,
		ApplicantName: params.ApplicantName,
		FlowID:        params.FlowID,
		IsTimeout:     params.IsTimeout,
		CreatedAtFrom: params.CreatedAtFrom,
		CreatedAtTo:   params.CreatedAtTo,
		Page:          params.Page,
		Size:          params.PageSize,
	})
	if err != nil {
		return err
	}

	return result.Ok(res).Response(ctx)
}

// FindCompletedTasksParams contains the parameters for querying completed tasks.
// The time bounds are inclusive.
type FindCompletedTasksParams struct {
	api.P

	Keyword        *string                  `json:"keyword"`
	ApplicantID    *string                  `json:"applicantId"`
	ApplicantName  *string                  `json:"applicantName"`
	FlowID         *string                  `json:"flowId"`
	Status         *approval.TaskStatus     `json:"status"`
	InstanceStatus *approval.InstanceStatus `json:"instanceStatus"`
	FinishedAtFrom *timex.DateTime          `json:"finishedAtFrom"`
	FinishedAtTo   *timex.DateTime          `json:"finishedAtTo"`
	Page           int                      `json:"page"`
	PageSize       int                      `json:"pageSize"`
}

// FindCompletedTasks queries tasks already processed by the current user.
func (r *MyResource) FindCompletedTasks(ctx fiber.Ctx, principal *security.Principal, params FindCompletedTasksParams) error {
	res, err := cqrs.Send[query.FindMyCompletedTasksQuery, *page.Page[my.CompletedTask]](ctx.Context(), r.bus, query.FindMyCompletedTasksQuery{
		UserID:         principal.ID,
		Keyword:        params.Keyword,
		ApplicantID:    params.ApplicantID,
		ApplicantName:  params.ApplicantName,
		FlowID:         params.FlowID,
		Status:         params.Status,
		InstanceStatus: params.InstanceStatus,
		FinishedAtFrom: params.FinishedAtFrom,
		FinishedAtTo:   params.FinishedAtTo,
		Page:           params.Page,
		Size:           params.PageSize,
	})
	if err != nil {
		return err
	}

	return result.Ok(res).Response(ctx)
}

// FindCCRecordsParams contains the parameters for querying CC records.
type FindCCRecordsParams struct {
	api.P

	IsRead   *bool `json:"isRead"`
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
}

// FindCCRecords queries CC records addressed to the current user.
func (r *MyResource) FindCCRecords(ctx fiber.Ctx, principal *security.Principal, params FindCCRecordsParams) error {
	res, err := cqrs.Send[query.FindMyCCRecordsQuery, *page.Page[my.CCRecord]](ctx.Context(), r.bus, query.FindMyCCRecordsQuery{
		UserID: principal.ID,
		IsRead: params.IsRead,
		Page:   params.Page,
		Size:   params.PageSize,
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

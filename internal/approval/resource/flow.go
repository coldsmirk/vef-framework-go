package resource

import (
	"github.com/gofiber/fiber/v3"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/command"
	"github.com/coldsmirk/vef-framework-go/internal/approval/query"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/page"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/security"
)

// FlowResource handles flow definition management.
type FlowResource struct {
	api.Resource

	bus cqrs.Bus
}

// NewFlowResource creates a new flow resource.
func NewFlowResource(bus cqrs.Bus) api.Resource {
	return &FlowResource{
		bus: bus,
		Resource: api.NewRPCResource(
			"approval/flow",
			api.WithOperations(
				api.OperationSpec{Action: "create", PermToken: "approval:flow:create"},
				api.OperationSpec{Action: "deploy", PermToken: "approval:flow:deploy"},
				api.OperationSpec{Action: "publish_version", PermToken: "approval:flow:publish"},
				api.OperationSpec{Action: "get_graph", PermToken: "approval:flow:query"},
				api.OperationSpec{Action: "find_flows", PermToken: "approval:flow:query"},
				api.OperationSpec{Action: "update_flow", PermToken: "approval:flow:update"},
				api.OperationSpec{Action: "toggle_active", PermToken: "approval:flow:update"},
				api.OperationSpec{Action: "find_versions", PermToken: "approval:flow:query"},
			),
		),
	}
}

// CreateFlowParams contains the parameters for creating a flow.
type CreateFlowParams struct {
	api.P

	TenantID               string                  `json:"tenantId" validate:"required"`
	Code                   string                  `json:"code" validate:"required"`
	Name                   string                  `json:"name" validate:"required"`
	CategoryID             string                  `json:"categoryId" validate:"required"`
	Icon                   *string                 `json:"icon"`
	Description            *string                 `json:"description"`
	BindingMode            approval.BindingMode    `json:"bindingMode" validate:"required"`
	BusinessTable          *string                 `json:"businessTable"`
	BusinessPkField        *string                 `json:"businessPkField"`
	BusinessTitleField     *string                 `json:"businessTitleField"`
	BusinessStatusField    *string                 `json:"businessStatusField"`
	AdminUserIDs           []string                `json:"adminUserIds"`
	IsAllInitiationAllowed bool                    `json:"isAllInitiationAllowed"`
	InstanceTitleTemplate  string                  `json:"instanceTitleTemplate"`
	Initiators             []CreateInitiatorParams `json:"initiators"`
}

// CreateInitiatorParams contains the parameters for a flow initiator.
type CreateInitiatorParams struct {
	Kind approval.InitiatorKind `json:"kind" validate:"required"`
	IDs  []string               `json:"ids" validate:"required"`
}

// Create creates a new flow.
func (r *FlowResource) Create(ctx fiber.Ctx, params CreateFlowParams) error {
	initiators := make([]shared.CreateFlowInitiatorCmd, len(params.Initiators))
	for i, initiator := range params.Initiators {
		initiators[i] = shared.CreateFlowInitiatorCmd{
			Kind: initiator.Kind,
			IDs:  initiator.IDs,
		}
	}

	flow, err := cqrs.Send[command.CreateFlowCmd, *approval.Flow](
		ctx.Context(),
		r.bus,
		command.CreateFlowCmd{
			TenantID:               params.TenantID,
			Code:                   params.Code,
			Name:                   params.Name,
			CategoryID:             params.CategoryID,
			Icon:                   params.Icon,
			Description:            params.Description,
			BindingMode:            params.BindingMode,
			BusinessTable:          params.BusinessTable,
			BusinessPkField:        params.BusinessPkField,
			BusinessTitleField:     params.BusinessTitleField,
			BusinessStatusField:    params.BusinessStatusField,
			AdminUserIDs:           params.AdminUserIDs,
			IsAllInitiationAllowed: params.IsAllInitiationAllowed,
			InstanceTitleTemplate:  params.InstanceTitleTemplate,
			Initiators:             initiators,
		},
	)
	if err != nil {
		return err
	}

	return result.Ok(flow).Response(ctx)
}

// DeployFlowParams contains the parameters for deploying a flow definition.
type DeployFlowParams struct {
	api.P

	FlowID         string                   `json:"flowId" validate:"required"`
	Description    *string                  `json:"description"`
	FlowDefinition approval.FlowDefinition  `json:"flowDefinition" validate:"required"`
	FormDefinition *approval.FormDefinition `json:"formDefinition"`
}

// Deploy deploys a flow definition.
func (r *FlowResource) Deploy(ctx fiber.Ctx, params DeployFlowParams) error {
	version, err := cqrs.Send[command.DeployFlowCmd, *approval.FlowVersion](
		ctx.Context(),
		r.bus,
		command.DeployFlowCmd{
			FlowID:         params.FlowID,
			Description:    params.Description,
			FlowDefinition: params.FlowDefinition,
			FormDefinition: params.FormDefinition,
		},
	)
	if err != nil {
		return err
	}

	return result.Ok(version).Response(ctx)
}

// PublishVersionParams contains the parameters for publishing a version.
type PublishVersionParams struct {
	api.P

	VersionID string `json:"versionId" validate:"required"`
}

// PublishVersion publishes a flow version.
func (r *FlowResource) PublishVersion(ctx fiber.Ctx, principal *security.Principal, params PublishVersionParams) error {
	if _, err := cqrs.Send[command.PublishVersionCmd, cqrs.Unit](
		ctx.Context(),
		r.bus,
		command.PublishVersionCmd{
			VersionID:  params.VersionID,
			OperatorID: principal.ID,
		},
	); err != nil {
		return err
	}

	return result.Ok().Response(ctx)
}

// GetGraphParams contains the parameters for getting a flow graph.
type GetGraphParams struct {
	api.P

	FlowID   string `json:"flowId" validate:"required"`
	TenantID string `json:"tenantId"`
}

// GetGraph returns the flow graph for the published version.
func (r *FlowResource) GetGraph(ctx fiber.Ctx, params GetGraphParams) error {
	graph, err := cqrs.Send[query.GetFlowGraphQuery, *shared.FlowGraph](
		ctx.Context(),
		r.bus,
		query.GetFlowGraphQuery{
			FlowID:   params.FlowID,
			TenantID: params.TenantID,
		},
	)
	if err != nil {
		return err
	}

	return result.Ok(graph).Response(ctx)
}

// FindFlowsParams contains the parameters for finding flows.
type FindFlowsParams struct {
	api.P

	TenantID   *string `json:"tenantId"`
	CategoryID *string `json:"categoryId"`
	Keyword    *string `json:"keyword"`
	IsActive   *bool   `json:"isActive"`
	Page       int     `json:"page"`
	PageSize   int     `json:"pageSize"`
}

// FindFlows queries flows for admin management.
func (r *FlowResource) FindFlows(ctx fiber.Ctx, params FindFlowsParams) error {
	res, err := cqrs.Send[query.FindFlowsQuery, *page.Page[approval.Flow]](
		ctx.Context(),
		r.bus,
		query.FindFlowsQuery{
			TenantID:   params.TenantID,
			CategoryID: params.CategoryID,
			Keyword:    params.Keyword,
			IsActive:   params.IsActive,
			Pageable:   page.Pageable{Page: params.Page, Size: params.PageSize},
		},
	)
	if err != nil {
		return err
	}

	return result.Ok(res).Response(ctx)
}

// UpdateFlowParams contains the parameters for updating a flow.
type UpdateFlowParams struct {
	api.P

	FlowID                 string                  `json:"flowId" validate:"required"`
	Name                   string                  `json:"name" validate:"required"`
	Icon                   *string                 `json:"icon"`
	Description            *string                 `json:"description"`
	AdminUserIDs           []string                `json:"adminUserIds"`
	IsAllInitiationAllowed bool                    `json:"isAllInitiationAllowed"`
	InstanceTitleTemplate  string                  `json:"instanceTitleTemplate" validate:"required"`
	Initiators             []CreateInitiatorParams `json:"initiators"`
}

// UpdateFlow updates an existing flow.
func (r *FlowResource) UpdateFlow(ctx fiber.Ctx, params UpdateFlowParams) error {
	initiators := make([]shared.CreateFlowInitiatorCmd, len(params.Initiators))
	for i, initiator := range params.Initiators {
		initiators[i] = shared.CreateFlowInitiatorCmd{
			Kind: initiator.Kind,
			IDs:  initiator.IDs,
		}
	}

	flow, err := cqrs.Send[command.UpdateFlowCmd, *approval.Flow](
		ctx.Context(),
		r.bus,
		command.UpdateFlowCmd{
			FlowID:                 params.FlowID,
			Name:                   params.Name,
			Icon:                   params.Icon,
			Description:            params.Description,
			AdminUserIDs:           params.AdminUserIDs,
			IsAllInitiationAllowed: params.IsAllInitiationAllowed,
			InstanceTitleTemplate:  params.InstanceTitleTemplate,
			Initiators:             initiators,
		},
	)
	if err != nil {
		return err
	}

	return result.Ok(flow).Response(ctx)
}

// ToggleActiveParams contains the parameters for toggling flow active status.
type ToggleActiveParams struct {
	api.P

	FlowID   string `json:"flowId" validate:"required"`
	IsActive bool   `json:"isActive"`
}

// ToggleActive toggles the active status of a flow.
func (r *FlowResource) ToggleActive(ctx fiber.Ctx, params ToggleActiveParams) error {
	if _, err := cqrs.Send[command.ToggleFlowActiveCmd, cqrs.Unit](
		ctx.Context(),
		r.bus,
		command.ToggleFlowActiveCmd{
			FlowID:   params.FlowID,
			IsActive: params.IsActive,
		},
	); err != nil {
		return err
	}

	return result.Ok().Response(ctx)
}

// FindVersionsParams contains the parameters for finding flow versions.
type FindVersionsParams struct {
	api.P

	FlowID   string  `json:"flowId" validate:"required"`
	TenantID *string `json:"tenantId"`
}

// FindVersions queries flow versions for a specific flow.
func (r *FlowResource) FindVersions(ctx fiber.Ctx, params FindVersionsParams) error {
	versions, err := cqrs.Send[query.FindFlowVersionsQuery, []approval.FlowVersion](
		ctx.Context(),
		r.bus,
		query.FindFlowVersionsQuery{
			FlowID:   params.FlowID,
			TenantID: params.TenantID,
		},
	)
	if err != nil {
		return err
	}

	return result.Ok(versions).Response(ctx)
}

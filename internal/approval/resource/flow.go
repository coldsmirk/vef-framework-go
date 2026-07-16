package resource

import (
	"encoding/json"

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

	bus            cqrs.Bus
	tenantResolver approval.PrincipalTenantResolver
}

// NewFlowResource creates a new flow resource.
func NewFlowResource(bus cqrs.Bus, tenantResolver approval.PrincipalTenantResolver) api.Resource {
	return &FlowResource{
		bus:            bus,
		tenantResolver: tenantResolver,
		Resource: api.NewRPCResource(
			"approval/flow",
			api.WithOperations(
				// Flow CRUD writes touch shared definition state and warrant
				// framework-level audit beyond business events.
				api.OperationSpec{Action: "create", RequiredPermission: "approval.flow.create", EnableAudit: true},
				api.OperationSpec{Action: "deploy", RequiredPermission: "approval.flow.deploy", EnableAudit: true},
				api.OperationSpec{Action: "publish_version", RequiredPermission: "approval.flow.publish", EnableAudit: true},
				api.OperationSpec{Action: "update", RequiredPermission: "approval.flow.update", EnableAudit: true},
				api.OperationSpec{Action: "toggle_active", RequiredPermission: "approval.flow.update", EnableAudit: true},
				api.OperationSpec{Action: "get_graph", RequiredPermission: "approval.flow.query"},
				api.OperationSpec{Action: "find_flows", RequiredPermission: "approval.flow.query"},
				api.OperationSpec{Action: "find_versions", RequiredPermission: "approval.flow.query"},
				api.OperationSpec{Action: "find_initiators", RequiredPermission: "approval.flow.query"},
			),
		),
	}
}

// CreateFlowParams contains the parameters for creating a flow.
type CreateFlowParams struct {
	api.P

	TenantID               string                          `json:"tenantId" validate:"required"`
	Code                   string                          `json:"code" validate:"required"`
	Name                   string                          `json:"name" validate:"required"`
	CategoryID             string                          `json:"categoryId" validate:"required"`
	Icon                   *string                         `json:"icon"`
	Description            *string                         `json:"description"`
	Labels                 map[string]string               `json:"labels"`
	BindingMode            approval.BindingMode            `json:"bindingMode" validate:"required"`
	BusinessBinding        *approval.BusinessBindingConfig `json:"businessBinding"`
	AdminUserIDs           []string                        `json:"adminUserIds"`
	IsAllInitiationAllowed bool                            `json:"isAllInitiationAllowed"`
	InstanceTitleTemplate  string                          `json:"instanceTitleTemplate"`
	Initiators             []CreateInitiatorParams         `json:"initiators"`
}

// CreateInitiatorParams contains the parameters for a flow initiator.
type CreateInitiatorParams struct {
	Kind approval.InitiatorKind `json:"kind" validate:"required"`
	IDs  []string               `json:"ids" validate:"required"`
}

// Create creates a new flow.
func (r *FlowResource) Create(ctx fiber.Ctx, principal *security.Principal, params CreateFlowParams) error {
	caller, err := resolveCaller(ctx.Context(), r.tenantResolver, principal)
	if err != nil {
		return err
	}

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
			Labels:                 params.Labels,
			BindingMode:            params.BindingMode,
			BusinessBinding:        params.BusinessBinding,
			AdminUserIDs:           params.AdminUserIDs,
			IsAllInitiationAllowed: params.IsAllInitiationAllowed,
			InstanceTitleTemplate:  params.InstanceTitleTemplate,
			Initiators:             initiators,
			Caller:                 caller,
		},
	)
	if err != nil {
		return err
	}

	return result.Ok(flow).Response(ctx)
}

// DeployFlowParams contains the parameters for deploying a flow definition.
// FormSchema is the host-owned form designer document, passed through opaque
// and optional — flows without forms exist.
type DeployFlowParams struct {
	api.P

	FlowID         string                  `json:"flowId" validate:"required"`
	Description    *string                 `json:"description"`
	StorageMode    approval.StorageMode    `json:"storageMode"`
	FlowDefinition approval.FlowDefinition `json:"flowDefinition" validate:"required"`
	FormSchema     json.RawMessage         `json:"formSchema"`
}

// Deploy deploys a flow definition.
func (r *FlowResource) Deploy(ctx fiber.Ctx, principal *security.Principal, params DeployFlowParams) error {
	caller, err := resolveCaller(ctx.Context(), r.tenantResolver, principal)
	if err != nil {
		return err
	}

	version, err := cqrs.Send[command.DeployFlowCmd, *approval.FlowVersion](
		ctx.Context(),
		r.bus,
		command.DeployFlowCmd{
			FlowID:         params.FlowID,
			Description:    params.Description,
			StorageMode:    params.StorageMode,
			FlowDefinition: params.FlowDefinition,
			FormSchema:     params.FormSchema,
			Caller:         caller,
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
	caller, err := resolveCaller(ctx.Context(), r.tenantResolver, principal)
	if err != nil {
		return err
	}

	if _, err := cqrs.Send[command.PublishVersionCmd, cqrs.Unit](
		ctx.Context(),
		r.bus,
		command.PublishVersionCmd{
			VersionID:  params.VersionID,
			OperatorID: principal.ID,
			Caller:     caller,
		},
	); err != nil {
		return err
	}

	return result.Ok().Response(ctx)
}

// GetGraphParams contains the parameters for getting a flow graph.
type GetGraphParams struct {
	api.P

	FlowID string `json:"flowId" validate:"required"`
	// TenantID is an optional pre-filter (matching the *string shape of the
	// other flow params); the actual cross-tenant gate is Caller.Allows in the
	// query handler, so this only narrows the lookup.
	TenantID *string `json:"tenantId"`
	// VersionID selects an explicit version (a designer resuming from the
	// newest deployment, published or not); omitted resolves the latest
	// published version.
	VersionID *string `json:"versionId"`
}

// GetGraph returns the flow graph: the latest published version by default,
// or an explicit version when versionId is given.
func (r *FlowResource) GetGraph(ctx fiber.Ctx, principal *security.Principal, params GetGraphParams) error {
	caller, err := resolveCaller(ctx.Context(), r.tenantResolver, principal)
	if err != nil {
		return err
	}

	tenantID := ""
	if params.TenantID != nil {
		tenantID = *params.TenantID
	}

	versionID := ""
	if params.VersionID != nil {
		versionID = *params.VersionID
	}

	graph, err := cqrs.Send[query.GetFlowGraphQuery, *shared.FlowGraph](
		ctx.Context(),
		r.bus,
		query.GetFlowGraphQuery{
			FlowID:    params.FlowID,
			TenantID:  tenantID,
			VersionID: versionID,
			Caller:    caller,
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

	TenantID   *string           `json:"tenantId"`
	CategoryID *string           `json:"categoryId"`
	Keyword    *string           `json:"keyword"`
	IsActive   *bool             `json:"isActive"`
	Labels     map[string]string `json:"labels"`
	Page       int               `json:"page"`
	PageSize   int               `json:"pageSize"`
}

// FindFlows queries flows for admin management.
func (r *FlowResource) FindFlows(ctx fiber.Ctx, principal *security.Principal, params FindFlowsParams) error {
	caller, err := resolveCaller(ctx.Context(), r.tenantResolver, principal)
	if err != nil {
		return err
	}

	res, err := cqrs.Send[query.FindFlowsQuery, *page.Page[approval.Flow]](
		ctx.Context(),
		r.bus,
		query.FindFlowsQuery{
			TenantID:   params.TenantID,
			CategoryID: params.CategoryID,
			Keyword:    params.Keyword,
			IsActive:   params.IsActive,
			Labels:     params.Labels,
			Pageable:   page.Pageable{Page: params.Page, Size: params.PageSize},
			Caller:     caller,
		},
	)
	if err != nil {
		return err
	}

	return result.Ok(res).Response(ctx)
}

// UpdateParams contains the parameters for updating a flow.
type UpdateParams struct {
	api.P

	FlowID                 string                          `json:"flowId" validate:"required"`
	Name                   string                          `json:"name" validate:"required"`
	Icon                   *string                         `json:"icon"`
	Description            *string                         `json:"description"`
	Labels                 map[string]string               `json:"labels"`
	BindingMode            approval.BindingMode            `json:"bindingMode" validate:"required"`
	BusinessBinding        *approval.BusinessBindingConfig `json:"businessBinding"`
	AdminUserIDs           []string                        `json:"adminUserIds"`
	IsAllInitiationAllowed bool                            `json:"isAllInitiationAllowed"`
	InstanceTitleTemplate  string                          `json:"instanceTitleTemplate" validate:"required"`
	Initiators             []CreateInitiatorParams         `json:"initiators"`
}

// Update updates an existing flow.
func (r *FlowResource) Update(ctx fiber.Ctx, principal *security.Principal, params UpdateParams) error {
	caller, err := resolveCaller(ctx.Context(), r.tenantResolver, principal)
	if err != nil {
		return err
	}

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
			Labels:                 params.Labels,
			BindingMode:            params.BindingMode,
			BusinessBinding:        params.BusinessBinding,
			AdminUserIDs:           params.AdminUserIDs,
			IsAllInitiationAllowed: params.IsAllInitiationAllowed,
			InstanceTitleTemplate:  params.InstanceTitleTemplate,
			Initiators:             initiators,
			Caller:                 caller,
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
func (r *FlowResource) ToggleActive(ctx fiber.Ctx, principal *security.Principal, params ToggleActiveParams) error {
	caller, err := resolveCaller(ctx.Context(), r.tenantResolver, principal)
	if err != nil {
		return err
	}

	if _, err := cqrs.Send[command.ToggleFlowActiveCmd, cqrs.Unit](
		ctx.Context(),
		r.bus,
		command.ToggleFlowActiveCmd{
			FlowID:   params.FlowID,
			IsActive: params.IsActive,
			Caller:   caller,
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
func (r *FlowResource) FindVersions(ctx fiber.Ctx, principal *security.Principal, params FindVersionsParams) error {
	caller, err := resolveCaller(ctx.Context(), r.tenantResolver, principal)
	if err != nil {
		return err
	}

	versions, err := cqrs.Send[query.FindFlowVersionsQuery, []shared.FlowVersionSummary](
		ctx.Context(),
		r.bus,
		query.FindFlowVersionsQuery{
			FlowID:   params.FlowID,
			TenantID: params.TenantID,
			Caller:   caller,
		},
	)
	if err != nil {
		return err
	}

	return result.Ok(versions).Response(ctx)
}

// FindInitiatorsParams contains the parameters for finding flow initiators.
type FindInitiatorsParams struct {
	api.P

	FlowID   string  `json:"flowId" validate:"required"`
	TenantID *string `json:"tenantId"`
}

// FindInitiators queries the initiator configurations of a specific flow.
func (r *FlowResource) FindInitiators(ctx fiber.Ctx, principal *security.Principal, params FindInitiatorsParams) error {
	caller, err := resolveCaller(ctx.Context(), r.tenantResolver, principal)
	if err != nil {
		return err
	}

	initiators, err := cqrs.Send[query.FindFlowInitiatorsQuery, []approval.FlowInitiator](
		ctx.Context(),
		r.bus,
		query.FindFlowInitiatorsQuery{
			FlowID:   params.FlowID,
			TenantID: params.TenantID,
			Caller:   caller,
		},
	)
	if err != nil {
		return err
	}

	return result.Ok(initiators).Response(ctx)
}

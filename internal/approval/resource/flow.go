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

	bus cqrs.Bus
}

// NewFlowResource creates a new flow resource. Flow definitions are managed
// across tenants: each operation is gated by its permission, and a flow's tenant
// is whatever the request names.
func NewFlowResource(bus cqrs.Bus) api.Resource {
	return &FlowResource{
		bus: bus,
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
				api.OperationSpec{Action: "list_kind_options", RequiredPermission: "approval.flow.query"},
			),
		),
	}
}

// CreateFlowParams contains the parameters for creating a flow.
type CreateFlowParams struct {
	api.P

	// TenantID is the tenant the flow belongs to — approval.SharedTenantID for
	// a flow offered to every tenant.
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
			Labels:                 params.Labels,
			BindingMode:            params.BindingMode,
			BusinessBinding:        params.BusinessBinding,
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
func (r *FlowResource) Deploy(ctx fiber.Ctx, params DeployFlowParams) error {
	version, err := cqrs.Send[command.DeployFlowCmd, *approval.FlowVersion](
		ctx.Context(),
		r.bus,
		command.DeployFlowCmd{
			FlowID:         params.FlowID,
			Description:    params.Description,
			StorageMode:    params.StorageMode,
			FlowDefinition: params.FlowDefinition,
			FormSchema:     params.FormSchema,
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

	FlowID string `json:"flowId" validate:"required"`
	// VersionID selects an explicit version (a designer resuming from the
	// newest deployment, published or not); omitted resolves the latest
	// published version.
	VersionID *string `json:"versionId"`
}

// GetGraph returns the flow graph: the latest published version by default,
// or an explicit version when versionId is given.
func (r *FlowResource) GetGraph(ctx fiber.Ctx, params GetGraphParams) error {
	versionID := ""
	if params.VersionID != nil {
		versionID = *params.VersionID
	}

	graph, err := cqrs.Send[query.GetFlowGraphQuery, *shared.FlowGraph](
		ctx.Context(),
		r.bus,
		query.GetFlowGraphQuery{
			FlowID:    params.FlowID,
			VersionID: versionID,
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

	// TenantID narrows the list to one tenant's flows; omitted, every
	// tenant's are listed.
	TenantID    *string               `json:"tenantId"`
	CategoryID  *string               `json:"categoryId"`
	Keyword     *string               `json:"keyword"`
	IsActive    *bool                 `json:"isActive"`
	Labels      map[string]string     `json:"labels"`
	BindingMode *approval.BindingMode `json:"bindingMode"`
	Page        int                   `json:"page"`
	PageSize    int                   `json:"pageSize"`
}

// FindFlows queries flows for admin management.
func (r *FlowResource) FindFlows(ctx fiber.Ctx, params FindFlowsParams) error {
	res, err := cqrs.Send[query.FindFlowsQuery, *page.Page[approval.Flow]](
		ctx.Context(),
		r.bus,
		query.FindFlowsQuery{
			TenantID:    params.TenantID,
			CategoryID:  params.CategoryID,
			Keyword:     params.Keyword,
			IsActive:    params.IsActive,
			Labels:      params.Labels,
			BindingMode: params.BindingMode,
			Page:        params.Page,
			Size:        params.PageSize,
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
func (r *FlowResource) Update(ctx fiber.Ctx, params UpdateFlowParams) error {
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

	FlowID string `json:"flowId" validate:"required"`
}

// FindVersions queries flow versions for a specific flow.
func (r *FlowResource) FindVersions(ctx fiber.Ctx, params FindVersionsParams) error {
	versions, err := cqrs.Send[query.FindFlowVersionsQuery, []shared.FlowVersionSummary](
		ctx.Context(),
		r.bus,
		query.FindFlowVersionsQuery{
			FlowID: params.FlowID,
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

	FlowID string `json:"flowId" validate:"required"`
}

// FindInitiators queries the initiator configurations of a specific flow.
func (r *FlowResource) FindInitiators(ctx fiber.Ctx, params FindInitiatorsParams) error {
	initiators, err := cqrs.Send[query.FindFlowInitiatorsQuery, []approval.FlowInitiator](
		ctx.Context(),
		r.bus,
		query.FindFlowInitiatorsQuery{
			FlowID: params.FlowID,
		},
	)
	if err != nil {
		return err
	}

	return result.Ok(initiators).Response(ctx)
}

// ListKindOptionsParams takes no input: the catalog is the application's, not
// a flow's — every flow is designed against the same registered vocabularies.
type ListKindOptionsParams struct {
	api.P
}

// ListKindOptions returns the assignee / CC / initiator kinds this application
// accepts, so the flow designer offers exactly what a deploy will validate.
func (r *FlowResource) ListKindOptions(ctx fiber.Ctx, _ ListKindOptionsParams) error {
	options, err := cqrs.Send[query.ListKindOptionsQuery, *approval.KindOptions](
		ctx.Context(),
		r.bus,
		query.ListKindOptionsQuery{},
	)
	if err != nil {
		return err
	}

	return result.Ok(options).Response(ctx)
}

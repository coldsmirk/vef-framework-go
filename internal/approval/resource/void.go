package resource

import (
	"github.com/gofiber/fiber/v3"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/query"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/security"
)

type VoidInstanceParams struct {
	api.P

	InstanceID  string         `json:"instanceId" validate:"required"`
	Reason      string         `json:"reason" validate:"required,max=2000"`
	Attachments []string       `json:"attachments" validate:"max=20"`
	Options     map[string]any `json:"options"`
}

func (r *AdminResource) VoidInstance(ctx fiber.Ctx, db orm.DB, principal *security.Principal, p VoidInstanceParams) error {
	actor, err := resolveActor(ctx.Context(), r.departmentResolver, r.tenantResolver, principal)
	if err != nil {
		return err
	}

	if err := r.svc.VoidInstance(
		ctx.Context(),
		db,
		approval.VoidInstanceInput{InstanceID: p.InstanceID, Operator: actor.Operator, Reason: p.Reason, Attachments: p.Attachments, Options: p.Options, Caller: actor.Caller},
	); err != nil {
		return err
	}

	return result.Ok().Response(ctx)
}

type PlanVoidParams struct {
	api.P

	InstanceID string         `json:"instanceId" validate:"required"`
	Options    map[string]any `json:"options"`
}

func (r *AdminResource) PlanVoid(ctx fiber.Ctx, principal *security.Principal, p PlanVoidParams) error {
	return planVoid(ctx, principal, p, r.bus, r.departmentResolver, r.tenantResolver, approval.VoidDirect)
}

func (r *MyResource) PlanVoid(ctx fiber.Ctx, principal *security.Principal, p PlanVoidParams) error {
	return planVoid(ctx, principal, p, r.bus, r.departmentResolver, r.tenantResolver, approval.VoidApproval)
}

func planVoid(
	ctx fiber.Ctx,
	principal *security.Principal,
	p PlanVoidParams,
	bus cqrs.Bus,
	departments approval.PrincipalDepartmentResolver,
	tenants approval.PrincipalTenantResolver,
	policy approval.VoidPolicy,
) error {
	actor, err := resolveActor(ctx.Context(), departments, tenants, principal)
	if err != nil {
		return err
	}

	preview, err := cqrs.Send[query.PlanVoidQuery, *approval.VoidPreview](
		ctx.Context(),
		bus,
		query.PlanVoidQuery{InstanceID: p.InstanceID, Options: p.Options, Operator: actor.Operator, Caller: actor.Caller, Policy: policy},
	)
	if err != nil {
		return err
	}

	return result.Ok(preview).Response(ctx)
}

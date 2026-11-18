package resource

import (
	"encoding/json"

	"github.com/gofiber/fiber/v3"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/security"
)

type SaveAnnotationParams struct {
	api.P

	ID         string                         `json:"id"`
	TaskID     string                         `json:"taskId"`
	ParentID   string                         `json:"parentId"`
	Anchor     json.RawMessage                `json:"anchor"`
	Content    string                         `json:"content" validate:"max=4000"`
	Audience   *approval.AnnotationAudience   `json:"audience"`
	Resolution *approval.AnnotationResolution `json:"resolution"`
}

func (r *InstanceResource) SaveAnnotation(ctx fiber.Ctx, db orm.DB, principal *security.Principal, params SaveAnnotationParams) error {
	actor, err := resolveActor(ctx.Context(), r.departmentResolver, r.tenantResolver, principal)
	if err != nil {
		return err
	}

	a, err := r.svc.SaveAnnotation(
		ctx.Context(),
		db,
		approval.SaveAnnotationInput{
			ID:         params.ID,
			TaskID:     params.TaskID,
			ParentID:   params.ParentID,
			Anchor:     params.Anchor,
			Content:    params.Content,
			Audience:   params.Audience,
			Resolution: params.Resolution,
			Operator:   actor.Operator,
			Caller:     actor.Caller,
		},
	)
	if err != nil {
		return err
	}

	return result.Ok(a).Response(ctx)
}

type DeleteAnnotationParams struct {
	api.P

	ID string `json:"id" validate:"required"`
}

func (r *InstanceResource) DeleteAnnotation(ctx fiber.Ctx, db orm.DB, principal *security.Principal, params DeleteAnnotationParams) error {
	actor, err := resolveActor(ctx.Context(), r.departmentResolver, r.tenantResolver, principal)
	if err != nil {
		return err
	}

	if err := r.svc.DeleteAnnotation(ctx.Context(), db, approval.DeleteAnnotationInput{ID: params.ID, Operator: actor.Operator, Caller: actor.Caller}); err != nil {
		return err
	}

	return result.Ok().Response(ctx)
}

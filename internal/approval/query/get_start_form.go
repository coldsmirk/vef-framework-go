package query

import (
	"context"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/approval/my"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
)

// GetStartFormQuery loads the published form document of a flow the current
// user wants to initiate. Applicant and Caller are resolved as Start resolves
// them: the applicant is the identity the initiation gate checks and the
// snapshot the form returns, and the caller decides the tenant the instance
// would run in — the one it acts in.
type GetStartFormQuery struct {
	cqrs.BaseQuery

	FlowCode  string
	Applicant approval.UserInfo
	Caller    approval.CallerContext
}

// GetStartFormHandler handles the GetStartFormQuery.
type GetStartFormHandler struct {
	db            orm.DB
	validationSvc *service.ValidationService
}

// NewGetStartFormHandler creates a new GetStartFormHandler.
func NewGetStartFormHandler(db orm.DB, validationSvc *service.ValidationService) *GetStartFormHandler {
	return &GetStartFormHandler{db: db, validationSvc: validationSvc}
}

// Handle mirrors the start-instance gate — the tenant the instance would run
// in, the flow a code names there, active flow, initiation permission,
// published version — so a successful load implies the caller could start
// this flow with the returned form.
func (h *GetStartFormHandler) Handle(ctx context.Context, query GetStartFormQuery) (*my.StartForm, error) {
	db := contextx.DB(ctx, h.db)

	tenantID, err := query.Caller.ResolveWriteTenant("")
	if err != nil {
		return nil, approval.ErrFlowNotFound
	}

	flow, err := service.FindStartFlow(ctx, db, tenantID, query.FlowCode)
	if err != nil {
		return nil, err
	}

	if !flow.IsActive {
		return nil, approval.ErrFlowNotActive
	}

	allowed, err := h.validationSvc.CheckInitiationPermission(ctx, db, flow, tenantID, query.Applicant)
	if err != nil {
		return nil, fmt.Errorf("check initiation permission: %w", err)
	}

	if !allowed {
		return nil, approval.ErrNotAllowedInitiate
	}

	var version approval.FlowVersion
	if err := db.NewSelect().
		Model(&version).
		Select("id", "version", "form_schema").
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("flow_id", flow.ID).
				Equals("status", approval.VersionPublished)
		}).
		Scan(ctx); err != nil {
		if result.IsRecordNotFound(err) {
			return nil, approval.ErrNoPublishedVersion
		}

		return nil, fmt.Errorf("load published version: %w", err)
	}

	return &my.StartForm{
		FlowID:      flow.ID,
		FlowCode:    flow.Code,
		FlowName:    flow.Name,
		FlowIcon:    flow.Icon,
		Description: flow.Description,
		VersionID:   version.ID,
		Version:     version.Version,
		FormSchema:  version.FormSchema,
		Applicant:   query.Applicant,
	}, nil
}

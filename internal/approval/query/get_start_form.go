package query

import (
	"context"
	"fmt"

	"github.com/samber/lo"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/approval/my"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
)

// GetStartFormQuery loads the published form document of a flow the current
// user wants to initiate.
type GetStartFormQuery struct {
	cqrs.BaseQuery

	TenantID              string
	FlowCode              string
	UserID                string
	ApplicantDepartmentID *string
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

// Handle mirrors the start-instance gate — active flow, initiation
// permission, published version — so a successful load implies the caller
// could start this flow with the returned form.
func (h *GetStartFormHandler) Handle(ctx context.Context, query GetStartFormQuery) (*my.StartForm, error) {
	db := contextx.DB(ctx, h.db)

	tenantID := lo.CoalesceOrEmpty(query.TenantID, approval.DefaultTenantID)

	var flow approval.Flow
	if err := db.NewSelect().
		Model(&flow).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("tenant_id", tenantID).
				Equals("code", query.FlowCode)
		}).
		Scan(ctx); err != nil {
		if result.IsRecordNotFound(err) {
			return nil, shared.ErrFlowNotFound
		}

		return nil, fmt.Errorf("load flow: %w", err)
	}

	if !flow.IsActive {
		return nil, shared.ErrFlowNotActive
	}

	if !flow.IsAllInitiationAllowed {
		allowed, err := h.validationSvc.CheckInitiationPermission(ctx, db, &flow, approval.UserInfo{
			ID:           query.UserID,
			DepartmentID: query.ApplicantDepartmentID,
		})
		if err != nil {
			return nil, fmt.Errorf("check initiation permission: %w", err)
		}

		if !allowed {
			return nil, shared.ErrNotAllowedInitiate
		}
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
			return nil, shared.ErrNoPublishedVersion
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
	}, nil
}

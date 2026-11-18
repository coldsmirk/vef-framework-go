package query

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/approval/my"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/orm"
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

	start, err := h.validationSvc.LoadInitiableFlow(ctx, db, tenantID, query.FlowCode, query.Applicant, "id", "version", "form_schema", "business_kind")
	if err != nil {
		return nil, err
	}

	flow, version := start.Flow, start.Version

	return &my.StartForm{
		FlowID:           flow.ID,
		FlowCode:         flow.Code,
		FlowName:         flow.Name,
		FlowIcon:         flow.Icon,
		Description:      flow.Description,
		VersionID:        version.ID,
		Version:          version.Version,
		FormSchema:       version.FormSchema,
		BusinessKind:     version.BusinessKind,
		Applicant:        query.Applicant,
		InitiationWindow: start.Window,
	}, nil
}

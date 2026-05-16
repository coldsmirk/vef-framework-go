package command

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"text/template"

	"github.com/samber/lo"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/approval/behavior"
	"github.com/coldsmirk/vef-framework-go/internal/approval/engine"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
)

// StartInstanceCmd starts a new approval flow instance.
type StartInstanceCmd struct {
	cqrs.BaseCommand

	TenantID         string
	FlowCode         string
	Applicant        approval.OperatorInfo
	BusinessRecordID *string
	FormData         map[string]any
}

// StartInstanceHandler handles the StartInstanceCmd command.
type StartInstanceHandler struct {
	db                  orm.DB
	engine              *engine.FlowEngine
	instanceNoGenerator approval.InstanceNoGenerator
	validationSvc       *service.ValidationService
	bindingHook         approval.BusinessBindingHook
}

// NewStartInstanceHandler creates a new StartInstanceHandler.
func NewStartInstanceHandler(
	db orm.DB,
	engine *engine.FlowEngine,
	instanceNoGenerator approval.InstanceNoGenerator,
	validationSvc *service.ValidationService,
	bindingHook approval.BusinessBindingHook,
) *StartInstanceHandler {
	return &StartInstanceHandler{
		db:                  db,
		engine:              engine,
		instanceNoGenerator: instanceNoGenerator,
		validationSvc:       validationSvc,
		bindingHook:         bindingHook,
	}
}

func (h *StartInstanceHandler) Handle(ctx context.Context, cmd StartInstanceCmd) (*approval.Instance, error) {
	db := contextx.DB(ctx, h.db)

	var (
		tenantID = lo.CoalesceOrEmpty(cmd.TenantID, "default")
		flow     approval.Flow
	)

	if err := db.NewSelect().
		Model(&flow).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("tenant_id", tenantID).
				Equals("code", cmd.FlowCode)
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
		allowed, err := h.validationSvc.CheckInitiationPermission(ctx, db, flow.ID, cmd.Applicant.ID, cmd.Applicant.DepartmentID)
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

	if err := h.validationSvc.ValidateFormData(version.FormSchema, cmd.FormData); err != nil {
		return nil, err
	}

	instanceNo, err := h.instanceNoGenerator.Generate(ctx, cmd.FlowCode)
	if err != nil {
		return nil, fmt.Errorf("generate instance number: %w", err)
	}

	title, err := renderInstanceTitle(
		flow.InstanceTitleTemplate,
		map[string]any{
			"flowName":      flow.Name,
			"flowCode":      flow.Code,
			"instanceNo":    instanceNo,
			"formData":      cmd.FormData,
			"applicantId":   cmd.Applicant.ID,
			"applicantName": cmd.Applicant.Name,
			"flow": map[string]any{
				"name": flow.Name,
				"code": flow.Code,
			},
			"applicant": map[string]any{
				"id":   cmd.Applicant.ID,
				"name": cmd.Applicant.Name,
			},
		},
	)
	if err != nil {
		return nil, fmt.Errorf("render instance title: %w", err)
	}

	instance := &approval.Instance{
		TenantID:                flow.TenantID,
		FlowID:                  flow.ID,
		FlowVersionID:           version.ID,
		Title:                   title,
		InstanceNo:              instanceNo,
		ApplicantID:             cmd.Applicant.ID,
		ApplicantName:           cmd.Applicant.Name,
		ApplicantDepartmentID:   cmd.Applicant.DepartmentID,
		ApplicantDepartmentName: cmd.Applicant.DepartmentName,
		Status:                  approval.InstanceRunning,
		BusinessRecordID:        cmd.BusinessRecordID,
		FormData:                cmd.FormData,
	}

	if _, err := db.NewInsert().
		Model(instance).
		Exec(ctx); err != nil {
		return nil, fmt.Errorf("insert instance: %w", err)
	}

	// Resolve the business binding (if any). The default hook is a no-op;
	// hosts that override it can allocate a business row inside the same
	// transaction. Returning empty string keeps BusinessRecordID nil.
	if flow.BindingMode == approval.BindingBusiness && h.bindingHook != nil {
		businessID, err := h.bindingHook.OnInstanceCreated(ctx, db, &flow, instance)
		if err != nil {
			return nil, fmt.Errorf("business binding on create: %w", err)
		}

		trimmed := strings.TrimSpace(businessID)
		if trimmed != "" && (instance.BusinessRecordID == nil || *instance.BusinessRecordID == "") {
			instance.BusinessRecordID = &trimmed
			if _, err := db.NewUpdate().
				Model(instance).
				Select("business_record_id").
				WherePK().
				Exec(ctx); err != nil {
				return nil, fmt.Errorf("persist business_record_id: %w", err)
			}
		}
	}

	submitLog := cmd.Applicant.NewActionLog(instance.ID, approval.ActionSubmit)
	behavior.ActionLogCollectorFromContext(ctx).Add(submitLog)

	if hooks := h.engine.LifecycleHooks(); hooks != nil {
		if err := hooks.OnInstanceCreated(ctx, db, instance); err != nil {
			return nil, fmt.Errorf("lifecycle hooks on instance created: %w", err)
		}
	}

	if err := h.engine.StartProcess(ctx, db, instance); err != nil {
		return nil, fmt.Errorf("start process: %w", err)
	}

	behavior.CollectorFromContext(ctx).Append(
		approval.NewInstanceCreatedEvent(instance.ID, instance.TenantID, flow.ID, title, cmd.Applicant.ID, cmd.Applicant.Name),
	)

	return instance, nil
}

// renderInstanceTitle renders an instance title from a Go text/template
// string. text/template (unlike Jinja2) cannot execute arbitrary code, so
// the only escape hatch is reading fields out of the data map. The data
// map exposes flow / applicant / formData verbatim; the trust boundary is
// the flow-definition admin, who already sees the same form payload they
// could embed here. Future tightening (allowlisting formData keys) would
// be a host-policy concern rather than a framework concern.
func renderInstanceTitle(titleTemplate string, data map[string]any) (string, error) {
	if titleTemplate == "" {
		flowName, _ := data["flowName"].(string)
		instanceNo, _ := data["instanceNo"].(string)

		return flowName + "-" + instanceNo, nil
	}

	tmpl, err := template.New("title").Parse(titleTemplate)
	if err != nil {
		return "", fmt.Errorf("parse title template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute title template: %w", err)
	}

	return buf.String(), nil
}

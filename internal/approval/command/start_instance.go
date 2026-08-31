package command

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"text/template"
	"unicode/utf8"

	"github.com/samber/lo"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/approval/behavior"
	"github.com/coldsmirk/vef-framework-go/internal/approval/binding"
	"github.com/coldsmirk/vef-framework-go/internal/approval/engine"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/internal/approval/storage"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
)

// StartInstanceCmd starts a new approval flow instance.
type StartInstanceCmd struct {
	cqrs.BaseCommand

	TenantID    string
	FlowCode    string
	Applicant   approval.UserInfo
	BusinessRef *string
	FormData    map[string]any
	// Globals is the host-supplied global-variable snapshot persisted onto the
	// instance (Instance.Globals) and resolved by condition evaluation — field
	// subjects and expression bindings alike. Snapshotting at start keeps
	// routing deterministic across re-evaluation.
	Globals map[string]any
	Caller  approval.CallerContext
}

// StartInstanceHandler handles the StartInstanceCmd command.
type StartInstanceHandler struct {
	db                  orm.DB
	engine              *engine.FlowEngine
	instanceNoGenerator approval.InstanceNoGenerator
	validationSvc       *service.ValidationService
	refProvider         approval.BusinessRefProvider
	bindingProjector    *binding.Projector
	formStorage         *storage.Dispatcher
}

// NewStartInstanceHandler creates a new StartInstanceHandler. formStorage
// projects the submitted form data into the version's physical table when its
// StorageMode is StorageTable; it may be nil in test fixtures that do not
// exercise table-mode storage.
func NewStartInstanceHandler(
	db orm.DB,
	engine *engine.FlowEngine,
	instanceNoGenerator approval.InstanceNoGenerator,
	validationSvc *service.ValidationService,
	refProvider approval.BusinessRefProvider,
	bindingProjector *binding.Projector,
	formStorage *storage.Dispatcher,
) *StartInstanceHandler {
	return &StartInstanceHandler{
		db:                  db,
		engine:              engine,
		instanceNoGenerator: instanceNoGenerator,
		validationSvc:       validationSvc,
		refProvider:         refProvider,
		bindingProjector:    bindingProjector,
		formStorage:         formStorage,
	}
}

func (h *StartInstanceHandler) Handle(ctx context.Context, cmd StartInstanceCmd) (*approval.Instance, error) {
	db := contextx.DB(ctx, h.db)

	var (
		tenantID = lo.CoalesceOrEmpty(cmd.TenantID, approval.DefaultTenantID)
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

	// Tenant guard: cmd.TenantID is client-supplied, so the caller must own
	// or supervise that tenant. Return FlowNotFound (rather than
	// CrossTenantAccess) so a probing caller cannot distinguish "no such
	// flow in your tenant" from "exists but belongs to another tenant".
	if !cmd.Caller.Allows(flow.TenantID) {
		return nil, shared.ErrFlowNotFound
	}

	if !flow.IsActive {
		return nil, shared.ErrFlowNotActive
	}

	if !flow.IsAllInitiationAllowed {
		allowed, err := h.validationSvc.CheckInitiationPermission(ctx, db, &flow, cmd.Applicant)
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

	if err := h.validationSvc.ValidateFormData(version.FormFields, cmd.FormData); err != nil {
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
		FlowCode:                flow.Code,
		FlowVersionID:           version.ID,
		Title:                   title,
		InstanceNo:              instanceNo,
		ApplicantID:             cmd.Applicant.ID,
		ApplicantName:           cmd.Applicant.Name,
		ApplicantDepartmentID:   cmd.Applicant.DepartmentID,
		ApplicantDepartmentName: cmd.Applicant.DepartmentName,
		Status:                  approval.InstanceRunning,
		BusinessRef:             cmd.BusinessRef,
		FormData:                cmd.FormData,
		Globals:                 cmd.Globals,
	}

	if _, err := db.NewInsert().
		Model(instance).
		Exec(ctx); err != nil {
		return nil, fmt.Errorf("insert instance: %w", err)
	}

	// Resolve and claim the version-pinned business binding. The mutable flow
	// may already carry configuration for a later version; existing published
	// versions deliberately keep the snapshot captured at deploy.
	if version.BusinessBinding != nil {
		bindingFlow := flow
		bindingFlow.BindingMode = approval.BindingBusiness
		bindingFlow.BusinessBinding = version.BusinessBinding

		businessRef, err := h.refProvider.OnInstanceCreated(ctx, db, &bindingFlow, instance)
		if err != nil {
			return nil, fmt.Errorf("business binding on create: %w", err)
		}

		trimmed := strings.TrimSpace(businessRef)
		if trimmed != "" && (instance.BusinessRef == nil || strings.TrimSpace(*instance.BusinessRef) == "") {
			instance.BusinessRef = &trimmed
			if _, err := db.NewUpdate().
				Model(instance).
				Select("business_ref").
				WherePK().
				Exec(ctx); err != nil {
				return nil, fmt.Errorf("persist business_ref: %w", err)
			}
		}

		if h.bindingProjector != nil {
			if err := h.bindingProjector.Bind(ctx, db, &bindingFlow, &version, instance); err != nil {
				return nil, fmt.Errorf("bind business projection on start: %w", err)
			}
		}
	}

	// Project the form data into the version's physical table when the version
	// uses StorageTable. The instance row (with form_data) is already persisted,
	// so JSON mode is a no-op; table mode inserts the structured projection row
	// inside this same transaction, so a projection failure rolls the start back.
	if h.formStorage != nil {
		if err := h.formStorage.SyncInstanceProjection(ctx, db, instance); err != nil {
			return nil, fmt.Errorf("sync form projection: %w", err)
		}
	}

	submitLog := cmd.Applicant.NewActionLog(instance.ID, approval.ActionSubmit)
	behavior.ActionLogCollectorFromContext(ctx).Add(submitLog)

	if hooks := h.engine.LifecycleHooks(); hooks != nil {
		if err := hooks.OnInstanceCreated(ctx, db, instance); err != nil {
			return nil, fmt.Errorf("lifecycle hooks on instance created: %w", err)
		}
	}

	// Announced before the engine runs: StartProcess emits every event the
	// traversal produces, and a flow whose start node reaches an end node
	// without stopping completes inside this call — deferring the creation
	// event would put approval.instance.completed ahead of the
	// approval.instance.created of the very instance it completed. The payload
	// is a snapshot of fields the traversal does not touch, so the earlier
	// position changes only the order.
	behavior.EventCollectorFromContext(ctx).Add(
		approval.NewInstanceCreatedEvent(instance),
	)

	if err := h.engine.StartProcess(ctx, db, instance); err != nil {
		return nil, fmt.Errorf("start process: %w", err)
	}

	return instance, nil
}

// instanceTitleMaxRunes bounds the rendered instance title to the width of
// the apv_instance.title column (VARCHAR(256) in every dialect). The title
// template input is admin-controlled but interpolates applicant-controlled
// form fields, so an oversize value is degraded gracefully here rather than
// failing the whole StartInstance INSERT (Postgres rejects, MySQL silently
// truncates). Keep in sync with the column definition in migration scripts.
const instanceTitleMaxRunes = 256

// renderInstanceTitle renders an instance title from a Go text/template
// string. text/template (unlike Jinja2) cannot execute arbitrary code, so
// the only escape hatch is reading fields out of the data map. The data
// map exposes flow / applicant / formData verbatim; the trust boundary is
// the flow-definition admin, who already sees the same form payload they
// could embed here. Future tightening (allowlisting formData keys) would
// be a host-policy concern rather than a framework concern.
//
// The result is rune-aware truncated to instanceTitleMaxRunes so an
// oversize template output (e.g. a template interpolating a multi-KB form
// field) cannot overflow the title column.
func renderInstanceTitle(titleTemplate string, data map[string]any) (string, error) {
	if titleTemplate == "" {
		flowName, _ := data["flowName"].(string)
		instanceNo, _ := data["instanceNo"].(string)

		return truncateRunes(flowName+"-"+instanceNo, instanceTitleMaxRunes), nil
	}

	tmpl, err := template.New("title").Parse(titleTemplate)
	if err != nil {
		return "", fmt.Errorf("parse title template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute title template: %w", err)
	}

	return truncateRunes(buf.String(), instanceTitleMaxRunes), nil
}

// truncateRunes returns s limited to at most maxRunes runes, counting
// characters rather than bytes so multi-byte text (e.g. CJK) is never split
// mid-rune — matching the rune-counting idiom used by form-field length
// validation (service/validation.go).
func truncateRunes(s string, maxRunes int) string {
	if utf8.RuneCountInString(s) <= maxRunes {
		return s
	}

	return string([]rune(s)[:maxRunes])
}

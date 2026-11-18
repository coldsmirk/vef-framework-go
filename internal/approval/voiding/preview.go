package voiding

import (
	"context"
	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
	"time"
)

// CheckInitiation evaluates the same live flow, initiation rules, window and published kind that a new request would use.
func CheckInitiation(ctx context.Context, db orm.DB, validator *service.ValidationService, tenant, code string, user approval.UserInfo) error {
	flow, err := service.FindStartFlow(ctx, db, tenant, code)
	if err != nil {
		return err
	}
	if !flow.IsActive {
		return approval.ErrFlowNotActive
	}
	allowed, err := validator.CheckInitiationPermission(ctx, db, flow, tenant, user)
	if err != nil {
		return err
	}
	if !allowed {
		return approval.ErrNotAllowedInitiate
	}
	if _, err := validator.CheckInitiationWindow(ctx, flow.InitiationWindow, time.Now()); err != nil {
		return err
	}
	var version approval.FlowVersion
	err = db.NewSelect().Model(&version).Select("business_kind").Where(func(cb orm.ConditionBuilder) {
		cb.Equals("flow_id", flow.ID).Equals("status", approval.VersionPublished)
	}).Scan(ctx)
	if result.IsRecordNotFound(err) {
		return approval.ErrNoPublishedVersion
	}
	if err != nil {
		return err
	}
	if version.BusinessKind == nil || *version.BusinessKind != approval.BusinessKindVoidRequest {
		return approval.ErrVoidNotAllowed
	}
	return nil
}

func (c *Core) Preview(ctx context.Context, db orm.DB, in approval.VoidInstanceInput, policy approval.VoidPolicy, validator *service.ValidationService) (*approval.VoidPreview, error) {
	if policy == approval.VoidApproval {
		tenant, err := in.Caller.ResolveWriteTenant("")
		if err != nil {
			return nil, approval.ErrInstanceNotFound
		}
		in.Caller = approval.CallerContext{TenantID: tenant}
	}
	vc, err := c.target(ctx, db, in, false)
	if err != nil {
		return nil, err
	}
	if err := c.eligible(vc, policy, ""); err != nil {
		return nil, err
	}
	rec, err := pending(ctx, db, in.InstanceID, false)
	if err != nil {
		return nil, err
	}
	if rec != nil {
		if policy != approval.VoidApproval || rec.RequestInstanceID == nil {
			return nil, approval.ErrVoidRequestPending
		}
		req := &approval.Instance{ID: *rec.RequestInstanceID}
		if err := db.NewSelect().Model(req).WherePK().Scan(ctx); err != nil {
			return nil, err
		}
		if req.Status != approval.InstanceReturned || req.ApplicantID != in.Operator.ID {
			return nil, approval.ErrVoidRequestPending
		}
	} else if policy == approval.VoidApproval {
		if err := CheckInitiation(ctx, db, validator, vc.Instance.TenantID, *vc.Flow.VoidFlowCode, in.Operator); err != nil {
			return nil, err
		}
	}
	p, err := c.plan(ctx, db, vc)
	if err != nil {
		return nil, err
	}
	return &approval.VoidPreview{Policy: policy, VoidFlowCode: vc.Flow.VoidFlowCode, Plan: p}, nil
}

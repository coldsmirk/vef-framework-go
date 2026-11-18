package voiding

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// CheckInitiation evaluates the same live flow, initiation rules, window and published kind that a new request would use.
func CheckInitiation(ctx context.Context, db orm.DB, validator *service.ValidationService, tenant, code string, user approval.UserInfo) error {
	start, err := validator.LoadInitiableFlow(ctx, db, tenant, code, user, "business_kind")
	if err != nil {
		return err
	}

	if start.Version.BusinessKind == nil || *start.Version.BusinessKind != approval.BusinessKindVoidRequest {
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
		if err := db.NewSelect().Model(req).Select("status", "flow_code", "applicant_id").WherePK().Scan(ctx); err != nil {
			return nil, err
		}

		if req.Status != approval.InstanceReturned || req.ApplicantID != in.Operator.ID {
			return nil, approval.ErrVoidRequestPending
		}

		if req.FlowCode != *vc.Flow.VoidFlowCode {
			return nil, approval.ErrVoidNotAllowed
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

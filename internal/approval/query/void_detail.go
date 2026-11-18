package query

import (
	"context"
	"errors"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/business"
	"github.com/coldsmirk/vef-framework-go/internal/approval/engine"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/internal/approval/voiding"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
)

func loadVoidDetail(ctx context.Context, db orm.DB, inst *approval.Instance) (*approval.VoidRecordView, *approval.VoidRequestView, error) {
	if inst.Status == approval.InstanceVoided {
		var rec approval.VoidRecord

		err := db.NewSelect().Model(&rec).Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", inst.ID).Equals("status", approval.VoidVoided) }).Scan(ctx)
		if err != nil {
			return nil, nil, err
		}

		return &rec, nil, nil
	}

	if !engine.InstanceStateMachine.CanTransition(inst.Status, approval.InstanceVoided) {
		return nil, nil, nil
	}

	var req approval.Instance

	err := db.NewSelect().
		Model(&req).
		Select("id", "instance_no", "applicant_id", "applicant_name", "applicant_department_id", "applicant_department_name", "status", "created_at").
		Join((*approval.VoidRecord)(nil), func(cb orm.ConditionBuilder) {
			cb.EqualsColumn("id", "r.request_instance_id")
		}, "r").
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("r.instance_id", inst.ID).Equals("r.status", approval.VoidRequested)
		}).
		Scan(ctx)
	if result.IsRecordNotFound(err) {
		return nil, nil, nil
	}

	if err != nil {
		return nil, nil, err
	}

	return nil, &approval.VoidRequestView{RequestInstanceID: req.ID, RequestInstanceNo: req.InstanceNo, RequestStatus: req.Status, Applicant: req.Applicant(), CreatedAt: req.CreatedAt}, nil
}

func canOfferVoid(bundle *instanceDetailBundle, registry *business.Registry, pending *approval.VoidRequestView, policy approval.VoidPolicy) bool {
	if pending != nil || bundle.Flow.VoidPolicy != policy || !engine.InstanceStateMachine.CanTransition(bundle.Instance.Status, approval.InstanceVoided) {
		return false
	}

	if bundle.Instance.BusinessKind == nil {
		return true
	}

	_, ok := registry.Voider(*bundle.Instance.BusinessKind)

	return ok
}

func canRequestVoid(ctx context.Context, db orm.DB, validation *service.ValidationService, bundle *instanceDetailBundle, q GetMyInstanceDetailQuery) (bool, error) {
	tenant, err := q.Caller.ResolveWriteTenant("")
	if err != nil {
		if errors.Is(err, approval.ErrCrossTenantAccess) {
			return false, nil
		}

		return false, err
	}

	if tenant != bundle.Instance.TenantID || bundle.Flow.VoidFlowCode == nil {
		return false, nil
	}

	err = voiding.CheckInitiation(ctx, db, validation, tenant, *bundle.Flow.VoidFlowCode, q.Viewer)
	if apiErr, ok := result.AsErr(err); ok {
		switch apiErr.Code {
		case approval.ErrCodeFlowNotFound,
			approval.ErrCodeFlowNotActive,
			approval.ErrCodeNoPublishedVersion,
			approval.ErrCodeNotAllowedInitiate,
			approval.ErrCodeInitiationWindowClosed,
			approval.ErrCodeVoidNotAllowed:
			return false, nil
		}
	}

	return err == nil, err
}

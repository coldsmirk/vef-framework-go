package voiding

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/event"
	"github.com/coldsmirk/vef-framework-go/i18n"
	"github.com/coldsmirk/vef-framework-go/internal/approval/behavior"
	"github.com/coldsmirk/vef-framework-go/internal/approval/business"
	"github.com/coldsmirk/vef-framework-go/internal/approval/engine"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/timex"
)

type Core struct {
	instances *service.InstanceService
	registry  *business.Registry
	bus       event.Bus
}

func NewCore(instances *service.InstanceService, registry *business.Registry, bus event.Bus) *Core {
	return &Core{instances: instances, registry: registry, bus: bus}
}

// target loads the live policy. A writer locks the instance before reading policy or pending records.
func (c *Core) target(ctx context.Context, db orm.DB, in approval.VoidInstanceInput, lock bool) (*approval.VoidContext, error) {
	var inst *approval.Instance
	if lock {
		var err error
		inst, err = c.instances.LoadForUpdate(ctx, db, in.InstanceID, in.Caller)
		if err != nil {
			return nil, err
		}
	} else {
		inst = &approval.Instance{ID: in.InstanceID}
		if err := db.NewSelect().Model(inst).WherePK().Scan(ctx); err != nil {
			if result.IsRecordNotFound(err) {
				return nil, approval.ErrInstanceNotFound
			}
			return nil, err
		}
		if !in.Caller.Allows(inst.TenantID) {
			return nil, approval.ErrInstanceNotFound
		}
	}
	flow := &approval.Flow{ID: inst.FlowID}
	q := db.NewSelect().Model(flow).WherePK()
	if lock {
		q = q.ForShare()
	}
	if err := q.Scan(ctx); err != nil {
		return nil, err
	}
	return &approval.VoidContext{Instance: inst, Flow: flow, Operator: in.Operator, Reason: in.Reason, Options: in.Options}, nil
}

func (c *Core) voider(inst *approval.Instance) (approval.BusinessVoider, error) {
	if inst.BusinessKind == nil {
		return nil, nil
	}
	if v, ok := c.registry.Voider(*inst.BusinessKind); ok {
		return v, nil
	}
	return nil, approval.ErrVoidNotAllowed
}

func (c *Core) eligible(vc *approval.VoidContext, policy approval.VoidPolicy, code string) error {
	if !engine.InstanceStateMachine.CanTransition(vc.Instance.Status, approval.InstanceVoided) {
		return approval.ErrInstanceNotVoidable
	}
	if vc.Flow.VoidPolicy != policy {
		return approval.ErrVoidNotAllowed
	}
	if policy == approval.VoidApproval && (vc.Flow.VoidFlowCode == nil || (code != "" && *vc.Flow.VoidFlowCode != code)) {
		return approval.ErrVoidNotAllowed
	}
	_, err := c.voider(vc.Instance)
	return err
}

// pending uses a current read for writers, including under MySQL REPEATABLE READ.
func pending(ctx context.Context, db orm.DB, instanceID string, lock bool) (*approval.VoidRecord, error) {
	var record approval.VoidRecord
	q := db.NewSelect().Model(&record).Where(func(cb orm.ConditionBuilder) {
		cb.Equals("instance_id", instanceID).Equals("status", approval.VoidRequested)
	})
	if lock {
		q = q.ForUpdate()
	}
	if err := q.Scan(ctx); err != nil {
		if result.IsRecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &record, nil
}

func (c *Core) plan(ctx context.Context, db orm.DB, vc *approval.VoidContext) (*approval.VoidPlan, error) {
	v, err := c.voider(vc.Instance)
	if err != nil {
		return nil, err
	}
	if v == nil {
		return &approval.VoidPlan{}, nil
	}
	p, err := v.PlanVoid(ctx, db, vc)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, errMissingPlan
	}
	return p, nil
}

func (c *Core) executablePlan(ctx context.Context, db orm.DB, vc *approval.VoidContext) (*approval.VoidPlan, error) {
	p, err := c.plan(ctx, db, vc)
	if err != nil {
		return nil, err
	}
	if p.IsBlocked() {
		return nil, result.Err(i18n.T(approval.ErrMessageVoidBlocked, map[string]any{"blockers": strings.Join(p.Blockers, ", ")}), result.WithCode(approval.ErrCodeVoidBlocked))
	}
	return p, nil
}

func validateReason(reason string, attachments []string) error {
	if strings.TrimSpace(reason) == "" || utf8.RuneCountInString(reason) > 2000 || len(attachments) > 20 {
		return errInvalidVoidInput
	}
	return nil
}

func (c *Core) Direct(ctx context.Context, db orm.DB, in approval.VoidInstanceInput) error {
	vc, err := c.target(ctx, db, in, true)
	if err != nil {
		return err
	}
	if err := c.eligible(vc, approval.VoidDirect, ""); err != nil {
		return err
	}
	current, err := pending(ctx, db, in.InstanceID, true)
	if err != nil {
		return err
	}
	if current != nil {
		return approval.ErrVoidRequestPending
	}
	if err := validateReason(in.Reason, in.Attachments); err != nil {
		return err
	}
	record := &approval.VoidRecord{TenantID: vc.Instance.TenantID, InstanceID: in.InstanceID, Operator: in.Operator, Reason: in.Reason, Options: in.Options, Attachments: in.Attachments}
	return c.execute(ctx, db, vc, record)
}

// execute reverses business effects and records their outcome under the target instance lock.
func (c *Core) execute(ctx context.Context, db orm.DB, vc *approval.VoidContext, record *approval.VoidRecord) error {
	p, err := c.executablePlan(ctx, db, vc)
	if err != nil {
		return err
	}
	if vc.Instance.BusinessKind != nil {
		v, err := c.voider(vc.Instance)
		if err != nil {
			return err
		}
		if err := v.Void(ctx, db, vc); err != nil {
			return err
		}
	}
	if err := c.instances.Transition(ctx, db, vc.Instance, approval.InstanceVoided); err != nil {
		return err
	}
	record.Status = approval.VoidVoided
	record.Reversals = append([]string{}, p.Reversals...)
	record.FollowUps = append([]string{}, p.FollowUps...)
	record.VoidedAt = new(timex.Now())
	if record.RequestInstanceID == nil {
		_, err = db.NewInsert().Model(record).Exec(ctx)
	} else {
		_, err = db.NewUpdate().Model(record).Select("status", "reversals", "follow_ups", "voided_at").WherePK().Exec(ctx)
	}
	if err != nil {
		return err
	}
	log := record.Operator.NewActionLog(record.InstanceID, approval.ActionVoid)
	log.Opinion = &record.Reason
	log.Attachments = record.Attachments
	if collector, ok := behavior.TryActionLogCollectorFromContext(ctx); ok {
		collector.Add(log)
	} else if _, err := db.NewInsert().Model(log).Exec(ctx); err != nil {
		return err
	}
	return behavior.EmitEvents(ctx, c.bus, db, &approval.InstanceVoidedEvent{InstanceEventBase: approval.NewInstanceEventBase(vc.Instance), Operator: record.Operator, Reason: record.Reason, RequestInstanceID: record.RequestInstanceID, Reversals: record.Reversals, FollowUps: record.FollowUps})
}

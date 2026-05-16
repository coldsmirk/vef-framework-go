package command

import (
	"context"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/event"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
)

// ToggleFlowActiveCmd toggles the active status of a flow.
type ToggleFlowActiveCmd struct {
	cqrs.BaseCommand

	FlowID   string
	IsActive bool
}

// ToggleFlowActiveHandler handles the ToggleFlowActiveCmd command.
type ToggleFlowActiveHandler struct {
	db  orm.DB
	bus event.Bus
}

// NewToggleFlowActiveHandler creates a new ToggleFlowActiveHandler.
func NewToggleFlowActiveHandler(db orm.DB, bus event.Bus) *ToggleFlowActiveHandler {
	return &ToggleFlowActiveHandler{db: db, bus: bus}
}

func (h *ToggleFlowActiveHandler) Handle(ctx context.Context, cmd ToggleFlowActiveCmd) (cqrs.Unit, error) {
	db := contextx.DB(ctx, h.db)

	var flow approval.Flow

	flow.ID = cmd.FlowID
	if err := db.NewSelect().Model(&flow).Select("tenant_id").WherePK().Scan(ctx); err != nil {
		if result.IsRecordNotFound(err) {
			return cqrs.Unit{}, shared.ErrFlowNotFound
		}

		return cqrs.Unit{}, fmt.Errorf("load flow tenant: %w", err)
	}

	updateResult, err := db.NewUpdate().
		Model((*approval.Flow)(nil)).
		Set("is_active", cmd.IsActive).
		Where(func(cb orm.ConditionBuilder) {
			cb.PKEquals(cmd.FlowID)
		}).
		Exec(ctx)
	if err != nil {
		return cqrs.Unit{}, fmt.Errorf("update flow active status: %w", err)
	}

	affected, err := updateResult.RowsAffected()
	if err != nil {
		return cqrs.Unit{}, fmt.Errorf("get affected rows: %w", err)
	}

	if affected == 0 {
		return cqrs.Unit{}, shared.ErrFlowNotFound
	}

	if err := h.bus.PublishBatch(ctx, event.AsEvents([]approval.DomainEvent{
		approval.NewFlowToggledEvent(cmd.FlowID, flow.TenantID, cmd.IsActive),
	}), event.WithTx(db)); err != nil {
		return cqrs.Unit{}, fmt.Errorf("publish flow toggled event: %w", err)
	}

	return cqrs.Unit{}, nil
}

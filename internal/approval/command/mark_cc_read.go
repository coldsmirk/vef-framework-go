package command

import (
	"context"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// MarkCCReadCmd marks CC records as read for a user.
type MarkCCReadCmd struct {
	cqrs.BaseCommand

	InstanceID string
	UserID     string
	Caller     approval.CallerContext
}

// MarkCCReadHandler handles the MarkCCReadCmd command.
type MarkCCReadHandler struct {
	db      orm.DB
	nodeSvc *service.NodeService
}

// NewMarkCCReadHandler creates a new MarkCCReadHandler.
func NewMarkCCReadHandler(db orm.DB, nodeSvc *service.NodeService) *MarkCCReadHandler {
	return &MarkCCReadHandler{db: db, nodeSvc: nodeSvc}
}

func (h *MarkCCReadHandler) Handle(ctx context.Context, cmd MarkCCReadCmd) (cqrs.Unit, error) {
	db := contextx.DB(ctx, h.db)

	// Tenant guard: the request carries an instance id from the client, so
	// load the instance's tenant_id first and verify the caller can act on
	// it. Cross-tenant probes get a zero-records response — same shape as
	// "no unread CC records" so the existence of the instance is opaque.
	var instance approval.Instance

	instance.ID = cmd.InstanceID
	if err := db.NewSelect().Model(&instance).Select("tenant_id").WherePK().Scan(ctx); err != nil {
		// Treat "not found" the same as "no unread records" to avoid leaking
		// existence. Non-not-found errors still propagate.
		return cqrs.Unit{}, nil //nolint:nilerr // tenant isolation requires opaque response
	}

	if !cmd.Caller.Allows(instance.TenantID) {
		return cqrs.Unit{}, nil
	}

	var records []approval.CCRecord

	if err := db.NewSelect().
		Model(&records).
		Select("id", "node_id").
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("instance_id", cmd.InstanceID).
				Equals("cc_user_id", cmd.UserID).
				IsNull("read_at")
		}).
		Scan(ctx); err != nil {
		return cqrs.Unit{}, fmt.Errorf("query unread cc records: %w", err)
	}

	if len(records) == 0 {
		return cqrs.Unit{}, nil
	}

	now := timex.Now()

	recordIDs := make([]string, 0, len(records))
	for _, record := range records {
		recordIDs = append(recordIDs, record.ID)
	}

	if _, err := db.NewUpdate().
		Model((*approval.CCRecord)(nil)).
		Set("read_at", now).
		Where(func(cb orm.ConditionBuilder) { cb.In("id", recordIDs) }).
		Exec(ctx); err != nil {
		return cqrs.Unit{}, fmt.Errorf("update cc records read_at: %w", err)
	}

	if err := h.nodeSvc.CheckCCNodeCompletion(ctx, db, cmd.InstanceID, records); err != nil {
		return cqrs.Unit{}, err
	}

	return cqrs.Unit{}, nil
}

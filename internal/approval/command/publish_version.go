package command

import (
	"context"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/approval/behavior"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// PublishVersionCmd publishes a flow version.
type PublishVersionCmd struct {
	cqrs.BaseCommand

	VersionID  string
	OperatorID string
}

// PublishVersionHandler handles the PublishVersionCmd command.
type PublishVersionHandler struct {
	db orm.DB
}

// NewPublishVersionHandler creates a new PublishVersionHandler.
func NewPublishVersionHandler(db orm.DB) *PublishVersionHandler {
	return &PublishVersionHandler{db: db}
}

func (h *PublishVersionHandler) Handle(ctx context.Context, cmd PublishVersionCmd) (cqrs.Unit, error) {
	db := contextx.DB(ctx, h.db)

	var version approval.FlowVersion

	version.ID = cmd.VersionID
	if err := db.NewSelect().
		Model(&version).
		WherePK().
		ForUpdate().
		Scan(ctx); err != nil {
		if result.IsRecordNotFound(err) {
			return cqrs.Unit{}, shared.ErrVersionNotFound
		}

		return cqrs.Unit{}, fmt.Errorf("load flow version: %w", err)
	}

	if version.Status != approval.VersionDraft {
		return cqrs.Unit{}, shared.ErrVersionNotDraft
	}

	// Archive old published versions
	if _, err := db.NewUpdate().
		Model((*approval.FlowVersion)(nil)).
		Set("status", approval.VersionArchived).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("flow_id", version.FlowID).
				Equals("status", approval.VersionPublished)
		}).
		Exec(ctx); err != nil {
		return cqrs.Unit{}, fmt.Errorf("archive old versions: %w", err)
	}

	// Publish this version
	now := timex.Now()
	version.Status = approval.VersionPublished
	version.PublishedAt = &now
	version.PublishedBy = &cmd.OperatorID

	if _, err := db.NewUpdate().
		Model(&version).
		Select("status", "published_at", "published_by").
		WherePK().
		Exec(ctx); err != nil {
		return cqrs.Unit{}, fmt.Errorf("publish version: %w", err)
	}

	// Update flow's current version number
	if _, err := db.NewUpdate().
		Model((*approval.Flow)(nil)).
		Set("current_version", version.Version).
		Where(func(cb orm.ConditionBuilder) {
			cb.PKEquals(version.FlowID)
		}).
		Exec(ctx); err != nil {
		return cqrs.Unit{}, fmt.Errorf("update flow current version: %w", err)
	}

	var flow approval.Flow

	flow.ID = version.FlowID
	if err := db.NewSelect().Model(&flow).Select("tenant_id").WherePK().Scan(ctx); err != nil {
		return cqrs.Unit{}, fmt.Errorf("load flow tenant: %w", err)
	}

	behavior.CollectorFromContext(ctx).Append(
		approval.NewFlowPublishedEvent(version.FlowID, flow.TenantID, cmd.VersionID),
	)

	return cqrs.Unit{}, nil
}

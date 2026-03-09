package command

import (
	"context"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
)

// UpdateFlowCmd updates an existing flow.
type UpdateFlowCmd struct {
	cqrs.BaseCommand

	FlowID                 string
	Name                   string
	Icon                   *string
	Description            *string
	AdminUserIDs           []string
	IsAllInitiationAllowed bool
	InstanceTitleTemplate  string
	Initiators             []shared.CreateFlowInitiatorCmd
}

// UpdateFlowHandler handles the UpdateFlowCmd command.
type UpdateFlowHandler struct {
	db orm.DB
}

// NewUpdateFlowHandler creates a new UpdateFlowHandler.
func NewUpdateFlowHandler(db orm.DB) *UpdateFlowHandler {
	return &UpdateFlowHandler{db: db}
}

func (h *UpdateFlowHandler) Handle(ctx context.Context, cmd UpdateFlowCmd) (*approval.Flow, error) {
	db := contextx.DB(ctx, h.db)

	var flow approval.Flow

	flow.ID = cmd.FlowID

	if err := db.NewSelect().
		Model(&flow).
		WherePK().
		Scan(ctx); err != nil {
		if result.IsRecordNotFound(err) {
			return nil, shared.ErrFlowNotFound
		}

		return nil, fmt.Errorf("query flow: %w", err)
	}

	flow.Name = cmd.Name
	flow.Icon = cmd.Icon
	flow.Description = cmd.Description
	flow.AdminUserIDs = cmd.AdminUserIDs
	flow.IsAllInitiationAllowed = cmd.IsAllInitiationAllowed
	flow.InstanceTitleTemplate = cmd.InstanceTitleTemplate

	if _, err := db.NewUpdate().
		Model(&flow).
		Select("name", "icon", "description", "admin_user_ids", "is_all_initiation_allowed", "instance_title_template").
		WherePK().
		Exec(ctx); err != nil {
		return nil, fmt.Errorf("update flow: %w", err)
	}

	if _, err := db.NewDelete().
		Model((*approval.FlowInitiator)(nil)).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("flow_id", cmd.FlowID)
		}).
		Exec(ctx); err != nil {
		return nil, fmt.Errorf("delete existing initiators: %w", err)
	}

	for _, init := range cmd.Initiators {
		initiator := approval.FlowInitiator{
			FlowID: cmd.FlowID,
			Kind:   init.Kind,
			IDs:    init.IDs,
		}
		if _, err := db.NewInsert().
			Model(&initiator).
			Exec(ctx); err != nil {
			return nil, fmt.Errorf("insert flow initiator: %w", err)
		}
	}

	return &flow, nil
}

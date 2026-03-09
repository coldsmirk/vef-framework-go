package engine

import (
	"context"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// CCProcessor handles CC (carbon copy) notification nodes.
type CCProcessor struct{}

// NewCCProcessor creates a CCProcessor.
func NewCCProcessor() *CCProcessor { return &CCProcessor{} }

func (*CCProcessor) NodeKind() approval.NodeKind { return approval.NodeCC }

func (p *CCProcessor) Process(ctx context.Context, pc *ProcessContext) (*ProcessResult, error) {
	ccUserIDs, ccUserNames, err := p.createCCRecords(ctx, pc)
	if err != nil {
		return nil, err
	}

	var events []approval.DomainEvent
	if len(ccUserIDs) > 0 {
		events = []approval.DomainEvent{
			approval.NewCCNotifiedEvent(pc.Instance.ID, pc.Node.ID, ccUserIDs, ccUserNames, false),
		}
	}

	if pc.Node.IsReadConfirmRequired {
		return &ProcessResult{Action: NodeActionWait, Events: events}, nil
	}

	return &ProcessResult{Action: NodeActionContinue, Events: events}, nil
}

// createCCRecords loads FlowNodeCC configurations and creates CC records for all CC users.
// Returns the list of CC user IDs and their names for event publishing.
func (*CCProcessor) createCCRecords(ctx context.Context, pc *ProcessContext) ([]string, map[string]string, error) {
	var ccConfigs []approval.FlowNodeCC

	if err := pc.DB.NewSelect().
		Model(&ccConfigs).
		Select("kind", "ids", "form_field").
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("node_id", pc.Node.ID)
		}).
		Scan(ctx); err != nil {
		return nil, nil, fmt.Errorf("load cc configs: %w", err)
	}

	resolved, err := shared.CollectUniqueCCUserIDs(ccConfigs, pc.FormData, ResolveCCUserIDs, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve cc users: %w", err)
	}

	if len(resolved) == 0 {
		return nil, nil, nil
	}

	ccUserNames, err := shared.ResolveUserNameMap(ctx, pc.UserResolver, resolved)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve cc user names: %w", err)
	}

	insertedUserIDs, err := shared.InsertAutoCCRecords(ctx, pc.DB, pc.Instance.ID, pc.Node.ID, resolved, ccUserNames)
	if err != nil {
		return nil, nil, fmt.Errorf("insert cc records: %w", err)
	}

	return insertedUserIDs, ccUserNames, nil
}

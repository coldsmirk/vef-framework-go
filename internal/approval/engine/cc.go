package engine

import (
	"context"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// CCProcessor handles CC (carbon copy) notification nodes.
type CCProcessor struct {
	ccResolver *shared.CCRecipientResolver
}

// NewCCProcessor creates a CCProcessor.
func NewCCProcessor(ccResolver *shared.CCRecipientResolver) *CCProcessor {
	return &CCProcessor{ccResolver: ccResolver}
}

func (*CCProcessor) NodeKind() approval.NodeKind { return approval.NodeCC }

func (p *CCProcessor) Process(ctx context.Context, pc *ProcessContext) (*ProcessResult, error) {
	recipients, err := p.createCCRecords(ctx, pc)
	if err != nil {
		return nil, err
	}

	var events []approval.DomainEvent
	if len(recipients) > 0 {
		events = []approval.DomainEvent{
			approval.NewCCNotifiedEvent(pc.Instance, pc.Node, recipients, false),
		}
	}

	if !pc.Node.IsReadConfirmRequired {
		return &ProcessResult{Action: NodeActionContinue, Events: events}, nil
	}

	// A read-confirm CC node may only wait when a record actually awaits
	// confirmation. Consult the same source of truth the mark-read path uses
	// (NodeService.AdvanceCCNodeIfAllRead) so entry and exit cannot disagree: if
	// the node resolved to zero recipients — no configs, or configs that yield
	// nobody such as a role/department CC skipped best-effort with no
	// AssigneeService — there is no record to confirm and nobody to ever drive
	// AdvanceCCNodeIfAllRead, so the node must continue rather than wait forever.
	hasUnread, err := shared.HasUnreadCCRecords(ctx, pc.DB, pc.Instance.ID, pc.Node.ID, pc.Visit.ID)
	if err != nil {
		return nil, err
	}

	if hasUnread {
		return &ProcessResult{Action: NodeActionWait, Events: events}, nil
	}

	return &ProcessResult{Action: NodeActionContinue, Events: events}, nil
}

// createCCRecords loads FlowNodeCC configurations and creates CC records for all CC users.
// Returns the notified recipients for event publishing.
func (p *CCProcessor) createCCRecords(ctx context.Context, pc *ProcessContext) ([]approval.UserInfo, error) {
	var ccConfigs []approval.FlowNodeCC

	if err := pc.DB.NewSelect().
		Model(&ccConfigs).
		Select("kind", "ids", "form_field").
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("node_id", pc.Node.ID)
		}).
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("load cc configs: %w", err)
	}

	// CC resolution is best-effort (unresolvable configs are logged and skipped);
	// it never fails the approval that triggered the CC node.
	resolved := shared.CollectUniqueCCUserIDs(ctx, ccConfigs, pc.FormData, p.ccResolver.Resolve, nil)

	if len(resolved) == 0 {
		return nil, nil
	}

	// Display-info lookup is likewise best-effort: a resolution failure must
	// not roll back the approval (matches the timing-based CC path in
	// NodeService.TriggerNodeCC).
	ccUserInfos := shared.ResolveUserInfoMapSilent(ctx, pc.UserResolver, resolved)

	insertedUserIDs, err := shared.InsertAutoCCRecords(ctx, pc.DB, pc.Instance.ID, pc.Node.ID, pc.Visit.ID, resolved, ccUserInfos)
	if err != nil {
		return nil, fmt.Errorf("insert cc records: %w", err)
	}

	return shared.UserInfos(insertedUserIDs, ccUserInfos), nil
}

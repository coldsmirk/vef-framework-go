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
	ccUserIDs, ccUserNames, err := p.createCCRecords(ctx, pc)
	if err != nil {
		return nil, err
	}

	var events []approval.DomainEvent
	if len(ccUserIDs) > 0 {
		events = []approval.DomainEvent{
			approval.NewCCNotifiedEvent(pc.Instance.ID, pc.Instance.TenantID, pc.Node.ID, ccUserIDs, ccUserNames, false),
		}
	}

	if !pc.Node.IsReadConfirmRequired {
		return &ProcessResult{Action: NodeActionContinue, Events: events}, nil
	}

	// A read-confirm CC node may only wait when a record actually awaits
	// confirmation. Consult the same source of truth the mark-read path uses
	// (NodeService.CheckCCNodeCompletion) so entry and exit cannot disagree: if
	// the node resolved to zero recipients — no configs, or configs that yield
	// nobody such as a role/department CC skipped best-effort with no
	// AssigneeService — there is no record to confirm and nobody to ever drive
	// CheckCCNodeCompletion, so the node must continue rather than wait forever.
	hasUnread, err := shared.HasUnreadCCRecords(ctx, pc.DB, pc.Instance.ID, pc.Node.ID)
	if err != nil {
		return nil, err
	}

	if hasUnread {
		return &ProcessResult{Action: NodeActionWait, Events: events}, nil
	}

	return &ProcessResult{Action: NodeActionContinue, Events: events}, nil
}

// createCCRecords loads FlowNodeCC configurations and creates CC records for all CC users.
// Returns the list of CC user IDs and their names for event publishing.
func (p *CCProcessor) createCCRecords(ctx context.Context, pc *ProcessContext) ([]string, map[string]string, error) {
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

	// CC resolution is best-effort (unresolvable configs are logged and skipped);
	// it never fails the approval that triggered the CC node.
	resolved := shared.CollectUniqueCCUserIDs(ctx, ccConfigs, pc.FormData, p.ccResolver.Resolve, nil)

	if len(resolved) == 0 {
		return nil, nil, nil
	}

	// Display-name lookup is likewise best-effort: a name-resolution failure
	// must not roll back the approval (matches the timing-based CC path in
	// NodeService.TriggerNodeCC).
	ccUserNames := shared.ResolveUserNameMapSilent(ctx, pc.UserResolver, resolved)

	insertedUserIDs, err := shared.InsertAutoCCRecords(ctx, pc.DB, pc.Instance.ID, pc.Node.ID, resolved, ccUserNames)
	if err != nil {
		return nil, nil, fmt.Errorf("insert cc records: %w", err)
	}

	return insertedUserIDs, ccUserNames, nil
}

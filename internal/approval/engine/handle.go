package engine

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/approval"
)

// HandleProcessor handles handle nodes (claim mode).
// Unlike approval nodes, handle nodes create all tasks with sortOrder=0,
// allowing any candidate to claim and complete the task.
type HandleProcessor struct {
	assigneeService approval.AssigneeService
}

// NewHandleProcessor creates a new handle processor.
func NewHandleProcessor(assigneeService approval.AssigneeService) *HandleProcessor {
	return &HandleProcessor{assigneeService: assigneeService}
}

func (*HandleProcessor) NodeKind() approval.NodeKind { return approval.NodeHandle }

func (p *HandleProcessor) Process(ctx context.Context, pc *ProcessContext) (*ProcessResult, error) {
	if err := saveFormSnapshot(ctx, pc); err != nil {
		return nil, err
	}

	assignees, err := p.resolveAndDeduplicateAssignees(ctx, pc)
	if err != nil {
		return nil, err
	}

	if len(assignees) == 0 {
		return handleEmptyAssignee(ctx, pc, p.assigneeService)
	}

	assignees, err = applyDelegation(ctx, pc.DB, pc.Instance.FlowID, assignees, pc.UserResolver)
	if err != nil {
		return nil, err
	}

	if err := createTasksWithDelegation(ctx, pc, assignees); err != nil {
		return nil, err
	}

	return &ProcessResult{Action: NodeActionWait}, nil
}

func (*HandleProcessor) resolveAndDeduplicateAssignees(ctx context.Context, pc *ProcessContext) ([]approval.ResolvedAssignee, error) {
	assignees, err := resolveAssignees(ctx, pc)
	if err != nil {
		return nil, err
	}

	return deduplicateAssignees(assignees), nil
}

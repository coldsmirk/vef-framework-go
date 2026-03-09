package engine

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/approval"
)

// EndProcessor handles end nodes by completing the flow as approved.
type EndProcessor struct{}

// NewEndProcessor creates an EndProcessor.
func NewEndProcessor() NodeProcessor { return &EndProcessor{} }

func (*EndProcessor) NodeKind() approval.NodeKind { return approval.NodeEnd }

func (*EndProcessor) Process(context.Context, *ProcessContext) (*ProcessResult, error) {
	return &ProcessResult{
		Action:      NodeActionComplete,
		FinalStatus: new(approval.InstanceApproved),
	}, nil
}

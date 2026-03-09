package engine

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/approval"
)

// StartProcessor handles start nodes by auto-advancing to the next node.
type StartProcessor struct{}

// NewStartProcessor creates a StartProcessor.
func NewStartProcessor() NodeProcessor { return &StartProcessor{} }

func (*StartProcessor) NodeKind() approval.NodeKind { return approval.NodeStart }

func (*StartProcessor) Process(context.Context, *ProcessContext) (*ProcessResult, error) {
	return &ProcessResult{Action: NodeActionContinue}, nil
}

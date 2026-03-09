package engine

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/strategy"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// NodeAction represents the result action of node processing.
type NodeAction int

const (
	NodeActionWait     NodeAction = iota // Wait for user action
	NodeActionContinue                   // Auto-advance to next node
	NodeActionComplete                   // Flow ends
)

// ProcessResult contains the outcome of node processing.
type ProcessResult struct {
	Action      NodeAction
	FinalStatus *approval.InstanceStatus // Only set when Action == NodeActionComplete
	BranchID    *string                  // Only set when Action == NodeActionContinue (condition node)
	Events      []approval.DomainEvent   // Events to publish after processing
}

// ProcessContext provides context for node processing.
type ProcessContext struct {
	DB            orm.DB
	Instance      *approval.Instance
	Node          *approval.FlowNode
	FormData      approval.FormData
	ApplicantID   string
	ApplicantName string
	UserResolver  approval.UserInfoResolver
	Registry      *strategy.StrategyRegistry
}

// NodeProcessor processes a specific node kind.
type NodeProcessor interface {
	// NodeKind returns the node kind this processor handles (approval, handle, cc, condition, etc.).
	NodeKind() approval.NodeKind
	// Process executes the node logic and returns the action to take (wait, continue, or complete).
	Process(ctx context.Context, pc *ProcessContext) (*ProcessResult, error)
}

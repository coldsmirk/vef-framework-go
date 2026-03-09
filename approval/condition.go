package approval

import "context"

// Condition represents a branch condition evaluated by condition nodes.
type Condition struct {
	Kind       ConditionKind `json:"kind"`
	Subject    string        `json:"subject"`
	Operator   string        `json:"operator"`
	Value      any           `json:"value"`
	Expression string        `json:"expression"`
}

// ConditionGroup represents a group of conditions evaluated with AND logic.
// Multiple groups in a branch are evaluated with OR logic.
type ConditionGroup struct {
	Conditions []Condition `json:"conditions"`
}

// ConditionBranch represents a branch in a condition node.
// Each branch has its own condition groups and can be linked to an edge via its ID.
type ConditionBranch struct {
	ID              string           `json:"id"`
	Label           string           `json:"label"`
	ConditionGroups []ConditionGroup `json:"conditionGroups,omitempty"`
	IsDefault       bool             `json:"isDefault,omitempty"`
	Priority        int              `json:"priority"`
}

// EvaluationContext provides context for condition evaluation.
type EvaluationContext struct {
	FormData        FormData
	ApplicantID     string
	ApplicantDeptID *string
}

// ConditionEvaluator evaluates branch conditions.
type ConditionEvaluator interface {
	// Kind returns the condition kind this evaluator handles.
	Kind() ConditionKind
	// Evaluate evaluates a single condition against the given evaluation context.
	Evaluate(ctx context.Context, cond Condition, ec *EvaluationContext) (bool, error)
}

// PassRuleResult indicates the outcome of pass rule evaluation.
type PassRuleResult int

const (
	PassRulePending  PassRuleResult = iota // Still waiting for more actions
	PassRulePassed                         // Node passed
	PassRuleRejected                       // Node rejected
)

// PassRuleContext provides context for pass rule evaluation.
type PassRuleContext struct {
	ApprovedCount int
	RejectedCount int
	TotalCount    int
	PassRatio     float64
}

// PassRuleStrategy evaluates whether a node passes based on task results.
type PassRuleStrategy interface {
	// Rule returns the pass rule this strategy handles.
	Rule() PassRule
	// Evaluate determines the pass/reject/pending outcome based on task approval counts.
	Evaluate(ctx PassRuleContext) PassRuleResult
}

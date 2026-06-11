package approval

import "context"

// ConditionOperator enumerates the comparison operators a field condition may
// use. The set is the shared contract between the flow designer (which offers
// operators per field kind) and the engine's field-condition evaluator; both
// sides must stay in lockstep with this list.
type ConditionOperator string

const (
	OperatorEquals      ConditionOperator = "eq"
	OperatorNotEquals   ConditionOperator = "ne"
	OperatorGreater     ConditionOperator = "gt"
	OperatorGreaterOrEq ConditionOperator = "gte"
	OperatorLess        ConditionOperator = "lt"
	OperatorLessOrEq    ConditionOperator = "lte"
	OperatorIn          ConditionOperator = "in"
	OperatorNotIn       ConditionOperator = "not_in"
	OperatorContains    ConditionOperator = "contains"
	OperatorNotContains ConditionOperator = "not_contains"
	OperatorStartsWith  ConditionOperator = "starts_with"
	OperatorEndsWith    ConditionOperator = "ends_with"
	OperatorIsEmpty     ConditionOperator = "is_empty"
	OperatorIsNotEmpty  ConditionOperator = "is_not_empty"
)

// IsValid reports whether the operator is one of the defined values.
func (o ConditionOperator) IsValid() bool {
	switch o {
	case OperatorEquals, OperatorNotEquals,
		OperatorGreater, OperatorGreaterOrEq, OperatorLess, OperatorLessOrEq,
		OperatorIn, OperatorNotIn,
		OperatorContains, OperatorNotContains, OperatorStartsWith, OperatorEndsWith,
		OperatorIsEmpty, OperatorIsNotEmpty:
		return true
	default:
		return false
	}
}

// Condition represents a branch condition evaluated by condition nodes.
type Condition struct {
	Kind       ConditionKind     `json:"kind"`
	Subject    string            `json:"subject"`
	Operator   ConditionOperator `json:"operator"`
	Value      any               `json:"value"`
	Expression string            `json:"expression"`
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
	FormData              FormData
	ApplicantID           string
	ApplicantDepartmentID *string
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

package approval

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/security"
)

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
	// Globals carries host-supplied global variables snapshotted at instance
	// start (Instance.Globals). Field conditions resolve a subject against
	// Globals before form data — the same shadowing the built-in applicant
	// subjects apply — and expression conditions see each entry as a top-level
	// binding (the built-in formData / applicantId / applicantDepartmentId
	// bindings always win a name collision).
	Globals map[string]any
}

// InstanceGlobalsResolver resolves the host-defined global variables for a
// new instance — attributes the flow's conditions may reference beyond the
// form data and the built-in applicant subjects (tenant attributes, applicant
// roles, business limits, …). Resolved SERVER-SIDE from the authenticated
// principal at instance start and snapshotted onto Instance.Globals: globals
// participate in routing, so they must never be accepted from the client
// request body. Implemented by host apps (override via fx.Replace); the
// default resolver returns no globals.
type InstanceGlobalsResolver interface {
	// Resolve returns the global-variable snapshot for an instance the given
	// principal is starting on the given flow. A nil map means "no globals".
	Resolve(ctx context.Context, principal *security.Principal, flowCode string) (map[string]any, error)
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

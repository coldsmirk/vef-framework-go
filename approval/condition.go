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

// AggregateKind enumerates the aggregations a field condition may apply
// over a detail-table field's rows. Semantics follow SQL aggregates:
// count is the row count, sum of an empty table is 0, and avg over an
// empty table matches no comparison (NULL semantics) instead of being 0.
// Folds compute in float64 — prefer ordering operators over eq/ne when
// comparing sums or averages of fractional amounts.
type AggregateKind string

const (
	AggregateSum   AggregateKind = "sum"
	AggregateCount AggregateKind = "count"
	AggregateAvg   AggregateKind = "avg"
)

// IsValid reports whether the aggregate is one of the defined values.
func (a AggregateKind) IsValid() bool {
	return a == AggregateSum || a == AggregateCount || a == AggregateAvg
}

// FoldsColumn reports whether the aggregate reduces a numeric column
// (sum / avg) or the rows themselves (count). It is the single source of
// the column contract: deploy validation derives the "column required /
// forbidden" rule from it and the evaluator derives what to extract, so a
// new aggregate kind declares its shape here once and both sides follow.
func (a AggregateKind) FoldsColumn() bool { return a != AggregateCount }

// Aggregator folds a detail-table field's rows into one comparable number
// for an aggregate field condition. Implementations are collected through
// the FX group "vef:approval:aggregators" — adding an aggregate (built-in
// or host-supplied) registers a new implementation; the evaluator, the
// deploy validation, and existing aggregators stay untouched.
type Aggregator interface {
	// Kind returns the aggregate kind this implementation folds.
	Kind() AggregateKind
	// Fold reduces the extracted column values (for column aggregates) or
	// the row count (for row aggregates such as count) into the comparison
	// operand. matchable=false means the aggregate has no defined value for
	// the input — e.g. avg over zero rows — and the condition must not
	// match, mirroring SQL NULL comparison semantics.
	Fold(values []float64, rowCount int) (result float64, matchable bool)
}

// Condition represents a branch condition evaluated by condition nodes.
type Condition struct {
	Kind    ConditionKind `json:"kind"`
	Subject string        `json:"subject"`
	// Aggregate, when set, evaluates the condition over a detail-table
	// field's rows instead of a scalar subject: Subject names the table
	// field, Column the numeric column to fold (sum / avg; count works on
	// rows and must leave Column empty). Structured on purpose — no string
	// DSL to parse, and the designer offers it as table → column → aggregate.
	Aggregate  AggregateKind     `json:"aggregate,omitempty"`
	Column     string            `json:"column,omitempty"`
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

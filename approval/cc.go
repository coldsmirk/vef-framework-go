package approval

import "context"

// CCDefinition represents a CC recipient in node data.
type CCDefinition struct {
	Kind      CCKind   `json:"kind"`
	IDs       []string `json:"ids,omitempty"`
	FormField *string  `json:"formField,omitempty"`
	Timing    CCTiming `json:"timing,omitempty"`
}

// CCResolveContext is what a CCResolver resolves against: the node-level
// runtime snapshot plus the one CC rule being resolved. It mirrors
// AssigneeResolveContext field for field, so a host kind that answers "the
// applicant's head nurse" is written once and registered on both sides.
//
// Timing is deliberately absent: whether a rule fires on entry, on approval,
// or on rejection is the engine's decision, made before the resolver is
// consulted. A resolver that read it could contradict that decision.
type CCResolveContext struct {
	NodeResolveContext

	// Kind is the CC kind of the rule being resolved.
	Kind CCKind
	// IDs are the designer-selected IDs of the rule, meaning whatever the
	// kind says they mean.
	IDs []string
	// FormField names the form field carrying the recipient IDs, set only for
	// kinds whose SelectionMode is SelectionFormField.
	FormField *string
}

// CCResolver turns one CC rule into the concrete recipients notified. The
// framework registers a resolver per built-in kind and hosts add their own
// with vef.ProvideApprovalCCResolver; a host resolver whose kind matches a
// built-in replaces it.
//
// CC delivery is a notification side effect, never a decision: the engine
// resolves it best-effort, logging and skipping a rule whose resolver fails
// rather than rolling back the approval that triggered it. Report failures
// honestly anyway — the log entry is what makes a broken CC rule visible.
type CCResolver interface {
	// Describe returns the kind this resolver handles together with the
	// designer metadata for it — the label to show and the input to collect.
	Describe() KindDescriptor[CCKind]
	// Resolve returns the recipient user IDs for the rule in rc. Duplicates
	// and blanks are filtered by the caller, which also snapshots each
	// recipient's display info, so returning bare IDs is enough.
	Resolve(ctx context.Context, rc *CCResolveContext) ([]string, error)
}

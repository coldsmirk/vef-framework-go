package approval

import "context"

// InitiatorResolveContext is what an InitiatorResolver decides against: the
// flow whose initiation is being checked and the person asking to start it.
//
// There is no instance and no form data — the check runs before either exists,
// which is the point: who may start a flow is a property of the person and the
// flow, never of what they typed. A rule that needs the form belongs in a
// condition branch, where the form is real.
type InitiatorResolveContext struct {
	// FlowID identifies the flow being started.
	FlowID string
	// TenantID is the flow's tenant.
	TenantID string
	// Applicant is the person asking to start the flow. The ID and the
	// department resolved from their security principal are always present;
	// the display name is not — the start-form lookup checks permission
	// without resolving one — so decide on the ID and the department, never on
	// the name.
	Applicant UserInfo
	// Kind is the initiator kind of the rule being evaluated.
	Kind InitiatorKind
	// IDs are the designer-selected IDs of the rule.
	IDs []string
}

// InitiatorResolver decides whether one initiator rule admits the applicant.
// The framework registers a resolver per built-in kind and hosts add their own
// with vef.ProvideApprovalInitiatorResolver; a host resolver whose kind
// matches a built-in replaces it.
//
// It is a membership predicate rather than a people-producer on purpose:
// answering "may this user start it" by enumerating everyone a rule admits
// does not scale for a role spanning thousands of users, and the framework
// already prefers the direct check elsewhere (RoleMembershipChecker).
type InitiatorResolver interface {
	// Describe returns the kind this resolver handles together with the
	// designer metadata for it — the label to show and the input to collect.
	Describe() KindDescriptor[InitiatorKind]
	// Permits reports whether the applicant in rc matches this rule. Rules are
	// evaluated in order and the first match admits, so a false answer means
	// "this rule does not admit them", not "they are refused". An error stops
	// the evaluation and refuses the start — initiation permission fails
	// closed.
	Permits(ctx context.Context, rc *InitiatorResolveContext) (bool, error)
}

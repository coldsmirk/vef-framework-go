package strategy

import (
	"context"
	"fmt"
	"slices"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/i18n"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
)

// NewUserInitiatorResolver creates a new UserInitiatorResolver.
func NewUserInitiatorResolver() approval.InitiatorResolver {
	return new(UserInitiatorResolver)
}

// UserInitiatorResolver admits the applicant when they are listed by ID.
type UserInitiatorResolver struct{}

func (*UserInitiatorResolver) Describe() approval.KindDescriptor[approval.InitiatorKind] {
	return approval.KindDescriptor[approval.InitiatorKind]{
		Kind:      approval.InitiatorUser,
		Label:     i18n.T("approval_initiator_kind_user"),
		Selection: approval.SelectionUser,
	}
}

func (*UserInitiatorResolver) Permits(_ context.Context, rc *approval.InitiatorResolveContext) (bool, error) {
	return slices.Contains(rc.IDs, rc.Applicant.ID), nil
}

// NewDepartmentInitiatorResolver creates a new DepartmentInitiatorResolver.
func NewDepartmentInitiatorResolver() approval.InitiatorResolver {
	return new(DepartmentInitiatorResolver)
}

// DepartmentInitiatorResolver admits the applicant when their department is
// listed. An applicant with no resolved department matches no department rule.
type DepartmentInitiatorResolver struct{}

func (*DepartmentInitiatorResolver) Describe() approval.KindDescriptor[approval.InitiatorKind] {
	return approval.KindDescriptor[approval.InitiatorKind]{
		Kind:      approval.InitiatorDepartment,
		Label:     i18n.T("approval_initiator_kind_department"),
		Selection: approval.SelectionDepartment,
	}
}

func (*DepartmentInitiatorResolver) Permits(_ context.Context, rc *approval.InitiatorResolveContext) (bool, error) {
	departmentID := rc.Applicant.DepartmentID
	if departmentID == nil {
		return false, nil
	}

	return slices.Contains(rc.IDs, *departmentID), nil
}

// NewRoleInitiatorResolver creates a new RoleInitiatorResolver.
func NewRoleInitiatorResolver(svc approval.AssigneeService) approval.InitiatorResolver {
	return &RoleInitiatorResolver{svc: svc}
}

// RoleInitiatorResolver admits the applicant when they hold one of the listed
// roles. It asks the host for membership directly where the AssigneeService
// implements RoleMembershipChecker, and falls back to listing the role's
// members otherwise.
type RoleInitiatorResolver struct {
	svc approval.AssigneeService
}

func (*RoleInitiatorResolver) Describe() approval.KindDescriptor[approval.InitiatorKind] {
	return approval.KindDescriptor[approval.InitiatorKind]{
		Kind:      approval.InitiatorRole,
		Label:     i18n.T("approval_initiator_kind_role"),
		Selection: approval.SelectionRole,
	}
}

func (r *RoleInitiatorResolver) Permits(ctx context.Context, rc *approval.InitiatorResolveContext) (bool, error) {
	// A host with no AssigneeService cannot answer role membership;
	// shared.UserHasRole reports "not a member" rather than an error so the
	// remaining rules still get their chance to admit the applicant.
	for _, roleID := range rc.IDs {
		member, err := shared.UserHasRole(ctx, r.svc, rc.Applicant.ID, roleID)
		if err != nil {
			return false, err
		}

		if member {
			return true, nil
		}
	}

	return false, nil
}

// CompositeInitiatorResolver dispatches each initiator rule to the resolver
// registered for its kind and answers the one question the flow asks: may this
// applicant start it.
type CompositeInitiatorResolver struct {
	ordered   []approval.InitiatorResolver
	resolvers map[approval.InitiatorKind]approval.InitiatorResolver
}

// NewCompositeInitiatorResolver merges host-registered resolvers onto the
// built-ins with the same override-by-kind semantics as the assignee side.
func NewCompositeInitiatorResolver(builtins, hosts []approval.InitiatorResolver) (*CompositeInitiatorResolver, error) {
	ordered, index, err := overlayKinds(builtins, hosts)
	if err != nil {
		return nil, err
	}

	// Initiation is checked before an instance — and therefore before a form —
	// exists, so a kind reading its value out of the form could never be
	// evaluated. Rejecting it here turns a contradiction that would otherwise
	// surface as an unfillable designer field into a boot failure naming it.
	for _, resolver := range ordered {
		if descriptor := resolver.Describe(); descriptor.Selection == approval.SelectionFormField {
			return nil, fmt.Errorf("%w: initiator kind %s cannot select from the form", errInvalidKindDescriptor, descriptor.Kind)
		}
	}

	return &CompositeInitiatorResolver{ordered: ordered, resolvers: index}, nil
}

// Descriptors returns the registered kinds in designer order.
func (c *CompositeInitiatorResolver) Descriptors() []approval.KindDescriptor[approval.InitiatorKind] {
	return describeAll(c.ordered)
}

// Describe returns the descriptor registered for a kind.
func (c *CompositeInitiatorResolver) Describe(kind approval.InitiatorKind) (approval.KindDescriptor[approval.InitiatorKind], bool) {
	resolver, ok := c.resolvers[kind]
	if !ok {
		return approval.KindDescriptor[approval.InitiatorKind]{}, false
	}

	return resolver.Describe(), true
}

// PermitsAny reports whether any rule admits the applicant. Rules are
// evaluated in order and the first match wins; an empty rule set admits
// nobody, which is what makes "no rules stored" mean "open to everyone" only
// in combination with Flow.IsAllInitiationAllowed.
//
// An unregistered kind is an error rather than a silent non-match: save-time
// validation accepts exactly the registered kinds, so a rule naming one that
// is gone means the host dropped a resolver its flows still reference, and
// treating that as "denied" would look like a permission bug instead of a
// configuration one.
func (c *CompositeInitiatorResolver) PermitsAny(
	ctx context.Context,
	initiators []approval.FlowInitiator,
	base *approval.InitiatorResolveContext,
) (bool, error) {
	for _, initiator := range initiators {
		resolver, ok := c.resolvers[initiator.Kind]
		if !ok {
			return false, fmt.Errorf("%w: %s", ErrInitiatorResolverNotFound, initiator.Kind)
		}

		rc := *base
		rc.Kind = initiator.Kind
		rc.IDs = initiator.IDs

		permitted, err := resolver.Permits(ctx, &rc)
		if err != nil {
			return false, fmt.Errorf("initiator resolver %q: %w", initiator.Kind, err)
		}

		if permitted {
			return true, nil
		}
	}

	return false, nil
}

package strategy

import (
	"context"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/approval"
)

// RoleCheckerService answers role membership directly, the capability
// shared.UserHasRole prefers over listing a role's whole membership.
type RoleCheckerService struct {
	MockAssigneeService

	memberships map[string][]string
}

func (s *RoleCheckerService) UserHasRole(_ context.Context, userID, roleID string) (bool, error) {
	return slices.Contains(s.memberships[roleID], userID), nil
}

// newInitiatorComposite builds a composite over exactly the given resolvers.
func newInitiatorComposite(t *testing.T, resolvers ...approval.InitiatorResolver) *CompositeInitiatorResolver {
	t.Helper()

	composite, err := NewCompositeInitiatorResolver(resolvers, nil)
	require.NoError(t, err, "Should build composite initiator resolver")

	return composite
}

func TestInitiatorResolvers(t *testing.T) {
	ctx := context.Background()

	t.Run("UserMatchesByID", func(t *testing.T) {
		r := NewUserInitiatorResolver()

		permitted, err := r.Permits(ctx, &approval.InitiatorResolveContext{
			IDs:       []string{"u1", "u2"},
			Applicant: approval.UserInfo{ID: "u2"},
		})
		require.NoError(t, err, "Should decide without error")
		assert.True(t, permitted, "A listed user should be admitted")

		permitted, err = r.Permits(ctx, &approval.InitiatorResolveContext{
			IDs:       []string{"u1"},
			Applicant: approval.UserInfo{ID: "u2"},
		})
		require.NoError(t, err, "Should decide without error")
		assert.False(t, permitted, "An unlisted user should not be admitted")
	})

	t.Run("DepartmentMatchesApplicantDepartment", func(t *testing.T) {
		r := NewDepartmentInitiatorResolver()

		permitted, err := r.Permits(ctx, &approval.InitiatorResolveContext{
			IDs:       []string{"dept-1"},
			Applicant: approval.UserInfo{ID: "u1", DepartmentID: new("dept-1")},
		})
		require.NoError(t, err, "Should decide without error")
		assert.True(t, permitted, "A member of a listed department should be admitted")
	})

	t.Run("DepartmentWithoutApplicantDepartmentMatchesNothing", func(t *testing.T) {
		permitted, err := NewDepartmentInitiatorResolver().Permits(ctx, &approval.InitiatorResolveContext{
			IDs:       []string{"dept-1"},
			Applicant: approval.UserInfo{ID: "u1"},
		})
		require.NoError(t, err, "An unresolved department is not an error")
		assert.False(t, permitted, "An applicant with no department matches no department rule")
	})

	t.Run("RoleUsesMembershipChecker", func(t *testing.T) {
		svc := &RoleCheckerService{memberships: map[string][]string{"nurse": {"u1"}}}

		permitted, err := NewRoleInitiatorResolver(svc).Permits(ctx, &approval.InitiatorResolveContext{
			IDs:       []string{"nurse"},
			Applicant: approval.UserInfo{ID: "u1"},
		})
		require.NoError(t, err, "Should decide without error")
		assert.True(t, permitted, "A role member should be admitted")
	})

	t.Run("RoleWithoutServiceMatchesNothing", func(t *testing.T) {
		// A host with no AssigneeService cannot answer membership. Reporting
		// "not a member" rather than an error leaves the remaining rules free
		// to admit the applicant.
		permitted, err := NewRoleInitiatorResolver(nil).Permits(ctx, &approval.InitiatorResolveContext{
			IDs:       []string{"nurse"},
			Applicant: approval.UserInfo{ID: "u1"},
		})
		require.NoError(t, err, "A missing AssigneeService is not an error here")
		assert.False(t, permitted, "Role membership cannot be proven, so the rule does not admit")
	})

	t.Run("RoleLookupErrorIsReported", func(t *testing.T) {
		_, err := NewRoleInitiatorResolver(&ErrAssigneeService{}).Permits(ctx, &approval.InitiatorResolveContext{
			IDs:       []string{"nurse"},
			Applicant: approval.UserInfo{ID: "u1"},
		})
		require.ErrorIs(t, err, errAssigneeSvc, "Should wrap the underlying service error")
	})
}

func TestCompositeInitiatorResolverPermitsAny(t *testing.T) {
	ctx := context.Background()
	composite := newInitiatorComposite(t, NewUserInitiatorResolver(), NewDepartmentInitiatorResolver())

	t.Run("FirstMatchWins", func(t *testing.T) {
		permitted, err := composite.PermitsAny(ctx, []approval.FlowInitiator{
			{Kind: approval.InitiatorDepartment, IDs: []string{"dept-9"}},
			{Kind: approval.InitiatorUser, IDs: []string{"u1"}},
		}, &approval.InitiatorResolveContext{Applicant: approval.UserInfo{ID: "u1"}})
		require.NoError(t, err, "Should decide without error")
		assert.True(t, permitted, "A later matching rule should still admit")
	})

	t.Run("NoRuleMatches", func(t *testing.T) {
		permitted, err := composite.PermitsAny(ctx, []approval.FlowInitiator{
			{Kind: approval.InitiatorUser, IDs: []string{"someone-else"}},
		}, &approval.InitiatorResolveContext{Applicant: approval.UserInfo{ID: "u1"}})
		require.NoError(t, err, "Should decide without error")
		assert.False(t, permitted, "An applicant no rule names is not admitted")
	})

	t.Run("EmptyRuleSetAdmitsNobody", func(t *testing.T) {
		permitted, err := composite.PermitsAny(ctx, nil, &approval.InitiatorResolveContext{Applicant: approval.UserInfo{ID: "u1"}})
		require.NoError(t, err, "Should decide without error")
		assert.False(t, permitted, "An empty rule set fails closed")
	})

	t.Run("UnregisteredKindIsAnError", func(t *testing.T) {
		_, err := composite.PermitsAny(ctx, []approval.FlowInitiator{
			{Kind: approval.InitiatorRole, IDs: []string{"nurse"}},
		}, &approval.InitiatorResolveContext{Applicant: approval.UserInfo{ID: "u1"}})
		require.ErrorIs(t, err, ErrInitiatorResolverNotFound, "A rule naming a kind nothing resolves is a configuration error, not a denial")
	})
}

// TestInitiatorKindCannotSelectFromTheForm pins the one selection mode the
// initiator vocabulary refuses: initiation is checked before a form exists, so
// a form-field kind could never be evaluated.
func TestInitiatorKindCannotSelectFromTheForm(t *testing.T) {
	_, err := NewCompositeInitiatorResolver(
		[]approval.InitiatorResolver{&StubInitiatorResolver{
			descriptor: approval.KindDescriptor[approval.InitiatorKind]{
				Kind:      "from_form",
				Label:     "From form",
				Selection: approval.SelectionFormField,
			},
		}},
		nil,
	)
	require.ErrorIs(t, err, errInvalidKindDescriptor, "A form-field initiator kind must fail at boot")
}

// StubInitiatorResolver is a minimal resolver used to register a descriptor.
type StubInitiatorResolver struct {
	descriptor approval.KindDescriptor[approval.InitiatorKind]
	permits    bool
}

func (r *StubInitiatorResolver) Describe() approval.KindDescriptor[approval.InitiatorKind] {
	return r.descriptor
}

func (r *StubInitiatorResolver) Permits(context.Context, *approval.InitiatorResolveContext) (bool, error) {
	return r.permits, nil
}

package strategy

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/approval"
)

// newCCComposite builds a composite over exactly the given resolvers, with no
// framework built-ins underneath.
func newCCComposite(t *testing.T, resolvers ...approval.CCResolver) *CompositeCCResolver {
	t.Helper()

	composite, err := NewCompositeCCResolver(resolvers, nil)
	require.NoError(t, err, "Should build composite CC resolver")

	return composite
}

func TestCCResolvers(t *testing.T) {
	ctx := context.Background()
	svc := &MockAssigneeService{
		roleUsers: map[string][]approval.UserInfo{
			"role-a": {{ID: "u1", Name: "U1"}, {ID: "u2", Name: "U2"}},
			"role-b": {{ID: "u2", Name: "U2"}, {ID: "u3", Name: "U3"}},
		},
		departmentLeaders: map[string][]approval.UserInfo{
			"dept-1": {{ID: "leader-1", Name: "Leader"}},
		},
	}

	t.Run("UserResolvesStaticUniqueIDs", func(t *testing.T) {
		got, err := NewUserCCResolver().Resolve(ctx, &approval.CCResolveContext{IDs: []string{"a", "b", "a"}})
		require.NoError(t, err, "Should resolve without error")
		assert.Equal(t, []string{"a", "b"}, got, "User CC should resolve static unique IDs")
	})

	t.Run("RoleDedupsAcrossRoles", func(t *testing.T) {
		got, err := NewRoleCCResolver(svc).Resolve(ctx, &approval.CCResolveContext{IDs: []string{"role-a", "role-b"}})
		require.NoError(t, err, "Should resolve without error")
		assert.Equal(t, []string{"u1", "u2", "u3"}, got, "Role CC should dedup members in first-seen order")
	})

	t.Run("DepartmentResolvesToLeaders", func(t *testing.T) {
		got, err := NewDepartmentCCResolver(svc).Resolve(ctx, &approval.CCResolveContext{IDs: []string{"dept-1"}})
		require.NoError(t, err, "Should resolve without error")
		assert.Equal(t, []string{"leader-1"}, got, "Department CC should resolve to department leaders")
	})

	t.Run("FormFieldReadsTheField", func(t *testing.T) {
		got, err := NewFormFieldCCResolver().Resolve(ctx, &approval.CCResolveContext{
			Kind:      approval.CCFormField,
			FormField: new("watchers"),
			FormData:  approval.FormData{"watchers": []any{"u1", " u2 ", "u1"}},
		})
		require.NoError(t, err, "Should resolve without error")
		assert.Equal(t, []string{"u1", "u2"}, got, "Form-field CC should trim, dedup and preserve order")
	})

	t.Run("OrgKindsReportMissingService", func(t *testing.T) {
		_, err := NewRoleCCResolver(nil).Resolve(ctx, &approval.CCResolveContext{IDs: []string{"role-a"}})
		require.ErrorIs(t, err, ErrAssigneeServiceNil, "Role CC without an AssigneeService must surface an error")

		_, err = NewDepartmentCCResolver(nil).Resolve(ctx, &approval.CCResolveContext{IDs: []string{"dept-1"}})
		require.ErrorIs(t, err, ErrAssigneeServiceNil, "Department CC without an AssigneeService must surface an error")
	})

	t.Run("OrgLookupErrorIsReported", func(t *testing.T) {
		_, err := NewRoleCCResolver(&ErrAssigneeService{}).Resolve(ctx, &approval.CCResolveContext{IDs: []string{"role-a"}})
		require.ErrorIs(t, err, errAssigneeSvc, "Should wrap the underlying service error")
	})
}

// TestCompositeCCResolverCollectUserIDs pins the best-effort boundary: a config
// the resolver cannot resolve (a role CC with no AssigneeService, or an
// unregistered kind) is logged and skipped, while the resolvable configs in the
// same batch still produce recipients — so a CC notification never fails the
// approval that triggered it.
func TestCompositeCCResolverCollectUserIDs(t *testing.T) {
	ctx := context.Background()
	composite := newCCComposite(t, NewUserCCResolver(), NewRoleCCResolver(nil))

	t.Run("BestEffortSkipsUnresolvable", func(t *testing.T) {
		configs := []approval.FlowNodeCC{
			{Kind: approval.CCRole, IDs: []string{"role-a"}},     // no service → skipped
			{Kind: approval.CCUser, IDs: []string{"u1", "u2"}},   // resolvable
			{Kind: approval.CCKind("bogus"), IDs: []string{"x"}}, // unregistered → skipped
		}

		got := composite.CollectUserIDs(ctx, configs, &approval.NodeResolveContext{}, nil)
		assert.Equal(t, []string{"u1", "u2"}, got, "Unresolvable configs are skipped; resolvable ones still yield recipients")
	})

	t.Run("SelectorFiltersConfigs", func(t *testing.T) {
		configs := []approval.FlowNodeCC{
			{Kind: approval.CCUser, IDs: []string{"u1"}, Timing: approval.CCTimingOnApprove},
			{Kind: approval.CCUser, IDs: []string{"u2"}, Timing: approval.CCTimingOnReject},
		}

		got := composite.CollectUserIDs(ctx, configs, &approval.NodeResolveContext{}, func(cfg approval.FlowNodeCC) bool {
			return cfg.Timing == approval.CCTimingOnApprove
		})
		assert.Equal(t, []string{"u1"}, got, "Only configs the selector admits should be resolved")
	})

	t.Run("UnregisteredKindIsAnErrorFromResolve", func(t *testing.T) {
		_, err := composite.Resolve(ctx, approval.FlowNodeCC{Kind: "bogus"}, &approval.NodeResolveContext{})
		require.ErrorIs(t, err, ErrCCResolverNotFound, "Resolve reports the unregistered kind; only CollectUserIDs skips it")
	})
}

package strategy

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/approval"
)

// StubAssigneeResolver registers an arbitrary descriptor and resolves to one
// fixed user, so a test can tell which registration actually ran.
type StubAssigneeResolver struct {
	descriptor approval.KindDescriptor[approval.AssigneeKind]
	userID     string
}

func (r *StubAssigneeResolver) Describe() approval.KindDescriptor[approval.AssigneeKind] {
	return r.descriptor
}

func (r *StubAssigneeResolver) Resolve(context.Context, *approval.AssigneeResolveContext) ([]approval.ResolvedAssignee, error) {
	return []approval.ResolvedAssignee{{User: approval.UserInfo{ID: r.userID}}}, nil
}

func stubAssignee(kind approval.AssigneeKind, selection approval.SelectionMode, userID string) *StubAssigneeResolver {
	return &StubAssigneeResolver{
		descriptor: approval.KindDescriptor[approval.AssigneeKind]{Kind: kind, Label: string(kind), Selection: selection},
		userID:     userID,
	}
}

// kindsOf lists the kinds a composite offers, in designer order.
func kindsOf(composite *CompositeAssigneeResolver) []approval.AssigneeKind {
	kinds := make([]approval.AssigneeKind, 0, len(composite.ordered))
	for _, descriptor := range composite.Descriptors() {
		kinds = append(kinds, descriptor.Kind)
	}

	return kinds
}

func TestOverlayKinds(t *testing.T) {
	ctx := context.Background()

	t.Run("HostKindIsAppended", func(t *testing.T) {
		composite, err := NewCompositeAssigneeResolver(
			BuiltinAssigneeResolvers(nil),
			[]approval.AssigneeResolver{stubAssignee("expert_panel", approval.SelectionCustom, "expert-1")},
		)
		require.NoError(t, err, "Should build the composite")

		kinds := kindsOf(composite)
		require.Len(t, kinds, len(expectedAssigneeKinds)+1, "The host kind should join the built-ins")
		assert.Equal(t, approval.AssigneeKind("expert_panel"), kinds[len(kinds)-1], "A new kind is appended after the built-ins")

		resolved, err := composite.ResolveAll(ctx,
			[]approval.FlowNodeAssignee{{Kind: "expert_panel", IDs: []string{"panel-a"}}},
			&approval.NodeResolveContext{},
		)
		require.NoError(t, err, "Should resolve the host kind")
		assertUserIDs(t, resolved, "expert-1")
	})

	t.Run("HostKindOverridesBuiltinInPlace", func(t *testing.T) {
		builtins := BuiltinAssigneeResolvers(nil)
		position := 0

		for i, resolver := range builtins {
			if resolver.Describe().Kind == approval.AssigneeDepartmentLeader {
				position = i
			}
		}

		composite, err := NewCompositeAssigneeResolver(
			builtins,
			[]approval.AssigneeResolver{stubAssignee(approval.AssigneeDepartmentLeader, approval.SelectionNone, "host-leader")},
		)
		require.NoError(t, err, "Should build the composite")

		kinds := kindsOf(composite)
		require.Len(t, kinds, len(expectedAssigneeKinds), "An override replaces rather than adds")
		assert.Equal(t, approval.AssigneeDepartmentLeader, kinds[position], "The overridden kind keeps its designer position")

		resolved, err := composite.ResolveAll(ctx,
			[]approval.FlowNodeAssignee{{Kind: approval.AssigneeDepartmentLeader}},
			&approval.NodeResolveContext{},
		)
		require.NoError(t, err, "Should resolve through the override")
		assertUserIDs(t, resolved, "host-leader")
	})

	t.Run("RejectsADuplicateHostKind", func(t *testing.T) {
		_, err := NewCompositeAssigneeResolver(
			nil,
			[]approval.AssigneeResolver{
				stubAssignee("panel", approval.SelectionCustom, "first"),
				stubAssignee("panel", approval.SelectionCustom, "second"),
			},
		)
		require.ErrorIs(t, err, errDuplicateHostKind, "Two host resolvers claiming one kind must fail at boot")
		assert.Contains(t, err.Error(), "panel", "The failure must name the contested kind")
	})

	t.Run("AppendsHostKindsInAscendingKindOrder", func(t *testing.T) {
		// fx delivers a value group in a randomized order, so the registration
		// order below must NOT be the designer order — otherwise the dropdown
		// reshuffles between restarts.
		composite, err := NewCompositeAssigneeResolver(
			nil,
			[]approval.AssigneeResolver{
				stubAssignee("head_nurse", approval.SelectionNone, "nurse"),
				stubAssignee("committee", approval.SelectionCustom, "committee"),
				stubAssignee("department_director", approval.SelectionNone, "director"),
			},
		)
		require.NoError(t, err, "Should build the composite")

		assert.Equal(
			t,
			[]approval.AssigneeKind{"committee", "department_director", "head_nurse"},
			kindsOf(composite),
			"Host kinds are appended in ascending kind order regardless of registration order",
		)
	})

	t.Run("RejectsNilResolver", func(t *testing.T) {
		_, err := NewCompositeAssigneeResolver(nil, []approval.AssigneeResolver{nil})
		require.ErrorIs(t, err, errNilResolver, "A nil registration must fail at boot, not at first use")
	})

	t.Run("RejectsFaultyDescriptors", func(t *testing.T) {
		faults := map[string]approval.KindDescriptor[approval.AssigneeKind]{
			"BlankKind":        {Label: "Panel", Selection: approval.SelectionCustom},
			"BlankLabel":       {Kind: "panel", Selection: approval.SelectionCustom},
			"InvalidSelection": {Kind: "panel", Label: "Panel", Selection: "whatever"},
		}

		for name, descriptor := range faults {
			t.Run(name, func(t *testing.T) {
				_, err := NewCompositeAssigneeResolver(nil, []approval.AssigneeResolver{
					&StubAssigneeResolver{descriptor: descriptor},
				})
				require.ErrorIs(t, err, errInvalidKindDescriptor, "A faulty descriptor must fail at boot")
			})
		}
	})
}

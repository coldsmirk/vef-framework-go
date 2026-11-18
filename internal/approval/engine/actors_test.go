package engine_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/engine"
)

type handoverMap map[string]approval.Handover

func (m handoverMap) ResolveHandovers(context.Context, *approval.HandoverContext, []string) (map[string]approval.Handover, error) {
	return m, nil
}

func successor(id string) approval.Handover {
	return approval.Handover{Successor: &approval.UserInfo{ID: id, Name: id}, Reason: "Changed role"}
}

func TestHandoverChains(t *testing.T) {
	for _, tc := range []struct {
		name  string
		links handoverMap
		want  string
	}{
		{"SuccessiveTransfers", handoverMap{"A": successor("B"), "B": successor("C")}, "C"},
		{"EndsWithoutSuccessor", handoverMap{"A": successor("B"), "B": {}}, ""},
		{"BeforeCycle", handoverMap{"A": successor("B"), "B": successor("C"), "C": successor("D"), "D": successor("C")}, "B"},
		{"PureCycle", handoverMap{"A": successor("B"), "B": successor("A")}, "A"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := engine.NewActorResolution(&approval.Instance{}, &approval.FlowNode{}, tc.links)
			changes, err := r.Handovers(t.Context(), []string{"A"})
			require.NoError(t, err, "Should resolve handover chain")

			h := changes["A"]
			require.Equal(t, "Changed role", h.Reason, "Should retain the original seat's reason")

			if tc.want == "" {
				require.Nil(t, h.Successor, "A chain whose last user left drops the seat")

				return
			}

			require.Equal(t, tc.want, h.Successor.ID, "Should resolve final seat")
		})
	}

	t.Run("InvalidSuccessor", func(t *testing.T) {
		r := engine.NewActorResolution(&approval.Instance{}, nil, handoverMap{"A": successor("")})
		_, err := r.Handovers(t.Context(), []string{"A"})
		require.ErrorIs(t, err, engine.ErrInvalidHandover, "Malformed host identities should stop assignment")
	})
	t.Run("DepthLimit", func(t *testing.T) {
		links := handoverMap{}
		for i := range 10 {
			links[fmt.Sprint(i)] = successor(fmt.Sprint(i + 1))
		}

		resolve := func() (map[string]approval.Handover, error) {
			return engine.NewActorResolution(&approval.Instance{}, nil, links).Handovers(t.Context(), []string{"0"})
		}
		changes, err := resolve()
		require.NoError(t, err, "Ten hops to a terminal user are allowed")
		require.Equal(t, "10", changes["0"].Successor.ID, "Should reach the tenth successor")

		links["10"] = successor("11")
		_, err = resolve()
		require.ErrorIs(t, err, engine.ErrHandoverDepth, "Longer chains must abort")
		require.ErrorContains(t, err, "[0 1 2 3 4 5 6 7 8 9 10 11]", "Error should identify the entire chain")
	})
}

package business_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/business"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
)

type described struct {
	approval.BusinessHandler

	d approval.BusinessKindDescriptor
}

func (h *described) Describe() approval.BusinessKindDescriptor { return h.d }
func TestRegistry(t *testing.T) {
	validate := service.NewFlowDefinitionService().ValidateFormFields
	h := func(k approval.BusinessKind) *described {
		return &described{d: approval.BusinessKindDescriptor{Kind: k, Label: "Label"}}
	}
	t.Run("RejectsInvalidRegistrations", func(t *testing.T) {
		for _, k := range []approval.BusinessKind{"A", "a..b", "approval.reserved", approval.BusinessKind(strings.Repeat("a", 65))} {
			_, err := business.NewRegistry([]approval.BusinessHandler{h(k)}, validate)
			require.ErrorContains(t, err, string(k), "Error should identify the invalid kind")
		}

		a := h("site.booking")
		_, err := business.NewRegistry([]approval.BusinessHandler{a, a}, validate)
		require.ErrorIs(t, err, business.ErrDuplicateKind, "Kinds must be unique")

		a.d.Label = " "
		_, err = business.NewRegistry([]approval.BusinessHandler{a}, validate)
		require.ErrorIs(t, err, business.ErrInvalidDescriptor, "A kind needs a label")

		a.d.Label = "Label"
		a.d.FormFields = []approval.FormFieldDefinition{{Key: "field"}}
		_, err = business.NewRegistry([]approval.BusinessHandler{a}, validate)
		require.ErrorContains(t, err, "site.booking", "Catalog errors identify their handler")
	})
	t.Run("OrdersKindsAndRefreshesLabels", func(t *testing.T) {
		a, b := h("x.a"), h("x.b")
		r, err := business.NewRegistry([]approval.BusinessHandler{b, a}, validate)
		require.NoError(t, err, "Valid handlers should register")

		a.d.Label = "Localized"
		ds := r.Descriptors()
		require.Equal(t, a.d.Kind, ds[0].Kind, "Catalog sorts by kind")
		require.Equal(t, "Localized", ds[0].Label, "Each read resolves the current label")
		require.Equal(t, b.d.Kind, ds[1].Kind, "Second entry follows first")
	})
}

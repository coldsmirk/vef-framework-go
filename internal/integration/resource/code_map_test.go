package resource

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/integration"
)

func TestSealCodeMap(t *testing.T) {
	newCodeMap := func(codeSet string) *integration.CodeMap {
		return &integration.CodeMap{CodeSet: codeSet}
	}

	t.Run("LoaderInspector", func(t *testing.T) {
		inspector := resolveCodeSetInspector(new(StubInspectorLoader), new(StubInspectorResolver))

		err := sealCodeMap(t.Context(), inspector, newCodeMap("from-loader"))

		require.NoError(t, err, "The loader inspector should validate its registered code set")
	})

	t.Run("ResolverInspectorFallback", func(t *testing.T) {
		inspector := resolveCodeSetInspector(new(StubLoader), new(StubInspectorResolver))

		err := sealCodeMap(t.Context(), inspector, newCodeMap("from-resolver"))

		require.NoError(t, err, "The resolver inspector should validate when the loader is not enumerable")
	})

	t.Run("UnknownCodeSet", func(t *testing.T) {
		inspector := resolveCodeSetInspector(new(StubInspectorLoader), new(StubInspectorResolver))

		err := sealCodeMap(t.Context(), inspector, newCodeMap("unknown"))

		require.Error(t, err, "An enumerable catalog should reject an unknown code set")
		assert.ErrorIs(t, err, integration.ErrInvalidCodeMap(""), "An unknown code set should be an invalid code map")
	})

	t.Run("NoInspector", func(t *testing.T) {
		inspector := resolveCodeSetInspector(new(StubLoader), new(StubResolver))

		err := sealCodeMap(t.Context(), inspector, newCodeMap("free-form"))

		require.NoError(t, err, "A host without an inspector should preserve free-form code sets")
	})

	t.Run("ListError", func(t *testing.T) {
		inspector := resolveCodeSetInspector(new(FailingInspectorLoader), new(StubInspectorResolver))

		err := sealCodeMap(t.Context(), inspector, newCodeMap("from-resolver"))

		require.Error(t, err, "A catalog enumeration failure should reject the save")
		assert.ErrorIs(t, err, integration.ErrCodeSetCatalogFailed(""),
			"A catalog enumeration failure should classify as a catalog fault, not a raw host error or a verdict on the definition")
		assert.Contains(t, err.Error(), errCatalogUnavailable.Error(), "The underlying catalog fault should stay visible in the detail")
	})
}

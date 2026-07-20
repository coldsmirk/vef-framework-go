package resource

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/integration"
	"github.com/coldsmirk/vef-framework-go/mold"
)

var errListCodeSets = errors.New("list code sets failed")

type FailingInspectorLoader struct {
	StubLoader
}

func (*FailingInspectorLoader) ListCodeSets(context.Context) ([]mold.CodeSetInfo, error) {
	return nil, errListCodeSets
}

func (*FailingInspectorLoader) ListCodes(context.Context, string) ([]mold.CodeInfo, error) {
	return nil, nil
}

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

		require.ErrorIs(t, err, errListCodeSets, "A catalog enumeration failure should reject the save")
	})
}

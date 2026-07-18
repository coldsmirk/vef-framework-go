package resource

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/mold"
)

type StubLoader struct{}

func (*StubLoader) Load(context.Context, string) (map[string]string, error) {
	return map[string]string{}, nil
}

type StubResolver struct{}

func (*StubResolver) Resolve(context.Context, string, string) (string, error) {
	return "", nil
}

type StubInspectorLoader struct {
	StubLoader
}

func (*StubInspectorLoader) ListCodeSets(context.Context) ([]mold.CodeSetInfo, error) {
	return []mold.CodeSetInfo{{CodeSet: "from-loader", Name: "Loader"}}, nil
}

func (*StubInspectorLoader) ListCodes(context.Context, string) ([]mold.CodeInfo, error) {
	return nil, nil
}

type StubInspectorResolver struct {
	StubResolver
}

func (*StubInspectorResolver) ListCodeSets(context.Context) ([]mold.CodeSetInfo, error) {
	return []mold.CodeSetInfo{{CodeSet: "from-resolver", Name: "Resolver"}}, nil
}

func (*StubInspectorResolver) ListCodes(context.Context, string) ([]mold.CodeInfo, error) {
	return nil, nil
}

// TestNewCodeSetResource pins the inspector selection: the loader is asserted
// first because the mold module wraps loaders in its cached resolver, which
// would hide the enumeration capability behind the resolver seam.
func TestNewCodeSetResource(t *testing.T) {
	inspectorOf := func(loader mold.CodeSetLoader, resolver mold.CodeSetResolver) mold.CodeSetInspector {
		res, ok := NewCodeSetResource(loader, resolver).(*CodeSetResource)
		require.True(t, ok, "the constructor should return the concrete resource")

		return res.inspector
	}

	t.Run("NilSourcesReportUnsupported", func(t *testing.T) {
		assert.Nil(t, inspectorOf(nil, nil), "no host registration means no inspector")
	})

	t.Run("NonEnumerableSourcesReportUnsupported", func(t *testing.T) {
		assert.Nil(t, inspectorOf(new(StubLoader), new(StubResolver)), "sources without the inspector stay unsupported")
	})

	t.Run("LoaderInspectorWins", func(t *testing.T) {
		inspector := inspectorOf(new(StubInspectorLoader), new(StubInspectorResolver))
		require.NotNil(t, inspector, "an enumerable loader should light up the catalog")

		codeSets, err := inspector.ListCodeSets(t.Context())
		require.NoError(t, err, "the stub should enumerate")
		assert.Equal(t, "from-loader", codeSets[0].CodeSet, "the loader's inspector should take precedence")
	})

	t.Run("ResolverInspectorFallsBack", func(t *testing.T) {
		inspector := inspectorOf(new(StubLoader), new(StubInspectorResolver))
		require.NotNil(t, inspector, "an enumerable resolver should light up the catalog when the loader cannot")

		codeSets, err := inspector.ListCodeSets(t.Context())
		require.NoError(t, err, "the stub should enumerate")
		assert.Equal(t, "from-resolver", codeSets[0].CodeSet, "the resolver's inspector is the fallback")
	})
}

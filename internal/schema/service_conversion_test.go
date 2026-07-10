package schema

import (
	"testing"

	"ariga.io/atlas/sql/postgres"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	as "ariga.io/atlas/sql/schema"

	pkgschema "github.com/coldsmirk/vef-framework-go/schema"
)

func TestConvertIndexesPreservesUniqueKeyShape(t *testing.T) {
	t.Parallel()

	tenantID := &as.Column{Name: "tenant_id"}
	orderNo := &as.Column{Name: "order_no"}
	table := &as.Table{Indexes: []*as.Index{
		{
			Name:   "uk_order_active",
			Unique: true,
			Parts: []*as.IndexPart{
				{C: tenantID},
				{C: orderNo},
			},
			Attrs: []as.Attr{&postgres.IndexPredicate{P: "deleted_at IS NULL"}},
		},
		{
			Name:   "uk_order_expression",
			Unique: true,
			Parts:  []*as.IndexPart{{X: &as.RawExpr{X: "lower(order_no)"}}},
		},
	}}

	info := new(pkgschema.TableSchema)
	convertIndexes(table, info)
	require.Len(t, info.UniqueKeys, 2, "Both unique indexes should be exposed")
	assert.Equal(t, "deleted_at IS NULL", info.UniqueKeys[0].Predicate,
		"Partial-index predicate must survive schema conversion")
	assert.False(t, info.UniqueKeys[0].HasExpressions,
		"Plain composite unique index should remain column-backed")
	assert.True(t, info.UniqueKeys[1].HasExpressions,
		"Expression unique index must not masquerade as a column key")
}

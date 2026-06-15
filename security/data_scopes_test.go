package security

import (
	"database/sql"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/sqliteshim"

	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// newTestBunForScopes creates an in-memory SQLite bun.DB for schema table construction.
func newTestBunForScopes(t *testing.T) *bun.DB {
	t.Helper()

	sqldb, err := sql.Open(sqliteshim.ShimName, ":memory:")
	require.NoError(t, err, "Should open SQLite in-memory database")

	t.Cleanup(func() {
		require.NoError(t, sqldb.Close(), "Should close SQLite in-memory database")
	})

	return bun.NewDB(sqldb, sqlitedialect.New())
}

// tableFor returns the bun schema.Table for the given model type.
func tableFor(db *bun.DB, model any) *orm.Table {
	return db.Table(reflect.TypeOf(model).Elem())
}

// TestAllDataScope tests AllDataScope functionality.
func TestAllDataScope(t *testing.T) {
	scope := NewAllDataScope()

	t.Run("Key", func(t *testing.T) {
		assert.Equal(t, "all", scope.Key(), "Should return 'all'")
	})

	t.Run("Priority", func(t *testing.T) {
		assert.Equal(t, PriorityAll, scope.Priority(), "Should return PriorityAll")
	})

	t.Run("SupportsAlwaysTrue", func(t *testing.T) {
		assert.True(t, scope.Supports(nil, nil), "Should always return true")
		assert.True(t, scope.Supports(NewUser("u1", "Alice"), nil), "Should return true for any principal")
	})

	t.Run("ApplyAlwaysNil", func(t *testing.T) {
		assert.NoError(t, scope.Apply(nil, nil), "Should always return nil")
	})
}

// TestNewAllDataScope tests NewAllDataScope constructor.
func TestNewAllDataScope(t *testing.T) {
	scope := NewAllDataScope()

	_, ok := scope.(*AllDataScope)
	assert.True(t, ok, "Should return *AllDataScope")
}

// TestSelfDataScope tests SelfDataScope Key and Priority.
func TestSelfDataScope(t *testing.T) {
	t.Run("Key", func(t *testing.T) {
		scope := NewSelfDataScope("")
		assert.Equal(t, "self", scope.Key(), "Should return 'self'")
	})

	t.Run("Priority", func(t *testing.T) {
		scope := NewSelfDataScope("")
		assert.Equal(t, PrioritySelf, scope.Priority(), "Should return PrioritySelf")
	})
}

// TestNewSelfDataScope tests NewSelfDataScope constructor defaults.
func TestNewSelfDataScope(t *testing.T) {
	t.Run("EmptyColumnUsesDefault", func(t *testing.T) {
		scope := NewSelfDataScope("").(*SelfDataScope)
		assert.Equal(t, "created_by", scope.createdByColumn, "Should default to 'created_by'")
	})

	t.Run("CustomColumn", func(t *testing.T) {
		scope := NewSelfDataScope("creator_id").(*SelfDataScope)
		assert.Equal(t, "creator_id", scope.createdByColumn, "Should use custom column")
	})
}

// Models for schema table construction in SelfDataScope tests.
type selfScopeModel struct {
	bun.BaseModel `bun:"table:self_scope_model"`

	ID        string `bun:"id,pk"`
	CreatedBy string `bun:"created_by"`
}

type noCreatedByModel struct {
	bun.BaseModel `bun:"table:no_created_by_model"`

	ID   string `bun:"id,pk"`
	Name string `bun:"name"`
}

type customCreatorModel struct {
	bun.BaseModel `bun:"table:custom_creator_model"`

	ID        string `bun:"id,pk"`
	CreatorID string `bun:"creator_id"`
}

// TestSelfDataScopeSupports tests SelfDataScope.Supports behavior.
func TestSelfDataScopeSupports(t *testing.T) {
	db := newTestBunForScopes(t)

	t.Run("ReturnsTrueWhenTableHasCreatedByColumn", func(t *testing.T) {
		scope := NewSelfDataScope("")
		table := tableFor(db, (*selfScopeModel)(nil))

		assert.True(t, scope.Supports(NewUser("u1", "Alice"), table), "Should support table with created_by column")
	})

	t.Run("ReturnsFalseWhenTableLacksCreatedByColumn", func(t *testing.T) {
		scope := NewSelfDataScope("")
		table := tableFor(db, (*noCreatedByModel)(nil))

		assert.False(t, scope.Supports(NewUser("u1", "Alice"), table), "Should not support table without created_by column")
	})

	t.Run("ReturnsTrueForCustomColumn", func(t *testing.T) {
		scope := NewSelfDataScope("creator_id")
		table := tableFor(db, (*customCreatorModel)(nil))

		assert.True(t, scope.Supports(NewUser("u1", "Alice"), table), "Should support table with custom creator column")
	})

	t.Run("ReturnsFalseWhenCustomColumnAbsent", func(t *testing.T) {
		scope := NewSelfDataScope("creator_id")
		table := tableFor(db, (*selfScopeModel)(nil))

		assert.False(t, scope.Supports(NewUser("u1", "Alice"), table), "Should not support table missing the custom creator column")
	})
}

// TestSelfDataScopeApply tests SelfDataScope.Apply behavior.
// Apply must add a WHERE clause filtering on the creator column bound to
// principal.ID; we materialize the query to SQL and assert both the column and
// the bound principal value appear so a wrong column or value is caught.
func TestSelfDataScopeApply(t *testing.T) {
	t.Run("FiltersOnDefaultCreatedByColumn", func(t *testing.T) {
		db := testx.NewTestDB(t)
		scope := NewSelfDataScope("")
		principal := NewUser("user-42", "Alice")

		query := db.NewSelect().Model((*selfScopeModel)(nil))
		err := scope.Apply(principal, query)
		require.NoError(t, err, "Apply should return nil for default created_by column")

		sql := query.String()
		assert.Contains(t, sql, "created_by", "WHERE clause should filter on the default created_by column")
		assert.Contains(t, sql, "user-42", "WHERE clause should bind the principal ID")
	})

	t.Run("FiltersOnCustomColumn", func(t *testing.T) {
		db := testx.NewTestDB(t)
		scope := NewSelfDataScope("creator_id")
		principal := NewUser("user-99", "Bob")

		query := db.NewSelect().Model((*customCreatorModel)(nil))
		err := scope.Apply(principal, query)
		require.NoError(t, err, "Apply should return nil for custom creator column")

		sql := query.String()
		assert.Contains(t, sql, "creator_id", "WHERE clause should filter on the custom creator column")
		assert.Contains(t, sql, "user-99", "WHERE clause should bind the principal ID")
		assert.NotContains(t, sql, "created_by", "WHERE clause should not reference the default column when a custom one is set")
	})
}

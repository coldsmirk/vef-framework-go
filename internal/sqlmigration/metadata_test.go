package sqlmigration

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
)

func TestLoadTableMetadata(t *testing.T) {
	testx.ForEachDB(t, func(t *testing.T, env *testx.DBEnv) {
		ddl := `CREATE TABLE smig_meta (
    id VARCHAR(32) NOT NULL PRIMARY KEY,
    payload TEXT NOT NULL,
    occurred_at_unix_ms BIGINT,
    attempt_count INTEGER NOT NULL
);
CREATE UNIQUE INDEX uk_smig_meta__payload_occurred
    ON smig_meta(payload, occurred_at_unix_ms)`
		if env.DS.Kind == config.MySQL {
			// MySQL cannot index an unbounded TEXT column without a prefix
			// length; a bounded VARCHAR keeps the index shape identical.
			ddl = `CREATE TABLE smig_meta (
    id VARCHAR(32) NOT NULL PRIMARY KEY,
    payload VARCHAR(64) NOT NULL,
    occurred_at_unix_ms BIGINT,
    attempt_count INTEGER NOT NULL
);
CREATE UNIQUE INDEX uk_smig_meta__payload_occurred
    ON smig_meta(payload, occurred_at_unix_ms)`
		}

		_, err := env.DB.NewRaw(ddl).Exec(env.Ctx)
		require.NoError(t, err, "The metadata fixture table should be created for %s", env.DS.Kind)

		columns, err := LoadTableColumns(env.Ctx, env.DB, env.DS.Kind, "smig_meta")
		require.NoError(t, err, "Loading columns should succeed for %s", env.DS.Kind)

		id, exists := columns["id"]
		require.True(t, exists, "The id column must be observed for %s", env.DS.Kind)
		assert.Equal(t, ColumnVarchar, id.Kind, "A VARCHAR declaration must normalize for %s", env.DS.Kind)
		assert.False(t, id.Nullable, "A NOT NULL declaration must be observed for %s", env.DS.Kind)

		if env.DS.Kind != config.SQLite {
			assert.Equal(t, 32, id.MaxLength, "The character bound must be observed for %s", env.DS.Kind)
		}

		occurred, exists := columns["occurred_at_unix_ms"]
		require.True(t, exists, "The timeline column must be observed for %s", env.DS.Kind)
		assert.Equal(t, ColumnInt64, occurred.Kind, "A BIGINT declaration must normalize for %s", env.DS.Kind)
		assert.True(t, occurred.Nullable, "A nullable declaration must be observed for %s", env.DS.Kind)

		attempts, exists := columns["attempt_count"]
		require.True(t, exists, "The counter column must be observed for %s", env.DS.Kind)
		assert.Equal(t, ColumnInt32, attempts.Kind, "An INTEGER declaration must normalize for %s", env.DS.Kind)

		indexes, err := LoadTableIndexes(env.Ctx, env.DB, env.DS.Kind, "smig_meta")
		require.NoError(t, err, "Loading indexes should succeed for %s", env.DS.Kind)

		assert.True(t, containsIndex(indexes, []string{"payload", "occurred_at_unix_ms"}, true),
			"The composite unique index must be observed with its exact column sequence for %s", env.DS.Kind)
		assert.True(t, containsIndex(indexes, []string{"id"}, true),
			"The primary key must be observed as a unique index capability for %s", env.DS.Kind)
		assert.False(t, containsIndex(indexes, []string{"occurred_at_unix_ms", "payload"}, true),
			"A reversed column sequence must not match for %s", env.DS.Kind)
	})
}

func containsIndex(indexes []Index, columns []string, unique bool) bool {
	return slices.ContainsFunc(indexes, func(index Index) bool {
		return index.Unique == unique && slices.Equal(index.Columns, columns)
	})
}

// indexShapeDDL returns the dialect's partial, prefix and expression index
// statements plus one ordinary index as the control. Each dialect supports a
// different subset: MySQL has no partial indexes, and only MySQL indexes a
// column prefix.
func indexShapeDDL(kind config.DBKind) []string {
	plain := "CREATE INDEX ix_smig_shape__plain ON smig_shape (attempt_count)"

	switch kind {
	case config.MySQL:
		return []string{
			"CREATE INDEX ix_smig_shape__prefix ON smig_shape (payload(10))",
			"CREATE INDEX ix_smig_shape__expr ON smig_shape ((LOWER(payload)))",
			plain,
		}

	default:
		return []string{
			"CREATE INDEX ix_smig_shape__partial ON smig_shape (payload) WHERE attempt_count > 0",
			"CREATE INDEX ix_smig_shape__expr ON smig_shape (LOWER(payload))",
			plain,
		}
	}
}

func TestLoadTableIndexesRejectsIncompleteIndexes(t *testing.T) {
	testx.ForEachDB(t, func(t *testing.T, env *testx.DBEnv) {
		// MySQL cannot index an unbounded TEXT column without a prefix length.
		payloadType := "TEXT"
		if env.DS.Kind == config.MySQL {
			payloadType = "VARCHAR(64)"
		}

		_, err := env.DB.NewRaw(`CREATE TABLE smig_shape (
    id VARCHAR(32) NOT NULL PRIMARY KEY,
    payload ` + payloadType + ` NOT NULL,
    attempt_count INTEGER NOT NULL
)`).Exec(env.Ctx)
		require.NoError(t, err, "The index-shape fixture table should be created for %s", env.DS.Kind)

		for _, ddl := range indexShapeDDL(env.DS.Kind) {
			_, err := env.DB.NewRaw(ddl).Exec(env.Ctx)
			require.NoError(t, err, "The fixture index %q should be created for %s", ddl, env.DS.Kind)
		}

		indexes, err := LoadTableIndexes(env.Ctx, env.DB, env.DS.Kind, "smig_shape")
		require.NoError(t, err, "Loading indexes should succeed for %s", env.DS.Kind)

		assert.False(t, containsIndex(indexes, []string{"payload"}, false),
			"A prefix, partial or expression index covers only part of the column and must satisfy no capability for %s",
			env.DS.Kind)
		assert.True(t, containsIndex(indexes, []string{"attempt_count"}, false),
			"An ordinary index on the same table must still be observed for %s", env.DS.Kind)
		assert.True(t, containsIndex(indexes, []string{"id"}, true),
			"The primary key must still be observed for %s", env.DS.Kind)
	})
}

func TestNormalizeColumnTypeRejectsUnsignedMySQLIntegers(t *testing.T) {
	tests := []struct {
		name       string
		dataType   string
		columnType string
		signedKind ColumnKind
	}{
		{name: "BigInt", dataType: "bigint", columnType: "bigint unsigned", signedKind: ColumnInt64},
		{name: "Int", dataType: "int", columnType: "int unsigned", signedKind: ColumnInt32},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kind, _ := normalizeColumnType(config.MySQL, columnMetadataRow{
				DataType:   tt.dataType,
				ColumnType: tt.columnType,
			})
			assert.NotEqual(t, tt.signedKind, kind,
				"An unsigned MySQL integer must not normalize to its signed schema kind")
		})
	}
}

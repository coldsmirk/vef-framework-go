package sqlmigration

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
)

func TestAddedColumnStatements(t *testing.T) {
	column := AddedColumn{Table: "t", Name: "c", Definition: "INTEGER NOT NULL DEFAULT 0", Comment: "Pass Count"}

	tests := []struct {
		name   string
		column AddedColumn
		kind   config.DBKind
		want   []string
	}{
		{"MySQLInlinesComment", column, config.MySQL, []string{"ALTER TABLE t ADD COLUMN c INTEGER NOT NULL DEFAULT 0 COMMENT 'Pass Count'"}},
		{"PostgresCommentsSeparately", column, config.Postgres, []string{
			"ALTER TABLE t ADD COLUMN c INTEGER NOT NULL DEFAULT 0",
			"COMMENT ON COLUMN t.c IS 'Pass Count'",
		}},
		{"SQLiteHasNoComments", column, config.SQLite, []string{"ALTER TABLE t ADD COLUMN c INTEGER NOT NULL DEFAULT 0"}},
		{"EmptyCommentAddsOnly", AddedColumn{Table: "t", Name: "c", Definition: "INTEGER"}, config.Postgres, []string{"ALTER TABLE t ADD COLUMN c INTEGER"}},
		{
			"QuoteInCommentIsEscaped",
			AddedColumn{Table: "t", Name: "c", Definition: "INTEGER", Comment: "Owner's"},
			config.MySQL,
			[]string{"ALTER TABLE t ADD COLUMN c INTEGER COMMENT 'Owner''s'"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.column.Statements(tt.kind), "The dialect's DDL should add the column the way its scripts declare it")
		})
	}
}

func TestRunAddsMissingColumns(t *testing.T) {
	testx.ForEachDB(t, func(t *testing.T, env *testx.DBEnv) {
		_, err := env.DB.NewRaw("CREATE TABLE smig_added (id VARCHAR(32) NOT NULL PRIMARY KEY)").Exec(env.Ctx)
		require.NoError(t, err, "The fixture table should create")
		_, err = env.DB.NewRaw("INSERT INTO smig_added (id) VALUES ('row-1')").Exec(env.Ctx)
		require.NoError(t, err, "The fixture row should insert")

		added := []AddedColumn{
			{Table: "smig_added", Name: "pass_count", Definition: "INTEGER NOT NULL DEFAULT 0", Comment: "Pass Count"},
			{Table: "smig_added", Name: "label", Definition: "VARCHAR(64)"},
			// A table the script has yet to create is left to that script.
			{Table: "smig_not_created", Name: "extra", Definition: "INTEGER"},
		}
		plan := Plan{
			Label:          "sqlmigration added columns",
			Kind:           env.DS.Kind,
			ExpectedTables: []string{"smig_added"},
			AddedColumns:   added,
		}

		missing, err := MissingColumns(env.Ctx, env.DB, env.DS.Kind, added)
		require.NoError(t, err, "Missing columns should be computed")
		assert.Equal(t, added[:2], missing, "Only columns of existing tables should be reported, in order")

		require.NoError(t, Run(env.Ctx, env.DB, plan), "Run should add the missing columns")
		require.NoError(t, Run(env.Ctx, env.DB, plan), "A second Run should find nothing to add")

		missing, err = MissingColumns(env.Ctx, env.DB, env.DS.Kind, added)
		require.NoError(t, err, "Missing columns should be computed after the upgrade")
		assert.Empty(t, missing, "Every column of an existing table should now be present")

		var count int
		require.NoError(t, env.DB.NewRaw("SELECT pass_count FROM smig_added WHERE id = 'row-1'").Scan(env.Ctx, &count),
			"The existing row should read the added column")
		assert.Zero(t, count, "The existing row should take the column default")
	})
}

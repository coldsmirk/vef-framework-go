package migration

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/sqlmigration"
)

// TestMigrationScripts is a smoke check that this module's embedded DDL resolves
// through the shared sqlmigration.LoadScript loader (the same one Migrate uses)
// and that every expected table is created in EVERY supported dialect — so the
// three scripts and expectedTables can never silently drift apart.
func TestMigrationScripts(t *testing.T) {
	for _, kind := range []config.DBKind{config.Postgres, config.MySQL, config.SQLite} {
		t.Run(string(kind), func(t *testing.T) {
			sql, err := sqlmigration.LoadScript(scripts, kind)
			require.NoErrorf(t, err, "Should load %s migration SQL", kind)

			for _, table := range expectedTables {
				// Trailing \b is a word boundary so apv_form_table does not
				// satisfy the assertion for apv_form_table_column (its prefix).
				pattern := regexp.MustCompile("CREATE TABLE IF NOT EXISTS " + regexp.QuoteMeta(table) + "\\b")
				assert.Regexpf(t, pattern, sql, "%s script should create table %s", kind, table)
			}
		})
	}

	t.Run("UnsupportedKind", func(t *testing.T) {
		_, err := sqlmigration.LoadScript(scripts, "unknown")
		require.Error(t, err, "Should error for unsupported database kind")
		assert.Contains(t, err.Error(), "unsupported database kind", "Should include kind info in error")
	})
}

// formFieldsColumn matches a form_fields column definition — the column name
// followed by any JSON/text column type — inside the apv_flow_version CREATE
// TABLE. The (?s) flag lets .*? span the intervening column lines, and \s
// tolerance keeps it resilient to reformatting. Anchoring on the CREATE TABLE
// header (with a non-greedy reach to the first form_fields token) scopes the
// assertion to the table body: the postgres COMMENT ON COLUMN line naming the
// same column appears only after the statement closes.
var formFieldsColumn = regexp.MustCompile(`(?s)CREATE TABLE IF NOT EXISTS\s+apv_flow_version\s*\(.*?\bform_fields\b\s+(?:JSONB|JSON|TEXT)`)

// TestFlowVersionFormFieldsColumn guards the form_fields column: every dialect
// script must define it on apv_flow_version, so a dialect that forgets the
// deploy-derived field list (form_schema stored verbatim, form_fields the flat
// projection the framework reads) fails here instead of at first deploy on that
// database.
func TestFlowVersionFormFieldsColumn(t *testing.T) {
	for _, kind := range []config.DBKind{config.Postgres, config.MySQL, config.SQLite} {
		t.Run(string(kind), func(t *testing.T) {
			sql, err := sqlmigration.LoadScript(scripts, kind)
			require.NoErrorf(t, err, "Should load %s migration SQL", kind)

			assert.Regexpf(t, formFieldsColumn, sql,
				"%s apv_flow_version CREATE TABLE must define a form_fields column", kind)
		})
	}
}

func TestBusinessProjectionSchemaContract(t *testing.T) {
	requiredColumns := map[string][]string{
		"apv_flow_version": {"business_binding"},
		"apv_instance":     {"business_projection_id"},
		"apv_business_projection": {
			"owner_instance_id",
			"applied_owner_instance_id",
			"target_hash",
			"consistency",
			"binding",
			"record_key",
			"desired_status",
			"desired_revision",
			"applied_revision",
			"status",
			"next_attempt_at",
			"lease_until",
		},
	}

	for _, kind := range []config.DBKind{config.Postgres, config.MySQL, config.SQLite} {
		t.Run(string(kind), func(t *testing.T) {
			sql, err := sqlmigration.LoadScript(scripts, kind)
			require.NoErrorf(t, err, "Should load %s migration SQL", kind)

			for table, columns := range requiredColumns {
				for _, column := range columns {
					pattern := regexp.MustCompile(`(?s)CREATE TABLE IF NOT EXISTS\s+` +
						regexp.QuoteMeta(table) + `\s*\([^;]*\b` + regexp.QuoteMeta(column) + `\b`)
					assert.Regexpf(t, pattern, sql,
						"%s table %s should define business projection column %s", kind, table, column)
				}
			}
		})
	}
}

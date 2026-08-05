package migration

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/id"
	"github.com/coldsmirk/vef-framework-go/internal/sqlmigration"
	"github.com/coldsmirk/vef-framework-go/internal/storage/store"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/storage"
	"github.com/coldsmirk/vef-framework-go/timex"
)

var dialects = []config.DBKind{config.Postgres, config.MySQL, config.SQLite}

// TestMigrationScripts checks that this module's embedded DDL resolves
// through the shared sqlmigration.LoadScript loader (the same one Migrate
// uses) and that every expected table is created in EVERY supported
// dialect — so the three scripts and expectedTables cannot silently
// drift apart.
func TestMigrationScripts(t *testing.T) {
	for _, kind := range dialects {
		t.Run(string(kind), func(t *testing.T) {
			sql, err := sqlmigration.LoadScript(scripts, kind)
			require.NoErrorf(t, err, "Should load %s migration SQL", kind)

			for _, table := range expectedTables {
				// The trailing word boundary stops sys_storage_upload_claim
				// from satisfying the assertion for a table whose name is
				// its prefix.
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

// TestMigrationScriptsAreIdempotent is the regression fence for the
// whole script re-running on an existing deployment. sqlmigration.Run
// re-executes the entire script whenever ANY expected table is missing,
// so adding one table replays every statement against a database that
// already has the others. CREATE TABLE IF NOT EXISTS survives that;
// a bare CREATE INDEX does not.
//
// Postgres and SQLite express the guard as CREATE INDEX IF NOT EXISTS.
// MySQL has no such form, so its indexes must be declared inline in the
// CREATE TABLE body, where CREATE TABLE IF NOT EXISTS skips them.
func TestMigrationScriptsAreIdempotent(t *testing.T) {
	standaloneIndex := regexp.MustCompile(`(?i)CREATE\s+INDEX\s+(?:IF\s+NOT\s+EXISTS\s+)?`)
	guardedIndex := regexp.MustCompile(`(?i)CREATE\s+INDEX\s+IF\s+NOT\s+EXISTS\s+`)

	for _, kind := range dialects {
		t.Run(string(kind), func(t *testing.T) {
			raw, err := sqlmigration.LoadScript(scripts, kind)
			require.NoErrorf(t, err, "Should load %s migration SQL", kind)

			// The scripts explain this very rule in their comments, so
			// match statements only.
			sql := stripComments(raw)
			total := len(standaloneIndex.FindAllString(sql, -1))

			if kind == config.MySQL {
				assert.Zero(t, total,
					"MySQL has no CREATE INDEX IF NOT EXISTS; every index must be declared inline in the CREATE TABLE body")
				assert.Contains(t, sql, "INDEX idx_sys_storage_file__status_created_at",
					"MySQL should declare the registry's status index inline")

				return
			}

			assert.Equal(t, total, len(guardedIndex.FindAllString(sql, -1)),
				"Every %s CREATE INDEX must carry IF NOT EXISTS or a script re-run fails on the existing index", kind)
			assert.Positive(t, total, "The %s script should create indexes", kind)
		})
	}
}

// TestFileRegistryColumns pins the registry columns the framework reads
// back through storage.FileRecord. The module never ALTERs a table, so a
// column missing from one dialect can only be fixed by recreating the
// table on that database — this catches it at build time instead.
func TestFileRegistryColumns(t *testing.T) {
	columns := []string{
		"object_key", "original_filename", "content_type", "size",
		"public", "status", "started_at", "claimed_at", "deleted_at", "delete_reason",
	}

	for _, kind := range dialects {
		t.Run(string(kind), func(t *testing.T) {
			sql, err := sqlmigration.LoadScript(scripts, kind)
			require.NoErrorf(t, err, "Should load %s migration SQL", kind)

			body := fileTableBody(t, sql)

			for _, column := range columns {
				assert.Regexpf(t, regexp.MustCompile(`(?m)^\s+`+regexp.QuoteMeta(column)+`\s`), body,
					"%s script should define sys_storage_file.%s", kind, column)
			}
		})
	}
}

// TestFileRegistryBackfill guards the one-time recovery of files that
// finalized before the registry existed: unadopted claims still carry
// their metadata and are never swept, so every dialect must replay them
// into the new table — under a conflict guard, because the script can
// re-run.
func TestFileRegistryBackfill(t *testing.T) {
	guards := map[config.DBKind]string{
		config.Postgres: "ON CONFLICT DO NOTHING",
		config.MySQL:    "INSERT IGNORE INTO sys_storage_file",
		config.SQLite:   "INSERT OR IGNORE INTO sys_storage_file",
	}

	for _, kind := range dialects {
		t.Run(string(kind), func(t *testing.T) {
			raw, err := sqlmigration.LoadScript(scripts, kind)
			require.NoErrorf(t, err, "Should load %s migration SQL", kind)

			// Comments stripped: each script explains its own conflict
			// guard in prose, and Postgres' guard string appears verbatim
			// there — matching it would fence nothing.
			sql := stripComments(raw)

			assert.Contains(t, sql, "FROM sys_storage_upload_claim",
				"%s script should backfill the registry from finalized claims", kind)
			assert.Contains(t, sql, guards[kind],
				"%s backfill must tolerate a script re-run", kind)
		})
	}
}

// TestMigrationScriptReRunRecoversUnadoptedClaims executes the whole
// SQLite script a second time against a live database — exactly what
// sqlmigration.Run does on an existing deployment the moment a new table
// joins expectedTables — and proves the two properties that replay must
// have: it does not error, and it recovers the metadata of uploads that
// finalized before the registry existed.
func TestMigrationScriptReRunRecoversUnadoptedClaims(t *testing.T) {
	ctx := context.Background()
	db := testx.NewTestDB(t)

	require.NoError(t, Migrate(ctx, db, config.SQLite), "First migration should succeed")

	// A claim that finalized before this release: uploaded, unadopted,
	// and never swept (ListExpired skips 'uploaded'), so its metadata is
	// still recoverable.
	claim := &store.UploadClaim{
		ID:               id.GenerateUUID(),
		Key:              "priv/2026/01/01/legacy.pdf",
		Size:             2048,
		ContentType:      "application/pdf",
		OriginalFilename: "旧报告.pdf",
		CreatedBy:        "legacy-uploader",
		Status:           store.ClaimStatusUploaded,
		ExpiresAt:        timex.Now().AddHours(-100),
		CreatedAt:        timex.Now().AddHours(-100),
	}

	files := store.NewFileStore(db)
	require.NoError(t, store.NewClaimStore(db, files).Create(ctx, claim), "Legacy claim creation should succeed")

	found, err := files.Lookup(ctx, []string{claim.Key})
	require.NoError(t, err, "Registry lookup should succeed")
	require.Empty(t, found, "The legacy fixture must start without a record for this test to mean anything")

	sql, err := sqlmigration.LoadScript(scripts, config.SQLite)
	require.NoError(t, err, "Should load the SQLite script")

	_, err = db.NewRaw(sql).Exec(ctx)
	require.NoError(t, err, "Re-executing the whole script must be safe on a populated database")

	found, err = files.Lookup(ctx, []string{claim.Key})
	require.NoError(t, err, "Registry lookup should succeed after the replay")
	require.Len(t, found, 1, "The backfill should recover the unadopted claim")

	assert.Equal(t, "旧报告.pdf", found[claim.Key].OriginalFilename, "The backfill should carry the original filename over")
	assert.Equal(t, claim.CreatedBy, found[claim.Key].CreatedBy, "The backfill should preserve the original uploader")
	assert.Equal(t, storage.FileStatusUploaded, found[claim.Key].Status, "A backfilled file is uploaded, not yet claimed")

	// And once more, to prove the conflict guard holds.
	_, err = db.NewRaw(sql).Exec(ctx)
	require.NoError(t, err, "A third execution must not collide with the rows the second one backfilled")

	found, err = files.Lookup(ctx, []string{claim.Key})
	require.NoError(t, err, "Registry lookup should succeed after the second replay")
	assert.Len(t, found, 1, "Replaying the backfill must not duplicate records")
}

// fileTableBody returns the CREATE TABLE body for sys_storage_file so
// column assertions cannot be satisfied by a COMMENT ON line or by
// another table that happens to share a column name.
func fileTableBody(t *testing.T, sql string) string {
	t.Helper()

	const header = "CREATE TABLE IF NOT EXISTS sys_storage_file ("

	start := strings.Index(sql, header)
	require.NotEqual(t, -1, start, "Script should create sys_storage_file")

	// Closing paren at column 0. MySQL follows it with a table COMMENT,
	// the others with a bare semicolon, so match the paren alone.
	end := strings.Index(sql[start:], "\n)")
	require.NotEqual(t, -1, end, "sys_storage_file definition should terminate")

	return sql[start : start+end]
}

// stripComments removes -- line comments so assertions about statements
// are not satisfied by prose that discusses them.
func stripComments(sql string) string {
	lines := strings.Split(sql, "\n")
	kept := make([]string, 0, len(lines))

	for _, line := range lines {
		if idx := strings.Index(line, "--"); idx >= 0 {
			line = line[:idx]
		}

		kept = append(kept, line)
	}

	return strings.Join(kept, "\n")
}

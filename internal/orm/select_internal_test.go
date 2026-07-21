package orm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/database"
	"github.com/coldsmirk/vef-framework-go/logx"
)

// resetLockClauseWarnings clears the process-wide warn-once state so the test
// observes the first warning for every dialect and lock mode it exercises,
// whatever other tests in this package already emitted.
func resetLockClauseWarnings() {
	warnedLockClauses.Range(func(key, _ any) bool {
		warnedLockClauses.Delete(key)

		return true
	})
}

// TestForLockUnsupportedDialectWarnsOncePerMode pins that dropping a locking
// clause the dialect cannot honor is reported once per process. Whether a
// dialect supports row-level locking is static, while locking SELECTs are
// issued on every tick of the framework's polling loops — warning per call
// would bury every other log line without adding information.
func TestForLockUnsupportedDialectWarnsOncePerMode(t *testing.T) {
	rawDB, err := database.Open(config.DataSourceConfig{Kind: config.SQLite})
	require.NoError(t, err, "Opening the SQLite test database should succeed")

	t.Cleanup(func() { _ = rawDB.Close() })

	db, err := Open(rawDB, config.SQLite)
	require.NoError(t, err, "Opening the ORM over SQLite should succeed")

	resetLockClauseWarnings()
	t.Cleanup(resetLockClauseWarnings)

	recorder := new(LevelRecordingLogger)
	restore := logger
	logger = recorder

	t.Cleanup(func() { logger = restore })

	for range 5 {
		db.NewSelect().ForUpdate()
	}

	assert.Equal(t, []logx.Level{logx.LevelWarn}, recorder.levels,
		"A dialect without row-level locking must warn once per process, not once per query")

	for range 5 {
		db.NewSelect().ForShare()
	}

	assert.Equal(t, []logx.Level{logx.LevelWarn, logx.LevelWarn}, recorder.levels,
		"Every distinct lock mode must still get its own first warning")
}

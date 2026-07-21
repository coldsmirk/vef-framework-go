package orm

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/database"
	"github.com/coldsmirk/vef-framework-go/internal/orm/sqlguard"
	"github.com/coldsmirk/vef-framework-go/logx"
)

// LevelRecordingLogger captures the level of every log call so tests can assert
// routing and emission counts without a real logging backend.
type LevelRecordingLogger struct {
	levels []logx.Level
}

func (l *LevelRecordingLogger) record(level logx.Level) { l.levels = append(l.levels, level) }

func (l *LevelRecordingLogger) Named(string) logx.Logger       { return l }
func (l *LevelRecordingLogger) WithCallerSkip(int) logx.Logger { return l }
func (*LevelRecordingLogger) Enabled(logx.Level) bool          { return true }
func (*LevelRecordingLogger) Sync()                            {}
func (l *LevelRecordingLogger) Debug(string)                   { l.record(logx.LevelDebug) }
func (l *LevelRecordingLogger) Debugf(string, ...any)          { l.record(logx.LevelDebug) }
func (l *LevelRecordingLogger) Info(string)                    { l.record(logx.LevelInfo) }
func (l *LevelRecordingLogger) Infof(string, ...any)           { l.record(logx.LevelInfo) }
func (l *LevelRecordingLogger) Warn(string)                    { l.record(logx.LevelWarn) }
func (l *LevelRecordingLogger) Warnf(string, ...any)           { l.record(logx.LevelWarn) }
func (l *LevelRecordingLogger) Error(string)                   { l.record(logx.LevelError) }
func (l *LevelRecordingLogger) Errorf(string, ...any)          { l.record(logx.LevelError) }
func (l *LevelRecordingLogger) Panic(string)                   { l.record(logx.LevelPanic) }
func (l *LevelRecordingLogger) Panicf(string, ...any)          { l.record(logx.LevelPanic) }

// TestSQLGuard tests SQL guard integration through the orm query hook. The
// GoSQLX parser handles bun's default double-quoted identifiers, so the guard
// blocks both raw SQL and the bun-generated quoted DDL emitted by the typed
// builders (see the TypedDropBlocked subtest). Each subtest opens its own data
// source and closes it via t.Cleanup so the shared in-memory SQLite database is
// fresh between subtests.
func TestSQLGuard(t *testing.T) {
	ctx := context.Background()

	newGuardedDB := func(t *testing.T, enableGuard bool) DB {
		t.Helper()

		rawDB, err := database.Open(config.DataSourceConfig{Kind: config.SQLite})
		require.NoError(t, err, "Database.Open should succeed")

		t.Cleanup(func() { _ = rawDB.Close() })

		db, err := Open(rawDB, config.SQLite, WithSQLGuard(enableGuard))
		require.NoError(t, err, "ORM open should succeed")

		_, err = db.NewRaw("CREATE TABLE IF NOT EXISTS test_guard (id INTEGER PRIMARY KEY, name TEXT)").Exec(ctx)
		require.NoError(t, err, "Creating test table should succeed")

		return db
	}

	t.Run("DropStatementBlocked", func(t *testing.T) {
		db := newGuardedDB(t, true)

		_, err := db.NewRaw("DROP TABLE test_guard").Exec(ctx)
		require.Error(t, err, "DROP should be blocked by SQL guard")
		require.ErrorIs(t, err, context.Canceled, "Blocked query should cancel the context")

		var count int
		require.NoError(t, db.NewRaw("SELECT COUNT(*) FROM test_guard").Scan(ctx, &count),
			"Table should still exist after blocked DROP")
	})

	t.Run("TypedDropBlocked", func(t *testing.T) {
		db := newGuardedDB(t, true)

		// The typed builder emits a bun-quoted identifier (DROP TABLE "test_guard").
		// This pins that the guard parses and blocks bun-generated quoted DDL, not
		// only hand-written unquoted raw SQL.
		_, err := db.NewDropTable().Table("test_guard").Exec(ctx)
		require.Error(t, err, "Typed DROP should be blocked by SQL guard")
		require.ErrorIs(t, err, context.Canceled, "Blocked query should cancel the context")

		var count int
		require.NoError(t, db.NewRaw("SELECT COUNT(*) FROM test_guard").Scan(ctx, &count),
			"Table should still exist after blocked typed DROP")
	})

	t.Run("TruncateStatementBlocked", func(t *testing.T) {
		db := newGuardedDB(t, true)

		_, err := db.NewRaw("INSERT INTO test_guard (name) VALUES ('test')").Exec(ctx)
		require.NoError(t, err, "Insert should succeed")

		_, err = db.NewRaw("TRUNCATE TABLE test_guard").Exec(ctx)
		require.Error(t, err, "TRUNCATE should be blocked by SQL guard")
		require.ErrorIs(t, err, context.Canceled, "Blocked query should cancel the context")

		var count int
		require.NoError(t, db.NewRaw("SELECT COUNT(*) FROM test_guard").Scan(ctx, &count),
			"Count query should succeed after blocked TRUNCATE")
		require.Equal(t, 1, count, "Data should still exist after blocked TRUNCATE")
	})

	t.Run("DeleteWithoutWhereBlocked", func(t *testing.T) {
		db := newGuardedDB(t, true)

		_, err := db.NewRaw("INSERT INTO test_guard (name) VALUES ('test')").Exec(ctx)
		require.NoError(t, err, "Insert should succeed")

		_, err = db.NewRaw("DELETE FROM test_guard").Exec(ctx)
		require.Error(t, err, "DELETE without WHERE should be blocked by SQL guard")
		require.ErrorIs(t, err, context.Canceled, "Blocked query should cancel the context")

		var count int
		require.NoError(t, db.NewRaw("SELECT COUNT(*) FROM test_guard").Scan(ctx, &count),
			"Count query should succeed after blocked DELETE")
		require.Equal(t, 1, count, "Data should still exist after blocked DELETE without WHERE")
	})

	t.Run("DeleteWithWhereAllowed", func(t *testing.T) {
		db := newGuardedDB(t, true)

		_, err := db.NewRaw("DELETE FROM test_guard WHERE name = 'nonexistent'").Exec(ctx)
		require.NoError(t, err, "DELETE with WHERE should be allowed")
	})

	t.Run("UpdateWithoutWhereBlocked", func(t *testing.T) {
		db := newGuardedDB(t, true)

		_, err := db.NewRaw("INSERT INTO test_guard (name) VALUES ('test')").Exec(ctx)
		require.NoError(t, err, "Insert should succeed")

		_, err = db.NewRaw("UPDATE test_guard SET name = 'changed'").Exec(ctx)
		require.Error(t, err, "UPDATE without WHERE should be blocked by SQL guard")
		require.ErrorIs(t, err, context.Canceled, "Blocked query should cancel the context")

		var name string
		require.NoError(t, db.NewRaw("SELECT name FROM test_guard").Scan(ctx, &name),
			"Select should succeed after blocked UPDATE")
		require.Equal(t, "test", name, "Row should be unchanged after blocked UPDATE without WHERE")
	})

	t.Run("UpdateWithWhereAllowed", func(t *testing.T) {
		db := newGuardedDB(t, true)

		_, err := db.NewRaw("UPDATE test_guard SET name = 'x' WHERE name = 'nonexistent'").Exec(ctx)
		require.NoError(t, err, "UPDATE with WHERE should be allowed")
	})

	t.Run("SelectAllowed", func(t *testing.T) {
		db := newGuardedDB(t, true)

		var result []struct {
			ID   int
			Name string
		}

		require.NoError(t, db.NewRaw("SELECT id, name FROM test_guard").Scan(ctx, &result),
			"SELECT should be allowed")
	})

	t.Run("WhitelistBypassesGuard", func(t *testing.T) {
		db := newGuardedDB(t, true)

		_, err := db.NewRaw("DROP TABLE test_guard").Exec(ctx)
		require.Error(t, err, "DROP should be blocked without whitelist")

		_, err = db.NewRaw("DROP TABLE test_guard").Exec(sqlguard.WithWhitelist(ctx))
		require.NoError(t, err, "DROP should work with whitelisted context")
	})

	t.Run("DisabledGuardAllowsDangerousSQL", func(t *testing.T) {
		db := newGuardedDB(t, false)

		_, err := db.NewRaw("DROP TABLE test_guard").Exec(ctx)
		require.NoError(t, err, "DROP should work when SQL guard is disabled")
	})
}

func TestQueryHookLogLevelRouting(t *testing.T) {
	newEvent := func(elapsed time.Duration, err error) *bun.QueryEvent {
		return &bun.QueryEvent{
			Query:     "SELECT 1",
			StartTime: time.Now().Add(-elapsed),
			Err:       err,
		}
	}

	tests := []struct {
		name      string
		ctx       context.Context //nolint:containedctx // table-driven test input
		event     *bun.QueryEvent
		wantLevel logx.Level
	}{
		{
			name:      "RegularQueryLogsAtInfo",
			ctx:       context.Background(),
			event:     newEvent(0, nil),
			wantLevel: logx.LevelInfo,
		},
		{
			name:      "QuietContextDemotesToDebug",
			ctx:       WithQuietSQLLog(context.Background()),
			event:     newEvent(0, nil),
			wantLevel: logx.LevelDebug,
		},
		{
			name:      "LiftedQuietContextLogsAtInfoAgain",
			ctx:       WithoutQuietSQLLog(WithQuietSQLLog(context.Background())),
			event:     newEvent(0, nil),
			wantLevel: logx.LevelInfo,
		},
		{
			name:      "SlowQueryOutranksTheQuietMark",
			ctx:       WithQuietSQLLog(context.Background()),
			event:     newEvent(time.Second, nil),
			wantLevel: logx.LevelWarn,
		},
		{
			name:      "FailureOutranksTheQuietMark",
			ctx:       WithQuietSQLLog(context.Background()),
			event:     newEvent(0, errors.New("connection refused")),
			wantLevel: logx.LevelError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := new(LevelRecordingLogger)
			hook := &queryHook{logger: logger, output: termenv.DefaultOutput()}

			hook.AfterQuery(tt.ctx, tt.event)

			assert.Equal(t, []logx.Level{tt.wantLevel}, logger.levels,
				"The statement must log exactly once at the routed level")
		})
	}
}

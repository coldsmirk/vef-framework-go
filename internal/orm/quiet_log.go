package orm

import "context"

type quietSQLLogKey struct{}

// WithQuietSQLLog marks the context so the SQL query hook demotes successful
// statement logs from Info to Debug. Framework maintenance loops (the durable
// cron engine's claim/sweep polling) run their periodic queries under this
// mark so steady-state logs stay readable; host applications may mark their
// own polling work the same way. Slow-query warnings and failures keep their
// level — noise reduction must never hide a problem.
func WithQuietSQLLog(ctx context.Context) context.Context {
	return context.WithValue(ctx, quietSQLLogKey{}, true)
}

// IsQuietSQLLog reports whether the context carries the quiet-SQL-log mark.
func IsQuietSQLLog(ctx context.Context) bool {
	quiet, _ := ctx.Value(quietSQLLogKey{}).(bool)

	return quiet
}

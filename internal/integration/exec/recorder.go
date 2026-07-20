package exec

import (
	"context"
	"time"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/integration"
	"github.com/coldsmirk/vef-framework-go/internal/logx"
	"github.com/coldsmirk/vef-framework-go/orm"
)

var logger = logx.Named("integration")

// logInsertTimeout bounds the best-effort log insert so a stalled database
// cannot hold a finished invocation hostage.
const logInsertTimeout = 10 * time.Second

// logRecorder persists invocation logs per the vef.integration.log mode.
// Recording is best-effort: a failed insert is logged, never surfaced to the
// caller.
type logRecorder struct {
	db   orm.DB
	mode config.IntegrationLogMode
}

func newLogRecorder(db orm.DB, cfg *config.IntegrationConfig) *logRecorder {
	return &logRecorder{db: db, mode: cfg.Log.EffectiveMode()}
}

// ShouldRecord reports whether the mode selects an invocation with the given
// failure kind (empty = success). Callers check it before assembling the
// masked captures, so a discarded entry costs nothing to build.
func (r *logRecorder) ShouldRecord(kind integration.FailureKind) bool {
	switch r.mode {
	case config.IntegrationLogOff:
		return false
	case config.IntegrationLogErrors:
		return kind != ""
	default:
		return true
	}
}

// Record persists one invocation entry when the mode selects it. The insert
// uses a cancellation-free (but deadline-bounded) context so a timed-out
// invocation still gets its log row.
func (r *logRecorder) Record(ctx context.Context, entry *integration.InvocationLog) {
	if !r.ShouldRecord(entry.FailureKind) {
		return
	}

	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), logInsertTimeout)
	defer cancel()

	if _, err := r.db.NewInsert().Model(entry).Exec(ctx); err != nil {
		logger.Errorf("Failed to record integration invocation log: %v", err)
	}
}

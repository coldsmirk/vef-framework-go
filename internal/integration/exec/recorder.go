package exec

import (
	"context"
	"time"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/id"
	"github.com/coldsmirk/vef-framework-go/integration"
	"github.com/coldsmirk/vef-framework-go/internal/logx"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/timex"
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

// Record persists one invocation entry, with its sealed replay payload when
// one is given, if the mode selects it. The inserts use a cancellation-free
// (but deadline-bounded) context so a timed-out invocation still gets its log
// row. The payload is written first, so the entry claims to be replayable only
// once its payload is stored.
func (r *logRecorder) Record(ctx context.Context, entry *integration.InvocationLog, replay string) {
	if !r.ShouldRecord(entry.FailureKind) {
		return
	}

	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), logInsertTimeout)
	defer cancel()

	if replay != "" {
		entry.ID = id.Generate()

		row := &InvocationReplay{ID: entry.ID, Payload: replay, CreatedAt: timex.Now()}
		if _, err := r.db.NewInsert().Model(row).Exec(ctx); err != nil {
			logger.Errorf("Failed to record integration replay payload: %v", err)
		} else {
			entry.Replayable = true
		}
	}

	if _, err := r.db.NewInsert().Model(entry).Exec(ctx); err != nil {
		logger.Errorf("Failed to record integration invocation log: %v", err)
	}
}

package exec

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/integration"
	"github.com/coldsmirk/vef-framework-go/internal/logx"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// logger is the integration module's framework logger, following the
// repo-wide package-level convention.
var logger = logx.Named("integration")

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

// Record persists one invocation entry when the mode selects it. The insert
// uses a cancellation-free context so a timed-out invocation still gets its
// log row.
func (r *logRecorder) Record(ctx context.Context, entry *integration.InvocationLog) {
	if r.mode == config.IntegrationLogOff {
		return
	}

	if r.mode == config.IntegrationLogErrors && entry.FailureKind == "" {
		return
	}

	if _, err := r.db.NewInsert().Model(entry).Exec(context.WithoutCancel(ctx)); err != nil {
		logger.Errorf("Failed to record integration invocation log: %v", err)
	}
}

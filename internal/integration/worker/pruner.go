package worker

import (
	"context"
	"time"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/integration"
	"github.com/coldsmirk/vef-framework-go/internal/integration/exec"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// LogPruner deletes invocation log rows — and their replay payloads — older
// than the configured retention window. Deletion by cutoff is idempotent, so
// concurrent runs across replicas are harmless and no distributed lock is
// needed.
type LogPruner struct {
	db        orm.DB
	retention time.Duration
}

// NewLogPruner builds the pruner from the configured retention.
func NewLogPruner(db orm.DB, cfg *config.IntegrationConfig) *LogPruner {
	return &LogPruner{db: db, retention: cfg.Log.Retention}
}

// Run performs one sweep; failures are logged, never propagated to the
// scheduler.
func (p *LogPruner) Run(ctx context.Context) {
	// Polling bookkeeping logs at Debug; failures keep their level.
	ctx = orm.WithQuietSQLLog(ctx)

	cutoff := timex.Now().Add(-p.retention)

	olderThanCutoff := func(cb orm.ConditionBuilder) {
		cb.LessThan("created_at", cutoff)
	}

	// Replay payloads go first: a payload outliving its entry could never be
	// reached again, while an entry briefly outliving its payload is merely no
	// longer replayable.
	if _, err := p.db.NewDelete().Model((*exec.InvocationReplay)(nil)).Where(olderThanCutoff).Exec(ctx); err != nil {
		logger.Errorf("Failed to prune invocation replay payloads: %v", err)

		return
	}

	result, err := p.db.NewDelete().Model((*integration.InvocationLog)(nil)).Where(olderThanCutoff).Exec(ctx)
	if err != nil {
		logger.Errorf("Failed to prune invocation logs: %v", err)

		return
	}

	if pruned, err := result.RowsAffected(); err == nil && pruned > 0 {
		logger.Infof("Pruned %d invocation log rows older than %s", pruned, p.retention)
	}
}

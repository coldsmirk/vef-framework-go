package store

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/cron"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// pruneJournal deletes terminal journal rows older than the retention
// window. Idempotent by cutoff, so concurrently sweeping replicas need no
// coordination; running rows are never touched regardless of age.
func (e *Engine) pruneJournal(ctx context.Context) {
	cutoff := e.now().Add(-e.config.RunRetention).UnixMilli()

	pruned, err := e.db.NewDelete().
		Model((*cron.Run)(nil)).
		Where(func(cb orm.ConditionBuilder) {
			cb.NotEquals("status", cron.RunRunning).LessThan("finished_at_unix_ms", cutoff)
		}).
		Exec(ctx)
	if err != nil {
		if ctx.Err() == nil {
			logger.Errorf("Prune run journal: %v", err)
		}

		return
	}

	if affected, _ := pruned.RowsAffected(); affected > 0 {
		logger.Infof("Pruned %d run journal row(s) beyond retention", affected)
	}
}

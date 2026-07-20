package store

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/cron"
	"github.com/coldsmirk/vef-framework-go/orm"
)

const (
	// pruneBatchSize bounds one delete statement so a large backlog (first
	// retention enablement over months of journal) never holds a long write
	// lock behind a single statement.
	pruneBatchSize = 1000
	// pruneBatchesPerSweep caps one sweep's work, bounding how long the loop
	// goroutine stays in maintenance; a remaining backlog drains across the
	// hourly cadence and concurrently sweeping replicas.
	pruneBatchesPerSweep = 16
)

// pruneJournal deletes terminal journal rows older than the retention
// window, in bounded batches. Idempotent by cutoff, so concurrently sweeping
// replicas need no coordination; running rows are never touched regardless
// of age.
func (e *Engine) pruneJournal(ctx context.Context) {
	cutoff := e.now().Add(-e.config.RunRetention).UnixMilli()

	pruned := int64(0)

	for range pruneBatchesPerSweep {
		affected, err := e.pruneBatch(ctx, cutoff)
		if err != nil {
			if ctx.Err() == nil {
				logger.Errorf("Prune run journal: %v", err)
			}

			return
		}

		pruned += affected

		if affected < pruneBatchSize {
			break
		}
	}

	if pruned > 0 {
		logger.Infof("Pruned %d run journal rows", pruned)
	}
}

func (e *Engine) pruneBatch(ctx context.Context, cutoff int64) (int64, error) {
	var ids []string
	if err := e.db.NewSelect().
		Model((*cron.Run)(nil)).
		Select("id").
		Where(func(cb orm.ConditionBuilder) {
			cb.NotEquals("status", cron.RunRunning).LessThan("finished_at_unix_ms", cutoff)
		}).
		Limit(pruneBatchSize).
		Scan(ctx, &ids); err != nil {
		return 0, err
	}

	if len(ids) == 0 {
		return 0, nil
	}

	// Terminal rows never revert to running, so deleting by primary key
	// needs no status re-check.
	deleted, err := e.db.NewDelete().
		Model((*cron.Run)(nil)).
		Where(func(cb orm.ConditionBuilder) { cb.PKIn(ids) }).
		Exec(ctx)
	if err != nil {
		return 0, err
	}

	return deleted.RowsAffected()
}

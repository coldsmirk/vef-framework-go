package store

import (
	"sync"
	"time"

	"github.com/coldsmirk/vef-framework-go/cron"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// heartbeatTracker holds the run IDs this node is currently executing, so
// the heartbeat loop renews their liveness in one batched update.
type heartbeatTracker struct {
	mu  sync.Mutex
	ids map[string]struct{}
}

func newHeartbeatTracker() *heartbeatTracker {
	return &heartbeatTracker{ids: make(map[string]struct{})}
}

// Track registers a running run.
func (t *heartbeatTracker) Track(id string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.ids[id] = struct{}{}
}

// Untrack removes a finished run.
func (t *heartbeatTracker) Untrack(id string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	delete(t.ids, id)
}

// Snapshot returns the tracked run IDs.
func (t *heartbeatTracker) Snapshot() []string {
	t.mu.Lock()
	defer t.mu.Unlock()

	ids := make([]string, 0, len(t.ids))
	for id := range t.ids {
		ids = append(ids, id)
	}

	return ids
}

// heartbeatLoop renews this node's running rows on the configured cadence.
// It outlives the claim loop so draining runs keep proving liveness until
// they finish.
func (e *Engine) heartbeatLoop() {
	defer e.background.Done()

	ticker := time.NewTicker(e.config.EffectiveHeartbeatInterval())
	defer ticker.Stop()

	for {
		select {
		case <-e.heartbeatCtx.Done():
			return
		case <-ticker.C:
			e.renewHeartbeats()
		}
	}
}

// renewHeartbeats stamps every tracked running row in one update. The
// status guard keeps a recovered (abandoned) row from being resurrected.
func (e *Engine) renewHeartbeats() {
	ids := e.heartbeats.Snapshot()
	if len(ids) == 0 {
		return
	}

	if _, err := e.db.NewUpdate().
		Model((*cron.Run)(nil)).
		Set("heartbeat_at", timex.DateTime(e.now())).
		Where(func(cb orm.ConditionBuilder) {
			cb.PKIn(ids).Equals("status", cron.RunRunning)
		}).
		Exec(e.heartbeatCtx); err != nil && e.heartbeatCtx.Err() == nil {
		logger.Errorf("Renew heartbeats for %d run(s): %v", len(ids), err)
	}
}

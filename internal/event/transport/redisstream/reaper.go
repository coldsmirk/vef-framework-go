package redisstream

import (
	"sync"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// reaperLoop periodically scans every active subscription for pending
// messages older than the configured idle threshold and XCLAIMs them
// to the current consumer, so messages from a crashed peer keep moving.
func (t *Transport) reaperLoop() {
	defer t.wg.Done()

	interval := t.cfg.EffectiveClaimInterval()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-t.stopCh:
			return
		case <-ticker.C:
			t.reapOnce()
		}
	}
}

// reapOnce reclaims pending messages for every active subscription. It
// fans out across subscriptions with bounded concurrency so a slow or
// hung handler on one stream cannot serialize failover for the others;
// the call blocks until the whole cycle drains, so the reaper ticker
// never overlaps two reaps of the same subscription set.
func (t *Transport) reapOnce() {
	t.mu.Lock()
	subs := make([]*subscription, len(t.subs))
	copy(subs, t.subs)
	t.mu.Unlock()

	if len(subs) == 0 {
		return
	}

	limit := min(t.cfg.EffectiveReaperConcurrency(), len(subs))
	sem := make(chan struct{}, limit)

	var wg sync.WaitGroup
	for _, sub := range subs {
		select {
		case <-t.stopCh:
			wg.Wait()

			return
		case sem <- struct{}{}:
		}

		wg.Go(func() {
			defer func() { <-sem }()

			t.reapSub(sub)
		})
	}

	wg.Wait()
}

// sweepLoop periodically reclaims orphaned consumer groups when
// IdleGroupRetention is enabled. It runs alongside the reaper: the reaper
// keeps live groups moving, the sweeper retires dead ones.
func (t *Transport) sweepLoop() {
	defer t.wg.Done()

	ticker := time.NewTicker(t.cfg.EffectiveIdleGroupSweepInterval())
	defer ticker.Stop()

	for {
		select {
		case <-t.stopCh:
			return
		case <-ticker.C:
			t.sweepIdleGroupsOnce()
		}
	}
}

// sweepIdleGroupsOnce destroys consumer groups that are demonstrably dead:
// not an active subscription of this process, no pending entries, and every
// consumer record idle beyond the retention window. Groups without any
// consumer record are left alone — they may belong to a peer racing between
// XGROUP CREATE and its first read, and a group that never consumed carries
// no state worth reclaiming anyway.
func (t *Transport) sweepIdleGroupsOnce() {
	retention := t.cfg.IdleGroupRetention
	if retention <= 0 {
		return
	}

	keys, err := scanStreamKeys(t.ctx, t.client, t.cfg.EffectiveStreamPrefix())
	if err != nil {
		t.logger.Warnf("redis_stream sweeper: %v", err)

		return
	}

	active := t.activeGroups()

	for _, stream := range keys {
		groups, err := t.client.XInfoGroups(t.ctx, stream).Result()
		if err != nil {
			t.logger.Warnf("redis_stream sweeper: xinfo groups %s: %v", stream, err)

			continue
		}

		for _, group := range groups {
			if _, isOurs := active[stream+"\x00"+group.Name]; isOurs {
				continue
			}

			if group.Pending > 0 {
				continue
			}

			consumers, err := t.client.XInfoConsumers(t.ctx, stream, group.Name).Result()
			if err != nil {
				t.logger.Warnf("redis_stream sweeper: xinfo consumers %s %s: %v", stream, group.Name, err)

				continue
			}

			if len(consumers) == 0 || !allConsumersIdle(consumers, retention) {
				continue
			}

			if err := t.client.XGroupDestroy(t.ctx, stream, group.Name).Err(); err != nil {
				t.logger.Warnf("redis_stream sweeper: destroy group %s on %s: %v", group.Name, stream, err)

				continue
			}

			t.logger.Infof("redis_stream sweeper: reclaimed idle consumer group %q on %s (all consumers idle > %s, no pending)",
				group.Name, stream, retention)
		}
	}
}

// activeGroups snapshots the (stream, group) pairs this process is
// subscribed to, keyed with a NUL separator that cannot appear inside a
// stream key or group name from configuration.
func (t *Transport) activeGroups() map[string]struct{} {
	t.mu.Lock()
	defer t.mu.Unlock()

	set := make(map[string]struct{}, len(t.subs))
	for _, sub := range t.subs {
		set[sub.stream+"\x00"+sub.group] = struct{}{}
	}

	return set
}

// allConsumersIdle reports whether every consumer record has been idle
// longer than the retention window.
func allConsumersIdle(consumers []goredis.XInfoConsumer, retention time.Duration) bool {
	for _, c := range consumers {
		if c.Idle < retention {
			return false
		}
	}

	return true
}

func (t *Transport) reapSub(sub *subscription) {
	pending, err := t.client.XPendingExt(t.ctx, &goredis.XPendingExtArgs{
		Stream: sub.stream,
		Group:  sub.group,
		Idle:   t.cfg.EffectiveClaimIdle(),
		Start:  "-",
		End:    "+",
		Count:  t.cfg.EffectiveClaimBatchSize(),
	}).Result()
	if err != nil {
		t.logger.Warnf("redis_stream reaper: xpending %s: %v", sub.stream, err)

		return
	}

	if len(pending) == 0 {
		return
	}

	// Build a retry-count map from XPENDING so deliver can report the
	// actual Redis delivery count rather than a hardcoded value.
	retryCounts := make(map[string]int64, len(pending))
	ids := make([]string, 0, len(pending))

	for _, p := range pending {
		if p.Consumer == sub.consumer {
			continue
		}

		ids = append(ids, p.ID)
		retryCounts[p.ID] = p.RetryCount
	}

	if len(ids) == 0 {
		return
	}

	claimed, err := t.client.XClaim(t.ctx, &goredis.XClaimArgs{
		Stream:   sub.stream,
		Group:    sub.group,
		Consumer: sub.consumer,
		MinIdle:  t.cfg.EffectiveClaimIdle(),
		Messages: ids,
	}).Result()
	if err != nil {
		t.logger.Warnf("redis_stream reaper: xclaim %s: %v", sub.stream, err)

		return
	}

	for _, msg := range claimed {
		// Use the Redis delivery count from XPENDING as the attempt number.
		// RetryCount reflects how many times Redis has delivered the message;
		// clamp to at least 2 to distinguish reaper redeliveries from first
		// delivery even if the map lookup misses (e.g. a race with XPENDING).
		attempt := max(int(retryCounts[msg.ID]), 2)

		t.deliver(t.ctx, sub, msg, attempt)
	}
}

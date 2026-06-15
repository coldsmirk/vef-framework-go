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

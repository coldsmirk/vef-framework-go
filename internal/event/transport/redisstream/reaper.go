package redisstream

import (
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

func (t *Transport) reapOnce() {
	t.mu.Lock()
	subs := make([]*subscription, len(t.subs))
	copy(subs, t.subs)
	t.mu.Unlock()

	for _, sub := range subs {
		t.reapSub(sub)
	}
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

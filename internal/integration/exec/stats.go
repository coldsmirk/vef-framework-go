package exec

import (
	"cmp"
	"maps"
	"slices"
	"sync"
	"time"

	"github.com/coldsmirk/vef-framework-go/integration"
)

// statsRecorder aggregates per-(system, contract, direction) invocation
// statistics in memory. Numbers are per node and reset on restart; the invocation log is
// the durable record.
type statsRecorder struct {
	mu      sync.Mutex
	entries map[statsKey]*statsEntry
}

type statsKey struct {
	system    string
	contract  string
	direction integration.Direction
}

type statsEntry struct {
	calls         int64
	successes     int64
	failures      map[integration.FailureKind]int64
	totalDuration time.Duration
	maxDuration   time.Duration
	lastError     string
	lastErrorAt   time.Time
}

func newStatsRecorder() *statsRecorder {
	return &statsRecorder{entries: make(map[statsKey]*statsEntry)}
}

// Record folds one invocation outcome into the aggregate. An empty kind
// means success; errMsg accompanies failures.
func (r *statsRecorder) Record(system, contract string, direction integration.Direction, kind integration.FailureKind, errMsg string, duration time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := statsKey{system: system, contract: contract, direction: direction}

	entry, ok := r.entries[key]
	if !ok {
		entry = &statsEntry{failures: make(map[integration.FailureKind]int64)}
		r.entries[key] = entry
	}

	entry.calls++
	entry.totalDuration += duration
	entry.maxDuration = max(entry.maxDuration, duration)

	if kind == "" {
		entry.successes++

		return
	}

	entry.failures[kind]++
	entry.lastError = errMsg
	entry.lastErrorAt = time.Now()
}

// Stats returns a snapshot ordered by system, contract, then direction.
func (r *statsRecorder) Stats() []integration.InvocationStats {
	r.mu.Lock()
	defer r.mu.Unlock()

	stats := make([]integration.InvocationStats, 0, len(r.entries))

	for key, entry := range r.entries {
		stat := integration.InvocationStats{
			System:        key.system,
			Contract:      key.contract,
			Direction:     key.direction,
			Calls:         entry.calls,
			Successes:     entry.successes,
			MaxDurationMs: entry.maxDuration.Milliseconds(),
			LastError:     entry.lastError,
			LastErrorAt:   entry.lastErrorAt,
		}

		if entry.calls > 0 {
			stat.AvgDurationMs = (entry.totalDuration / time.Duration(entry.calls)).Milliseconds()
		}

		if len(entry.failures) > 0 {
			stat.Failures = maps.Clone(entry.failures)
		}

		stats = append(stats, stat)
	}

	slices.SortFunc(stats, func(a, b integration.InvocationStats) int {
		if c := cmp.Compare(a.System, b.System); c != 0 {
			return c
		}

		if c := cmp.Compare(a.Contract, b.Contract); c != 0 {
			return c
		}

		return cmp.Compare(string(a.Direction), string(b.Direction))
	})

	return stats
}

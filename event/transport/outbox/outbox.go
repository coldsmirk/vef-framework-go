// Package outbox declares the persistent outbox Transport contract:
// records, status, repository interface, and configuration.
//
// An outbox Transport guarantees that events written in a caller
// transaction become visible to subscribers iff the transaction
// commits. A background relay claims pending records, dispatches them
// to a downstream sink Transport (typically memory for single-node or
// redis_stream for cross-process), and marks the record completed,
// failed, or dead.
//
// Subscription model. The outbox is publish-only: it persists frames
// but does not deliver them itself. Subscribers attach to the sink
// transport directly; the bus filters publish-only transports out of
// the Subscribe path so fan-out routes do not double-register handlers.
// Set vef.event.transports.outbox.sink to the transport that should
// receive the relayed frames (memory for single-node, redis_stream for
// cross-node — the framework warns at start-up when "memory" is paired
// with an enabled redis_stream module).
package outbox

import (
	"context"
	"encoding/json"
	"time"

	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// Name is the stable identifier used in routing configuration.
const Name = "outbox"

// Status is the lifecycle state of an outbox record.
type Status string

const (
	// StatusPending indicates the record is awaiting first dispatch.
	StatusPending Status = "pending"
	// StatusProcessing indicates the record is currently leased by a
	// relay worker; other workers must skip it until the lease expires.
	StatusProcessing Status = "processing"
	// StatusCompleted indicates the downstream sink accepted the record.
	StatusCompleted Status = "completed"
	// StatusFailed indicates the most recent dispatch attempt failed
	// and the record is scheduled for retry.
	StatusFailed Status = "failed"
	// StatusDead indicates the record exhausted its retry budget. Such
	// records are kept for diagnostics; the relay forwards them once
	// to the configured DLQ topic.
	StatusDead Status = "dead"
)

// Record is the persisted outbox row. Payload holds the canonical JSON
// of the original Event; Headers preserves transport-level metadata so
// the downstream sink sees an Envelope identical to the publish call.
type Record struct {
	orm.BaseModel `bun:"table:sys_event_outbox,alias:seo"`
	orm.Model
	orm.CreationTrackedModel

	EventID       string            `json:"eventId" bun:"event_id"`
	EventType     string            `json:"eventType" bun:"event_type"`
	Source        string            `json:"source" bun:"source"`
	TraceID       string            `json:"traceId,omitempty" bun:"trace_id,nullzero"`
	SpanID        string            `json:"spanId,omitempty" bun:"span_id,nullzero"`
	CorrelationID string            `json:"correlationId,omitempty" bun:"correlation_id,nullzero"`
	Headers       map[string]string `json:"headers,omitempty" bun:"headers,type:jsonb,nullzero"`
	Payload       json.RawMessage   `json:"payload" bun:"payload,type:jsonb"`
	Status        Status            `json:"status" bun:"status"`
	RetryCount    int               `json:"retryCount" bun:"retry_count"`
	LastError     *string           `json:"lastError,omitempty" bun:"last_error,nullzero"`
	ProcessedAt   *timex.DateTime   `json:"processedAt,omitempty" bun:"processed_at,nullzero"`
	RetryAfter    *timex.DateTime   `json:"retryAfter,omitempty" bun:"retry_after,nullzero"`
	OccurredAt    timex.DateTime    `json:"occurredAt" bun:"occurred_at"`
}

// Repository is the persistence boundary for outbox records. The
// default implementation uses orm.DB with FOR UPDATE SKIP LOCKED for
// safe multi-worker claiming.
type Repository interface {
	// InsertBatch persists pending records. Used by the non-tx Publish
	// path on the outbox transport.
	InsertBatch(ctx context.Context, records []Record) error
	// InsertBatchTx persists pending records within the caller's
	// transaction; visibility hinges on the caller committing.
	InsertBatchTx(ctx context.Context, tx orm.DB, records []Record) error
	// ClaimBatch atomically transitions a batch of pending or
	// retry-eligible records to processing with the supplied lease
	// deadline, returning the claimed records.
	ClaimBatch(ctx context.Context, batchSize, maxRetries int, leaseUntil timex.DateTime) ([]Record, error)
	// MarkCompleted transitions a processing record to completed.
	MarkCompleted(ctx context.Context, id string) error
	// MarkFailed transitions a processing record to failed and
	// schedules a retry. When retryCount >= maxRetries the
	// transition is to StatusDead.
	MarkFailed(ctx context.Context, id, errMsg string, retryCount int, retryAfter timex.DateTime, maxRetries int) error
	// DeleteCompletedOlderThan removes completed rows older than the
	// supplied cutoff. Used by the optional cleanup job.
	DeleteCompletedOlderThan(ctx context.Context, cutoff timex.DateTime) (int64, error)
}

// Config configures an outbox Transport instance.
type Config struct {
	// RelayInterval is the polling cadence of the relay loop.
	RelayInterval time.Duration
	// MaxRetries is the per-record retry budget before StatusDead.
	MaxRetries int
	// BatchSize is the maximum rows claimed per poll cycle.
	BatchSize int
	// LeaseMultiplier sets the processing lease duration as a
	// multiple of RelayInterval (clamped to MinLease).
	LeaseMultiplier int
	// MinLease is the absolute floor for the processing lease.
	MinLease time.Duration
	// SinkName is the name of the downstream Transport that the relay
	// dispatches claimed frames to. Defaults to "memory".
	SinkName string
}

// EffectiveRelayInterval applies the default when unset.
func (c Config) EffectiveRelayInterval() time.Duration {
	if c.RelayInterval > 0 {
		return c.RelayInterval
	}

	return 5 * time.Second
}

// EffectiveMaxRetries applies the default when unset.
func (c Config) EffectiveMaxRetries() int {
	if c.MaxRetries > 0 {
		return c.MaxRetries
	}

	return 10
}

// EffectiveBatchSize applies the default when unset.
func (c Config) EffectiveBatchSize() int {
	if c.BatchSize > 0 {
		return c.BatchSize
	}

	return 100
}

// EffectiveLeaseMultiplier applies the default when unset.
func (c Config) EffectiveLeaseMultiplier() int {
	if c.LeaseMultiplier > 0 {
		return c.LeaseMultiplier
	}

	return 4
}

// EffectiveMinLease applies the default when unset.
func (c Config) EffectiveMinLease() time.Duration {
	if c.MinLease > 0 {
		return c.MinLease
	}

	return 15 * time.Second
}

// EffectiveSinkName applies the default when unset.
func (c Config) EffectiveSinkName() string {
	if c.SinkName != "" {
		return c.SinkName
	}

	return "memory"
}

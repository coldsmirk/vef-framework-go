package outbox

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/coldsmirk/vef-framework-go/event/transport"
	"github.com/coldsmirk/vef-framework-go/event/transport/outbox"
	ilogx "github.com/coldsmirk/vef-framework-go/internal/logx"
	"github.com/coldsmirk/vef-framework-go/logx"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// maxLastErrorBytes caps the persisted error message so the column
// stays metadata-sized and reduces the surface for accidental
// credential leakage from low-level driver error strings.
const maxLastErrorBytes = 256

// errorRedactionPatterns drop sensitive substrings before persisting
// last_error. Patterns are intentionally conservative: anything more
// nuanced belongs in a structured logger, not the audit table.
var errorRedactionPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)password=\S+`),
	regexp.MustCompile(`(?i)passwd[=: ]+\S+`),
	regexp.MustCompile(`(?i)bearer\s+[A-Za-z0-9._\-]+`),
	regexp.MustCompile(`(?i)authorization[=: ]+\S+`),
	regexp.MustCompile(`\b\d{1,3}(?:\.\d{1,3}){3}:\d{1,5}\b`),
}

// dlqHeader marks a frame as having been forwarded by the outbox DLQ
// path. The outbox transport refuses to persist frames carrying this
// header so a misconfigured "sink-back-to-self" routing cannot create
// an infinite-grow loop.
const dlqHeader = "vef.dlq"

// Relay polls the outbox table and dispatches claimed records to the
// configured sink Transport. Failures schedule an exponential-backoff
// retry; rows exceeding the retry budget transition to StatusDead and
// are forwarded once to the DLQ topic so operators have a single
// surface to inspect.
//
// The sink is resolved lazily on each cycle so the outbox transport
// can break the circular fx dependency on the transport registry.
type Relay struct {
	repo     outbox.Repository
	sinkFn   func() transport.Transport
	cfg      outbox.Config
	dlqTopic func(eventType string) string
	logger   logx.Logger
}

// NewRelay constructs a Relay. sinkFn returns the current sink — it is
// called once per cycle so installation can be deferred. dlqTopic
// computes the DLQ topic for an event type; pass nil to use the
// framework default ("vef-dlq." + type). A nil logger is replaced with
// logx.Discard so tests can omit it.
func NewRelay(
	repo outbox.Repository,
	sinkFn func() transport.Transport,
	cfg outbox.Config,
	log logx.Logger,
	dlqTopic func(eventType string) string,
) *Relay {
	if dlqTopic == nil {
		dlqTopic = defaultDLQTopic
	}

	if log == nil {
		log = ilogx.Discard()
	}

	return &Relay{
		repo:     repo,
		sinkFn:   sinkFn,
		cfg:      cfg,
		dlqTopic: dlqTopic,
		logger:   log,
	}
}

// RelayPending performs one poll/claim/dispatch cycle. Safe to invoke
// from a cron task; the per-record lease guarantees that overlapping
// invocations cannot dispatch the same record twice.
func (r *Relay) RelayPending(ctx context.Context) {
	sink := r.sinkFn()
	if sink == nil {
		r.logger.Warnf("outbox relay: sink not configured, skipping cycle")

		return
	}

	batchSize := r.cfg.EffectiveBatchSize()
	maxRetries := r.cfg.EffectiveMaxRetries()
	leaseUntil := r.leaseDeadline()

	claimed, err := r.repo.ClaimBatch(ctx, batchSize, maxRetries, leaseUntil)
	if err != nil {
		r.logger.Errorf("outbox relay: claim batch: %v", err)

		return
	}

	if len(claimed) == 0 {
		return
	}

	r.logger.Infof("outbox relay: dispatching %d record(s)", len(claimed))

	for i := range claimed {
		if err := r.dispatchOne(ctx, sink, &claimed[i]); err != nil {
			r.logger.Errorf("outbox relay: dispatch %s failed: %v", claimed[i].EventID, err)
		}
	}
}

func (r *Relay) dispatchOne(ctx context.Context, sink transport.Transport, record *outbox.Record) error {
	frame := toFrame(*record)

	if err := sink.Publish(ctx, []transport.Frame{frame}); err != nil {
		return r.handleFailure(ctx, sink, record, err)
	}

	return r.repo.MarkCompleted(ctx, record.ID)
}

// handleFailure tries the DLQ forward first when the retry budget is
// exhausted: only if the DLQ accepts the frame do we transition the
// record to StatusDead. Otherwise the record stays Failed and is
// retried on the next cycle so the DLQ payload is never silently lost.
func (r *Relay) handleFailure(ctx context.Context, sink transport.Transport, record *outbox.Record, dispatchErr error) error {
	maxRetries := r.cfg.EffectiveMaxRetries()
	retryCount := record.RetryCount + 1

	now := timex.Now()
	retryAfter := now.Add(backoffFor(retryCount))
	redacted := redactError(dispatchErr.Error())

	if retryCount >= maxRetries {
		if err := r.forwardDLQ(ctx, sink, record); err != nil {
			// The business retry budget is exhausted, but the DLQ forward
			// itself failed. Keep the record in a CLAIMABLE Failed state —
			// retry_count stays below maxRetries — so the next relay cycle
			// retries the DLQ hand-off. Advancing retry_count to the dead
			// threshold here would strand the record permanently, because
			// ClaimBatch only re-claims rows with retry_count < maxRetries.
			// The record transitions to Dead only once the DLQ forward
			// finally succeeds (the fall-through MarkFailed below).
			r.logger.Warnf("outbox relay: DLQ forward for %s failed (%v); record kept claimable for DLQ retry", record.EventID, err)

			if markErr := r.repo.MarkFailed(ctx, record.ID, redacted, record.RetryCount, retryAfter, maxRetries); markErr != nil {
				return fmt.Errorf("mark failed: %w", markErr)
			}

			return nil
		}

		r.logger.Warnf("outbox relay: record %s exhausted retries, DLQ forwarded", record.EventID)
	}

	if err := r.repo.MarkFailed(ctx, record.ID, redacted, retryCount, retryAfter, maxRetries); err != nil {
		return fmt.Errorf("mark failed: %w", err)
	}

	return nil
}

func (r *Relay) forwardDLQ(ctx context.Context, sink transport.Transport, record *outbox.Record) error {
	frame := toFrame(*record)

	frame.Type = r.dlqTopic(record.EventType)
	if frame.Headers == nil {
		frame.Headers = make(map[string]string, 1)
	}

	frame.Headers[dlqHeader] = "1"
	if err := sink.Publish(ctx, []transport.Frame{frame}); err != nil {
		return fmt.Errorf("dlq forward: %w", err)
	}

	return nil
}

// redactError trims and scrubs the persisted error string. Patterns
// remove common credential / network-topology fragments that low-level
// drivers may include verbatim.
func redactError(msg string) string {
	cleaned := msg
	for _, re := range errorRedactionPatterns {
		cleaned = re.ReplaceAllString(cleaned, "[redacted]")
	}

	if len(cleaned) > maxLastErrorBytes {
		cleaned = cleaned[:maxLastErrorBytes]
	}

	return strings.TrimSpace(cleaned)
}

func (r *Relay) leaseDeadline() timex.DateTime {
	multiplier := r.cfg.EffectiveLeaseMultiplier()
	candidate := max(r.cfg.EffectiveRelayInterval()*time.Duration(multiplier), r.cfg.EffectiveMinLease())

	return timex.Now().Add(candidate)
}

// maxBackoff caps the exponential retry delay so a record that keeps
// failing does not push retry_after absurdly far into the future.
const maxBackoff = time.Hour

// backoffFor returns the exponential retry delay (2^retryCount seconds)
// capped at maxBackoff. The exponent is clamped before shifting so the
// computation can never overflow time.Duration's underlying int64.
func backoffFor(retryCount int) time.Duration {
	const maxShift = 16 // 2^16 s ≈ 18h, already beyond maxBackoff

	shift := min(max(retryCount, 0), maxShift)

	return min(time.Second<<shift, maxBackoff)
}

func defaultDLQTopic(eventType string) string {
	return "vef-dlq." + eventType
}

// toFrame reconstructs a transport.Frame from a stored Record. The
// occurredAt and publishedAt values reflect when the original publisher
// committed; subscribers can use them for end-to-end latency metrics.
func toFrame(record outbox.Record) transport.Frame {
	return transport.Frame{
		ID:            record.EventID,
		Type:          record.EventType,
		Source:        record.Source,
		OccurredAt:    record.OccurredAt.Unwrap(),
		PublishedAt:   record.CreatedAt.Unwrap(),
		TraceID:       record.TraceID,
		SpanID:        record.SpanID,
		CorrelationID: record.CorrelationID,
		Headers:       record.Headers,
		Body:          record.Payload,
	}
}

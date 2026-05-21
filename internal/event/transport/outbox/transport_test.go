package outbox_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/event/transport"
	puboutbox "github.com/coldsmirk/vef-framework-go/event/transport/outbox"
	"github.com/coldsmirk/vef-framework-go/internal/event/transport/outbox"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// recordingSink is a stub Transport that records every Publish call and
// optionally returns an injected error to simulate downstream failure.
type recordingSink struct {
	mu       sync.Mutex
	frames   []transport.Frame
	failNext error
}

func (*recordingSink) Name() string { return "recording" }

func (*recordingSink) Capabilities() transport.Capabilities {
	return transport.Capabilities{Durable: false}
}

func (*recordingSink) Start(context.Context) error { return nil }
func (*recordingSink) Stop(context.Context) error  { return nil }

func (s *recordingSink) Publish(_ context.Context, frames []transport.Frame) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.failNext != nil {
		err := s.failNext
		s.failNext = nil

		return err
	}

	s.frames = append(s.frames, frames...)

	return nil
}

func (*recordingSink) Subscribe(string, string, transport.ConsumeFunc, transport.SubscribeConfig) (transport.Unsubscribe, error) {
	return func() {}, nil
}

func (s *recordingSink) Frames() []transport.Frame {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([]transport.Frame(nil), s.frames...)
}

func setupOutbox(t *testing.T) (orm.DB, *outbox.DefaultRepository, *outbox.Transport, *recordingSink) {
	t.Helper()

	ctx := context.Background()
	db := testx.NewTestDB(t)
	require.NoError(t, outbox.Migrate(ctx, db, config.SQLite), "outbox migration should succeed")

	repo := outbox.NewRepository(db)
	cfg := puboutbox.Config{
		RelayInterval:   time.Second,
		MaxRetries:      3,
		BatchSize:       10,
		LeaseMultiplier: 4,
		MinLease:        5 * time.Second,
	}

	t.Cleanup(func() { /* db cleanup handled by testx */ })

	tp := outbox.NewTransport(repo, cfg)
	sink := &recordingSink{}
	tp.SetSink(sink)

	return db, repo, tp, sink
}

func newFrame(eventID, eventType, body string) transport.Frame {
	now := time.Now()

	return transport.Frame{
		ID:          eventID,
		Type:        eventType,
		Source:      "test",
		OccurredAt:  now,
		PublishedAt: now,
		Body:        []byte(body),
	}
}

func TestOutboxTransportPublishPersistsRecord(t *testing.T) {
	ctx := context.Background()
	_, _, tp, _ := setupOutbox(t)

	frame := newFrame("evt-1", "test.created", `{"x":1}`)
	require.NoError(t, tp.Publish(ctx, []transport.Frame{frame}), "Publish should succeed")
}

func TestOutboxTransportPublishTxRollbackHidesRecord(t *testing.T) {
	ctx := context.Background()
	db, repo, tp, _ := setupOutbox(t)

	frame := newFrame("evt-rb", "test.created", `{"rolled":true}`)
	sentinel := errors.New("force rollback")
	err := db.RunInTX(ctx, func(ctx context.Context, tx orm.DB) error {
		if err := tp.PublishTx(ctx, tx, []transport.Frame{frame}); err != nil {
			return err
		}

		return sentinel
	})
	require.ErrorIs(t, err, sentinel, "RunInTX should surface the sentinel error")

	claimed, err := repo.ClaimBatch(ctx, 10, 3, timex.Now().Add(time.Minute))
	require.NoError(t, err)
	require.Empty(t, claimed, "rolled-back records must not be visible to the relay")
}

func TestOutboxRelayDispatchesAndMarksCompleted(t *testing.T) {
	ctx := context.Background()
	db, repo, tp, sink := setupOutbox(t)

	frame := newFrame("evt-ok", "test.created", `{"ok":true}`)
	require.NoError(t, tp.Publish(ctx, []transport.Frame{frame}), "Publish should persist relay record")

	relay := outbox.NewRelay(repo, tp.Sink, puboutbox.Config{
		RelayInterval: time.Second, MaxRetries: 3, BatchSize: 10,
		LeaseMultiplier: 4, MinLease: 5 * time.Second,
	}, nil, nil)
	relay.RelayPending(ctx)

	require.Len(t, sink.Frames(), 1, "sink should receive one frame")
	require.Equal(t, "evt-ok", sink.Frames()[0].ID)
	require.JSONEq(t, `{"ok":true}`, string(sink.Frames()[0].Body), "payload bytes must round-trip cleanly through the outbox table")

	// Second relay cycle is a no-op: the record is completed.
	relay.RelayPending(ctx)
	require.Len(t, sink.Frames(), 1, "completed records must not be redelivered")

	// Confirm row status using the repository: no claimable rows remain.
	leftover, err := repo.ClaimBatch(ctx, 10, 3, timex.Now().Add(time.Minute))
	require.NoError(t, err, "Claiming after completion should not error")
	require.Empty(t, leftover, "Completed record should not be claimable")

	_ = db
}

func TestOutboxRelayBackoffAndDeadAfterMaxRetries(t *testing.T) {
	ctx := context.Background()
	db, repo, tp, sink := setupOutbox(t)

	frame := newFrame("evt-dead", "test.flaky", `{"flaky":true}`)
	require.NoError(t, tp.Publish(ctx, []transport.Frame{frame}), "Publish should persist retryable record")

	cfg := puboutbox.Config{
		RelayInterval: time.Second, MaxRetries: 2, BatchSize: 10,
		LeaseMultiplier: 4, MinLease: 5 * time.Second,
	}
	relay := outbox.NewRelay(repo, tp.Sink, cfg, nil, nil)

	// First failure: status becomes Failed with a retry scheduled.
	sink.failNext = errors.New("boom-1")

	relay.RelayPending(ctx)

	// Force retry_after into the past so the next cycle re-claims.
	forceRetryReady(t, db, "evt-dead")

	// Second failure exhausts the budget (maxRetries=2): status → Dead.
	sink.failNext = errors.New("boom-2")

	relay.RelayPending(ctx)

	// Sink should have received one DLQ-forwarded frame (and no normal
	// dispatch frames after the second failure).
	frames := sink.Frames()
	require.NotEmpty(t, frames, "DLQ frame should be forwarded once")
	require.Equal(t, "vef-dlq.test.flaky", frames[len(frames)-1].Type, "Last sink frame should be DLQ forwarded")

	// No claimable rows remain — the dead record is excluded by
	// status filtering even with retry_after in the past.
	forceRetryReady(t, db, "evt-dead")

	left, err := repo.ClaimBatch(ctx, 10, 2, timex.Now().Add(time.Minute))
	require.NoError(t, err, "Claiming after dead transition should not error")
	require.Empty(t, left, "dead records must not be reclaimable")
}

func TestOutboxCleanerDeletesCompletedRowsByProcessedAt(t *testing.T) {
	ctx := context.Background()
	db, repo, tp, _ := setupOutbox(t)

	oldFrame := newFrame("evt-clean-old", "test.clean", `{"old":true}`)
	freshFrame := newFrame("evt-clean-fresh", "test.clean", `{"fresh":true}`)
	require.NoError(t, tp.Publish(ctx, []transport.Frame{oldFrame, freshFrame}), "Publish should persist records")

	oldRows, err := repo.ClaimBatch(ctx, 1, 3, timex.Now().Add(time.Minute))
	require.NoError(t, err, "First claim should succeed")
	require.Len(t, oldRows, 1, "First claim should return one row")
	require.NoError(t, repo.MarkCompleted(ctx, oldRows[0].ID), "Old row should be completed")

	freshRows, err := repo.ClaimBatch(ctx, 1, 3, timex.Now().Add(time.Minute))
	require.NoError(t, err, "Second claim should succeed")
	require.Len(t, freshRows, 1, "Second claim should return one row")
	require.NoError(t, repo.MarkCompleted(ctx, freshRows[0].ID), "Fresh row should be completed")

	oldProcessedAt := timex.Now().Add(-2 * time.Hour)
	_, err = db.NewUpdate().
		Model((*puboutbox.Record)(nil)).
		Set("processed_at", oldProcessedAt).
		Where(func(cb orm.ConditionBuilder) {
			cb.PKEquals(oldRows[0].ID)
		}).
		Exec(ctx)
	require.NoError(t, err, "Old processed timestamp should be adjustable")

	cleaner := outbox.NewCleaner(repo, time.Hour, nil)
	cleaner.Cleanup(ctx)

	count, err := db.NewSelect().Model((*puboutbox.Record)(nil)).Count(ctx)
	require.NoError(t, err, "Record count should be queryable")
	require.EqualValues(t, 1, count, "Cleaner should remove only rows older than completed TTL")

	var remaining puboutbox.Record

	err = db.NewSelect().Model(&remaining).Scan(ctx)
	require.NoError(t, err, "Remaining outbox row should be queryable")
	require.Equal(t, freshRows[0].ID, remaining.ID, "Fresh completed row should remain")
}

// forceRetryReady backs the retry_after timestamp into the past so a
// subsequent ClaimBatch call observes the row as retry-eligible.
func forceRetryReady(t *testing.T, db orm.DB, eventID string) {
	t.Helper()

	past := timex.Now().Add(-time.Hour)
	_, err := db.NewUpdate().
		Model((*puboutbox.Record)(nil)).
		Set("retry_after", past).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("event_id", eventID)
		}).
		Exec(context.Background())
	require.NoError(t, err, "Retry timestamp should be adjustable")
}

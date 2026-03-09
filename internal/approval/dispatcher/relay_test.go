package dispatcher

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/id"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// MockEventDispatcher captures dispatch calls and allows controlled failures.
type MockEventDispatcher struct {
	Dispatched []approval.EventOutbox
	Err        error
}

func (m *MockEventDispatcher) Dispatch(_ context.Context, record approval.EventOutbox) error {
	m.Dispatched = append(m.Dispatched, record)

	return m.Err
}

// StatusCheckDispatcher asserts an outbox row is claimed as processing before dispatch.
type StatusCheckDispatcher struct {
	T   *testing.T
	DB  orm.DB
	Ctx context.Context
}

func (d *StatusCheckDispatcher) Dispatch(_ context.Context, record approval.EventOutbox) error {
	d.T.Helper()

	var current approval.EventOutbox

	current.ID = record.ID

	err := d.DB.NewSelect().
		Model(&current).
		WherePK().
		Scan(d.Ctx)
	if err != nil {
		d.T.Fatalf("Should read claimed outbox row during dispatch: %v", err)
	}

	if current.Status != approval.EventOutboxProcessing {
		d.T.Fatalf("Outbox row should be claimed before dispatch: got %s", current.Status)
	}

	return nil
}

type RelayTestSuite struct {
	suite.Suite

	ctx        context.Context
	db         orm.DB
	dispatcher *MockEventDispatcher
	relay      *Relay
}

func (s *RelayTestSuite) SetupSuite() {
	s.ctx = context.Background()
	s.db = testx.NewTestDB(s.T())

	_, err := s.db.NewCreateTable().
		Model((*approval.EventOutbox)(nil)).
		IfNotExists().
		Exec(s.ctx)
	s.Require().NoError(err, "Should create EventOutbox table")
}

func (s *RelayTestSuite) SetupTest() {
	_, err := s.db.NewTruncateTable().Model((*approval.EventOutbox)(nil)).Exec(s.ctx)
	s.Require().NoError(err, "Should clean EventOutbox table")

	s.dispatcher = &MockEventDispatcher{}
	s.relay = NewRelay(s.db, s.dispatcher, &config.ApprovalConfig{
		OutboxBatchSize:  10,
		OutboxMaxRetries: 3,
	})
}

// insertRecord inserts an EventOutbox record, auto-generating the ID if empty.
func (s *RelayTestSuite) insertRecord(record *approval.EventOutbox) {
	s.T().Helper()

	_, err := s.db.NewInsert().Model(record).Exec(s.ctx)
	s.Require().NoError(err, "Should insert EventOutbox record")
}

// getRecord fetches an EventOutbox record by ID.
func (s *RelayTestSuite) getRecord(recordID string) *approval.EventOutbox {
	s.T().Helper()

	var record approval.EventOutbox

	record.ID = recordID

	err := s.db.NewSelect().
		Model(&record).
		WherePK().
		Scan(s.ctx)
	s.Require().NoError(err, "Should fetch EventOutbox record")

	return &record
}

func (s *RelayTestSuite) TestNoPendingEvents() {
	s.relay.RelayPending(s.ctx)

	s.Assert().Empty(s.dispatcher.Dispatched, "Should not dispatch any events")
}

func (s *RelayTestSuite) TestDispatchesPendingSuccessfully() {
	s.insertRecord(&approval.EventOutbox{
		EventID:   "evt-1",
		EventType: "approval.instance.created",
		Payload:   map[string]any{"instanceId": "inst-1"},
		Status:    approval.EventOutboxPending,
	})

	s.relay.RelayPending(s.ctx)

	s.Require().Len(s.dispatcher.Dispatched, 1, "Should dispatch one event")
	s.Assert().Equal("evt-1", s.dispatcher.Dispatched[0].EventID, "Should dispatch correct event")

	record := s.getRecord(s.dispatcher.Dispatched[0].ID)
	s.Assert().Equal(approval.EventOutboxCompleted, record.Status, "Should mark as completed")
	s.Assert().NotNil(record.ProcessedAt, "Should set ProcessedAt")
}

func (s *RelayTestSuite) TestClaimsEventAsProcessingBeforeDispatch() {
	rec := &approval.EventOutbox{
		EventID:   "evt-processing",
		EventType: "approval.test",
		Status:    approval.EventOutboxPending,
	}
	s.insertRecord(rec)

	s.relay = NewRelay(s.db, &StatusCheckDispatcher{
		T:   s.T(),
		DB:  s.db,
		Ctx: s.ctx,
	}, &config.ApprovalConfig{
		OutboxBatchSize:  10,
		OutboxMaxRetries: 3,
	})

	s.relay.RelayPending(s.ctx)

	record := s.getRecord(rec.ID)
	s.Assert().Equal(approval.EventOutboxCompleted, record.Status, "Should mark claimed row completed after successful dispatch")
}

func (s *RelayTestSuite) TestDispatchFailureMarksRecordFailed() {
	s.dispatcher.Err = errors.New("connection refused")

	rec := &approval.EventOutbox{
		EventID:   "evt-fail",
		EventType: "approval.test",
		Status:    approval.EventOutboxPending,
	}
	s.insertRecord(rec)

	s.relay.RelayPending(s.ctx)

	s.Require().Len(s.dispatcher.Dispatched, 1, "Should attempt to dispatch")

	record := s.getRecord(rec.ID)
	s.Assert().Equal(approval.EventOutboxFailed, record.Status, "Should mark as failed")
	s.Assert().Equal(1, record.RetryCount, "Should increment retry count")
	s.Require().NotNil(record.LastError, "Should set last error")
	s.Assert().Equal("connection refused", *record.LastError, "Should record error message")
	s.Require().NotNil(record.RetryAfter, "Should schedule retry")
}

func (s *RelayTestSuite) TestExponentialBackoff() {
	s.dispatcher.Err = errors.New("fail")

	rec := &approval.EventOutbox{
		EventID:   "evt-backoff",
		EventType: "approval.test",
		Status:    approval.EventOutboxPending,
	}
	s.insertRecord(rec)

	before := timex.Now()

	s.relay.RelayPending(s.ctx)

	record := s.getRecord(rec.ID)
	s.Require().NotNil(record.RetryAfter, "Should set retry_after")

	// retry_count=1 after first failure → backoff = 2^1 = 2s
	// Allow 1s tolerance for timex.DateTime second-level precision truncation
	expectedMin := before.Add(1 * time.Second)
	s.Assert().False(record.RetryAfter.Before(expectedMin),
		"RetryAfter should be ~2s after dispatch time (got %v, min %v)", *record.RetryAfter, expectedMin)
}

func (s *RelayTestSuite) TestRetriesFailedEventWithinLimit() {
	pastTime := timex.Now().Add(-time.Minute)
	rec := &approval.EventOutbox{
		EventID:    "evt-retry",
		EventType:  "approval.test",
		Status:     approval.EventOutboxFailed,
		RetryCount: 1,
		RetryAfter: &pastTime,
	}
	s.insertRecord(rec)

	s.relay.RelayPending(s.ctx)

	s.Require().Len(s.dispatcher.Dispatched, 1, "Should retry the failed event")
	s.Assert().Equal("evt-retry", s.dispatcher.Dispatched[0].EventID, "Should dispatch correct event")

	record := s.getRecord(rec.ID)
	s.Assert().Equal(approval.EventOutboxCompleted, record.Status, "Should mark as completed after successful retry")
}

func (s *RelayTestSuite) TestSkipsFailedEventExceedingMaxRetries() {
	pastTime := timex.Now().Add(-time.Minute)
	s.insertRecord(&approval.EventOutbox{
		EventID:    "evt-exhausted",
		EventType:  "approval.test",
		Status:     approval.EventOutboxFailed,
		RetryCount: 3, // equals maxRetries
		RetryAfter: &pastTime,
	})

	s.relay.RelayPending(s.ctx)

	s.Assert().Empty(s.dispatcher.Dispatched, "Should not dispatch event that exceeded max retries")
}

func (s *RelayTestSuite) TestSkipsFailedEventNotYetRetryable() {
	futureTime := timex.Now().Add(time.Hour)
	s.insertRecord(&approval.EventOutbox{
		EventID:    "evt-future",
		EventType:  "approval.test",
		Status:     approval.EventOutboxFailed,
		RetryCount: 1,
		RetryAfter: &futureTime,
	})

	s.relay.RelayPending(s.ctx)

	s.Assert().Empty(s.dispatcher.Dispatched, "Should not dispatch event with future retry_after")
}

func (s *RelayTestSuite) TestReclaimsExpiredProcessingEvent() {
	expired := timex.Now().Add(-time.Minute)
	rec := &approval.EventOutbox{
		EventID:    "evt-processing-expired",
		EventType:  "approval.test",
		Status:     approval.EventOutboxProcessing,
		RetryAfter: &expired,
	}
	s.insertRecord(rec)

	s.relay.RelayPending(s.ctx)

	s.Require().Len(s.dispatcher.Dispatched, 1, "Should reclaim and dispatch expired processing event")
	s.Assert().Equal("evt-processing-expired", s.dispatcher.Dispatched[0].EventID, "Should dispatch reclaimed processing event")

	record := s.getRecord(rec.ID)
	s.Assert().Equal(approval.EventOutboxCompleted, record.Status, "Should mark reclaimed processing event as completed")
}

func (s *RelayTestSuite) TestSkipsProcessingEventWithActiveLease() {
	activeLease := timex.Now().Add(time.Minute)
	s.insertRecord(&approval.EventOutbox{
		EventID:    "evt-processing-active",
		EventType:  "approval.test",
		Status:     approval.EventOutboxProcessing,
		RetryAfter: &activeLease,
	})

	s.relay.RelayPending(s.ctx)

	s.Assert().Empty(s.dispatcher.Dispatched, "Should not dispatch processing event with active lease")
}

func (s *RelayTestSuite) TestSkipsCompletedEvents() {
	processedAt := timex.Now()
	s.insertRecord(&approval.EventOutbox{
		EventID:     "evt-done",
		EventType:   "approval.test",
		Status:      approval.EventOutboxCompleted,
		ProcessedAt: &processedAt,
	})

	s.relay.RelayPending(s.ctx)

	s.Assert().Empty(s.dispatcher.Dispatched, "Should not dispatch completed events")
}

func (s *RelayTestSuite) TestBatchSizeLimit() {
	// Insert 15 records, batchSize is 10
	baseTime := timex.Now().Add(-time.Hour)
	for i := range 15 {
		rec := &approval.EventOutbox{
			EventID:   id.Generate(),
			EventType: "approval.test",
			Status:    approval.EventOutboxPending,
		}
		rec.CreatedAt = baseTime.Add(time.Duration(i) * time.Second)
		s.insertRecord(rec)
	}

	s.relay.RelayPending(s.ctx)

	s.Assert().Len(s.dispatcher.Dispatched, 10, "Should only dispatch batchSize events")
}

func (s *RelayTestSuite) TestMaxRetriesClearsRetryAfter() {
	s.dispatcher.Err = errors.New("permanent failure")

	pastTime := timex.Now().Add(-time.Minute)
	rec := &approval.EventOutbox{
		EventID:    "evt-last-retry",
		EventType:  "approval.test",
		Status:     approval.EventOutboxFailed,
		RetryCount: 2, // one more failure will reach maxRetries=3
		RetryAfter: &pastTime,
	}
	s.insertRecord(rec)

	s.relay.RelayPending(s.ctx)

	record := s.getRecord(rec.ID)
	s.Assert().Equal(approval.EventOutboxFailed, record.Status, "Should remain failed")
	s.Assert().Equal(3, record.RetryCount, "Should increment to max retries")
	s.Assert().Nil(record.RetryAfter, "Should clear retry_after when max retries reached")
}

func (s *RelayTestSuite) TestDispatchesMultiplePendingEvents() {
	baseTime := timex.Now().Add(-time.Hour)
	for i := range 3 {
		rec := &approval.EventOutbox{
			EventID:   id.Generate(),
			EventType: "approval.test",
			Status:    approval.EventOutboxPending,
		}
		rec.CreatedAt = baseTime.Add(time.Duration(i) * time.Second)
		s.insertRecord(rec)
	}

	s.relay.RelayPending(s.ctx)

	s.Assert().Len(s.dispatcher.Dispatched, 3, "Should dispatch all pending events")

	// Verify all marked as completed
	for _, dispatched := range s.dispatcher.Dispatched {
		record := s.getRecord(dispatched.ID)
		s.Assert().Equal(approval.EventOutboxCompleted, record.Status,
			"Event %s should be completed", dispatched.EventID)
	}
}

func (s *RelayTestSuite) TestOrdersByCreatedAt() {
	// Insert records with explicit staggered creation times
	baseTime := timex.Now().Add(-time.Hour)

	ids := make([]string, 3)
	for i := range 3 {
		rec := &approval.EventOutbox{
			EventID:   id.Generate(),
			EventType: "approval.test",
			Status:    approval.EventOutboxPending,
		}
		rec.CreatedAt = baseTime.Add(time.Duration(i) * time.Second)
		s.insertRecord(rec)
		ids[i] = rec.EventID
	}

	s.relay.RelayPending(s.ctx)

	s.Require().Len(s.dispatcher.Dispatched, 3, "Should dispatch all events")

	for i, dispatched := range s.dispatcher.Dispatched {
		s.Assert().Equal(ids[i], dispatched.EventID,
			"Event at position %d should match creation order", i)
	}
}

func TestRelay(t *testing.T) {
	suite.Run(t, new(RelayTestSuite))
}

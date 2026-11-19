package command_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/id"
	"github.com/coldsmirk/vef-framework-go/internal/approval/command"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/orm"
)

type VerifiedSubmission struct {
	StubBusinessHandler
	verify func(*approval.StartVerification) error
}

func (v VerifiedSubmission) VerifyStart(_ context.Context, _ orm.DB, entry *approval.StartVerification) error {
	return v.verify(entry)
}

func (s *StartInstanceTestSuite) TestVerifiedSubmissionReplay() {
	ctx, db := s.ctx, s.db
	fixture := deployAndPublishFlow(s.T(), ctx, db, "verified", approvalFlowDef())
	version := &approval.FlowVersion{ID: fixture.VersionID, BusinessKind: new(approval.BusinessKind("order"))}
	_, err := db.NewUpdate().Model(version).Select("business_kind").WherePK().Exec(ctx)
	s.Require().NoError(err, "Should pin the business kind")

	var calls atomic.Int32
	var entries []*approval.StartVerification
	veto := errors.New("entry veto")
	var decision error
	var entered, release chan struct{}
	handler := VerifiedSubmission{
		StubBusinessHandler: StubBusinessHandler{submit: func(ctx context.Context, tx orm.DB, sc *approval.SubmissionContext) (*approval.Submission, error) {
			calls.Add(1)
			_, err := tx.NewInsert().Model(&approval.FlowCategory{TenantID: sc.TenantID, Code: id.Generate(), Name: "host write"}).Exec(ctx)
			if err != nil {
				return nil, err
			}
			if entered != nil {
				select {
				case entered <- struct{}{}:
					<-release
				default:
				}
			}
			return &approval.Submission{BusinessRef: "host-reference"}, nil
		}},
		verify: func(entry *approval.StartVerification) error { entries = append(entries, entry); return decision },
	}
	reg := testRegistry(handler)
	eng := buildTestEngine(db)
	validation := service.NewValidationService(nil)
	startHandler := command.NewStartInstanceHandler(db, eng, new(MockInstanceNoGenerator), validation, reg, nil, nil)
	instSvc := service.NewInstanceService(nil)
	submitHandler := command.NewResubmitInstanceHandler(db, eng, validation, instSvc, nil, reg)
	input := approval.StartInstanceInput{FlowCode: "verified-flow", IdempotencyKey: "client/start", Applicant: approval.UserInfo{ID: "author"}, Caller: approval.SystemCaller, FormData: map[string]any{"nested": json.RawMessage(`{"b":2,"a":1}`)}}
	start := func(input approval.StartInstanceInput) (*approval.Instance, error) {
		var inst *approval.Instance
		err := db.RunInTx(ctx, func(ctx context.Context, tx orm.DB) error {
			var err error
			inst, err = startHandler.Handle(contextx.SetDB(ctx, tx), command.StartInstanceCmd{StartInstanceInput: input})
			return err
		})
		return inst, err
	}
	resubmit := func(in approval.ResubmitInstanceInput) error {
		return db.RunInTx(ctx, func(ctx context.Context, tx orm.DB) error {
			_, err := submitHandler.Handle(contextx.SetDB(ctx, tx), command.ResubmitInstanceCmd{ResubmitInstanceInput: in})
			return err
		})
	}
	hostWrites := func() int64 {
		n, err := db.NewSelect().Model((*approval.FlowCategory)(nil)).Where(func(cb orm.ConditionBuilder) { cb.Equals("name", "host write") }).Count(ctx)
		s.Require().NoError(err, "Should read committed host writes")
		return n
	}
	decision = veto
	_, err = start(input)
	s.ErrorIs(err, veto, "A verified start can be refused")
	s.EqualValues(0, hostWrites(), "A refused start must undo the host insert")
	s.Require().Len(entries, 1, "A failed start still reached the verifier")
	exists, err := db.NewSelect().Model((*approval.Instance)(nil)).Where(func(cb orm.ConditionBuilder) { cb.PKEquals(entries[0].Instance.ID) }).Exists(ctx)
	s.Require().NoError(err, "Should inspect the refused instance")
	s.False(exists, "A verifier veto must undo the instance")
	decision = nil
	entered, release = make(chan struct{}, 1), make(chan struct{})
	starts := raceSubmission(s.T(), entered, release, func() (*approval.Instance, error) { return start(input) })
	s.Equal(starts[0].ID, starts[1].ID, "Racing callers must receive the same instance")
	s.EqualValues(2, calls.Load(), "Only the veto and the winning start call Submit")
	s.EqualValues(1, hostWrites(), "Only the committed start leaves a host write")
	entered, release = nil, nil
	inst := starts[0]
	s.Require().Len(entries, 2, "A replay must skip VerifyStart")
	checkEntry := func(entry *approval.StartVerification, resubmission bool) {
		s.Equal(inst.ID, entry.Instance.ID, "The verifier receives the same instance")
		s.Equal(fixture.VersionID, entry.Version.ID, "The verifier receives the pinned version")
		s.Equal(resubmission, entry.IsResubmission, "The verifier can distinguish re-entry")
		s.Require().Len(entry.Tasks, 2, "Include precisely the two seats of this traversal")
		s.Equal(approval.TaskPending, entry.Tasks[0].Status, "The first sequential seat is actionable")
		s.Equal(approval.TaskWaiting, entry.Tasks[1].Status, "The next sequential seat is waiting")
	}
	checkEntry(entries[1], false)
	for _, modify := range []func(*approval.StartInstanceInput){
		func(i *approval.StartInstanceInput) { i.FlowCode = "different" },
		func(i *approval.StartInstanceInput) { i.BusinessRef = new("different") },
		func(i *approval.StartInstanceInput) { i.FormData = map[string]any{"different": true} },
		func(i *approval.StartInstanceInput) { i.Applicant.ID = "different" },
	} {
		changed := input
		modify(&changed)
		_, err = start(changed)
		s.ErrorIs(err, approval.ErrIdempotencyConflict, "Conflicting input must never replay")
	}
	withdraw := command.NewWithdrawInstanceHandler(db, service.NewTaskService(), instSvc)
	s.Require().NoError(db.RunInTx(ctx, func(ctx context.Context, tx orm.DB) error {
		_, err := withdraw.Handle(contextx.SetDB(ctx, tx), command.WithdrawInstanceCmd{InstanceID: inst.ID, Operator: input.Applicant, Caller: input.Caller})
		return err
	}), "Should withdraw the started instance")
	input.FormData = map[string]any{"nested": map[string]int{"a": 1, "b": 2}}
	replay, err := start(input)
	s.Require().NoError(err, "Canonical JSON and the original key should replay")
	s.Equal(approval.InstanceWithdrawn, replay.Status, "A replay returns the current instance")
	in := approval.ResubmitInstanceInput{InstanceID: inst.ID, Operator: input.Applicant, Caller: input.Caller, IdempotencyKey: input.IdempotencyKey}
	s.ErrorIs(resubmit(in), approval.ErrIdempotencyConflict, "An operation cannot take another operation's key")
	in.IdempotencyKey = "client/resubmit"
	decision = veto
	s.ErrorIs(resubmit(in), veto, "A verifier can veto re-entry")
	s.EqualValues(1, hostWrites(), "A resubmit veto rolls back the host update")
	persisted := approval.Instance{ID: inst.ID}
	s.Require().NoError(db.NewSelect().Model(&persisted).WherePK().Scan(ctx), "Should reload after veto")
	s.Equal(approval.InstanceWithdrawn, persisted.Status, "A resubmit veto rolls back the transition")
	decision = nil
	entered, release = make(chan struct{}, 1), make(chan struct{})
	raceSubmission(s.T(), entered, release, func() (struct{}, error) { return struct{}{}, resubmit(in) })
	s.Require().Len(entries, 4, "The resubmit race verifies only the winning retry")
	checkEntry(entries[3], true)
	s.NotEqual(entries[1].Tasks[0].VisitID, entries[3].Tasks[0].VisitID, "Old visits do not appear in the verification")
	s.EqualValues(4, calls.Load(), "Replays and key conflicts must bypass Submit")
	s.EqualValues(2, hostWrites(), "Exactly one start and one resubmission committed")
	changed := in
	changed.Operator.ID = "different"
	s.ErrorIs(resubmit(changed), approval.ErrIdempotencyConflict, "A different applicant cannot replay a resubmission")
	in.IdempotencyKey = ""
	s.ErrorIs(resubmit(in), approval.ErrResubmitNotAllowed, "An unkeyed resubmit still enforces the current state")
	entered, release = nil, nil
	input.IdempotencyKey = ""
	one, err := start(input)
	s.Require().NoError(err, "An unkeyed start should succeed")
	two, err := start(input)
	s.Require().NoError(err, "Repeating an unkeyed start creates another instance")
	s.NotEqual(one.ID, two.ID, "A missing key must never cause deduplication")
}

// Hold the winning transaction inside the host submission while its duplicate
// begins. Both callers use ordinary database transactions on all three dialects.
func raceSubmission[T any](t *testing.T, entered, release chan struct{}, run func() (T, error)) []T {
	t.Helper()
	type Attempt struct {
		value T
		err   error
	}
	done := make(chan Attempt, 2)
	submit := func() { v, err := run(); done <- Attempt{v, err} }
	go submit()
	select {
	case <-entered:
	case r := <-done:
		t.Fatalf("first submission exited before the host hook: %v", r.err)
	case <-time.After(10 * time.Second):
		t.Fatal("first submission did not enter the host hook")
	}
	go submit()
	select {
	case r := <-done:
		close(release)
		t.Fatalf("uncommitted duplicate returned early: %v", r.err)
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	var results []T
	for range 2 {
		r := <-done
		require.NoError(t, r.err, "Both submissions must succeed")
		results = append(results, r.value)
	}
	return results
}

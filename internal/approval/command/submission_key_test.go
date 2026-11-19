package command_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun/dialect"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/id"
	"github.com/coldsmirk/vef-framework-go/internal/approval/command"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
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

	var (
		calls   atomic.Int32
		entries []*approval.StartVerification
	)

	veto := errors.New("entry veto")

	var (
		decision         error
		entered, release chan struct{}
	)

	handler := VerifiedSubmission{
		submit: func(ctx context.Context, tx orm.DB, sc *approval.SubmissionContext) (*approval.Submission, error) {
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
		},
		verify: func(entry *approval.StartVerification) error {
			entries = append(entries, entry)

			return decision
		},
	}
	reg := testRegistry(handler)
	eng := buildTestEngine(db)
	validation := service.NewValidationService(nil)
	startHandler := command.NewStartInstanceHandler(db, eng, new(MockInstanceNoGenerator), validation, reg, nil, nil)
	instSvc := service.NewInstanceService(nil)
	submitHandler := command.NewResubmitInstanceHandler(db, eng, validation, instSvc, nil, reg)
	input := approval.StartInstanceInput{
		FlowCode:       "verified-flow",
		IdempotencyKey: "client/start",
		Applicant:      approval.UserInfo{ID: "author"},
		Caller:         approval.SystemCaller,
		FormData:       map[string]any{"nested": json.RawMessage(`{"b":2,"a":1}`)},
	}
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

	s.Run("TasklessEntryAndRetention", func() {
		direct := deployAndPublishFlow(s.T(), ctx, db, "verified-taskless", simpleFlowDef())
		pin := &approval.FlowVersion{ID: direct.VersionID, BusinessKind: version.BusinessKind}
		_, err := db.NewUpdate().Model(pin).Select("business_kind").WherePK().Exec(ctx)
		s.Require().NoError(err, "Should pin the verifier on the immediate-completion flow")

		input.FlowCode = "verified-taskless-flow"
		input.IdempotencyKey = "client/taskless"
		inst, err := start(input)
		s.Require().NoError(err, "A verified flow may complete on entry")

		entry := entries[len(entries)-1]
		s.Equal(inst.ID, entry.Instance.ID, "Should verify the instance that just completed")
		s.Equal(approval.InstanceApproved, entry.Instance.Status, "Verification sees the result of traversal")
		s.NotNil(entry.Tasks, "A taskless traversal supplies an empty slice")
		s.Empty(entry.Tasks, "Immediate completion creates no seats")

		_, err = db.NewDelete().Model(inst).WherePK().Exec(ctx)
		s.Require().NoError(err, "Should delete the completed test instance")
		recreated, err := start(input)
		s.Require().NoError(err, "Deleting an instance must release its submission key")
		s.NotEqual(inst.ID, recreated.ID, "The released key identifies a fresh submission")

		_, err = db.NewUpdate().Model(&approval.Flow{ID: direct.FlowID}).Set("is_shared", true).WherePK().Exec(ctx)
		s.Require().NoError(err, "Should make the same flow available in another tenant")

		foreign := input
		foreign.TenantID = "other-tenant"
		other, err := start(foreign)
		s.Require().NoError(err, "Two tenants can use the same key independently")
		s.Equal(foreign.TenantID, other.TenantID, "A shared flow uses the instance tenant's key space")
		s.NotEqual(recreated.ID, other.ID, "A tenant must never replay another tenant's instance")
	})
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

// MySQL hosts can supply a REPEATABLE READ transaction. RunInTx itself uses
// READ COMMITTED; using it for MySQL would hide stale consistent reads.
func runSubmissionTx(ctx context.Context, db orm.DB, fn func(context.Context, orm.DB) error) error {
	if db.NewSelect().Dialect().Name() != dialect.MySQL {
		return db.RunInTx(ctx, fn)
	}

	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if err := fn(ctx, tx); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *StartInstanceTestSuite) TestKeyedResubmissionRace() {
	ctx, cancel := context.WithTimeout(s.ctx, 30*time.Second)
	defer cancel()

	instSvc := service.NewInstanceService(nil)
	withdraw := command.NewWithdrawInstanceHandler(s.db, service.NewTaskService(), instSvc)
	resubmit := command.NewResubmitInstanceHandler(s.db, buildTestEngine(s.db), service.NewValidationService(nil), instSvc, nil, nil)
	start := command.StartInstanceCmd{FlowCode: "apv-cmd-test-flow", Applicant: approval.UserInfo{ID: "author"}, Caller: approval.SystemCaller}

	for _, tc := range []struct{ name, first, second string }{
		{"SameKey", "same", "same"}, {"DistinctKeys", "one", "two"},
	} {
		s.Run(tc.name, func() {
			inst, err := s.handler.Handle(ctx, start)
			s.Require().NoError(err, "Should start a fresh instance")
			_, err = withdraw.Handle(ctx, command.WithdrawInstanceCmd{InstanceID: inst.ID, Operator: start.Applicant, Caller: start.Caller})
			s.Require().NoError(err, "Should withdraw before the competing resubmissions")

			ready := make(chan struct{}, 2)
			begin := make(chan struct{})

			done := make(chan error, 2)
			for _, key := range []string{tc.first, tc.second} {
				go func() {
					done <- runSubmissionTx(ctx, s.db, func(ctx context.Context, tx orm.DB) error {
						ready <- struct{}{}

						select {
						case <-begin:
						case <-ctx.Done():
							return ctx.Err()
						}

						_, err := resubmit.Handle(contextx.SetDB(ctx, tx), command.ResubmitInstanceCmd{
							InstanceID: inst.ID, IdempotencyKey: key, Operator: start.Applicant, Caller: start.Caller,
						})

						return err
					})
				}()
			}

			for range 2 {
				select {
				case <-ready:
				case err := <-done:
					s.Require().FailNow("Transaction ended before the race", "%v", err)
				case <-ctx.Done():
					s.Require().FailNow("Timed out waiting for both transactions")
				}
			}

			close(begin)

			errorsSeen := []error{<-done, <-done}
			if tc.first == tc.second {
				s.Equal([]error{nil, nil}, errorsSeen, "The winner and duplicate should both succeed")
			} else {
				s.ElementsMatch([]error{nil, approval.ErrResubmitNotAllowed}, errorsSeen, "Only the already-running transition may refuse the losing key")
			}
		})
	}
}

func (s *StartInstanceTestSuite) TestSubmissionReplayWithPriorRead() {
	ctx := s.ctx
	input := command.StartInstanceCmd{
		FlowCode: "apv-cmd-test-flow", IdempotencyKey: "snapshot-start", Applicant: approval.UserInfo{ID: "author"}, Caller: approval.SystemCaller,
	}

	var inst *approval.Instance
	// Keep the caller's read transaction open while a competing request
	// starts and commits. The same schedule covers both operations.
	checkReplay := func(act func(context.Context, orm.DB) error) {
		s.T().Helper()
		err := runSubmissionTx(ctx, s.db, func(ctx context.Context, tx orm.DB) error {
			_, err := tx.NewSelect().Model((*approval.Instance)(nil)).Count(ctx)
			if err != nil {
				return err
			}

			if err = runSubmissionTx(ctx, s.db, act); err != nil {
				return err
			}

			return act(ctx, tx)
		})

		if s.db.NewSelect().Dialect().Name() == dialect.SQLite {
			// The old WAL snapshot cannot become a writer. A host that has
			// read before submission must retry its entire transaction.
			if coded, ok := errors.AsType[interface{ Code() int }](err); ok {
				s.Equal(517, coded.Code(), "A competing commit invalidated the read snapshot (SQLITE_BUSY_SNAPSHOT)")
			} else {
				// The cgo SQLite driver exposes the code as a field instead of a method.
				s.Require().ErrorContains(err, "database is locked", "The stale reader cannot upgrade to a writer")
			}

			err = runSubmissionTx(ctx, s.db, act)
		}

		s.Require().NoError(err, "Replaying a committed key should succeed")
	}

	s.Run("Start", func() {
		checkReplay(func(ctx context.Context, tx orm.DB) error {
			current, err := s.handler.Handle(contextx.SetDB(ctx, tx), input)
			if err == nil {
				if inst == nil {
					inst = current
				} else {
					s.Equal(inst.ID, current.ID, "The duplicate must return the winner's instance")
				}
			}

			return err
		})
	})
	s.Require().NotNil(inst, "The competing start must commit an instance")

	instSvc := service.NewInstanceService(nil)
	withdraw := command.NewWithdrawInstanceHandler(s.db, service.NewTaskService(), instSvc)
	_, err := withdraw.Handle(ctx, command.WithdrawInstanceCmd{InstanceID: inst.ID, Operator: input.Applicant, Caller: input.Caller})
	s.Require().NoError(err, "Should make the instance resubmittable")

	resubmit := command.NewResubmitInstanceHandler(s.db, buildTestEngine(s.db), service.NewValidationService(nil), instSvc, nil, nil)
	s.Run("Resubmit", func() {
		checkReplay(func(ctx context.Context, tx orm.DB) error {
			_, err := resubmit.Handle(contextx.SetDB(ctx, tx), command.ResubmitInstanceCmd{
				InstanceID: inst.ID, IdempotencyKey: "snapshot-resubmit", Operator: input.Applicant, Caller: input.Caller,
			})

			return err
		})
	})
}

func (s *StartInstanceTestSuite) TestSubmissionKeyBoundaries() {
	start := func(key string) (*approval.Instance, error) {
		return s.handler.Handle(s.ctx, command.StartInstanceCmd{
			FlowCode: "apv-cmd-test-flow", IdempotencyKey: key, Applicant: approval.UserInfo{ID: "author"}, Caller: approval.SystemCaller,
		})
	}
	for _, key := range []string{strings.Repeat("审", 129), "ab\x00cd", string([]byte{0xff})} {
		_, err := start(key)
		bad, ok := result.AsErr(err)
		s.Require().True(ok, "Invalid keys must return a business error")
		s.Equal(result.ErrCodeBadRequest, bad.Code, "Invalid keys must be rejected before reaching the database")
	}

	seen := make(map[string]bool)
	for _, key := range []string{strings.Repeat("审", 128), strings.Repeat("📄", 128), "Key", "key", "key "} {
		instance, err := start(key)
		s.Require().NoError(err, "A valid key should start the flow")
		s.False(seen[instance.ID], "Each byte-distinct key should claim a new instance")
		seen[instance.ID] = true
		replay, err := start(key)
		s.Require().NoError(err, "Same key should replay")
		s.Equal(instance.ID, replay.ID, "Only identical keys replay the same instance")
	}
}

package binding

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// compositeRefResolver decodes a JSON ref of the form {"id":"..."} — the
// canonical example of a host resolver for non-single-key refs.
type compositeRefResolver struct{}

func (*compositeRefResolver) ResolveRecordID(_ context.Context, _ *approval.Flow, businessRef string) (string, error) {
	var ref struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal([]byte(businessRef), &ref); err != nil {
		return "", fmt.Errorf("decode composite ref: %w", err)
	}

	return ref.ID, nil
}

// failingResolver simulates a host resolver hitting a transient fault.
type failingResolver struct{ err error }

func (r *failingResolver) ResolveRecordID(context.Context, *approval.Flow, string) (string, error) {
	return "", r.err
}

// emptyResolver resolves every ref to an empty record id.
type emptyResolver struct{}

func (*emptyResolver) ResolveRecordID(context.Context, *approval.Flow, string) (string, error) {
	return "", nil
}

func newBusinessFlow(table, pk, status string) *approval.Flow {
	flow := &approval.Flow{
		BindingMode:         approval.BindingBusiness,
		BusinessTable:       &table,
		BusinessPKField:     &pk,
		BusinessStatusField: &status,
	}
	flow.ID = "flow-1"

	return flow
}

// withLinkageColumns configures the optional write-back columns on the flow.
func withLinkageColumns(flow *approval.Flow, instanceID, startedAt, finishedAt string) *approval.Flow {
	if instanceID != "" {
		flow.BusinessInstanceIDField = &instanceID
	}

	if startedAt != "" {
		flow.BusinessStartedAtField = &startedAt
	}

	if finishedAt != "" {
		flow.BusinessFinishedAtField = &finishedAt
	}

	return flow
}

func newBoundInstance(ref string, status approval.InstanceStatus) *approval.Instance {
	instance := &approval.Instance{BusinessRef: &ref, Status: status}
	instance.ID = "inst-1"

	return instance
}

func setupBusinessTable(t *testing.T, db orm.DB) {
	t.Helper()

	_, err := db.NewRaw(`CREATE TABLE biz_order (
		id VARCHAR(64) PRIMARY KEY,
		approval_status VARCHAR(32),
		apv_instance_id VARCHAR(32),
		apv_started_at TIMESTAMP,
		apv_finished_at TIMESTAMP
	)`).Exec(t.Context())
	require.NoError(t, err, "test setup: create business table")

	_, err = db.NewRaw(`INSERT INTO biz_order (id, approval_status) VALUES ('ord-1', 'submitted')`).Exec(t.Context())
	require.NoError(t, err, "test setup: seed business row")
}

// seedLinkage stamps pre-existing linkage values on the seeded row so tests
// can tell "cleared" and "left untouched" apart.
func seedLinkage(t *testing.T, db orm.DB, instanceID, startedAt, finishedAt string) {
	t.Helper()

	_, err := db.NewRaw(
		`UPDATE biz_order SET apv_instance_id = ?, apv_started_at = ?, apv_finished_at = ? WHERE id = 'ord-1'`,
		nullable(instanceID), nullable(startedAt), nullable(finishedAt),
	).Exec(t.Context())
	require.NoError(t, err, "test setup: seed linkage columns")
}

func nullable(v string) any {
	if v == "" {
		return nil
	}

	return v
}

// orderRow is the business row projected into NULL-ness flags so assertions
// never depend on how the sqlite driver decodes timestamps.
type orderRow struct {
	Status         string
	InstanceID     string
	StartedAtNull  bool
	FinishedAtNull bool
}

func fetchOrder(t *testing.T, db orm.DB) orderRow {
	t.Helper()

	var row orderRow

	err := db.NewRaw(`SELECT approval_status, COALESCE(apv_instance_id, ''),
		apv_started_at IS NULL, apv_finished_at IS NULL
		FROM biz_order WHERE id = 'ord-1'`).
		Scan(t.Context(), &row.Status, &row.InstanceID, &row.StartedAtNull, &row.FinishedAtNull)
	require.NoError(t, err, "read back business row")

	return row
}

func TestWriterWriteBack(t *testing.T) {
	t.Run("SkipsStandaloneFlows", func(t *testing.T) {
		writer := NewWriter(NewIdentityResolver())
		flow := &approval.Flow{BindingMode: approval.BindingStandalone}

		err := writer.WriteBack(t.Context(), nil, flow, newBoundInstance("ord-1", approval.InstanceApproved), approval.BindingTriggerCompleted)
		assert.NoError(t, err, "Standalone flows must skip the write-back without touching the DB")
	})

	t.Run("SkipsInstancesWithoutRef", func(t *testing.T) {
		writer := NewWriter(NewIdentityResolver())
		flow := newBusinessFlow("biz_order", "id", "approval_status")

		err := writer.WriteBack(t.Context(), nil, flow, &approval.Instance{Status: approval.InstanceApproved}, approval.BindingTriggerCompleted)
		assert.NoError(t, err, "Nil BusinessRef must skip the write-back")

		blank := "   "
		err = writer.WriteBack(t.Context(), nil, flow, &approval.Instance{BusinessRef: &blank, Status: approval.InstanceApproved}, approval.BindingTriggerCompleted)
		assert.NoError(t, err, "Blank BusinessRef must skip the write-back")
	})

	t.Run("RejectsMissingConfiguration", func(t *testing.T) {
		writer := NewWriter(NewIdentityResolver())
		flow := newBusinessFlow("biz_order", "id", "approval_status")
		flow.BusinessStatusField = nil

		err := writer.WriteBack(t.Context(), nil, flow, newBoundInstance("ord-1", approval.InstanceApproved), approval.BindingTriggerCompleted)
		assert.ErrorIs(t, err, ErrBindingMisconfigured, "Missing status column must be misconfiguration, not a retryable fault")
	})

	t.Run("RejectsUnsafeIdentifiers", func(t *testing.T) {
		writer := NewWriter(NewIdentityResolver())
		flow := newBusinessFlow("biz_order; DROP TABLE x", "id", "approval_status")

		err := writer.WriteBack(t.Context(), nil, flow, newBoundInstance("ord-1", approval.InstanceApproved), approval.BindingTriggerCompleted)
		require.ErrorIs(t, err, ErrBindingMisconfigured, "Unsafe identifiers must be rejected before interpolation")
		assert.ErrorIs(t, err, approval.ErrInvalidBusinessIdentifier, "The identifier validation error should be preserved in the chain")
	})

	t.Run("RejectsUnsafeOptionalIdentifiers", func(t *testing.T) {
		writer := NewWriter(NewIdentityResolver())
		flow := withLinkageColumns(newBusinessFlow("biz_order", "id", "approval_status"), "col; --", "", "")

		err := writer.WriteBack(t.Context(), nil, flow, newBoundInstance("ord-1", approval.InstanceRunning), approval.BindingTriggerStarted)
		require.ErrorIs(t, err, ErrBindingMisconfigured, "Optional linkage columns must pass the same identifier whitelist")
		assert.ErrorIs(t, err, approval.ErrInvalidBusinessIdentifier, "The identifier validation error should be preserved in the chain")
	})

	t.Run("StartedProjectsConfiguredColumns", func(t *testing.T) {
		db := testx.NewTestDB(t)
		setupBusinessTable(t, db)
		// A previous approval round left its instance id and finish time behind.
		seedLinkage(t, db, "inst-0", "2020-01-01 00:00:00", "2020-01-02 00:00:00")

		writer := NewWriter(NewIdentityResolver())
		flow := withLinkageColumns(newBusinessFlow("biz_order", "id", "approval_status"),
			"apv_instance_id", "apv_started_at", "apv_finished_at")

		err := writer.WriteBack(t.Context(), db, flow, newBoundInstance("ord-1", approval.InstanceRunning), approval.BindingTriggerStarted)
		require.NoError(t, err, "Started write-back should succeed with all linkage columns configured")

		row := fetchOrder(t, db)
		assert.Equal(t, "running", row.Status, "Started must project the running status")
		assert.Equal(t, "inst-1", row.InstanceID, "Started must stamp the new instance id")
		assert.False(t, row.StartedAtNull, "Started must stamp the start time")
		assert.True(t, row.FinishedAtNull, "Started must clear a finish time left over from a previous round")
	})

	t.Run("StartedWithoutOptionalColumnsWritesStatusOnly", func(t *testing.T) {
		db := testx.NewTestDB(t)
		setupBusinessTable(t, db)
		seedLinkage(t, db, "inst-0", "", "2020-01-02 00:00:00")

		writer := NewWriter(NewIdentityResolver())
		flow := newBusinessFlow("biz_order", "id", "approval_status")

		err := writer.WriteBack(t.Context(), db, flow, newBoundInstance("ord-1", approval.InstanceRunning), approval.BindingTriggerStarted)
		require.NoError(t, err, "Started write-back should succeed without optional columns")

		row := fetchOrder(t, db)
		assert.Equal(t, "running", row.Status, "Status column must always be written")
		assert.Equal(t, "inst-0", row.InstanceID, "An unconfigured instance-id column must stay untouched")
		assert.False(t, row.FinishedAtNull, "An unconfigured finished-at column must stay untouched")
	})

	t.Run("CompletedStampsFinishedAt", func(t *testing.T) {
		db := testx.NewTestDB(t)
		setupBusinessTable(t, db)
		seedLinkage(t, db, "inst-1", "2020-01-01 00:00:00", "")

		writer := NewWriter(NewIdentityResolver())
		flow := withLinkageColumns(newBusinessFlow("biz_order", "id", "approval_status"),
			"apv_instance_id", "apv_started_at", "apv_finished_at")

		instance := newBoundInstance("ord-1", approval.InstanceApproved)
		now := timex.Now()
		instance.FinishedAt = &now

		err := writer.WriteBack(t.Context(), db, flow, instance, approval.BindingTriggerCompleted)
		require.NoError(t, err, "Completed write-back should succeed")

		row := fetchOrder(t, db)
		assert.Equal(t, "approved", row.Status, "Completed must project the final status")
		assert.Equal(t, "inst-1", row.InstanceID, "Completed must not rewrite the instance id")
		assert.False(t, row.StartedAtNull, "Completed must not touch the start time")
		assert.False(t, row.FinishedAtNull, "Completed must stamp the finish time")
	})

	t.Run("ReturnedWritesStatusOnly", func(t *testing.T) {
		db := testx.NewTestDB(t)
		setupBusinessTable(t, db)
		seedLinkage(t, db, "inst-1", "2020-01-01 00:00:00", "")

		writer := NewWriter(NewIdentityResolver())
		flow := withLinkageColumns(newBusinessFlow("biz_order", "id", "approval_status"),
			"apv_instance_id", "apv_started_at", "apv_finished_at")

		// The engine stamps Instance.FinishedAt when a flow returns to the
		// initiator, but the business row must NOT mirror it: for the business
		// side the round is paused, not finished.
		instance := newBoundInstance("ord-1", approval.InstanceReturned)
		now := timex.Now()
		instance.FinishedAt = &now

		err := writer.WriteBack(t.Context(), db, flow, instance, approval.BindingTriggerReturned)
		require.NoError(t, err, "Returned write-back should succeed")

		row := fetchOrder(t, db)
		assert.Equal(t, "returned", row.Status, "Returned must project the returned status")
		assert.True(t, row.FinishedAtNull, "Returned must not write the finish time even though the instance carries one")
		assert.False(t, row.StartedAtNull, "Returned must not touch the start time")
	})

	t.Run("WithdrawnWritesStatusOnly", func(t *testing.T) {
		db := testx.NewTestDB(t)
		setupBusinessTable(t, db)
		seedLinkage(t, db, "inst-1", "2020-01-01 00:00:00", "")

		writer := NewWriter(NewIdentityResolver())
		flow := withLinkageColumns(newBusinessFlow("biz_order", "id", "approval_status"),
			"apv_instance_id", "apv_started_at", "apv_finished_at")

		err := writer.WriteBack(t.Context(), db, flow, newBoundInstance("ord-1", approval.InstanceWithdrawn), approval.BindingTriggerWithdrawn)
		require.NoError(t, err, "Withdrawn write-back should succeed")

		row := fetchOrder(t, db)
		assert.Equal(t, "withdrawn", row.Status, "Withdrawn must project the withdrawn status")
		assert.True(t, row.FinishedAtNull, "Withdrawn must not write the finish time")
		assert.False(t, row.StartedAtNull, "Withdrawn must not touch the start time")
	})

	t.Run("ResubmittedClearsFinishedAt", func(t *testing.T) {
		db := testx.NewTestDB(t)
		setupBusinessTable(t, db)
		seedLinkage(t, db, "inst-1", "2020-01-01 00:00:00", "2020-01-02 00:00:00")

		writer := NewWriter(NewIdentityResolver())
		flow := withLinkageColumns(newBusinessFlow("biz_order", "id", "approval_status"),
			"apv_instance_id", "apv_started_at", "apv_finished_at")

		// resubmit_instance clears Instance.FinishedAt before the event fires;
		// the projection must propagate the NULL.
		err := writer.WriteBack(t.Context(), db, flow, newBoundInstance("ord-1", approval.InstanceRunning), approval.BindingTriggerResubmitted)
		require.NoError(t, err, "Resubmitted write-back should succeed")

		row := fetchOrder(t, db)
		assert.Equal(t, "running", row.Status, "Resubmitted must project the running status")
		assert.True(t, row.FinishedAtNull, "Resubmitted must clear the finish time")
		assert.False(t, row.StartedAtNull, "Resubmitted must keep the original start time")
		assert.Equal(t, "inst-1", row.InstanceID, "Resubmitted must keep the instance id")
	})

	t.Run("WritesStatusWithCompositeResolver", func(t *testing.T) {
		db := testx.NewTestDB(t)
		setupBusinessTable(t, db)

		writer := NewWriter(new(compositeRefResolver))
		flow := newBusinessFlow("biz_order", "id", "approval_status")
		instance := newBoundInstance(`{"id":"ord-1","region":"cn"}`, approval.InstanceRejected)

		err := writer.WriteBack(t.Context(), db, flow, instance, approval.BindingTriggerCompleted)
		require.NoError(t, err, "A host resolver should unlock composite refs for the built-in write-back")
		assert.Equal(t, "rejected", fetchOrder(t, db).Status, "The row located through the resolved key should be updated")
	})

	t.Run("PropagatesResolverErrorsAsTransient", func(t *testing.T) {
		cause := errors.New("mapping table unavailable")
		writer := NewWriter(&failingResolver{err: cause})
		flow := newBusinessFlow("biz_order", "id", "approval_status")

		err := writer.WriteBack(t.Context(), nil, flow, newBoundInstance("ord-1", approval.InstanceApproved), approval.BindingTriggerCompleted)
		require.ErrorIs(t, err, cause, "Resolver errors must propagate for outbox retry")
		assert.NotErrorIs(t, err, ErrBindingMisconfigured, "Resolver faults are transient, not misconfiguration")
	})

	t.Run("RejectsEmptyResolvedRecordID", func(t *testing.T) {
		writer := NewWriter(new(emptyResolver))
		flow := newBusinessFlow("biz_order", "id", "approval_status")

		err := writer.WriteBack(t.Context(), nil, flow, newBoundInstance("ord-1", approval.InstanceApproved), approval.BindingTriggerCompleted)
		assert.ErrorIs(t, err, ErrBindingMisconfigured, "An empty resolved record id can never match a row and must not be retried")
	})
}

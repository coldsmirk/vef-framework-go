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
		BusinessPkField:     &pk,
		BusinessStatusField: &status,
	}
	flow.ID = "flow-1"

	return flow
}

func newBoundInstance(ref string) *approval.Instance {
	instance := &approval.Instance{BusinessRef: &ref}
	instance.ID = "inst-1"

	return instance
}

func setupBusinessTable(t *testing.T, db orm.DB) {
	t.Helper()

	_, err := db.NewRaw(`CREATE TABLE biz_order (id VARCHAR(64) PRIMARY KEY, approval_status VARCHAR(32))`).Exec(t.Context())
	require.NoError(t, err, "test setup: create business table")

	_, err = db.NewRaw(`INSERT INTO biz_order (id, approval_status) VALUES ('ord-1', 'submitted')`).Exec(t.Context())
	require.NoError(t, err, "test setup: seed business row")
}

func fetchOrderStatus(t *testing.T, db orm.DB) string {
	t.Helper()

	var status string

	err := db.NewRaw(`SELECT approval_status FROM biz_order WHERE id = 'ord-1'`).Scan(t.Context(), &status)
	require.NoError(t, err, "read back business status")

	return status
}

func TestWriterWriteBackStatus(t *testing.T) {
	t.Run("SkipsStandaloneFlows", func(t *testing.T) {
		writer := NewWriter(NewIdentityResolver())
		flow := &approval.Flow{BindingMode: approval.BindingStandalone}

		err := writer.WriteBackStatus(t.Context(), nil, flow, newBoundInstance("ord-1"), approval.InstanceApproved)
		assert.NoError(t, err, "Standalone flows must skip the write-back without touching the DB")
	})

	t.Run("SkipsInstancesWithoutRef", func(t *testing.T) {
		writer := NewWriter(NewIdentityResolver())
		flow := newBusinessFlow("biz_order", "id", "approval_status")

		err := writer.WriteBackStatus(t.Context(), nil, flow, &approval.Instance{}, approval.InstanceApproved)
		assert.NoError(t, err, "Nil BusinessRef must skip the write-back")

		blank := "   "
		err = writer.WriteBackStatus(t.Context(), nil, flow, &approval.Instance{BusinessRef: &blank}, approval.InstanceApproved)
		assert.NoError(t, err, "Blank BusinessRef must skip the write-back")
	})

	t.Run("RejectsMissingConfiguration", func(t *testing.T) {
		writer := NewWriter(NewIdentityResolver())
		flow := newBusinessFlow("biz_order", "id", "approval_status")
		flow.BusinessStatusField = nil

		err := writer.WriteBackStatus(t.Context(), nil, flow, newBoundInstance("ord-1"), approval.InstanceApproved)
		assert.ErrorIs(t, err, ErrBindingMisconfigured, "Missing status column must be misconfiguration, not a retryable fault")
	})

	t.Run("RejectsUnsafeIdentifiers", func(t *testing.T) {
		writer := NewWriter(NewIdentityResolver())
		flow := newBusinessFlow("biz_order; DROP TABLE x", "id", "approval_status")

		err := writer.WriteBackStatus(t.Context(), nil, flow, newBoundInstance("ord-1"), approval.InstanceApproved)
		require.ErrorIs(t, err, ErrBindingMisconfigured, "Unsafe identifiers must be rejected before interpolation")
		assert.ErrorIs(t, err, approval.ErrInvalidBusinessIdentifier, "The identifier validation error should be preserved in the chain")
	})

	t.Run("WritesStatusWithIdentityResolver", func(t *testing.T) {
		db := testx.NewTestDB(t)
		setupBusinessTable(t, db)

		writer := NewWriter(NewIdentityResolver())
		flow := newBusinessFlow("biz_order", "id", "approval_status")

		err := writer.WriteBackStatus(t.Context(), db, flow, newBoundInstance("ord-1"), approval.InstanceApproved)
		require.NoError(t, err, "Write-back with a single-key ref should succeed")
		assert.Equal(t, "approved", fetchOrderStatus(t, db), "Business status column should carry the final status")
	})

	t.Run("WritesStatusWithCompositeResolver", func(t *testing.T) {
		db := testx.NewTestDB(t)
		setupBusinessTable(t, db)

		writer := NewWriter(new(compositeRefResolver))
		flow := newBusinessFlow("biz_order", "id", "approval_status")
		instance := newBoundInstance(`{"id":"ord-1","region":"cn"}`)

		err := writer.WriteBackStatus(t.Context(), db, flow, instance, approval.InstanceRejected)
		require.NoError(t, err, "A host resolver should unlock composite refs for the built-in write-back")
		assert.Equal(t, "rejected", fetchOrderStatus(t, db), "The row located through the resolved key should be updated")
	})

	t.Run("PropagatesResolverErrorsAsTransient", func(t *testing.T) {
		cause := errors.New("mapping table unavailable")
		writer := NewWriter(&failingResolver{err: cause})
		flow := newBusinessFlow("biz_order", "id", "approval_status")

		err := writer.WriteBackStatus(t.Context(), nil, flow, newBoundInstance("ord-1"), approval.InstanceApproved)
		require.ErrorIs(t, err, cause, "Resolver errors must propagate for outbox retry")
		assert.NotErrorIs(t, err, ErrBindingMisconfigured, "Resolver faults are transient, not misconfiguration")
	})

	t.Run("RejectsEmptyResolvedRecordID", func(t *testing.T) {
		writer := NewWriter(new(emptyResolver))
		flow := newBusinessFlow("biz_order", "id", "approval_status")

		err := writer.WriteBackStatus(t.Context(), nil, flow, newBoundInstance("ord-1"), approval.InstanceApproved)
		assert.ErrorIs(t, err, ErrBindingMisconfigured, "An empty resolved record id can never match a row and must not be retried")
	})
}

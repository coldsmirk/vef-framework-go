package approval_test

import (
	"context"
	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/stretchr/testify/require"
	"testing"
)

type orderRecord struct {
	orm.BaseModel `bun:"table:test_business_order"`
	ID            string `json:"id" bun:",pk"`
	Title         string `json:"title"`
	Restricted    string `json:"secret" bun:"private_note"`
}
type orderParams struct {
	ID         string `json:"id"`
	Title      string `json:"title" validate:"required"`
	Restricted string `json:"secret"`
}

func TestModelBusinessHandler(t *testing.T) {
	descriptor := approval.BusinessKindDescriptor{Kind: "order", Label: "Order", FormFields: []approval.FormFieldDefinition{{Key: "title"}, {Key: "secret"}}}
	t.Run("CompositeKey", func(t *testing.T) {
		type composite struct {
			Tenant string `bun:",pk"`
			ID     string `bun:",pk"`
		}
		_, err := approval.NewModelBusinessHandler[composite, orderParams](descriptor)
		require.Error(t, err, "Composite primary keys need a custom handler")
	})
	testx.ForEachDB(t, func(t *testing.T, env *testx.DBEnv) {
		ctx, db := env.Ctx, env.DB
		_, err := db.NewCreateTable().Model((*orderRecord)(nil)).Exec(ctx)
		require.NoError(t, err, "Should create the host table")
		h, err := approval.NewModelBusinessHandler[orderRecord, orderParams](descriptor)
		require.NoError(t, err, "A single string primary key should be supported")
		sc := &approval.SubmissionContext{FormData: map[string]any{"id": "one", "title": "first", "secret": "private"}}
		result, err := h.Submit(ctx, db, sc)
		require.NoError(t, err, "Should create the business record")
		require.Equal(t, "one", result.BusinessRef, "Reference should be the persisted key")
		require.Equal(t, map[string]any{"title": "first", "secret": "private"}, result.FormData, "Should project only catalog fields")
		sc.BusinessRef = new("one")
		sc.FormData = map[string]any{"id": "other", "title": "second", "secret": "private"}
		_, err = h.Submit(ctx, db, sc)
		require.ErrorIs(t, err, approval.ErrInvalidBusinessRef, "An existing reference requires authorization")
		accessCalled := false
		custom, err := approval.NewModelBusinessHandler[orderRecord, orderParams](descriptor, approval.WithRecordAccess(func(_ context.Context, _ orm.DB, in *approval.SubmissionContext, m *orderRecord) error {
			accessCalled = true
			require.Same(t, sc, in, "Access should receive the current submitter's context")
			require.Equal(t, "first", m.Title, "Should authorize the loaded record before updating it")
			return nil
		}), approval.WithSnapshot(func(m *orderRecord) map[string]any { return map[string]any{"summary": m.Title} }),
			approval.WithView(func(_ context.Context, _ orm.DB, vc *approval.ViewContext, m *orderRecord) (any, error) {
				return vc.Viewer.ID + "/" + m.Title, nil
			}))
		require.NoError(t, err, "Custom model projections should construct")
		result, err = custom.Submit(ctx, db, sc)
		require.NoError(t, err, "The host can authorize existing-record submissions")
		require.True(t, accessCalled, "The record access check must run")
		require.Equal(t, "one", result.BusinessRef, "The input cannot replace the existing primary key")
		require.Equal(t, map[string]any{"summary": "second"}, result.FormData, "Should use the custom snapshot")
		inst := &approval.Instance{BusinessRef: sc.BusinessRef}
		sc.Instance = inst
		sc.FormData["title"] = "resubmitted"
		_, err = h.Submit(ctx, db, sc)
		require.NoError(t, err, "The returned instance can update its record")
		ac := &approval.TaskActionContext{Instance: inst, FormData: map[string]any{"title": "approved", "secret": "attempt", "id": "other"}, FieldPermissions: map[string]approval.Permission{"title": approval.PermissionRequired, "secret": approval.PermissionHidden, "id": approval.PermissionEditable}}
		out, err := h.OnTaskAction(ctx, db, ac)
		require.NoError(t, err, "Should apply node-authorized edits")
		require.Equal(t, map[string]any{"title": "approved", "secret": "private"}, out.FormData, "Hidden fields and the primary key must survive unmodified")
		ac.FieldPermissions = nil
		out, err = h.OnTaskAction(ctx, db, ac)
		require.NoError(t, err, "A decision with no editable keys should succeed")
		require.Nil(t, out, "A no-op should preserve the snapshot")
		vc := &approval.ViewContext{Instance: inst, Viewer: approval.UserInfo{ID: "viewer"}, FieldPermissions: map[string]approval.Permission{"secret": approval.PermissionHidden}}
		view, err := h.View(ctx, db, vc)
		require.NoError(t, err, "Should read the current record")
		require.Equal(t, map[string]any{"id": "one", "title": "approved"}, view, "Hidden fields should be removed before returning the record")
		view, err = custom.View(ctx, db, vc)
		require.NoError(t, err, "Should run the custom view")
		require.Equal(t, "viewer/approved", view, "Custom views should receive both viewer and persisted model")
	})
}

package binding

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/schema"
)

type stubSchemaService struct {
	table *schema.TableSchema
	err   error
}

func (*stubSchemaService) ListTables(context.Context) ([]schema.Table, error) { return nil, nil }
func (s *stubSchemaService) GetTableSchema(context.Context, string) (*schema.TableSchema, error) {
	return s.table, s.err
}
func (*stubSchemaService) ListViews(context.Context) ([]schema.View, error) { return nil, nil }

func testBindingConfig() *approval.BusinessBindingConfig {
	instanceID := "approval_instance_id"

	return &approval.BusinessBindingConfig{
		TableName:        "biz_order",
		KeyColumns:       []string{"tenant_id", "order_no"},
		StatusColumn:     "approval_status",
		InstanceIDColumn: &instanceID,
	}
}

func testBusinessTable() *schema.TableSchema {
	return &schema.TableSchema{
		Name: "biz_order",
		Columns: []schema.Column{
			{Name: "tenant_id"},
			{Name: "order_no"},
			{Name: "approval_status"},
			{Name: "approval_instance_id"},
		},
		UniqueKeys: []schema.UniqueKey{{
			Name:    "uk_biz_order_tenant_order",
			Columns: []string{"tenant_id", "order_no"},
		}},
	}
}

func TestNormalizeConfig(t *testing.T) {
	t.Parallel()

	t.Run("Standalone", func(t *testing.T) {
		t.Parallel()

		config, err := NormalizeConfig(approval.BindingStandalone, nil)
		require.NoError(t, err, "Standalone flow without business binding should pass")
		assert.Nil(t, config, "Standalone flow should keep a nil binding")
	})

	t.Run("StandaloneRejectsBinding", func(t *testing.T) {
		t.Parallel()

		_, err := NormalizeConfig(approval.BindingStandalone, testBindingConfig())
		assert.ErrorIs(t, err, shared.ErrBindingUnexpected, "Standalone flow must reject dead business configuration")
	})

	t.Run("Normalizes", func(t *testing.T) {
		t.Parallel()

		blank := "  "
		config := testBindingConfig()
		config.TableName = "  biz_order "
		config.KeyColumns = []string{" order_no ", "tenant_id"}
		config.FinishedAtColumn = &blank
		config.StatusMapping = map[approval.InstanceStatus]string{
			approval.InstanceRunning: "  in_review  ",
		}

		normalized, err := NormalizeConfig(approval.BindingBusiness, config)
		require.NoError(t, err, "Complete business binding should normalize")
		assert.Equal(t, "biz_order", normalized.TableName, "Table name should be trimmed")
		assert.Equal(t, []string{"order_no", "tenant_id"}, normalized.KeyColumns, "Key columns should be sorted and trimmed")
		assert.Nil(t, normalized.FinishedAtColumn, "Blank optional column should normalize to nil")
		assert.Equal(t, "in_review", normalized.StatusMapping[approval.InstanceRunning], "Status mapping values should be trimmed")
		assert.NotSame(t, config, normalized, "Normalization should detach persisted configuration from request input")

		config.StatusMapping[approval.InstanceRunning] = "changed"
		assert.Equal(t, "in_review", normalized.StatusMapping[approval.InstanceRunning], "Normalized status mapping should not alias request input")
	})

	t.Run("RejectsMissing", func(t *testing.T) {
		t.Parallel()

		_, err := NormalizeConfig(approval.BindingBusiness, nil)
		assert.ErrorIs(t, err, shared.ErrBindingIncomplete, "Business flow requires binding configuration")
	})

	t.Run("RejectsMissingInstanceIDColumn", func(t *testing.T) {
		t.Parallel()

		config := testBindingConfig()
		config.InstanceIDColumn = nil

		_, err := NormalizeConfig(approval.BindingBusiness, config)
		assert.ErrorIs(t, err, shared.ErrBindingIncomplete, "Business flow requires an instance-ID fencing column")
	})

	t.Run("RejectsInvalidStatusMapping", func(t *testing.T) {
		t.Parallel()

		for name, mapping := range map[string]map[approval.InstanceStatus]string{
			"UnknownStatus": {approval.InstanceStatus("paused"): "paused"},
			"BlankValue":    {approval.InstanceRunning: "  "},
		} {
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				config := testBindingConfig()
				config.StatusMapping = mapping

				_, err := NormalizeConfig(approval.BindingBusiness, config)
				assert.ErrorIs(t, err, shared.ErrBindingStatusMappingInvalid,
					"Invalid status mapping should be rejected")
			})
		}
	})

	t.Run("RejectsUnsafeIdentifier", func(t *testing.T) {
		t.Parallel()

		config := testBindingConfig()
		config.TableName = "biz_order; DROP TABLE apv_flow"

		_, err := NormalizeConfig(approval.BindingBusiness, config)
		assert.ErrorIs(t, err, shared.ErrInvalidBusinessIdentifier, "Unsafe table name should be rejected")
	})

	t.Run("RejectsKeyWriteOverlap", func(t *testing.T) {
		t.Parallel()

		config := testBindingConfig()
		config.StatusColumn = "order_no"

		_, err := NormalizeConfig(approval.BindingBusiness, config)
		assert.ErrorIs(t, err, shared.ErrBindingColumnsConflict, "A write-back column must never mutate its own lookup key")
	})
}

func TestConfigValidatorValidateSchema(t *testing.T) {
	t.Parallel()

	t.Run("CompositeUnique", func(t *testing.T) {
		t.Parallel()

		validator := NewConfigValidator(&stubSchemaService{table: testBusinessTable()})
		err := validator.ValidateSchema(t.Context(), testBindingConfig())
		assert.NoError(t, err, "Complete composite unique key should pass")
	})

	t.Run("CompositePrimaryKey", func(t *testing.T) {
		t.Parallel()

		table := testBusinessTable()
		table.UniqueKeys = nil
		table.PrimaryKey = &schema.PrimaryKey{Columns: []string{"order_no", "tenant_id"}}

		validator := NewConfigValidator(&stubSchemaService{table: table})
		err := validator.ValidateSchema(t.Context(), testBindingConfig())
		assert.NoError(t, err, "Key column order should not affect an exact primary-key match")
	})

	t.Run("NonUnique", func(t *testing.T) {
		t.Parallel()

		table := testBusinessTable()
		table.UniqueKeys = nil

		validator := NewConfigValidator(&stubSchemaService{table: table})
		err := validator.ValidateSchema(t.Context(), testBindingConfig())
		assert.ErrorIs(t, err, shared.ErrBindingKeyNotUnique, "Ordinary columns cannot back a one-row binding")
	})

	t.Run("PartialUnique", func(t *testing.T) {
		t.Parallel()

		table := testBusinessTable()
		table.UniqueKeys[0].Predicate = "deleted_at IS NULL"

		validator := NewConfigValidator(&stubSchemaService{table: table})
		err := validator.ValidateSchema(t.Context(), testBindingConfig())
		assert.ErrorIs(t, err, shared.ErrBindingKeyNotUnique, "Partial unique index does not guarantee global uniqueness")
	})

	t.Run("ExpressionUnique", func(t *testing.T) {
		t.Parallel()

		table := testBusinessTable()
		table.UniqueKeys[0].HasExpressions = true

		validator := NewConfigValidator(&stubSchemaService{table: table})
		err := validator.ValidateSchema(t.Context(), testBindingConfig())
		assert.ErrorIs(t, err, shared.ErrBindingKeyNotUnique, "Expression index cannot back raw-column equality")
	})

	t.Run("NullableUnique", func(t *testing.T) {
		t.Parallel()

		table := testBusinessTable()
		table.Columns[1].Nullable = true

		validator := NewConfigValidator(&stubSchemaService{table: table})
		err := validator.ValidateSchema(t.Context(), testBindingConfig())
		assert.ErrorIs(t, err, shared.ErrBindingKeyNotUnique, "Nullable unique key has dialect-dependent NULL uniqueness")
	})

	t.Run("MissingColumn", func(t *testing.T) {
		t.Parallel()

		table := testBusinessTable()
		table.Columns = table.Columns[:3]

		validator := NewConfigValidator(&stubSchemaService{table: table})
		err := validator.ValidateSchema(t.Context(), testBindingConfig())
		assert.ErrorIs(t, err, shared.ErrBindingSchemaInvalid, "Every write-back column must exist")
	})

	t.Run("MissingTable", func(t *testing.T) {
		t.Parallel()

		validator := NewConfigValidator(&stubSchemaService{err: schema.ErrTableMissing})
		err := validator.ValidateSchema(t.Context(), testBindingConfig())
		assert.ErrorIs(t, err, shared.ErrBindingSchemaInvalid, "Missing business table should be rejected at flow save time")
	})
}

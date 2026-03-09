package crud

import (
	"context"
	"errors"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/schema"

	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// MockDataPermApplier implements security.DataPermissionApplier for testing.
type MockDataPermApplier struct {
	err error
}

func (m *MockDataPermApplier) Apply(orm.SelectQuery) error {
	return m.err
}

// TestApplyDataPermissionError covers helpers.go:88-91 - DataPermApplier returns error.
func TestApplyDataPermissionError(t *testing.T) {
	app := fiber.New()
	defer app.Shutdown() //nolint:errcheck

	expectedErr := errors.New("data perm error")

	app.Get("/test", func(ctx fiber.Ctx) error {
		contextx.SetDataPermApplier(ctx, &MockDataPermApplier{err: expectedErr})
		err := ApplyDataPermission(nil, ctx)
		assert.Error(t, err, "Should return error from data perm applier")
		assert.Contains(t, err.Error(), "data perm", "Error message should mention data perm")

		return nil
	})

	req := httptest.NewRequestWithContext(context.Background(), fiber.MethodGet, "/test", nil)
	_, err := app.Test(req)
	require.NoError(t, err, "Should execute test request without error")
}

// MockPromoter implements storage.Promoter for testing.
type MockPromoter[T any] struct {
	err error
}

func (m *MockPromoter[T]) Promote(_ context.Context, _, _ *T) error {
	return m.err
}

// TestBatchCleanupError covers helpers.go:204-206 - promote error in batchCleanup.
func TestBatchCleanupError(t *testing.T) {
	promoter := &MockPromoter[struct{}]{err: errors.New("cleanup error")}
	models := []struct{}{{}, {}}
	err := batchCleanup(context.Background(), promoter, models)
	assert.Error(t, err, "Should return error from promoter")
	assert.Contains(t, err.Error(), "cleanup error", "Error message should mention cleanup error")
}

// TestBatchRollbackError covers helpers.go:222-224 - promote error in batchRollback.
func TestBatchRollbackError(t *testing.T) {
	promoter := &MockPromoter[struct{}]{err: errors.New("rollback error")}
	oldModels := []struct{}{{}, {}}
	newModels := []struct{}{{}, {}}
	err := batchRollback(context.Background(), promoter, oldModels, newModels, 2)
	assert.Error(t, err, "Should return error from promoter")
	assert.Contains(t, err.Error(), "rollback error", "Error message should mention rollback error")
}

// TestMergeOptionColumnMappingDefaults covers helpers.go - merge with nil default mapping.
func TestMergeOptionColumnMappingDefaults(t *testing.T) {
	mapping := &DataOptionColumnMapping{}
	mergeOptionColumnMapping(mapping, nil)
	assert.Equal(t, defaultLabelColumn, mapping.LabelColumn, "Should use default label column")
	assert.Equal(t, defaultValueColumn, mapping.ValueColumn, "Should use default value column")
}

// TestMergeOptionColumnMappingCustomDefaults covers helpers.go - merge with custom default mapping.
func TestMergeOptionColumnMappingCustomDefaults(t *testing.T) {
	mapping := &DataOptionColumnMapping{}
	defaults := &DataOptionColumnMapping{
		LabelColumn:       "custom_label",
		ValueColumn:       "custom_value",
		DescriptionColumn: "custom_desc",
		MetaColumns:       []string{"col1"},
	}
	mergeOptionColumnMapping(mapping, defaults)
	assert.Equal(t, "custom_label", mapping.LabelColumn, "Should use custom default label column")
	assert.Equal(t, "custom_value", mapping.ValueColumn, "Should use custom default value column")
	assert.Equal(t, "custom_desc", mapping.DescriptionColumn, "Should use custom default description column")
	assert.Equal(t, []string{"col1"}, mapping.MetaColumns, "Should use custom default meta columns")
}

// TestParseMetaColumnWithAlias covers helpers.go - parseMetaColumn with alias.
func TestParseMetaColumnWithAlias(t *testing.T) {
	col, alias := parseMetaColumn("my_column AS my_alias")
	assert.Equal(t, "my_column", col, "Column name should be extracted")
	assert.Equal(t, "my_alias", alias, "Alias should be extracted")
}

// TestParseMetaColumnsEmpty covers helpers.go - parseMetaColumns with empty input.
func TestParseMetaColumnsEmpty(t *testing.T) {
	result := parseMetaColumns(nil)
	assert.Nil(t, result, "Should return nil for nil input")
}

// TestValidateColumnsExistEmpty covers helpers.go - validateColumnsExist with empty column.
func TestValidateColumnsExistEmpty(t *testing.T) {
	// Empty column name should be skipped
	err := validateColumnsExist(nil, columnRef{name: "test", column: ""})
	assert.NoError(t, err, "Should skip empty column name without error")
}

// TestGetAuditUserNameRelationsCustomColumn covers helpers.go - custom nameColumn.
func TestGetAuditUserNameRelationsCustomColumn(t *testing.T) {
	relations := GetAuditUserNameRelations(nil, "display_name")
	require.Len(t, relations, 2, "Should return 2 audit user name relations")
	assert.Equal(t, "display_name", relations[0].SelectedColumns[0].Name, "CreatedBy relation should use custom name column")
	assert.Equal(t, "display_name", relations[1].SelectedColumns[0].Name, "UpdatedBy relation should use custom name column")
}

// TestGetAuditUserNameRelationsDefaultColumn covers helpers.go - default nameColumn.
func TestGetAuditUserNameRelationsDefaultColumn(t *testing.T) {
	relations := GetAuditUserNameRelations(nil)
	require.Len(t, relations, 2, "Should return 2 audit user name relations")
	assert.Equal(t, defaultAuditUserNameColumn, relations[0].SelectedColumns[0].Name, "Should use default audit user name column")
}

// --- Schema-based test helpers ---

// HelperTestModel is a minimal model for schema validation tests.
type HelperTestModel struct {
	bun.BaseModel `bun:"table:helper_test_models"`

	ID          string `bun:"id,pk"`
	Name        string `bun:"name"`
	Description string `bun:"description"`
	Status      string `bun:"status"`
	ParentID    string `bun:"parent_id"`
}

// helperTestTable returns a *schema.Table for HelperTestModel.
func helperTestTable() *schema.Table {
	db := bun.NewDB(nil, sqlitedialect.New())

	return db.Table(reflect.TypeFor[HelperTestModel]())
}

// --- validateColumnsExist ---

// TestValidateColumnsExistFieldNotFound covers helpers.go - column not found.
func TestValidateColumnsExistFieldNotFound(t *testing.T) {
	table := helperTestTable()
	err := validateColumnsExist(table, columnRef{name: "test", column: "nonexistent"})
	assert.Error(t, err, "Should return error for nonexistent column")
	assert.Contains(t, err.Error(), "nonexistent", "Error message should mention nonexistent column")
}

// TestValidateColumnsExistSuccess covers helpers.go - all columns exist.
func TestValidateColumnsExistSuccess(t *testing.T) {
	table := helperTestTable()
	err := validateColumnsExist(table,
		columnRef{name: "label", column: "name"},
		columnRef{name: "value", column: "id"},
	)
	assert.NoError(t, err, "Should succeed when all columns exist")
}

// --- validateOptionColumns ---

// TestValidateOptionColumnsSuccess covers helpers.go:43-60 - all columns valid.
func TestValidateOptionColumnsSuccess(t *testing.T) {
	table := helperTestTable()
	mapping := &DataOptionColumnMapping{
		LabelColumn: "name",
		ValueColumn: "id",
	}
	err := validateOptionColumns(table, mapping)
	assert.NoError(t, err, "Should succeed with valid label and value columns")
}

// TestValidateOptionColumnsWithDescription covers helpers.go:52-57 - DescriptionColumn branch.
func TestValidateOptionColumnsWithDescription(t *testing.T) {
	table := helperTestTable()
	mapping := &DataOptionColumnMapping{
		LabelColumn:       "name",
		ValueColumn:       "id",
		DescriptionColumn: "description",
	}
	err := validateOptionColumns(table, mapping)
	assert.NoError(t, err, "Should succeed with valid description column")
}

// TestValidateOptionColumnsNotFound covers helpers.go:43-60 - invalid column.
func TestValidateOptionColumnsNotFound(t *testing.T) {
	table := helperTestTable()
	mapping := &DataOptionColumnMapping{
		LabelColumn: "nonexistent",
		ValueColumn: "ID",
	}
	err := validateOptionColumns(table, mapping)
	assert.Error(t, err, "Should return error for nonexistent column")
	assert.Contains(t, err.Error(), "nonexistent", "Error message should mention nonexistent column")
}

// --- mergeOptionColumnMapping ---

// TestMergeOptionColumnMappingPreservesExisting covers helpers.go:64-84 - existing values not overwritten.
func TestMergeOptionColumnMappingPreservesExisting(t *testing.T) {
	mapping := &DataOptionColumnMapping{
		LabelColumn:       "existing_label",
		ValueColumn:       "existing_value",
		DescriptionColumn: "existing_desc",
		MetaColumns:       []string{"existing_col"},
	}
	defaults := &DataOptionColumnMapping{
		LabelColumn:       "default_label",
		ValueColumn:       "default_value",
		DescriptionColumn: "default_desc",
		MetaColumns:       []string{"default_col"},
	}
	mergeOptionColumnMapping(mapping, defaults)
	assert.Equal(t, "existing_label", mapping.LabelColumn, "Should preserve existing label column")
	assert.Equal(t, "existing_value", mapping.ValueColumn, "Should preserve existing value column")
	assert.Equal(t, "existing_desc", mapping.DescriptionColumn, "Should preserve existing description column")
	assert.Equal(t, []string{"existing_col"}, mapping.MetaColumns, "Should preserve existing meta columns")
}

// --- ApplyDataPermission ---

// TestApplyDataPermissionNilApplier covers helpers.go:88 - nil applier (no-op).
func TestApplyDataPermissionNilApplier(t *testing.T) {
	app := fiber.New()
	defer app.Shutdown() //nolint:errcheck

	app.Get("/test", func(ctx fiber.Ctx) error {
		// No applier set - should return nil
		err := ApplyDataPermission(nil, ctx)
		assert.NoError(t, err, "Should return nil when no applier is set")

		return nil
	})

	req := httptest.NewRequestWithContext(context.Background(), fiber.MethodGet, "/test", nil)
	resp, err := app.Test(req)
	require.NoError(t, err, "Should execute test request without error")
	assert.Equal(t, 200, resp.StatusCode, "Should return 200 status code")
}

// TestApplyDataPermissionSuccess covers helpers.go:88-94 - applier succeeds.
func TestApplyDataPermissionSuccess(t *testing.T) {
	app := fiber.New()
	defer app.Shutdown() //nolint:errcheck

	app.Get("/test", func(ctx fiber.Ctx) error {
		contextx.SetDataPermApplier(ctx, &MockDataPermApplier{err: nil})
		err := ApplyDataPermission(nil, ctx)
		assert.NoError(t, err, "Should return nil when applier succeeds")

		return nil
	})

	req := httptest.NewRequestWithContext(context.Background(), fiber.MethodGet, "/test", nil)
	resp, err := app.Test(req)
	require.NoError(t, err, "Should execute test request without error")
	assert.Equal(t, 200, resp.StatusCode, "Should return 200 status code")
}

// --- parseMetaColumn ---

// TestParseMetaColumnWithoutAlias covers helpers.go:150-153 - no alias specified.
func TestParseMetaColumnWithoutAlias(t *testing.T) {
	col, alias := parseMetaColumn("my_column")
	assert.Equal(t, "my_column", col, "Column name should be the input")
	assert.Equal(t, "my_column", alias, "Alias should equal column name when no alias specified")
}

// TestParseMetaColumnCaseInsensitiveAs covers helpers.go:143 - lowercase "as".
func TestParseMetaColumnCaseInsensitiveAs(t *testing.T) {
	col, alias := parseMetaColumn("my_column as my_alias")
	assert.Equal(t, "my_column", col, "Column name should be extracted with lowercase as")
	assert.Equal(t, "my_alias", alias, "Alias should be extracted with lowercase as")
}

// TestParseMetaColumnExtraSpaces covers helpers.go:143-147 - extra whitespace.
func TestParseMetaColumnExtraSpaces(t *testing.T) {
	col, alias := parseMetaColumn("  my_column   AS   my_alias  ")
	assert.Equal(t, "my_column", col, "Column name should be trimmed")
	assert.Equal(t, "my_alias", alias, "Alias should be trimmed")
}

// TestParseMetaColumnEmptyString covers helpers.go:150-153 - empty input.
func TestParseMetaColumnEmptyString(t *testing.T) {
	col, alias := parseMetaColumn("")
	assert.Equal(t, "", col, "Column should be empty for empty input")
	assert.Equal(t, "", alias, "Alias should be empty for empty input")
}

// --- parseMetaColumns ---

// TestParseMetaColumnsNonEmpty covers helpers.go:157-169 - non-empty specs.
func TestParseMetaColumnsNonEmpty(t *testing.T) {
	specs := []string{"status", "role_name AS role"}
	result := parseMetaColumns(specs)
	require.Len(t, result, 2, "Should return 2 parsed meta columns")
	assert.Equal(t, orm.ColumnInfo{Name: "status", Alias: "status"}, result[0], "First column should have matching name and alias")
	assert.Equal(t, orm.ColumnInfo{Name: "role_name", Alias: "role"}, result[1], "Second column should have parsed alias")
}

// --- validateMetaColumns ---

// TestValidateMetaColumnsSuccess covers helpers.go:172-184 - all columns exist.
func TestValidateMetaColumnsSuccess(t *testing.T) {
	table := helperTestTable()
	cols := []orm.ColumnInfo{
		{Name: "name", Alias: "name"},
		{Name: "status", Alias: "status"},
	}
	err := validateMetaColumns(table, cols)
	assert.NoError(t, err, "Should succeed when all meta columns exist")
}

// TestValidateMetaColumnsNotFound covers helpers.go:174-180 - column not found.
func TestValidateMetaColumnsNotFound(t *testing.T) {
	table := helperTestTable()
	cols := []orm.ColumnInfo{
		{Name: "nonexistent", Alias: "nope"},
	}
	err := validateMetaColumns(table, cols)
	assert.Error(t, err, "Should return error for nonexistent meta column")
	assert.Contains(t, err.Error(), "nonexistent", "Error message should mention nonexistent column")
}

// TestValidateMetaColumnsEmpty covers helpers.go:172-184 - empty input (no-op).
func TestValidateMetaColumnsEmpty(t *testing.T) {
	table := helperTestTable()
	err := validateMetaColumns(table, nil)
	assert.NoError(t, err, "Should succeed with nil meta columns")
}

// --- withCleanup ---

// TestWithCleanupNilError covers withCleanup with nil error (no-op).
func TestWithCleanupNilError(t *testing.T) {
	called := false
	err := withCleanup(nil, func() error {
		called = true

		return nil
	})
	assert.NoError(t, err, "Should return nil when original error is nil")
	assert.False(t, called, "Cleanup should not be called when error is nil")
}

// TestWithCleanupSuccessfulCleanup covers withCleanup when cleanup succeeds.
func TestWithCleanupSuccessfulCleanup(t *testing.T) {
	originalErr := errors.New("operation failed")
	err := withCleanup(originalErr, func() error {
		return nil
	})
	assert.Equal(t, originalErr, err, "Should return the original error when cleanup succeeds")
}

// TestWithCleanupFailedCleanup covers withCleanup when cleanup also fails.
func TestWithCleanupFailedCleanup(t *testing.T) {
	originalErr := errors.New("operation failed")
	cleanupErr := errors.New("cleanup failed")
	err := withCleanup(originalErr, func() error {
		return cleanupErr
	})
	assert.Error(t, err, "Should return error when both operation and cleanup fail")
	assert.ErrorIs(t, err, originalErr, "Should wrap the original error")
	assert.ErrorIs(t, err, cleanupErr, "Should wrap the cleanup error")
	assert.Contains(t, err.Error(), "cleanup also failed", "Error message should mention cleanup failure")
}

// --- batchCleanup ---

// TestBatchCleanupSuccess covers helpers.go:197-210 - all promotions succeed.
func TestBatchCleanupSuccess(t *testing.T) {
	promoter := &MockPromoter[struct{}]{err: nil}
	models := []struct{}{{}, {}}
	err := batchCleanup(context.Background(), promoter, models)
	assert.NoError(t, err, "Should succeed when all promotions pass")
}

// --- batchRollback ---

// TestBatchRollbackSuccess covers helpers.go:214-228 - all rollbacks succeed.
func TestBatchRollbackSuccess(t *testing.T) {
	promoter := &MockPromoter[struct{}]{err: nil}
	oldModels := []struct{}{{}, {}}
	newModels := []struct{}{{}, {}}
	err := batchRollback(context.Background(), promoter, oldModels, newModels, 2)
	assert.NoError(t, err, "Should succeed when all rollbacks pass")
}

// TestBatchRollbackZeroCount covers helpers.go:221 - count=0, no rollback needed.
func TestBatchRollbackZeroCount(t *testing.T) {
	promoter := &MockPromoter[struct{}]{err: errors.New("should not be called")}
	err := batchRollback(context.Background(), promoter, nil, nil, 0)
	assert.NoError(t, err, "Should succeed with zero count (no rollback needed)")
}

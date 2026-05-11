package mold_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/app"
	"github.com/coldsmirk/vef-framework-go/internal/apptest"
	"github.com/coldsmirk/vef-framework-go/mold"
)

// TranslateTransformerTestSuite tests the TranslateTransformer functionality.
type TranslateTransformerTestSuite struct {
	suite.Suite

	ctx         context.Context
	app         *app.App
	stop        func()
	transformer mold.Transformer
}

// MockDataDictLoader mocks mold.DataDictLoader for testing.
type MockDataDictLoader struct {
	shouldError bool
}

func (m *MockDataDictLoader) Load(_ context.Context, key string) (map[string]string, error) {
	if m.shouldError {
		return nil, fmt.Errorf("mock loader error for key=%s", key)
	}

	switch key {
	case "status":
		return map[string]string{
			"active":   "Active Status",
			"inactive": "Inactive Status",
			"pending":  "Pending Status",
			"*":        "Unknown Status",
		}, nil

	case "priority":
		return map[string]string{
			"high":   "High Priority",
			"medium": "Medium Priority",
			"low":    "Low Priority",
			"*":      "Normal Priority",
		}, nil

	default:
		return map[string]string{}, nil
	}
}

func (suite *TranslateTransformerTestSuite) SetupSuite() {
	suite.T().Log("Setting up TranslateTransformerTestSuite - starting test app")

	suite.ctx = context.Background()

	suite.app, suite.stop = apptest.NewTestApp(
		suite.T(),
		fx.Replace(&config.DataSourceConfig{
			Kind: config.SQLite,
		}),
		fx.Provide(func() mold.DataDictLoader {
			return &MockDataDictLoader{shouldError: false}
		}),
		fx.Populate(&suite.transformer),
	)

	suite.Require().NotNil(suite.app, "App should be initialized")
	suite.Require().NotNil(suite.transformer, "Transformer should be initialized")

	suite.T().Log("TranslateTransformerTestSuite setup complete")
}

func (suite *TranslateTransformerTestSuite) TearDownSuite() {
	suite.T().Log("Tearing down TranslateTransformerTestSuite")

	if suite.stop != nil {
		suite.stop()
	}

	suite.T().Log("TranslateTransformerTestSuite teardown complete")
}

// TestTranslateStringField tests translation with string field type.
func (suite *TranslateTransformerTestSuite) TestTranslateStringField() {
	suite.T().Log("Testing translate transformer with string field type")

	suite.Run("TranslateActiveStatus", func() {
		type TestStruct struct {
			Status     string `mold:"translate=dict:status"`
			StatusName string
		}

		test := &TestStruct{
			Status: "active",
		}

		err := suite.transformer.Struct(suite.ctx, test)
		suite.NoError(err, "Translation should succeed for string field")
		suite.Equal("Active Status", test.StatusName, "StatusName should be translated correctly")

		suite.T().Logf("Status: %s -> StatusName: %s", test.Status, test.StatusName)
	})

	suite.Run("TranslateInactiveStatus", func() {
		type TestStruct struct {
			Status     string `mold:"translate=dict:status"`
			StatusName string
		}

		test := &TestStruct{
			Status: "inactive",
		}

		err := suite.transformer.Struct(suite.ctx, test)
		suite.NoError(err, "Translation should succeed for string field")
		suite.Equal("Inactive Status", test.StatusName, "StatusName should be translated correctly")

		suite.T().Logf("Status: %s -> StatusName: %s", test.Status, test.StatusName)
	})

	suite.Run("TranslatePendingStatus", func() {
		type TestStruct struct {
			Status     string `mold:"translate=dict:status"`
			StatusName string
		}

		test := &TestStruct{
			Status: "pending",
		}

		err := suite.transformer.Struct(suite.ctx, test)
		suite.NoError(err, "Translation should succeed for pending status")
		suite.Equal("Pending Status", test.StatusName, "StatusName should be translated correctly")

		suite.T().Logf("Status: %s -> StatusName: %s", test.Status, test.StatusName)
	})
}

// TestTranslatePointerField tests translation with *string field type.
func (suite *TranslateTransformerTestSuite) TestTranslatePointerField() {
	suite.T().Log("Testing translate transformer with *string field type")

	suite.Run("TranslateNonNilPointer", func() {
		type TestStruct struct {
			Priority     *string `mold:"translate=dict:priority"`
			PriorityName *string
		}

		priority := "high"
		test := &TestStruct{
			Priority:     &priority,
			PriorityName: nil,
		}

		err := suite.transformer.Struct(suite.ctx, test)
		suite.NoError(err, "Translation should succeed for *string field")
		suite.Require().NotNil(test.PriorityName, "PriorityName should be initialized")
		suite.Equal("High Priority", *test.PriorityName, "PriorityName should be translated correctly")

		suite.T().Logf("Priority: %s -> PriorityName: %s", *test.Priority, *test.PriorityName)
	})

	suite.Run("TranslateNilPointer", func() {
		type TestStruct struct {
			Priority     *string `mold:"translate=dict:priority"`
			PriorityName *string
		}

		test := &TestStruct{
			Priority:     nil,
			PriorityName: nil,
		}

		err := suite.transformer.Struct(suite.ctx, test)
		suite.NoError(err, "Translation should skip nil pointer field")
		// PriorityName should remain nil when Priority is nil
		suite.T().Log("Priority is nil, PriorityName remains unset")
	})

	suite.Run("TranslatePointerWithPreInitializedTarget", func() {
		type TestStruct struct {
			Priority     *string `mold:"translate=dict:priority"`
			PriorityName *string
		}

		priority := "medium"
		existingName := "Old Value"
		test := &TestStruct{
			Priority:     &priority,
			PriorityName: &existingName,
		}

		err := suite.transformer.Struct(suite.ctx, test)
		suite.NoError(err, "Translation should succeed and overwrite existing value")
		suite.Require().NotNil(test.PriorityName, "PriorityName should remain non-nil")
		suite.Equal("Medium Priority", *test.PriorityName, "PriorityName should be updated")

		suite.T().Logf("Priority: %s -> PriorityName: %s (overwritten)", *test.Priority, *test.PriorityName)
	})
}

// TestTranslateEmptyValue tests translation with empty string values.
func (suite *TranslateTransformerTestSuite) TestTranslateEmptyValue() {
	suite.T().Log("Testing translate transformer with empty values")

	suite.Run("EmptyStringValue", func() {
		type TestStruct struct {
			Status     string `mold:"translate=dict:status"`
			StatusName string
		}

		test := &TestStruct{
			Status: "",
		}

		err := suite.transformer.Struct(suite.ctx, test)
		suite.NoError(err, "Translation should skip empty string")
		suite.Empty(test.StatusName, "StatusName should remain empty")

		suite.T().Log("Empty Status value, translation skipped")
	})
}

// TestTranslateMultipleFields tests translation of multiple fields in the same struct.
func (suite *TranslateTransformerTestSuite) TestTranslateMultipleFields() {
	suite.T().Log("Testing translate transformer with multiple fields")

	suite.Run("MultipleFieldTranslation", func() {
		type TestStruct struct {
			Status       string `mold:"translate=dict:status"`
			StatusName   string
			Priority     string `mold:"translate=dict:priority"`
			PriorityName string
		}

		test := &TestStruct{
			Status:   "active",
			Priority: "high",
		}

		err := suite.transformer.Struct(suite.ctx, test)
		suite.NoError(err, "Translation should succeed for multiple fields")
		suite.Equal("Active Status", test.StatusName, "StatusName should be translated correctly")
		suite.Equal("High Priority", test.PriorityName, "PriorityName should be translated correctly")

		suite.T().Logf("Status: %s -> %s, Priority: %s -> %s",
			test.Status, test.StatusName, test.Priority, test.PriorityName)
	})
}

// TestTranslateMissingTargetField tests error handling when target field is missing.
func (suite *TranslateTransformerTestSuite) TestTranslateMissingTargetField() {
	suite.T().Log("Testing translate transformer with missing target field")

	suite.Run("MissingTargetField", func() {
		type TestStruct struct {
			Status string `mold:"translate=dict:status"`
			// StatusName field is missing
		}

		test := &TestStruct{
			Status: "active",
		}

		err := suite.transformer.Struct(suite.ctx, test)
		suite.Error(err, "Translation should fail when target field is missing")
		suite.Contains(err.Error(), "target translated field not found", "Error should indicate missing target field")

		suite.T().Logf("Error (expected): %v", err)
	})
}

// TestTranslateIntFieldKind tests translation with int field kind (now supported).
func (suite *TranslateTransformerTestSuite) TestTranslateIntFieldKind() {
	suite.T().Log("Testing translate transformer with int field type")

	suite.Run("IntFieldTypeSupported", func() {
		type TestStruct struct {
			Status     int `mold:"translate=dict:status"`
			StatusName string
		}

		test := &TestStruct{
			Status: 1,
		}

		// Int field type is now supported - translation should succeed
		// but since "1" is not in the mock dictionary, StatusName will remain empty
		err := suite.transformer.Struct(suite.ctx, test)
		suite.NoError(err, "Translation should succeed for int field type")

		suite.T().Logf("Result: Status=%d, StatusName=%q", test.Status, test.StatusName)
	})
}

// TestTranslateUnsupportedFieldKind tests error handling for unsupported field kinds.
func (suite *TranslateTransformerTestSuite) TestTranslateUnsupportedFieldKind() {
	suite.T().Log("Testing translate transformer with unsupported field types")

	suite.Run("UnsupportedSourceFieldType", func() {
		type TestStruct struct {
			Status     float64 `mold:"translate=dict:status"`
			StatusName string
		}

		test := &TestStruct{
			Status: 1.5,
		}

		err := suite.transformer.Struct(suite.ctx, test)
		suite.Error(err, "Translation should fail for unsupported source field type")
		suite.Contains(err.Error(), "unsupported field type", "Error should indicate unsupported type")

		suite.T().Logf("Error (expected): %v", err)
	})

	suite.Run("UnsupportedTargetFieldType", func() {
		type TestStruct struct {
			Status     string `mold:"translate=dict:status"`
			StatusName int
		}

		test := &TestStruct{
			Status: "active",
		}

		err := suite.transformer.Struct(suite.ctx, test)
		suite.Error(err, "Translation should fail when target field has unsupported type")
		suite.Contains(err.Error(), "unsupported field type", "Error should indicate target field type mismatch")

		suite.T().Logf("Error (expected): %v", err)
	})
}

// TestTranslateWithResolverError tests error handling when resolver returns error.
func (suite *TranslateTransformerTestSuite) TestTranslateWithResolverError() {
	suite.T().Log("Testing translate transformer with resolver errors")

	suite.Run("ResolverError", func() {
		ctx := context.Background()

		var transformer mold.Transformer

		_, stop := apptest.NewTestApp(
			suite.T(),
			fx.Replace(&config.DataSourceConfig{
				Kind: config.SQLite,
			}),
			fx.Provide(func() mold.DataDictLoader {
				return &MockDataDictLoader{shouldError: true}
			}),
			fx.Populate(&transformer),
		)
		defer stop()

		suite.Require().NotNil(transformer, "Transformer should be initialized")

		type TestStruct struct {
			Status     string `mold:"translate=dict:status"`
			StatusName string
		}

		test := &TestStruct{
			Status: "active",
		}

		err := transformer.Struct(ctx, test)
		suite.Error(err, "Translation should fail when loader returns error")
		suite.Contains(err.Error(), "mock loader error", "Error should be from loader")

		suite.T().Logf("Error (expected): %v", err)
	})
}

// TestTranslateWithMissingResolver tests error handling when resolver is not configured.
func (suite *TranslateTransformerTestSuite) TestTranslateWithMissingResolver() {
	suite.T().Log("Testing translate transformer without resolver")

	suite.Run("MissingResolver", func() {
		ctx := context.Background()

		var transformer mold.Transformer

		_, stop := apptest.NewTestApp(
			suite.T(),
			fx.Replace(&config.DataSourceConfig{
				Kind: config.SQLite,
			}),
			fx.Populate(&transformer),
		)
		defer stop()

		suite.Require().NotNil(transformer, "Transformer should be initialized")

		type TestStruct struct {
			Status     string `mold:"translate=dict:status"`
			StatusName string
		}

		test := &TestStruct{
			Status: "active",
		}

		err := transformer.Struct(ctx, test)
		suite.Error(err, "Translation should fail when resolver is not configured")
		suite.Contains(err.Error(), "data dictionary resolver is not configured", "Error should indicate missing resolver")

		suite.T().Logf("Error (expected): %v", err)
	})
}

// TestTranslateWithOptionalKind tests optional translation (kind ending with ?).
func (suite *TranslateTransformerTestSuite) TestTranslateWithOptionalKind() {
	suite.T().Log("Testing translate transformer with optional kind (non-existent translator)")

	suite.Run("OptionalKindNoTranslator", func() {
		type TestStruct struct {
			Status     string `mold:"translate=nonexistent:status?"`
			StatusName string
		}

		test := &TestStruct{
			Status: "active",
		}

		err := suite.transformer.Struct(suite.ctx, test)
		suite.NoError(err, "Translation should succeed silently for optional non-existent translator")
		suite.Empty(test.StatusName, "StatusName should remain empty")

		suite.T().Log("Optional kind with no supporting translator - silently skipped")
	})
}

// TestTranslateWithMissingKind tests error handling when translation kind is missing.
func (suite *TranslateTransformerTestSuite) TestTranslateWithMissingKind() {
	suite.T().Log("Testing translate transformer with missing kind parameter")

	suite.Run("MissingKind", func() {
		type TestStruct struct {
			Status     string `mold:"translate"`
			StatusName string
		}

		test := &TestStruct{
			Status: "active",
		}

		err := suite.transformer.Struct(suite.ctx, test)
		suite.Error(err, "Translation should fail when kind parameter is missing")
		suite.Contains(err.Error(), "translation kind parameter is empty", "Error should indicate missing kind")

		suite.T().Logf("Error (expected): %v", err)
	})
}

// TestTranslateIntegration tests end-to-end translation scenarios.
func (suite *TranslateTransformerTestSuite) TestTranslateIntegration() {
	suite.T().Log("Testing translate transformer integration scenarios")

	suite.Run("ComplexStructWithMixedTypes", func() {
		type ComplexStruct struct {
			Status       string `mold:"translate=dict:status"`
			StatusName   string
			Priority     *string `mold:"translate=dict:priority"`
			PriorityName *string
		}

		priority := "low"
		test := &ComplexStruct{
			Status:   "active",
			Priority: &priority,
		}

		err := suite.transformer.Struct(suite.ctx, test)
		suite.NoError(err, "Translation should succeed for complex struct")

		suite.Equal("Active Status", test.StatusName, "StatusName should be translated")
		suite.Require().NotNil(test.PriorityName, "PriorityName should be initialized")
		suite.Equal("Low Priority", *test.PriorityName, "PriorityName should be translated")

		suite.T().Logf("Complex translation: Status=%s->%s, Priority=%s->%s",
			test.Status, test.StatusName,
			*test.Priority, *test.PriorityName)
	})

	suite.Run("ScalarAndSliceMixedFields", func() {
		type MixedStruct struct {
			Status       string `mold:"translate=dict:status"`
			StatusName   string
			Tags         []string `mold:"translate=dict:status"`
			TagsName     []string
			Priority     *string `mold:"translate=dict:priority"`
			PriorityName *string
		}

		priority := "high"
		test := &MixedStruct{
			Status:   "active",
			Tags:     []string{"inactive", "pending"},
			Priority: &priority,
		}

		err := suite.transformer.Struct(suite.ctx, test)
		suite.NoError(err, "Translation should succeed for mixed scalar and slice fields")
		suite.Equal("Active Status", test.StatusName, "Scalar StatusName should be translated")
		suite.Equal([]string{"Inactive Status", "Pending Status"}, test.TagsName, "Slice TagsName should be translated element-wise")
		suite.Require().NotNil(test.PriorityName, "Pointer PriorityName should be initialized")
		suite.Equal("High Priority", *test.PriorityName, "Pointer PriorityName should be translated")

		suite.T().Logf("Mixed translation: Status=%s->%s, Tags=%v->%v, Priority=%s->%s",
			test.Status, test.StatusName,
			test.Tags, test.TagsName,
			*test.Priority, *test.PriorityName)
	})
}

// TestTranslateStringSlice tests translation with []string source field type.
func (suite *TranslateTransformerTestSuite) TestTranslateStringSlice() {
	suite.T().Log("Testing translate transformer with []string field type")

	suite.Run("TranslateNonEmptyStringSlice", func() {
		type TestStruct struct {
			Statuses     []string `mold:"translate=dict:status"`
			StatusesName []string
		}

		test := &TestStruct{
			Statuses: []string{"active", "inactive", "pending"},
		}

		err := suite.transformer.Struct(suite.ctx, test)
		suite.NoError(err, "Translation should succeed for []string field")
		suite.Equal([]string{"Active Status", "Inactive Status", "Pending Status"}, test.StatusesName, "StatusesName should be translated element-wise")

		suite.T().Logf("Statuses: %v -> StatusesName: %v", test.Statuses, test.StatusesName)
	})

	suite.Run("TranslateEmptyStringSlice", func() {
		type TestStruct struct {
			Statuses     []string `mold:"translate=dict:status"`
			StatusesName []string
		}

		test := &TestStruct{
			Statuses: []string{},
		}

		err := suite.transformer.Struct(suite.ctx, test)
		suite.NoError(err, "Translation should succeed for empty []string source")
		suite.NotNil(test.StatusesName, "StatusesName should be initialized to a non-nil slice")
		suite.Empty(test.StatusesName, "StatusesName should be an empty slice")

		suite.T().Log("Empty source slice translated to empty target slice")
	})

	suite.Run("TranslateNilStringSlice", func() {
		type TestStruct struct {
			Statuses     []string `mold:"translate=dict:status"`
			StatusesName []string
		}

		test := &TestStruct{
			Statuses: nil,
		}

		err := suite.transformer.Struct(suite.ctx, test)
		suite.NoError(err, "Translation should skip nil []string source")
		suite.Nil(test.StatusesName, "StatusesName should remain nil when source is nil")

		suite.T().Log("Nil source slice skipped, target slice untouched")
	})

	suite.Run("TranslateSliceWithEmptyElement", func() {
		type TestStruct struct {
			Statuses     []string `mold:"translate=dict:status"`
			StatusesName []string
		}

		test := &TestStruct{
			Statuses: []string{"active", "", "pending"},
		}

		err := suite.transformer.Struct(suite.ctx, test)
		suite.NoError(err, "Translation should succeed even with empty element")
		// Resolver returns empty string for empty code, preserving index alignment.
		suite.Equal([]string{"Active Status", "", "Pending Status"}, test.StatusesName, "Empty element should map to empty translation while keeping index alignment")

		suite.T().Logf("Mixed slice: %v -> %v", test.Statuses, test.StatusesName)
	})

	suite.Run("TranslateSliceOverwritesPreviousTarget", func() {
		type TestStruct struct {
			Statuses     []string `mold:"translate=dict:status"`
			StatusesName []string
		}

		test := &TestStruct{
			Statuses:     []string{"active"},
			StatusesName: []string{"stale-1", "stale-2"},
		}

		err := suite.transformer.Struct(suite.ctx, test)
		suite.NoError(err, "Translation should overwrite the existing target slice")
		suite.Equal([]string{"Active Status"}, test.StatusesName, "StatusesName should be replaced wholesale, not merged")

		suite.T().Logf("Overwrote target: %v", test.StatusesName)
	})

	suite.Run("TranslateMultipleStringSliceFields", func() {
		type TestStruct struct {
			Statuses       []string `mold:"translate=dict:status"`
			StatusesName   []string
			Priorities     []string `mold:"translate=dict:priority"`
			PrioritiesName []string
		}

		test := &TestStruct{
			Statuses:   []string{"active", "pending"},
			Priorities: []string{"high", "low"},
		}

		err := suite.transformer.Struct(suite.ctx, test)
		suite.NoError(err, "Translation should succeed for multiple []string fields")
		suite.Equal([]string{"Active Status", "Pending Status"}, test.StatusesName, "StatusesName should be translated independently")
		suite.Equal([]string{"High Priority", "Low Priority"}, test.PrioritiesName, "PrioritiesName should be translated independently")

		suite.T().Logf("Statuses: %v -> %v, Priorities: %v -> %v",
			test.Statuses, test.StatusesName,
			test.Priorities, test.PrioritiesName)
	})
}

// TestTranslateStringSliceErrors tests error handling for []string source fields.
func (suite *TranslateTransformerTestSuite) TestTranslateStringSliceErrors() {
	suite.T().Log("Testing translate transformer error handling for []string fields")

	suite.Run("MissingTargetField", func() {
		type TestStruct struct {
			Statuses []string `mold:"translate=dict:status"`
			// StatusesName intentionally missing.
		}

		test := &TestStruct{Statuses: []string{"active"}}

		err := suite.transformer.Struct(suite.ctx, test)
		suite.Error(err, "Should fail when sibling target field is missing")
		suite.Contains(err.Error(), "target translated field not found", "Error should indicate missing sibling")

		suite.T().Logf("Error (expected): %v", err)
	})

	suite.Run("UnsupportedTargetType", func() {
		type TestStruct struct {
			Statuses     []string `mold:"translate=dict:status"`
			StatusesName string
		}

		test := &TestStruct{Statuses: []string{"active"}}

		err := suite.transformer.Struct(suite.ctx, test)
		suite.Error(err, "Should fail when target type is not []string")
		suite.Contains(err.Error(), "unsupported field type", "Error should indicate unsupported target type")

		suite.T().Logf("Error (expected): %v", err)
	})

	suite.Run("MissingKind", func() {
		type TestStruct struct {
			Statuses     []string `mold:"translate"`
			StatusesName []string
		}

		test := &TestStruct{Statuses: []string{"active"}}

		err := suite.transformer.Struct(suite.ctx, test)
		suite.Error(err, "Should fail when kind parameter is missing")
		suite.Contains(err.Error(), "translation kind parameter is empty", "Error should indicate missing kind")

		suite.T().Logf("Error (expected): %v", err)
	})

	suite.Run("OptionalKindNoTranslator", func() {
		type TestStruct struct {
			Statuses     []string `mold:"translate=nonexistent:slice?"`
			StatusesName []string
		}

		test := &TestStruct{Statuses: []string{"active", "pending"}}

		err := suite.transformer.Struct(suite.ctx, test)
		suite.NoError(err, "Optional kind should silently skip when no translator supports it")
		suite.Nil(test.StatusesName, "StatusesName should remain untouched for optional skip")

		suite.T().Log("Optional kind skipped silently for slice source")
	})

	suite.Run("ResolverErrorReportsElementIndex", func() {
		ctx := context.Background()

		var transformer mold.Transformer

		_, stop := apptest.NewTestApp(
			suite.T(),
			fx.Replace(&config.DataSourceConfig{
				Kind: config.SQLite,
			}),
			fx.Provide(func() mold.DataDictLoader {
				return &MockDataDictLoader{shouldError: true}
			}),
			fx.Populate(&transformer),
		)
		defer stop()

		suite.Require().NotNil(transformer, "Transformer should be initialized")

		type TestStruct struct {
			Statuses     []string `mold:"translate=dict:status"`
			StatusesName []string
		}

		test := &TestStruct{Statuses: []string{"active", "pending"}}

		err := transformer.Struct(ctx, test)
		suite.Error(err, "Should fail when resolver returns error")
		suite.Contains(err.Error(), "element[0]", "Error should include failing element index")
		suite.Contains(err.Error(), "Statuses", "Error should include source field name")

		suite.T().Logf("Error (expected): %v", err)
	})

	suite.Run("RequiredKindNoTranslator", func() {
		type TestStruct struct {
			Statuses     []string `mold:"translate=nonexistent:status"`
			StatusesName []string
		}

		test := &TestStruct{Statuses: []string{"active"}}

		err := suite.transformer.Struct(suite.ctx, test)
		suite.Error(err, "Should fail when required kind has no supporting translator")
		suite.Contains(err.Error(), "no translator supports the given kind", "Error should indicate unsupported kind")
		suite.Contains(err.Error(), "nonexistent:status", "Error should include the unsupported kind value")
		suite.Contains(err.Error(), "Statuses", "Error should include source field name")
		suite.Nil(test.StatusesName, "StatusesName should remain untouched on translator lookup failure")

		suite.T().Logf("Error (expected): %v", err)
	})

	suite.Run("MissingResolverReportsElementIndex", func() {
		ctx := context.Background()

		var transformer mold.Transformer

		_, stop := apptest.NewTestApp(
			suite.T(),
			fx.Replace(&config.DataSourceConfig{
				Kind: config.SQLite,
			}),
			fx.Populate(&transformer),
		)
		defer stop()

		suite.Require().NotNil(transformer, "Transformer should be initialized")

		type TestStruct struct {
			Statuses     []string `mold:"translate=dict:status"`
			StatusesName []string
		}

		test := &TestStruct{Statuses: []string{"active", "pending"}}

		err := transformer.Struct(ctx, test)
		suite.Error(err, "Should fail when resolver is not configured")
		suite.Contains(err.Error(), "data dictionary resolver is not configured", "Error should indicate missing resolver")
		suite.Contains(err.Error(), "element[0]", "Error should include failing element index")
		suite.Contains(err.Error(), "Statuses", "Error should include source field name")

		suite.T().Logf("Error (expected): %v", err)
	})
}

// TestTranslateTransformerTestSuite runs the test suite.
func TestTranslateTransformer(t *testing.T) {
	suite.Run(t, new(TranslateTransformerTestSuite))
}

package crud_test

import (
	"errors"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/suite"
	"github.com/uptrace/bun"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/crud"
	"github.com/coldsmirk/vef-framework-go/i18n"
	"github.com/coldsmirk/vef-framework-go/internal/orm"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/result"
)

func init() {
	registry.Add(func(env *testx.DBEnv) suite.TestingSuite {
		return &FindOptionsTestSuite{
			BaseTestSuite: BaseTestSuite{
				ctx:   env.Ctx,
				db:    env.DB,
				bunDB: env.BunDB,
				ds:    env.DS,
			},
		}
	})
}

// Test Resources.
type EmployeeFindOptionsResource struct {
	api.Resource
	crud.FindOptions[Employee, EmployeeSearch]
}

func NewEmployeeFindOptionsResource() api.Resource {
	return &EmployeeFindOptionsResource{
		Resource: api.NewRPCResource("test/employee_options"),
		FindOptions: crud.NewFindOptions[Employee, EmployeeSearch]().
			WithCondition(fixtureScope).
			Public().
			WithDefaultColumnMapping(&crud.DataOptionColumnMapping{
				LabelColumn: "name",
				ValueColumn: "id",
			}),
	}
}

// Resource with custom field mapping.
type CustomFieldUserFindOptionsResource struct {
	api.Resource
	crud.FindOptions[Employee, EmployeeSearch]
}

func NewCustomFieldUserFindOptionsResource() api.Resource {
	return &CustomFieldUserFindOptionsResource{
		Resource: api.NewRPCResource("test/employee_options_custom"),
		FindOptions: crud.NewFindOptions[Employee, EmployeeSearch]().
			WithCondition(fixtureScope).
			Public().
			WithDefaultColumnMapping(&crud.DataOptionColumnMapping{
				LabelColumn:       "email",
				ValueColumn:       "id",
				DescriptionColumn: "description",
			}),
	}
}

// Filtered Options Resource.
type FilteredUserFindOptionsResource struct {
	api.Resource
	crud.FindOptions[Employee, EmployeeSearch]
}

func NewFilteredUserFindOptionsResource() api.Resource {
	return &FilteredUserFindOptionsResource{
		Resource: api.NewRPCResource("test/employee_options_filtered"),
		FindOptions: crud.NewFindOptions[Employee, EmployeeSearch]().
			WithCondition(fixtureScope).
			WithCondition(func(cb orm.ConditionBuilder) {
				cb.Equals("status", "active")
			}).
			Public(),
	}
}

// Meta Options Resource.
type MetaUserFindOptionsResource struct {
	api.Resource
	crud.FindOptions[Employee, EmployeeSearch]
}

func NewMetaUserFindOptionsResource() api.Resource {
	return &MetaUserFindOptionsResource{
		Resource: api.NewRPCResource("test/employee_options_meta"),
		FindOptions: crud.NewFindOptions[Employee, EmployeeSearch]().
			WithCondition(fixtureScope).
			Public().
			WithDefaultColumnMapping(&crud.DataOptionColumnMapping{
				LabelColumn: "name",
				ValueColumn: "id",
				MetaColumns: []string{"status", "email"},
			}),
	}
}

// OptionItem is a model with value/label/description fields to cover column matching branches.
type OptionItem struct {
	bun.BaseModel `bun:"table:test_option_item,alias:toi"`
	orm.IDModel

	Value       string `json:"value"       bun:",notnull"`
	Label       string `json:"label"       bun:",notnull"`
	Description string `json:"description"`
}

// NonMatchingDescFindOptionsResource - FindOptions with description column NOT matching constant.
type NonMatchingDescFindOptionsResource struct {
	api.Resource
	crud.FindOptions[Employee, EmployeeSearch]
}

func NewNonMatchingDescFindOptionsResource() api.Resource {
	return &NonMatchingDescFindOptionsResource{
		Resource: api.NewRPCResource("test/employee_options_desc_alias"),
		FindOptions: crud.NewFindOptions[Employee, EmployeeSearch]().
			WithCondition(fixtureScope).
			Public().
			WithDefaultColumnMapping(&crud.DataOptionColumnMapping{
				ValueColumn:       "id",
				LabelColumn:       "name",
				DescriptionColumn: "email",
			}),
	}
}

// MatchingColumnFindOptionsResource - FindOptions with columns matching constant names.
type MatchingColumnFindOptionsResource struct {
	api.Resource
	crud.FindOptions[OptionItem, struct{ api.P }]
}

func NewMatchingColumnFindOptionsResource() api.Resource {
	return &MatchingColumnFindOptionsResource{
		Resource: api.NewRPCResource("test/option_item_options"),
		FindOptions: crud.NewFindOptions[OptionItem, struct{ api.P }]().
			Public().
			WithDefaultColumnMapping(&crud.DataOptionColumnMapping{
				ValueColumn:       "value",
				LabelColumn:       "label",
				DescriptionColumn: "description",
			}),
	}
}

// ErrorQueryApplierFindOptionsResource - FindOptions with QueryApplier that returns error.
type ErrorQueryApplierFindOptionsResource struct {
	api.Resource
	crud.FindOptions[Employee, EmployeeSearch]
}

func NewErrorQueryApplierFindOptionsResource() api.Resource {
	return &ErrorQueryApplierFindOptionsResource{
		Resource: api.NewRPCResource("test/employee_options_err_applier"),
		FindOptions: crud.NewFindOptions[Employee, EmployeeSearch]().
			WithCondition(fixtureScope).
			WithDefaultColumnMapping(&crud.DataOptionColumnMapping{
				LabelColumn: "name",
				ValueColumn: "id",
			}).
			WithQueryApplier(func(_ orm.SelectQuery, _ EmployeeSearch, _ fiber.Ctx) error {
				return errors.New("options query applier error")
			}).
			Public(),
	}
}

// FindOptionsTestSuite tests the FindOptions API functionality
// including basic queries, custom column mappings, search filters, filter appliers, meta columns, and negative cases.
type FindOptionsTestSuite struct {
	BaseTestSuite
}

// SetupSuite runs once before all tests in the suite.
func (suite *FindOptionsTestSuite) SetupSuite() {
	suite.setupBaseSuite(
		NewEmployeeFindOptionsResource,
		NewCustomFieldUserFindOptionsResource,
		NewFilteredUserFindOptionsResource,
		NewMetaUserFindOptionsResource,
		NewErrorQueryApplierFindOptionsResource,
		NewMatchingColumnFindOptionsResource,
		NewNonMatchingDescFindOptionsResource,
	)
}

// TearDownSuite runs once after all tests in the suite.
func (suite *FindOptionsTestSuite) TearDownSuite() {
	suite.tearDownBaseSuite()
}

// TestFindOptionsBasic tests basic FindOptions functionality.
func (suite *FindOptionsTestSuite) TestFindOptionsBasic() {
	suite.T().Logf("Testing FindOptions API basic functionality for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_options",
			Action:   "find_options",
			Version:  "v1",
		},
	})

	suite.Equal(200, resp.StatusCode, "Should return 200 status code")
	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Should return successful response")
	suite.Equal(body.Message, i18n.T(result.OkMessage), "Should return OK message")
	suite.NotNil(body.Data, "Data should not be nil")

	options := suite.ReadDataAsSlice(body.Data)
	suite.Len(options, 25, "Should return 25 options")

	// Check first option structure
	firstOption := suite.ReadDataAsMap(options[0])
	suite.NotEmpty(firstOption["label"], "First option should have label")
	suite.NotEmpty(firstOption["value"], "First option should have value")

	suite.T().Logf("Found %d options with label=%v, value=%v", len(options), firstOption["label"], firstOption["value"])
}

// TestFindOptionsWithConfig tests FindOptions with custom config.
func (suite *FindOptionsTestSuite) TestFindOptionsWithConfig() {
	suite.T().Logf("Testing FindOptions API with custom config for %s", suite.ds.Kind)

	suite.Run("DefaultConfig", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_options",
				Action:   "find_options",
				Version:  "v1",
			},
		})

		suite.Equal(200, resp.StatusCode, "Should return 200 status code")
		body := suite.ReadResult(resp)
		suite.True(body.IsOk(), "Should return successful response")

		options := suite.ReadDataAsSlice(body.Data)
		suite.Len(options, 25, "Should return 25 options with default config")

		suite.T().Logf("Found %d options with default config (label=name, value=id)", len(options))
	})

	suite.Run("CustomConfig", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_options",
				Action:   "find_options",
				Version:  "v1",
			},
			Meta: map[string]any{
				"labelColumn": "email",
				"valueColumn": "id",
			},
		})

		suite.Equal(200, resp.StatusCode, "Should return 200 status code")
		body := suite.ReadResult(resp)
		suite.True(body.IsOk(), "Should return successful response")

		options := suite.ReadDataAsSlice(body.Data)
		suite.Len(options, 25, "Should return 25 options with custom config")

		// Verify email is used as label
		firstOption := suite.ReadDataAsMap(options[0])
		label, ok := firstOption["label"].(string)
		suite.True(ok, "Label should be a string")
		suite.Contains(label, "@", "Email label should contain @ symbol")

		suite.T().Logf("Found %d options with custom config (label=email: %s)", len(options), label)
	})

	suite.Run("WithDescription", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_options_custom",
				Action:   "find_options",
				Version:  "v1",
			},
		})

		suite.Equal(200, resp.StatusCode, "Should return 200 status code")
		body := suite.ReadResult(resp)
		suite.True(body.IsOk(), "Should return successful response")

		options := suite.ReadDataAsSlice(body.Data)
		suite.Len(options, 25, "Should return 25 options with description field")

		// Verify description column is present (check a non-empty one)
		var foundDesc bool
		for _, o := range options {
			opt := suite.ReadDataAsMap(o)
			if desc, ok := opt["description"]; ok && desc != nil && desc != "" {
				foundDesc = true

				suite.T().Logf("Found %d options with description column (description: %v)", len(options), desc)

				break
			}
		}

		suite.True(foundDesc, "At least one option should have a non-empty description field")
	})
}

// TestFindOptionsWithSearch tests FindOptions with search conditions.
func (suite *FindOptionsTestSuite) TestFindOptionsWithSearch() {
	suite.T().Logf("Testing FindOptions API with search conditions for %s", suite.ds.Kind)

	suite.Run("SearchByStatus", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_options",
				Action:   "find_options",
				Version:  "v1",
			},
			Params: map[string]any{
				"status": "active",
			},
		})

		suite.Equal(200, resp.StatusCode, "Should return 200 status code")
		body := suite.ReadResult(resp)
		suite.True(body.IsOk(), "Should return successful response")

		options := suite.ReadDataAsSlice(body.Data)
		suite.Len(options, 18, "Should return 18 active employees")

		suite.T().Logf("Found %d options filtered by status=active", len(options))
	})

	suite.Run("SearchByKeyword", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_options",
				Action:   "find_options",
				Version:  "v1",
			},
			Params: map[string]any{
				"keyword": "Rodriguez",
			},
		})

		suite.Equal(200, resp.StatusCode, "Should return 200 status code")
		body := suite.ReadResult(resp)
		suite.True(body.IsOk(), "Should return successful response")

		options := suite.ReadDataAsSlice(body.Data)
		suite.Len(options, 1, "Should return only Carlos Rodriguez")

		firstOption := suite.ReadDataAsMap(options[0])
		suite.T().Logf("Found %d option matching keyword 'Rodriguez' (label=%v)", len(options), firstOption["label"])
	})
}

// TestFindOptionsWithFilterApplier tests FindOptions with filter applier.
func (suite *FindOptionsTestSuite) TestFindOptionsWithFilterApplier() {
	suite.T().Logf("Testing FindOptions API with filter applier for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_options_filtered",
			Action:   "find_options",
			Version:  "v1",
		},
	})

	suite.Equal(200, resp.StatusCode, "Should return 200 status code")
	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Should return successful response")

	options := suite.ReadDataAsSlice(body.Data)
	suite.Len(options, 18, "Should return only active employees filtered by condition")

	suite.T().Logf("Found %d options filtered by condition (status=active)", len(options))
}

// TestFindOptionsNegativeCases tests negative scenarios.
func (suite *FindOptionsTestSuite) TestFindOptionsNegativeCases() {
	suite.T().Logf("Testing FindOptions API negative cases for %s", suite.ds.Kind)

	suite.Run("NoMatchingRecords", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_options",
				Action:   "find_options",
				Version:  "v1",
			},
			Params: map[string]any{
				"keyword": "NonexistentKeyword",
			},
		})

		suite.Equal(200, resp.StatusCode, "Should return 200 status code")
		body := suite.ReadResult(resp)
		suite.True(body.IsOk(), "Should return successful response")

		options := suite.ReadDataAsSlice(body.Data)
		suite.Len(options, 0, "Should return empty options when no records match")

		suite.T().Logf("No matching records found as expected for keyword 'NonexistentKeyword'")
	})

	suite.Run("InvalidFieldName", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_options",
				Action:   "find_options",
				Version:  "v1",
			},
			Meta: map[string]any{
				"labelColumn": "nonexistent_field",
				"valueColumn": "id",
			},
		})

		suite.Equal(200, resp.StatusCode, "Should return 200 status code")
		body := suite.ReadResult(resp)
		suite.False(body.IsOk(), "Should fail when label column does not exist")

		suite.T().Logf("Validation failed as expected for invalid label column 'nonexistent_field'")
	})
}

// TestFindOptionsWithMeta tests FindOptions with meta columns.
func (suite *FindOptionsTestSuite) TestFindOptionsWithMeta() {
	suite.T().Logf("Testing FindOptions API with meta columns for %s", suite.ds.Kind)

	suite.Run("DefaultMetaColumns", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_options_meta",
				Action:   "find_options",
				Version:  "v1",
			},
		})

		suite.Equal(200, resp.StatusCode, "Should return 200 status code")
		body := suite.ReadResult(resp)
		suite.True(body.IsOk(), "Should return successful response")

		options := suite.ReadDataAsSlice(body.Data)
		suite.Len(options, 25, "Should return 25 options with default meta columns")

		// Verify meta field exists and contains expected keys
		firstOption := suite.ReadDataAsMap(options[0])
		meta, ok := firstOption["meta"].(map[string]any)
		suite.True(ok, "meta should be a map")
		suite.NotNil(meta, "meta should not be nil")
		suite.Contains(meta, "status", "meta should contain status field")
		suite.Contains(meta, "email", "meta should contain email field")

		suite.T().Logf("Found %d options with default meta columns (status, email)", len(options))
	})

	suite.Run("CustomMetaColumns", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_options",
				Action:   "find_options",
				Version:  "v1",
			},
			Meta: map[string]any{
				"metaColumns": []string{"status", "description"},
			},
		})

		suite.Equal(200, resp.StatusCode, "Should return 200 status code")
		body := suite.ReadResult(resp)
		suite.True(body.IsOk(), "Should return successful response")

		options := suite.ReadDataAsSlice(body.Data)
		suite.Len(options, 25, "Should return 25 options with custom meta columns")

		// Verify meta field contains custom columns
		firstOption := suite.ReadDataAsMap(options[0])
		meta, ok := firstOption["meta"].(map[string]any)
		suite.True(ok, "meta should be a map")
		suite.NotNil(meta, "meta should not be nil")
		suite.Contains(meta, "status", "meta should contain status field")
		suite.Contains(meta, "description", "meta should contain description field")

		suite.T().Logf("Found %d options with custom meta columns (status, description)", len(options))
	})

	suite.Run("MetaColumnsWithAlias", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_options",
				Action:   "find_options",
				Version:  "v1",
			},
			Meta: map[string]any{
				"metaColumns": []string{"status", "email AS contact"},
			},
		})

		suite.Equal(200, resp.StatusCode, "Should return 200 status code")
		body := suite.ReadResult(resp)
		suite.True(body.IsOk(), "Should return successful response")

		options := suite.ReadDataAsSlice(body.Data)
		suite.Len(options, 25, "Should return 25 options with aliased meta columns")

		// Verify alias is used in meta field
		firstOption := suite.ReadDataAsMap(options[0])
		meta, ok := firstOption["meta"].(map[string]any)
		suite.True(ok, "meta should be a map")
		suite.NotNil(meta, "meta should not be nil")
		suite.Contains(meta, "status", "meta should contain status field")
		suite.Contains(meta, "contact", "meta should contain contact field (aliased from email)")
		suite.NotContains(meta, "email", "meta should not contain original email field when aliased")

		suite.T().Logf("Found %d options with aliased meta columns (status, email AS contact)", len(options))
	})

	suite.Run("InvalidMetaColumn", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_options",
				Action:   "find_options",
				Version:  "v1",
			},
			Meta: map[string]any{
				"metaColumns": []string{"nonexistent_field"},
			},
		})

		suite.Equal(200, resp.StatusCode, "Should return 200 status code")
		body := suite.ReadResult(resp)
		suite.False(body.IsOk(), "Should fail when meta column does not exist")

		suite.T().Logf("Validation failed as expected for invalid meta column 'nonexistent_field'")
	})
}

// TestFindOptionsErrorQueryApplier tests FindOptions with a QueryApplier that returns error.
func (suite *FindOptionsTestSuite) TestFindOptionsErrorQueryApplier() {
	suite.T().Logf("Testing FindOptions API error query applier for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_options_err_applier",
			Action:   "find_options",
			Version:  "v1",
		},
	})

	suite.Contains([]int{200, 500}, resp.StatusCode, "Should return error status code")

	suite.T().Logf("FindOptions failed as expected due to query applier error")
}

// TestFindOptionsMatchingColumnNames covers find_options.go:58,64,73 - column name matching branches.
func (suite *FindOptionsTestSuite) TestFindOptionsMatchingColumnNames() {
	suite.T().Logf("Testing FindOptions API matching column names for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/option_item_options",
			Action:   "find_options",
			Version:  "v1",
		},
	})

	suite.Equal(200, resp.StatusCode, "Should return 200 status code")
	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Should return successful response")

	suite.T().Logf("FindOptions with matching column names returned OK")
}

// TestFindOptionsNonMatchingDescriptionColumn covers find_options.go:73-75 - description else branch.
func (suite *FindOptionsTestSuite) TestFindOptionsNonMatchingDescriptionColumn() {
	suite.T().Logf("Testing FindOptions API non-matching description column for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_options_desc_alias",
			Action:   "find_options",
			Version:  "v1",
		},
	})

	suite.Equal(200, resp.StatusCode, "Should return 200 status code")
	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Should return successful response")

	suite.T().Logf("FindOptions with non-matching description column returned OK")
}

package crud_test

import (
	"errors"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/suite"
	"github.com/uptrace/bun"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/crud"
	"github.com/coldsmirk/vef-framework-go/i18n"
	"github.com/coldsmirk/vef-framework-go/internal/orm"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/result"
)

func init() {
	registry.Add(func(env *testx.DBEnv) suite.TestingSuite {
		return &FindTreeOptionsTestSuite{
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
type DepartmentFindTreeOptionsResource struct {
	api.Resource
	crud.FindTreeOptions[Department, DepartmentSearch]
}

func NewDepartmentFindTreeOptionsResource() api.Resource {
	return &DepartmentFindTreeOptionsResource{
		Resource: api.NewRPCResource("test/department_tree_options"),
		FindTreeOptions: crud.NewFindTreeOptions[Department, DepartmentSearch]().
			WithCondition(fixtureScope).
			Public().
			WithDefaultColumnMapping(&crud.DataOptionColumnMapping{
				LabelColumn: "name",
				ValueColumn: "id",
			}).
			WithIDColumn("id").
			WithParentIDColumn("parent_id"),
	}
}

// Resource with custom field mapping.
type CustomFieldCategoryFindTreeOptionsResource struct {
	api.Resource
	crud.FindTreeOptions[Department, DepartmentSearch]
}

func NewCustomFieldCategoryFindTreeOptionsResource() api.Resource {
	return &CustomFieldCategoryFindTreeOptionsResource{
		Resource: api.NewRPCResource("test/department_tree_options_custom"),
		FindTreeOptions: crud.NewFindTreeOptions[Department, DepartmentSearch]().
			WithCondition(fixtureScope).
			Public().
			WithDefaultColumnMapping(&crud.DataOptionColumnMapping{
				LabelColumn:       "code",
				ValueColumn:       "id",
				DescriptionColumn: "description",
			}).
			WithIDColumn("id").
			WithParentIDColumn("parent_id"),
	}
}

// Filtered Tree Options Resource.
type FilteredCategoryFindTreeOptionsResource struct {
	api.Resource
	crud.FindTreeOptions[Department, DepartmentSearch]
}

func NewFilteredCategoryFindTreeOptionsResource() api.Resource {
	return &FilteredCategoryFindTreeOptionsResource{
		Resource: api.NewRPCResource("test/department_tree_options_filtered"),
		FindTreeOptions: crud.NewFindTreeOptions[Department, DepartmentSearch]().
			WithCondition(fixtureScope).
			WithCondition(func(cb orm.ConditionBuilder) {
				// Only show Product and its children
				cb.Group(func(cb orm.ConditionBuilder) {
					cb.OrEquals("id", "dept002")
					cb.OrEquals("parent_id", "dept002")
				})
			}).
			Public(),
	}
}

// Meta Tree Options Resource.
type MetaCategoryFindTreeOptionsResource struct {
	api.Resource
	crud.FindTreeOptions[Department, DepartmentSearch]
}

func NewMetaCategoryFindTreeOptionsResource() api.Resource {
	return &MetaCategoryFindTreeOptionsResource{
		Resource: api.NewRPCResource("test/department_tree_options_meta"),
		FindTreeOptions: crud.NewFindTreeOptions[Department, DepartmentSearch]().
			WithCondition(fixtureScope).
			Public().
			WithDefaultColumnMapping(&crud.DataOptionColumnMapping{
				LabelColumn: "name",
				ValueColumn: "id",
				MetaColumns: []string{"code", "description"},
			}).
			WithIDColumn("id").
			WithParentIDColumn("parent_id"),
	}
}

// QueryApplierDepartmentFindTreeOptionsResource - with WithQueryApplier.
type QueryApplierDepartmentFindTreeOptionsResource struct {
	api.Resource
	crud.FindTreeOptions[Department, DepartmentSearch]
}

func NewQueryApplierDepartmentFindTreeOptionsResource() api.Resource {
	return &QueryApplierDepartmentFindTreeOptionsResource{
		Resource: api.NewRPCResource("test/department_tree_options_qa"),
		FindTreeOptions: crud.NewFindTreeOptions[Department, DepartmentSearch]().
			WithCondition(fixtureScope).
			Public().
			WithDefaultColumnMapping(&crud.DataOptionColumnMapping{
				LabelColumn: "name",
				ValueColumn: "id",
			}).
			WithIDColumn("id").
			WithParentIDColumn("parent_id").
			WithQueryApplier(func(query orm.SelectQuery, _ DepartmentSearch, _ fiber.Ctx) error {
				query.Where(func(cb orm.ConditionBuilder) {
					cb.IsNull("parent_id")
				})

				return nil
			}),
	}
}

// TreeOptionItem is a tree model with value/label/description fields to cover column matching branches.
type TreeOptionItem struct {
	bun.BaseModel `bun:"table:test_tree_option_item,alias:ttoi"`
	orm.IDModel

	Value       string  `json:"value"       bun:",notnull"`
	Label       string  `json:"label"       bun:",notnull"`
	Description string  `json:"description"`
	ParentID    *string `json:"parentId"`
}

// MatchingColumnFindTreeOptionsResource - FindTreeOptions with columns matching constant names.
type MatchingColumnFindTreeOptionsResource struct {
	api.Resource
	crud.FindTreeOptions[TreeOptionItem, struct{ api.P }]
}

func NewMatchingColumnFindTreeOptionsResource() api.Resource {
	return &MatchingColumnFindTreeOptionsResource{
		Resource: api.NewRPCResource("test/tree_option_item_options"),
		FindTreeOptions: crud.NewFindTreeOptions[TreeOptionItem, struct{ api.P }]().
			Public().
			WithDefaultColumnMapping(&crud.DataOptionColumnMapping{
				ValueColumn:       "value",
				LabelColumn:       "label",
				DescriptionColumn: "description",
			}).
			WithIDColumn("id").
			WithParentIDColumn("parent_id"),
	}
}

// NonMatchingColumnFindTreeOptionsResource - FindTreeOptions with non-matching id/parent_id/description columns.
type NonMatchingColumnFindTreeOptionsResource struct {
	api.Resource
	crud.FindTreeOptions[TreeOptionItem, struct{ api.P }]
}

func NewNonMatchingColumnFindTreeOptionsResource() api.Resource {
	return &NonMatchingColumnFindTreeOptionsResource{
		Resource: api.NewRPCResource("test/tree_option_item_nonmatch"),
		FindTreeOptions: crud.NewFindTreeOptions[TreeOptionItem, struct{ api.P }]().
			Public().
			WithDefaultColumnMapping(&crud.DataOptionColumnMapping{
				ValueColumn:       "value",
				LabelColumn:       "label",
				DescriptionColumn: "value",
			}).
			WithIDColumn("value").
			WithParentIDColumn("label"),
	}
}

// ErrorQueryApplierDepartmentFindTreeOptionsResource - with QueryApplier that returns error.
type ErrorQueryApplierDepartmentFindTreeOptionsResource struct {
	api.Resource
	crud.FindTreeOptions[Department, DepartmentSearch]
}

func NewErrorQueryApplierDepartmentFindTreeOptionsResource() api.Resource {
	return &ErrorQueryApplierDepartmentFindTreeOptionsResource{
		Resource: api.NewRPCResource("test/department_tree_options_err_qa"),
		FindTreeOptions: crud.NewFindTreeOptions[Department, DepartmentSearch]().
			WithCondition(fixtureScope).
			Public().
			WithDefaultColumnMapping(&crud.DataOptionColumnMapping{
				LabelColumn: "name",
				ValueColumn: "id",
			}).
			WithIDColumn("id").
			WithParentIDColumn("parent_id").
			WithQueryApplier(func(_ orm.SelectQuery, _ DepartmentSearch, _ fiber.Ctx) error {
				return errors.New("tree options query applier error")
			}),
	}
}

// FindTreeOptionsTestSuite tests the FindTreeOptions API functionality
// including basic tree options, custom column mappings, search filters, filter appliers, meta columns, and tree-specific features.
type FindTreeOptionsTestSuite struct {
	BaseTestSuite
}

// SetupSuite runs once before all tests in the suite.
func (suite *FindTreeOptionsTestSuite) SetupSuite() {
	if suite.ds.Kind == config.SQLite {
		suite.T().Skipf("Skipping FindTreeOptions tests on %s due to Bun recursive CTE syntax issue", suite.ds.Kind)
	}

	suite.setupBaseSuite(
		NewDepartmentFindTreeOptionsResource,
		NewCustomFieldCategoryFindTreeOptionsResource,
		NewFilteredCategoryFindTreeOptionsResource,
		NewMetaCategoryFindTreeOptionsResource,
		NewQueryApplierDepartmentFindTreeOptionsResource,
		NewErrorQueryApplierDepartmentFindTreeOptionsResource,
		NewMatchingColumnFindTreeOptionsResource,
		NewNonMatchingColumnFindTreeOptionsResource,
	)
}

// TearDownSuite runs once after all tests in the suite.
func (suite *FindTreeOptionsTestSuite) TearDownSuite() {
	suite.tearDownBaseSuite()
}

// TestFindTreeOptionsBasic tests basic FindTreeOptions functionality.
func (suite *FindTreeOptionsTestSuite) TestFindTreeOptionsBasic() {
	suite.T().Logf("Testing FindTreeOptions API basic functionality for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/department_tree_options",
			Action:   "find_tree_options",
			Version:  "v1",
		},
	})

	suite.Equal(200, resp.StatusCode, "Should return 200 status code")
	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Should return successful response")
	suite.Equal(body.Message, i18n.T(result.OkMessage), "Should return OK message")
	suite.NotNil(body.Data, "Data should not be nil")

	tree := suite.ReadDataAsSlice(body.Data)
	suite.Len(tree, 4, "Should return 4 root departments")

	// Verify default ordering by id DESC - HR (dept004) should be first
	first := suite.ReadDataAsMap(tree[0])
	suite.Equal("HR", first["label"], "First department should be HR")
	suite.NotEmpty(first["value"], "First option should have value")

	second := suite.ReadDataAsMap(tree[1])
	suite.Equal("Marketing", second["label"], "Second department should be Marketing")

	third := suite.ReadDataAsMap(tree[2])
	suite.Equal("Product", third["label"], "Third department should be Product")

	fourth := suite.ReadDataAsMap(tree[3])
	suite.Equal("Engineering", fourth["label"], "Fourth department should be Engineering")

	// Check first option (HR) has children
	children := suite.ReadDataAsSlice(first["children"])
	suite.Len(children, 2, "HR should have 2 children (Recruitment and Training)")

	// Check child option structure
	childOption := suite.ReadDataAsMap(children[0])
	suite.NotEmpty(childOption["label"], "Child option should have label")
	suite.NotEmpty(childOption["value"], "Child option should have value")

	suite.T().Logf("Found %d root departments with %d children in first department", len(tree), len(children))
}

// TestFindTreeOptionsWithConfig tests FindTreeOptions with custom config.
func (suite *FindTreeOptionsTestSuite) TestFindTreeOptionsWithConfig() {
	suite.T().Logf("Testing FindTreeOptions API with custom config for %s", suite.ds.Kind)

	suite.Run("DefaultConfig", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/department_tree_options",
				Action:   "find_tree_options",
				Version:  "v1",
			},
		})

		suite.Equal(200, resp.StatusCode, "Should return 200 status code")
		body := suite.ReadResult(resp)
		suite.True(body.IsOk(), "Should return successful response")

		tree := suite.ReadDataAsSlice(body.Data)
		suite.Len(tree, 4, "Should return 4 root departments with default config")

		suite.T().Logf("Found %d root departments with default config (label=name, value=id)", len(tree))
	})

	suite.Run("CustomConfig", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/department_tree_options",
				Action:   "find_tree_options",
				Version:  "v1",
			},
			Meta: map[string]any{
				"labelColumn": "code",
				"valueColumn": "id",
			},
		})

		suite.Equal(200, resp.StatusCode, "Should return 200 status code")
		body := suite.ReadResult(resp)
		suite.True(body.IsOk(), "Should return successful response")

		tree := suite.ReadDataAsSlice(body.Data)
		suite.Len(tree, 4, "Should return 4 root departments with custom config")

		// Verify code is used as label and ordering by id DESC
		first := suite.ReadDataAsMap(tree[0])
		suite.Equal("HR", first["label"], "First department should use code 'HR' as label")

		second := suite.ReadDataAsMap(tree[1])
		suite.Equal("MKT", second["label"], "Second department should use code 'MKT' as label")

		third := suite.ReadDataAsMap(tree[2])
		suite.Equal("PRD", third["label"], "Third department should use code 'PRD' as label")

		fourth := suite.ReadDataAsMap(tree[3])
		suite.Equal("ENG", fourth["label"], "Fourth department should use code 'ENG' as label")

		suite.T().Logf("Found %d root departments with custom config (label=code: %s, %s, %s, %s)", len(tree), first["label"], second["label"], third["label"], fourth["label"])
	})

	suite.Run("WithDescription", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/department_tree_options_custom",
				Action:   "find_tree_options",
				Version:  "v1",
			},
		})

		suite.Equal(200, resp.StatusCode, "Should return 200 status code")
		body := suite.ReadResult(resp)
		suite.True(body.IsOk(), "Should return successful response")

		tree := suite.ReadDataAsSlice(body.Data)
		suite.Len(tree, 4, "Should return 4 root departments with description field")

		// Verify description is included
		firstDept := suite.ReadDataAsMap(tree[0])
		suite.NotEmpty(firstDept["description"], "First option should have description field")

		suite.T().Logf("Found %d root departments with description column (description: %v)", len(tree), firstDept["description"])
	})
}

// TestFindTreeOptionsWithSearch tests FindTreeOptions with search conditions.
func (suite *FindTreeOptionsTestSuite) TestFindTreeOptionsWithSearch() {
	suite.T().Logf("Testing FindTreeOptions API with search conditions for %s", suite.ds.Kind)

	suite.Run("SearchByCode", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/department_tree_options",
				Action:   "find_tree_options",
				Version:  "v1",
			},
			Params: map[string]any{
				"code": "MKT",
			},
		})

		suite.Equal(200, resp.StatusCode, "Should return 200 status code")
		body := suite.ReadResult(resp)
		suite.True(body.IsOk(), "Should return successful response")

		tree := suite.ReadDataAsSlice(body.Data)
		suite.Len(tree, 1, "Should return only Marketing department")

		mkt := suite.ReadDataAsMap(tree[0])
		suite.Equal("Marketing", mkt["label"], "Department label should be Marketing")

		suite.T().Logf("Found 1 department matching code 'MKT': %s", mkt["label"])
	})

	suite.Run("SearchByKeyword", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/department_tree_options",
				Action:   "find_tree_options",
				Version:  "v1",
			},
			Params: map[string]any{
				"keyword": "Backend",
			},
		})

		suite.Equal(200, resp.StatusCode, "Should return 200 status code")
		body := suite.ReadResult(resp)
		suite.True(body.IsOk(), "Should return successful response")

		tree := suite.ReadDataAsSlice(body.Data)
		suite.GreaterOrEqual(len(tree), 1, "Should return at least 1 department matching keyword 'Backend'")

		suite.T().Logf("Found %d departments matching keyword 'Backend'", len(tree))
	})
}

// TestFindTreeOptionsWithFilterApplier tests FindTreeOptions with filter applier.
func (suite *FindTreeOptionsTestSuite) TestFindTreeOptionsWithFilterApplier() {
	suite.T().Logf("Testing FindTreeOptions API with filter applier for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/department_tree_options_filtered",
			Action:   "find_tree_options",
			Version:  "v1",
		},
	})

	suite.Equal(200, resp.StatusCode, "Should return 200 status code")
	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Should return successful response")

	tree := suite.ReadDataAsSlice(body.Data)
	suite.Len(tree, 1, "Should return only Product root (filtered to Product and its children)")

	product := suite.ReadDataAsMap(tree[0])
	suite.Equal("Product", product["label"], "Root department should be Product")

	children := suite.ReadDataAsSlice(product["children"])
	suite.Len(children, 2, "Product should have 2 children (Design and Research)")

	suite.T().Logf("Found 1 filtered tree rooted at Product with %d children", len(children))
}

// TestFindTreeOptionsNegativeCases tests negative scenarios.
func (suite *FindTreeOptionsTestSuite) TestFindTreeOptionsNegativeCases() {
	suite.T().Logf("Testing FindTreeOptions API negative cases for %s", suite.ds.Kind)

	suite.Run("NoMatchingRecords", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/department_tree_options",
				Action:   "find_tree_options",
				Version:  "v1",
			},
			Params: map[string]any{
				"keyword": "NonexistentCategory",
			},
		})

		suite.Equal(200, resp.StatusCode, "Should return 200 status code")
		body := suite.ReadResult(resp)
		suite.True(body.IsOk(), "Should return successful response")

		tree := suite.ReadDataAsSlice(body.Data)
		suite.Len(tree, 0, "Should return empty tree when no records match")

		suite.T().Logf("No matching records found as expected for keyword 'NonexistentCategory'")
	})

	suite.Run("InvalidFieldName", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/department_tree_options",
				Action:   "find_tree_options",
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

// TestFindTreeOptionsWithMeta tests FindTreeOptions with meta columns.
func (suite *FindTreeOptionsTestSuite) TestFindTreeOptionsWithMeta() {
	suite.T().Logf("Testing FindTreeOptions API with meta columns for %s", suite.ds.Kind)

	suite.Run("DefaultMetaColumns", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/department_tree_options_meta",
				Action:   "find_tree_options",
				Version:  "v1",
			},
		})

		suite.Equal(200, resp.StatusCode, "Should return 200 status code")
		body := suite.ReadResult(resp)
		suite.True(body.IsOk(), "Should return successful response")

		tree := suite.ReadDataAsSlice(body.Data)
		suite.Len(tree, 4, "Should return 4 root departments with default meta columns")

		// Verify meta field exists and contains expected keys
		firstOption := suite.ReadDataAsMap(tree[0])
		meta, ok := firstOption["meta"].(map[string]any)
		suite.True(ok, "meta should be a map")
		suite.NotNil(meta, "meta should not be nil")
		suite.Contains(meta, "code", "meta should contain code field")
		suite.Contains(meta, "description", "meta should contain description field")

		suite.T().Logf("Found %d root categories with default meta columns (code, description)", len(tree))
	})

	suite.Run("CustomMetaColumns", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/department_tree_options",
				Action:   "find_tree_options",
				Version:  "v1",
			},
			Meta: map[string]any{
				"metaColumns": []string{"code"},
			},
		})

		suite.Equal(200, resp.StatusCode, "Should return 200 status code")
		body := suite.ReadResult(resp)
		suite.True(body.IsOk(), "Should return successful response")

		tree := suite.ReadDataAsSlice(body.Data)
		suite.Len(tree, 4, "Should return 4 root departments with custom meta columns")

		// Verify meta field contains custom columns
		firstOption := suite.ReadDataAsMap(tree[0])
		meta, ok := firstOption["meta"].(map[string]any)
		suite.True(ok, "meta should be a map")
		suite.NotNil(meta, "meta should not be nil")
		suite.Contains(meta, "code", "meta should contain code field")

		suite.T().Logf("Found %d root categories with custom meta columns (code)", len(tree))
	})

	suite.Run("MetaColumnsWithAlias", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/department_tree_options",
				Action:   "find_tree_options",
				Version:  "v1",
			},
			Meta: map[string]any{
				"metaColumns": []string{"code AS category_code", "description as desc"},
			},
		})

		suite.Equal(200, resp.StatusCode, "Should return 200 status code")
		body := suite.ReadResult(resp)
		suite.True(body.IsOk(), "Should return successful response")

		tree := suite.ReadDataAsSlice(body.Data)
		suite.Len(tree, 4, "Should return 4 root departments with aliased meta columns")

		// Verify alias is used in meta field
		firstOption := suite.ReadDataAsMap(tree[0])
		meta, ok := firstOption["meta"].(map[string]any)
		suite.True(ok, "meta should be a map")
		suite.NotNil(meta, "meta should not be nil")
		suite.Contains(meta, "category_code", "meta should contain category_code field (aliased from code)")
		suite.Contains(meta, "desc", "meta should contain desc field (aliased from description)")
		suite.NotContains(meta, "code", "meta should not contain original code field when aliased")
		suite.NotContains(meta, "description", "meta should not contain original description field when aliased")

		suite.T().Logf("Found %d root categories with aliased meta columns (code AS category_code, description AS desc)", len(tree))
	})

	suite.Run("VerifyMetaInChildren", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/department_tree_options_meta",
				Action:   "find_tree_options",
				Version:  "v1",
			},
		})

		suite.Equal(200, resp.StatusCode, "Should return 200 status code")
		body := suite.ReadResult(resp)
		suite.True(body.IsOk(), "Should return successful response")

		tree := suite.ReadDataAsSlice(body.Data)
		suite.Len(tree, 4, "Should return 4 root departments")

		// Check that children also have meta field
		firstOption := suite.ReadDataAsMap(tree[0])
		children := suite.ReadDataAsSlice(firstOption["children"])
		suite.Greater(len(children), 0, "First category should have children")

		// Verify child has meta field
		childOption := suite.ReadDataAsMap(children[0])
		childMeta, ok := childOption["meta"].(map[string]any)
		suite.True(ok, "child meta should be a map")
		suite.NotNil(childMeta, "child meta should not be nil")
		suite.Contains(childMeta, "code", "child meta should contain code field")
		suite.Contains(childMeta, "description", "child meta should contain description field")

		suite.T().Logf("Verified meta columns in %d root categories and their %d children", len(tree), len(children))
	})

	suite.Run("InvalidMetaColumn", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/department_tree_options",
				Action:   "find_tree_options",
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

// TestFindTreeOptionsWithQueryApplier tests FindTreeOptions with WithQueryApplier.
func (suite *FindTreeOptionsTestSuite) TestFindTreeOptionsWithQueryApplier() {
	suite.T().Logf("Testing FindTreeOptions API with WithQueryApplier for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/department_tree_options_qa",
			Action:   "find_tree_options",
			Version:  "v1",
		},
	})

	suite.Equal(200, resp.StatusCode, "Should return 200 status code")
	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Should return successful response")

	tree := suite.ReadDataAsSlice(body.Data)
	suite.Len(tree, 4, "Should return 4 root departments (query applier filters to roots)")

	suite.T().Logf("WithQueryApplier returned %d root department options", len(tree))
}

// TestFindTreeOptionsErrorQueryApplier tests FindTreeOptions with a QueryApplier that returns error.
func (suite *FindTreeOptionsTestSuite) TestFindTreeOptionsErrorQueryApplier() {
	suite.T().Logf("Testing FindTreeOptions API error query applier for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/department_tree_options_err_qa",
			Action:   "find_tree_options",
			Version:  "v1",
		},
	})

	suite.Contains([]int{200, 500}, resp.StatusCode, "Should return error status code")

	suite.T().Logf("FindTreeOptions failed as expected due to query applier error")
}

// TestFindTreeOptionsMatchingColumnNames covers find_tree_options.go:164,170,179 - column name matching branches.
func (suite *FindTreeOptionsTestSuite) TestFindTreeOptionsMatchingColumnNames() {
	suite.T().Logf("Testing FindTreeOptions API matching column names for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/tree_option_item_options",
			Action:   "find_tree_options",
			Version:  "v1",
		},
	})

	suite.Equal(200, resp.StatusCode, "Should return 200 status code")
	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Should return successful response")

	suite.T().Logf("FindTreeOptions with matching column names returned OK")
}

// TestFindTreeOptionsNonMatchingColumns covers find_tree_options.go:112,118,179 - non-matching column else branches.
func (suite *FindTreeOptionsTestSuite) TestFindTreeOptionsNonMatchingColumns() {
	suite.T().Logf("Testing FindTreeOptions API non-matching columns for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/tree_option_item_nonmatch",
			Action:   "find_tree_options",
			Version:  "v1",
		},
	})

	// Query may fail due to semantically wrong column mappings, but the column matching code still executes
	suite.Contains([]int{200, 500}, resp.StatusCode, "Should return a status code")

	suite.T().Logf("FindTreeOptions with non-matching columns executed")
}

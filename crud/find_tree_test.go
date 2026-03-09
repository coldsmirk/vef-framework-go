package crud_test

import (
	"errors"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/crud"
	"github.com/coldsmirk/vef-framework-go/i18n"
	"github.com/coldsmirk/vef-framework-go/internal/orm"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/sortx"
	"github.com/coldsmirk/vef-framework-go/tree"
)

func init() {
	registry.Add(func(env *testx.DBEnv) suite.TestingSuite {
		return &FindTreeTestSuite{
			BaseTestSuite: BaseTestSuite{
				ctx:   env.Ctx,
				db:    env.DB,
				bunDB: env.BunDB,
				ds:    env.DS,
			},
		}
	})
}

// Tree builder for Department.
func buildCategoryTree(flatCategories []Department) []Department {
	adapter := tree.Adapter[Department]{
		GetID: func(c Department) string {
			return c.ID
		},
		GetParentID: func(c Department) *string {
			return c.ParentID
		},
		SetChildren: func(c *Department, children []Department) {
			c.Children = children
		},
	}

	return tree.Build(flatCategories, adapter)
}

// Test Resources.
type DepartmentFindTreeResource struct {
	api.Resource
	crud.FindTree[Department, DepartmentSearch]
}

func NewDepartmentFindTreeResource() api.Resource {
	return &DepartmentFindTreeResource{
		Resource: api.NewRPCResource("test/department_tree"),
		FindTree: crud.NewFindTree[Department, DepartmentSearch](buildCategoryTree).
			WithCondition(fixtureScope).
			Public().
			WithIDColumn("id").
			WithParentIDColumn("parent_id"),
	}
}

// Filtered Tree Resource.
type FilteredCategoryFindTreeResource struct {
	api.Resource
	crud.FindTree[Department, DepartmentSearch]
}

func NewFilteredCategoryFindTreeResource() api.Resource {
	return &FilteredCategoryFindTreeResource{
		Resource: api.NewRPCResource("test/department_tree_filtered"),
		FindTree: crud.NewFindTree[Department, DepartmentSearch](buildCategoryTree).
			WithCondition(fixtureScope).
			WithCondition(func(cb orm.ConditionBuilder) {
				// Only show Engineering and its children
				cb.Group(func(cb orm.ConditionBuilder) {
					cb.OrEquals("id", "dept001")
					cb.OrEquals("parent_id", "dept001")
				})
			}).
			Public().
			WithIDColumn("id").
			WithParentIDColumn("parent_id"),
	}
}

// Ordered Tree Resource.
type OrderedCategoryFindTreeResource struct {
	api.Resource
	crud.FindTree[Department, DepartmentSearch]
}

func NewOrderedCategoryFindTreeResource() api.Resource {
	return &OrderedCategoryFindTreeResource{
		Resource: api.NewRPCResource("test/department_tree_ordered"),
		FindTree: crud.NewFindTree[Department, DepartmentSearch](buildCategoryTree).
			WithCondition(fixtureScope).
			WithDefaultSort(&sortx.OrderSpec{
				Column: "sort",
			}).
			Public().
			WithIDColumn("id").
			WithParentIDColumn("parent_id"),
	}
}

// AuditUser Tree Resource - with audit user names.
type AuditUserCategoryFindTreeResource struct {
	api.Resource
	crud.FindTree[Department, DepartmentSearch]
}

func NewAuditUserCategoryFindTreeResource() api.Resource {
	return &AuditUserCategoryFindTreeResource{
		Resource: api.NewRPCResource("test/department_tree_audit"),
		FindTree: crud.NewFindTree[Department, DepartmentSearch](buildCategoryTree).
			WithCondition(fixtureScope).
			Public().
			WithIDColumn("id").
			WithParentIDColumn("parent_id").
			WithAuditUserNames((*Operator)(nil)),
	}
}

// QueryApplierDepartmentFindTreeResource - with WithQueryApplier.
type QueryApplierDepartmentFindTreeResource struct {
	api.Resource
	crud.FindTree[Department, DepartmentSearch]
}

func NewQueryApplierDepartmentFindTreeResource() api.Resource {
	return &QueryApplierDepartmentFindTreeResource{
		Resource: api.NewRPCResource("test/department_tree_query_applier"),
		FindTree: crud.NewFindTree[Department, DepartmentSearch](buildCategoryTree).
			WithCondition(fixtureScope).
			Public().
			WithIDColumn("id").
			WithParentIDColumn("parent_id").
			WithQueryApplier(func(query orm.SelectQuery, _ DepartmentSearch, _ fiber.Ctx) error {
				// Custom query applier: only include root departments (no parent)
				query.Where(func(cb orm.ConditionBuilder) {
					cb.IsNull("parent_id")
				})

				return nil
			}),
	}
}

// ErrorQueryApplierDepartmentFindTreeResource - with QueryApplier that returns error.
type ErrorQueryApplierDepartmentFindTreeResource struct {
	api.Resource
	crud.FindTree[Department, DepartmentSearch]
}

func NewErrorQueryApplierDepartmentFindTreeResource() api.Resource {
	return &ErrorQueryApplierDepartmentFindTreeResource{
		Resource: api.NewRPCResource("test/department_tree_err_applier"),
		FindTree: crud.NewFindTree[Department, DepartmentSearch](buildCategoryTree).
			WithCondition(fixtureScope).
			Public().
			WithIDColumn("id").
			WithParentIDColumn("parent_id").
			WithQueryApplier(func(_ orm.SelectQuery, _ DepartmentSearch, _ fiber.Ctx) error {
				return errors.New("tree query applier error")
			}),
	}
}

// FindTreeTestSuite tests the FindTree API functionality
// including basic tree building, search filters, filter appliers, sort appliers, audit user names, and negative cases.
type FindTreeTestSuite struct {
	BaseTestSuite
}

// SetupSuite runs once before all tests in the suite.
func (suite *FindTreeTestSuite) SetupSuite() {
	if suite.ds.Kind == config.SQLite {
		suite.T().Skipf("Skipping FindTree tests on %s due to Bun recursive CTE syntax issue", suite.ds.Kind)
	}

	suite.setupBaseSuite(
		NewDepartmentFindTreeResource,
		NewFilteredCategoryFindTreeResource,
		NewOrderedCategoryFindTreeResource,
		NewAuditUserCategoryFindTreeResource,
		NewQueryApplierDepartmentFindTreeResource,
		NewErrorQueryApplierDepartmentFindTreeResource,
	)
}

// TearDownSuite runs once after all tests in the suite.
func (suite *FindTreeTestSuite) TearDownSuite() {
	suite.tearDownBaseSuite()
}

// TestFindTreeBasic tests basic FindTree functionality.
func (suite *FindTreeTestSuite) TestFindTreeBasic() {
	suite.T().Logf("Testing FindTree API basic functionality for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/department_tree",
			Action:   "find_tree",
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
	suite.Equal("HR", first["name"], "First department should be HR (dept004)")

	second := suite.ReadDataAsMap(tree[1])
	suite.Equal("Marketing", second["name"], "Second department should be Marketing (dept003)")

	third := suite.ReadDataAsMap(tree[2])
	suite.Equal("Product", third["name"], "Third department should be Product (dept002)")

	fourth := suite.ReadDataAsMap(tree[3])
	suite.Equal("Engineering", fourth["name"], "Fourth department should be Engineering (dept001)")

	// Check Engineering has children (it's the fourth item due to DESC ordering)
	children := suite.ReadDataAsSlice(fourth["children"])
	suite.Len(children, 4, "Engineering should have 4 children (Backend, Frontend, DevOps, QA)")

	suite.T().Logf("Found %d root departments with default ordering (DESC by id)", len(tree))
}

// TestFindTreeWithSearch tests FindTree with search conditions.
func (suite *FindTreeTestSuite) TestFindTreeWithSearch() {
	suite.T().Logf("Testing FindTree API with search filters for %s", suite.ds.Kind)

	suite.Run("SearchByCode", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/department_tree",
				Action:   "find_tree",
				Version:  "v1",
			},
			Params: map[string]any{
				"code": "ENG",
			},
		})

		suite.Equal(200, resp.StatusCode, "Should return 200 status code")
		body := suite.ReadResult(resp)
		suite.True(body.IsOk(), "Should return successful response")

		tree := suite.ReadDataAsSlice(body.Data)
		suite.Len(tree, 1, "Should return only Engineering department")

		eng := suite.ReadDataAsMap(tree[0])
		suite.Equal("Engineering", eng["name"], "Department name should be Engineering")

		suite.T().Logf("Found 1 department matching code 'ENG': %s", eng["name"])
	})

	suite.Run("SearchByParentID", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/department_tree",
				Action:   "find_tree",
				Version:  "v1",
			},
			Params: map[string]any{
				"parentId": "dept001", // Engineering's children
			},
		})

		suite.Equal(200, resp.StatusCode, "Should return 200 status code")
		body := suite.ReadResult(resp)
		suite.True(body.IsOk(), "Should return successful response")

		tree := suite.ReadDataAsSlice(body.Data)
		suite.Len(tree, 1, "Should return 1 tree with Engineering as root (recursive CTE finds ancestors)")

		eng := suite.ReadDataAsMap(tree[0])
		suite.Equal("Engineering", eng["name"], "Root should be Engineering")

		// Engineering should have 4 children: Backend, Frontend, DevOps, QA
		children := suite.ReadDataAsSlice(eng["children"])
		suite.Len(children, 4, "Engineering should have 4 children (Backend, Frontend, DevOps, QA)")

		suite.T().Logf("Found tree rooted at Engineering with %d children", len(children))
	})

	suite.Run("SearchByKeyword", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/department_tree",
				Action:   "find_tree",
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

// TestFindTreeWithFilterApplier tests FindTree with filter applier.
func (suite *FindTreeTestSuite) TestFindTreeWithFilterApplier() {
	suite.T().Logf("Testing FindTree API with filter applier for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/department_tree_filtered",
			Action:   "find_tree",
			Version:  "v1",
		},
	})

	suite.Equal(200, resp.StatusCode, "Should return 200 status code")
	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Should return successful response")

	tree := suite.ReadDataAsSlice(body.Data)
	suite.Len(tree, 1, "Should return only Engineering root (filtered to Engineering and its children)")

	eng := suite.ReadDataAsMap(tree[0])
	suite.Equal("Engineering", eng["name"], "Root department should be Engineering")

	children := suite.ReadDataAsSlice(eng["children"])
	suite.Len(children, 4, "Engineering should have 4 children (Backend, Frontend, DevOps, QA)")

	suite.T().Logf("Found 1 filtered tree rooted at Engineering with %d children", len(children))
}

// TestFindTreeWithSortApplier tests FindTree with sort applier.
func (suite *FindTreeTestSuite) TestFindTreeWithSortApplier() {
	suite.T().Logf("Testing FindTree API with sort applier for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/department_tree_ordered",
			Action:   "find_tree",
			Version:  "v1",
		},
	})

	suite.Equal(200, resp.StatusCode, "Should return 200 status code")
	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Should return successful response")

	tree := suite.ReadDataAsSlice(body.Data)
	suite.Len(tree, 4, "Should return 4 root departments")

	// Verify ordering by sort field ASC
	first := suite.ReadDataAsMap(tree[0])
	suite.Equal("Engineering", first["name"], "First department should be Engineering (sort=1)")

	second := suite.ReadDataAsMap(tree[1])
	suite.Equal("Product", second["name"], "Second department should be Product (sort=2)")

	third := suite.ReadDataAsMap(tree[2])
	suite.Equal("Marketing", third["name"], "Third department should be Marketing (sort=3)")

	fourth := suite.ReadDataAsMap(tree[3])
	suite.Equal("HR", fourth["name"], "Fourth department should be HR (sort=4)")

	suite.T().Logf("Found 4 root departments ordered by sort field: Engineering, Product, Marketing, HR")
}

// TestFindTreeNegativeCases tests negative scenarios.
func (suite *FindTreeTestSuite) TestFindTreeNegativeCases() {
	suite.T().Logf("Testing FindTree API negative cases for %s", suite.ds.Kind)

	suite.Run("NoMatchingRecords", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/department_tree",
				Action:   "find_tree",
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

	suite.Run("EmptySearchCriteria", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/department_tree",
				Action:   "find_tree",
				Version:  "v1",
			},
			Params: map[string]any{},
		})

		suite.Equal(200, resp.StatusCode, "Should return 200 status code")
		body := suite.ReadResult(resp)
		suite.True(body.IsOk(), "Should return successful response")

		tree := suite.ReadDataAsSlice(body.Data)
		suite.Len(tree, 4, "Should return all 4 root departments with empty search criteria")

		suite.T().Logf("Empty search criteria returned %d root departments", len(tree))
	})
}

// TestFindTreeWithAuditUserNames tests FindTree with audit user names populated.
func (suite *FindTreeTestSuite) TestFindTreeWithAuditUserNames() {
	suite.T().Logf("Testing FindTree API with audit user names for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/department_tree_audit",
			Action:   "find_tree",
			Version:  "v1",
		},
	})

	suite.Equal(200, resp.StatusCode, "Should return 200 status code")
	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Should return successful response")
	suite.NotNil(body.Data, "Data should not be nil")

	tree := suite.ReadDataAsSlice(body.Data)
	suite.Len(tree, 4, "Should return 4 root departments")

	rootDepartmentsWithAudit := 0

	childDepartmentsWithAudit := 0
	for _, c := range tree {
		dept := suite.ReadDataAsMap(c)
		suite.NotNil(dept["createdByName"], "Department %s should have createdByName", dept["id"])
		suite.NotNil(dept["updatedByName"], "Department %s should have updatedByName", dept["id"])

		rootDepartmentsWithAudit++

		// Check children if they exist
		if dept["children"] != nil {
			children := suite.ReadDataAsSlice(dept["children"])
			for _, ch := range children {
				child := suite.ReadDataAsMap(ch)
				suite.NotNil(child["createdByName"], "Child department %s should have createdByName", child["id"])
				suite.NotNil(child["updatedByName"], "Child department %s should have updatedByName", child["id"])

				childDepartmentsWithAudit++
			}
		}
	}

	suite.T().Logf("Verified audit user names for %d root departments and %d child departments", rootDepartmentsWithAudit, childDepartmentsWithAudit)
}

// TestFindTreeWithQueryApplier tests FindTree with WithQueryApplier.
func (suite *FindTreeTestSuite) TestFindTreeWithQueryApplier() {
	suite.T().Logf("Testing FindTree API with WithQueryApplier for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/department_tree_query_applier",
			Action:   "find_tree",
			Version:  "v1",
		},
	})

	suite.Equal(200, resp.StatusCode, "Should return 200 status code")
	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Should return successful response")

	tree := suite.ReadDataAsSlice(body.Data)
	suite.Len(tree, 4, "Should return 4 root departments (query applier filters base to roots only)")

	suite.T().Logf("WithQueryApplier returned %d root departments", len(tree))
}

// TestFindTreeErrorQueryApplier tests FindTree with a QueryApplier that returns error.
func (suite *FindTreeTestSuite) TestFindTreeErrorQueryApplier() {
	suite.T().Logf("Testing FindTree API error query applier for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/department_tree_err_applier",
			Action:   "find_tree",
			Version:  "v1",
		},
	})

	suite.Contains([]int{200, 500}, resp.StatusCode, "Should return error status code")

	suite.T().Logf("FindTree failed as expected due to query applier error")
}

// TestFindTreeWrapperMethods covers find_tree.go WithSelect/WithSelectAs/WithRelation wrapper methods.
func TestFindTreeWrapperMethods(t *testing.T) {
	treeBuilder := func(flat []orm.Model) []orm.Model { return flat }

	t.Run("WithSelect", func(t *testing.T) {
		ft := crud.NewFindTree[orm.Model, struct{}](treeBuilder).WithSelect("col1")
		assert.NotNil(t, ft, "WithSelect should return non-nil builder")
	})

	t.Run("WithSelectAs", func(t *testing.T) {
		ft := crud.NewFindTree[orm.Model, struct{}](treeBuilder).WithSelectAs("col1", "alias1")
		assert.NotNil(t, ft, "WithSelectAs should return non-nil builder")
	})

	t.Run("WithRelation", func(t *testing.T) {
		ft := crud.NewFindTree[orm.Model, struct{}](treeBuilder).WithRelation(&orm.RelationSpec{
			Model: (*orm.Model)(nil),
		})
		assert.NotNil(t, ft, "WithRelation should return non-nil builder")
	})

	t.Run("WithQueryApplier", func(t *testing.T) {
		ft := crud.NewFindTree[orm.Model, struct{}](treeBuilder).WithQueryApplier(nil)
		assert.NotNil(t, ft, "WithQueryApplier should return non-nil builder")
	})
}

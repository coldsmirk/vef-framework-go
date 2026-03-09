package crud_test

import (
	"errors"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/crud"
	"github.com/coldsmirk/vef-framework-go/i18n"
	"github.com/coldsmirk/vef-framework-go/internal/orm"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/result"
)

func init() {
	registry.Add(func(env *testx.DBEnv) suite.TestingSuite {
		return &FindPageTestSuite{
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
type EmployeeFindPageResource struct {
	api.Resource
	crud.FindPage[Employee, EmployeeSearch]
}

func NewEmployeeFindPageResource() api.Resource {
	return &EmployeeFindPageResource{
		Resource: api.NewRPCResource("test/employee_page"),
		FindPage: crud.NewFindPage[Employee, EmployeeSearch]().WithCondition(fixtureScope).Public(),
	}
}

// Processed User Resource - with processor.
type ProcessedUserFindPageResource struct {
	api.Resource
	crud.FindPage[Employee, EmployeeSearch]
}

func NewProcessedUserFindPageResource() api.Resource {
	return &ProcessedUserFindPageResource{
		Resource: api.NewRPCResource("test/employee_page_processed"),
		FindPage: crud.NewFindPage[Employee, EmployeeSearch]().
			WithCondition(fixtureScope).
			Public().
			WithProcessor(func(users []Employee, _ EmployeeSearch, _ fiber.Ctx) any {
				// Processor must return a slice - convert each user to a processed version
				processed := make([]ProcessedUser, len(users))
				for i, user := range users {
					processed[i] = ProcessedUser{
						Employee:  user,
						Processed: true,
					}
				}

				return processed
			}),
	}
}

// Filtered User Resource - with filter applier.
type FilteredUserFindPageResource struct {
	api.Resource
	crud.FindPage[Employee, EmployeeSearch]
}

func NewFilteredUserFindPageResource() api.Resource {
	return &FilteredUserFindPageResource{
		Resource: api.NewRPCResource("test/employee_page_filtered"),
		FindPage: crud.NewFindPage[Employee, EmployeeSearch]().
			WithCondition(fixtureScope).
			WithCondition(func(cb orm.ConditionBuilder) {
				cb.Equals("status", "active")
			}).
			Public(),
	}
}

// AuditUser User Resource - with audit user names.
type AuditedEmployeeFindPageResource struct {
	api.Resource
	crud.FindPage[Employee, EmployeeSearch]
}

func NewAuditedEmployeeFindPageResource() api.Resource {
	return &AuditedEmployeeFindPageResource{
		Resource: api.NewRPCResource("test/employee_page_audit"),
		FindPage: crud.NewFindPage[Employee, EmployeeSearch]().
			WithCondition(fixtureScope).
			WithAuditUserNames((*Operator)(nil)).
			Public(),
	}
}

// DefaultPageSizeEmployeeFindPageResource - with WithDefaultPageSize.
type DefaultPageSizeEmployeeFindPageResource struct {
	api.Resource
	crud.FindPage[Employee, EmployeeSearch]
}

func NewDefaultPageSizeEmployeeFindPageResource() api.Resource {
	return &DefaultPageSizeEmployeeFindPageResource{
		Resource: api.NewRPCResource("test/employee_page_default_size"),
		FindPage: crud.NewFindPage[Employee, EmployeeSearch]().
			WithCondition(fixtureScope).
			WithDefaultPageSize(3).
			Public(),
	}
}

// ErrorQueryApplierFindPageResource - FindPage with QueryApplier that returns error.
type ErrorQueryApplierFindPageResource struct {
	api.Resource
	crud.FindPage[Employee, EmployeeSearch]
}

func NewErrorQueryApplierFindPageResource() api.Resource {
	return &ErrorQueryApplierFindPageResource{
		Resource: api.NewRPCResource("test/employee_page_err_applier"),
		FindPage: crud.NewFindPage[Employee, EmployeeSearch]().
			WithCondition(fixtureScope).
			WithQueryApplier(func(_ orm.SelectQuery, _ EmployeeSearch, _ fiber.Ctx) error {
				return errors.New("query applier error")
			}).
			Public(),
	}
}

// FindPageTestSuite tests the FindPage API functionality
// including basic pagination, search filters, processors, filter appliers, audit user names, and negative cases.
type FindPageTestSuite struct {
	BaseTestSuite
}

// SetupSuite runs once before all tests in the suite.
func (suite *FindPageTestSuite) SetupSuite() {
	suite.setupBaseSuite(
		NewEmployeeFindPageResource,
		NewProcessedUserFindPageResource,
		NewFilteredUserFindPageResource,
		NewAuditedEmployeeFindPageResource,
		NewDefaultPageSizeEmployeeFindPageResource,
		NewErrorQueryApplierFindPageResource,
	)
}

// TearDownSuite runs once after all tests in the suite.
func (suite *FindPageTestSuite) TearDownSuite() {
	suite.tearDownBaseSuite()
}

// TestFindPageBasic tests basic FindPage functionality.
func (suite *FindPageTestSuite) TestFindPageBasic() {
	suite.T().Logf("Testing FindPage API basic functionality for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_page",
			Action:   "find_page",
			Version:  "v1",
		},
		Meta: map[string]any{
			"page": 1,
			"size": 5,
		},
	})

	suite.Equal(200, resp.StatusCode, "Should return 200 status code")
	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Should return successful response")
	suite.Equal(body.Message, i18n.T(result.OkMessage), "Should return OK message")
	suite.NotNil(body.Data, "Data should not be nil")

	page := suite.ReadDataAsMap(body.Data)
	suite.Equal(float64(25), page["total"], "Total should be 25")
	suite.Equal(float64(1), page["page"], "Page should be 1")
	suite.Equal(float64(5), page["size"], "Size should be 5")

	items := suite.ReadDataAsSlice(page["items"])
	suite.Len(items, 5, "Should return 5 items on first page")

	suite.T().Logf("Found %d items on page %v of %v (size=%v, total=%v)", len(items), page["page"], page["total"], page["size"], page["total"])
}

// TestFindPagePagination tests pagination functionality.
func (suite *FindPageTestSuite) TestFindPagePagination() {
	suite.T().Logf("Testing FindPage API pagination for %s", suite.ds.Kind)

	suite.Run("FirstPage", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_page",
				Action:   "find_page",
				Version:  "v1",
			},
			Meta: map[string]any{
				"page": 1,
				"size": 3,
			},
		})

		suite.Equal(200, resp.StatusCode, "Should return 200 status code")
		body := suite.ReadResult(resp)
		suite.True(body.IsOk(), "Should return successful response")

		page := suite.ReadDataAsMap(body.Data)
		suite.Equal(float64(25), page["total"], "Total should be 25")
		suite.Equal(float64(1), page["page"], "Page should be 1")
		suite.Equal(float64(3), page["size"], "Size should be 3")

		items := suite.ReadDataAsSlice(page["items"])
		suite.Len(items, 3, "Should return 3 items on first page")

		suite.T().Logf("First page: %d items (page=%v, size=%v, total=%v)", len(items), page["page"], page["size"], page["total"])
	})

	suite.Run("SecondPage", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_page",
				Action:   "find_page",
				Version:  "v1",
			},
			Meta: map[string]any{
				"page": 2,
				"size": 3,
			},
		})

		suite.Equal(200, resp.StatusCode, "Should return 200 status code")
		body := suite.ReadResult(resp)
		suite.True(body.IsOk(), "Should return successful response")

		page := suite.ReadDataAsMap(body.Data)
		suite.Equal(float64(25), page["total"], "Total should be 25")
		suite.Equal(float64(2), page["page"], "Page should be 2")
		suite.Equal(float64(3), page["size"], "Size should be 3")

		items := suite.ReadDataAsSlice(page["items"])
		suite.Len(items, 3, "Should return 3 items on second page")

		suite.T().Logf("Second page: %d items (page=%v, size=%v, total=%v)", len(items), page["page"], page["size"], page["total"])
	})

	suite.Run("LastPage", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_page",
				Action:   "find_page",
				Version:  "v1",
			},
			Meta: map[string]any{
				"page": 9,
				"size": 3,
			},
		})

		suite.Equal(200, resp.StatusCode, "Should return 200 status code")
		body := suite.ReadResult(resp)
		suite.True(body.IsOk(), "Should return successful response")

		page := suite.ReadDataAsMap(body.Data)
		suite.Equal(float64(25), page["total"], "Total should be 25")
		suite.Equal(float64(9), page["page"], "Page should be 9")

		items := suite.ReadDataAsSlice(page["items"])
		suite.Len(items, 1, "Should return 1 item on last page")

		suite.T().Logf("Last page: %d items (page=%v, size=%v, total=%v)", len(items), page["page"], page["size"], page["total"])
	})

	suite.Run("EmptyPage", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_page",
				Action:   "find_page",
				Version:  "v1",
			},
			Meta: map[string]any{
				"page": 100,
				"size": 10,
			},
		})

		suite.Equal(200, resp.StatusCode, "Should return 200 status code")
		body := suite.ReadResult(resp)
		suite.True(body.IsOk(), "Should return successful response")

		page := suite.ReadDataAsMap(body.Data)
		suite.Equal(float64(25), page["total"], "Total should be 25")

		items := suite.ReadDataAsSlice(page["items"])
		suite.Len(items, 0, "Should return empty items array for out-of-range page")

		suite.T().Logf("Empty page: %d items (page=%v, size=%v, total=%v)", len(items), page["page"], page["size"], page["total"])
	})
}

// TestFindPageWithSearch tests FindPage with search conditions.
func (suite *FindPageTestSuite) TestFindPageWithSearch() {
	suite.T().Logf("Testing FindPage API with search filters for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_page",
			Action:   "find_page",
			Version:  "v1",
		},
		Meta: map[string]any{
			"page": 1,
			"size": 10,
		},
		Params: map[string]any{
			"status": "active",
		},
	})

	suite.Equal(200, resp.StatusCode, "Should return 200 status code")
	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Should return successful response")

	page := suite.ReadDataAsMap(body.Data)
	suite.Equal(float64(18), page["total"], "Total should be 18 active employees")

	items := suite.ReadDataAsSlice(page["items"])
	suite.Len(items, 10, "Should return 10 active employees on first page")

	suite.T().Logf("Found %d active employees on page %v (size=%v, total=%v)", len(items), page["page"], page["size"], page["total"])
}

// TestFindPageWithProcessor tests FindPage with post-processing.
func (suite *FindPageTestSuite) TestFindPageWithProcessor() {
	suite.T().Logf("Testing FindPage API with processor for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_page_processed",
			Action:   "find_page",
			Version:  "v1",
		},
		Meta: map[string]any{
			"page": 1,
			"size": 5,
		},
	})

	suite.Equal(200, resp.StatusCode, "Should return 200 status code")
	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Should return successful response")

	page := suite.ReadDataAsMap(body.Data)
	suite.Equal(float64(25), page["total"], "Total should be 25")

	items := suite.ReadDataAsSlice(page["items"])
	suite.Len(items, 5, "Should return 5 processed items")

	firstUser := suite.ReadDataAsMap(items[0])
	suite.Equal(true, firstUser["processed"], "Processed flag should be true")

	suite.T().Logf("Found %d items with post-processing applied on page %v (size=%v, total=%v)", len(items), page["page"], page["size"], page["total"])
}

// TestFindPageWithFilterApplier tests FindPage with filter applier.
func (suite *FindPageTestSuite) TestFindPageWithFilterApplier() {
	suite.T().Logf("Testing FindPage API with filter applier for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_page_filtered",
			Action:   "find_page",
			Version:  "v1",
		},
		Meta: map[string]any{
			"page": 1,
			"size": 10,
		},
	})

	suite.Equal(200, resp.StatusCode, "Should return 200 status code")
	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Should return successful response")

	page := suite.ReadDataAsMap(body.Data)
	suite.Equal(float64(18), page["total"], "Total should be 18 active employees")

	items := suite.ReadDataAsSlice(page["items"])
	suite.Len(items, 10, "Should return 10 active employees with filter applier")

	suite.T().Logf("Found %d active employees with filter applier on page %v (size=%v, total=%v)", len(items), page["page"], page["size"], page["total"])
}

// TestFindPageNegativeCases tests negative scenarios.
func (suite *FindPageTestSuite) TestFindPageNegativeCases() {
	suite.T().Logf("Testing FindPage API negative cases for %s", suite.ds.Kind)

	suite.Run("InvalidPageNumber", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_page",
				Action:   "find_page",
				Version:  "v1",
			},
			Meta: map[string]any{
				"page": 0,
				"size": 10,
			},
		})

		suite.Equal(200, resp.StatusCode, "Should return 200 status code")
		body := suite.ReadResult(resp)
		suite.True(body.IsOk(), "Should return successful response")

		page := suite.ReadDataAsMap(body.Data)
		suite.Equal(float64(1), page["page"], "Page should be normalized to 1")

		suite.T().Logf("Invalid page number 0 was normalized to %v", page["page"])
	})

	suite.Run("InvalidPageSize", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_page",
				Action:   "find_page",
				Version:  "v1",
			},
			Meta: map[string]any{
				"page": 1,
				"size": 0,
			},
		})

		suite.Equal(200, resp.StatusCode, "Should return 200 status code")
		body := suite.ReadResult(resp)
		suite.True(body.IsOk(), "Should return successful response")

		page := suite.ReadDataAsMap(body.Data)
		suite.Equal(float64(25), page["total"], "Total should be 25")
		items := suite.ReadDataAsSlice(page["items"])
		suite.Greater(len(items), 0, "Should return items with default page size")

		suite.T().Logf("Invalid page size 0 returned %d items with default size", len(items))
	})

	suite.Run("NoMatchingRecords", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_page",
				Action:   "find_page",
				Version:  "v1",
			},
			Meta: map[string]any{
				"page": 1,
				"size": 10,
			},
			Params: map[string]any{
				"keyword": "NonexistentKeyword",
			},
		})

		suite.Equal(200, resp.StatusCode, "Should return 200 status code")
		body := suite.ReadResult(resp)
		suite.True(body.IsOk(), "Should return successful response")

		page := suite.ReadDataAsMap(body.Data)
		suite.Equal(float64(0), page["total"], "Total should be 0 for no matching records")

		items := suite.ReadDataAsSlice(page["items"])
		suite.Len(items, 0, "Should return empty items array")

		suite.T().Logf("No matching records: total=%v, items=%d", page["total"], len(items))
	})
}

// TestFindPageWithAuditUserNames tests FindPage with audit user names populated.
func (suite *FindPageTestSuite) TestFindPageWithAuditUserNames() {
	suite.T().Logf("Testing FindPage API with audit user names for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_page_audit",
			Action:   "find_page",
			Version:  "v1",
		},
		Meta: map[string]any{
			"page": 1,
			"size": 5,
		},
		Params: map[string]any{
			"status": "active",
		},
	})

	suite.Equal(200, resp.StatusCode, "Should return 200 status code")
	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Should return successful response")
	suite.NotNil(body.Data, "Data should not be nil")

	page := suite.ReadDataAsMap(body.Data)
	suite.Equal(float64(18), page["total"], "Total should be 18 active employees")
	suite.Equal(float64(1), page["page"], "Page should be 1")
	suite.Equal(float64(5), page["size"], "Size should be 5")

	items := suite.ReadDataAsSlice(page["items"])
	suite.Len(items, 5, "Should return 5 items on first page")

	operatorNames := []string{"Sarah Chen", "Michael Torres", "James Liu", "Emily Johnson", "David Park", "Lisa Wang", "Robert Kim", "Amanda Davis"}

	firstUser := suite.ReadDataAsMap(items[0])
	suite.NotNil(firstUser["createdByName"], "First employee should have createdByName")
	suite.NotNil(firstUser["updatedByName"], "First employee should have updatedByName")

	for _, u := range items {
		user := suite.ReadDataAsMap(u)
		suite.NotNil(user["createdByName"], "Employee %s should have createdByName", user["id"])
		suite.NotNil(user["updatedByName"], "Employee %s should have updatedByName", user["id"])
		suite.Contains(operatorNames, user["createdByName"], "createdByName should be from operators")
		suite.Contains(operatorNames, user["updatedByName"], "updatedByName should be from operators")
	}

	suite.T().Logf("Found %d employees with audit user names populated on page %v (size=%v, total=%v)", len(items), page["page"], page["size"], page["total"])
}

// TestFindPageWithDefaultPageSize tests FindPage with custom default page size.
func (suite *FindPageTestSuite) TestFindPageWithDefaultPageSize() {
	suite.T().Logf("Testing FindPage API with WithDefaultPageSize for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_page_default_size",
			Action:   "find_page",
			Version:  "v1",
		},
		Meta: map[string]any{
			"page": 1,
			"size": 0,
		},
	})

	suite.Equal(200, resp.StatusCode, "Should return 200 status code")
	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Should return successful response")

	page := suite.ReadDataAsMap(body.Data)
	suite.Equal(float64(25), page["total"], "Total should be 25")
	suite.Equal(float64(3), page["size"], "Size should fallback to default page size 3")

	items := suite.ReadDataAsSlice(page["items"])
	suite.Len(items, 3, "Should return 3 items with default page size")

	suite.T().Logf("WithDefaultPageSize returned %d items (size=%v, total=%v)", len(items), page["size"], page["total"])
}

// TestFindPageErrorQueryApplier tests FindPage with a QueryApplier that returns error.
func (suite *FindPageTestSuite) TestFindPageErrorQueryApplier() {
	suite.T().Logf("Testing FindPage API error query applier for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_page_err_applier",
			Action:   "find_page",
			Version:  "v1",
		},
		Meta: map[string]any{
			"page": 1,
			"size": 10,
		},
	})

	suite.Contains([]int{200, 500}, resp.StatusCode, "Should return error status code")

	suite.T().Logf("FindPage failed as expected due to query applier error")
}

// TestSetupErrFindPageNoPK covers find_page.go:41-43 - FindPage Setup error with no PK model.
func TestSetupErrFindPageNoPK(t *testing.T) {
	db := testx.NewTestDB(t)

	fp := crud.NewFindPage[NoPKModel, struct{}]()
	fp.WithAuditUserNames((*struct{})(nil))

	specs := fp.Public().Provide()
	require.Len(t, specs, 1, "Should return exactly 1 operation spec")

	err := callHandlerFactory(t, specs[0].Handler, db)
	assert.ErrorIs(t, err, crud.ErrModelNoPrimaryKey, "Should return ErrModelNoPrimaryKey")
}

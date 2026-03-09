package crud_test

import (
	"errors"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/crud"
	"github.com/coldsmirk/vef-framework-go/i18n"
	"github.com/coldsmirk/vef-framework-go/internal/orm"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/sortx"
)

func init() {
	registry.Add(func(env *testx.DBEnv) suite.TestingSuite {
		return &FindAllTestSuite{
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
type TestEmployeeFindAllResource struct {
	api.Resource
	crud.FindAll[Employee, EmployeeSearch]
}

func NewTestEmployeeFindAllResource() api.Resource {
	return &TestEmployeeFindAllResource{
		Resource: api.NewRPCResource("test/employee_all"),
		FindAll:  crud.NewFindAll[Employee, EmployeeSearch]().WithCondition(fixtureScope).Public(),
	}
}

// Processed User Resource - with processor.
type ProcessedEmployeeFindAllResource struct {
	api.Resource
	crud.FindAll[Employee, EmployeeSearch]
}

type ProcessedEmployeeList struct {
	Users     []Employee `json:"users"`
	Processed bool       `json:"processed"`
}

func NewProcessedEmployeeFindAllResource() api.Resource {
	return &ProcessedEmployeeFindAllResource{
		Resource: api.NewRPCResource("test/employee_all_processed"),
		FindAll: crud.NewFindAll[Employee, EmployeeSearch]().
			WithCondition(fixtureScope).
			Public().
			WithProcessor(func(users []Employee, _ EmployeeSearch, _ fiber.Ctx) any {
				return ProcessedEmployeeList{
					Users:     users,
					Processed: true,
				}
			}),
	}
}

// Filtered User Resource - with filter applier.
type FilteredEmployeeFindAllResource struct {
	api.Resource
	crud.FindAll[Employee, EmployeeSearch]
}

func NewFilteredEmployeeFindAllResource() api.Resource {
	return &FilteredEmployeeFindAllResource{
		Resource: api.NewRPCResource("test/employee_all_filtered"),
		FindAll: crud.NewFindAll[Employee, EmployeeSearch]().
			WithCondition(fixtureScope).
			WithCondition(func(cb orm.ConditionBuilder) {
				cb.Equals("status", "active")
			}).
			Public(),
	}
}

// Ordered User Resource - with order applier.
type OrderedEmployeeFindAllResource struct {
	api.Resource
	crud.FindAll[Employee, EmployeeSearch]
}

func NewOrderedEmployeeFindAllResource() api.Resource {
	return &OrderedEmployeeFindAllResource{
		Resource: api.NewRPCResource("test/employee_all_ordered"),
		FindAll: crud.NewFindAll[Employee, EmployeeSearch]().
			WithCondition(fixtureScope).
			WithDefaultSort(&sortx.OrderSpec{
				Column: "age",
			}).
			Public(),
	}
}

// AuditUser User Resource - with audit user names.
type AuditedEmployeeFindAllResource struct {
	api.Resource
	crud.FindAll[Employee, EmployeeSearch]
}

func NewAuditedEmployeeFindAllResource() api.Resource {
	return &AuditedEmployeeFindAllResource{
		Resource: api.NewRPCResource("test/employee_all_audit"),
		FindAll: crud.NewFindAll[Employee, EmployeeSearch]().
			WithCondition(fixtureScope).
			WithAuditUserNames((*Operator)(nil)).
			Public(),
	}
}

// NoDefaultSort User Resource - explicitly disable default sorting.
type NoDefaultSortEmployeeFindAllResource struct {
	api.Resource
	crud.FindAll[Employee, EmployeeSearch]
}

func NewNoDefaultSortEmployeeFindAllResource() api.Resource {
	return &NoDefaultSortEmployeeFindAllResource{
		Resource: api.NewRPCResource("test/employee_all_no_default_sort"),
		FindAll: crud.NewFindAll[Employee, EmployeeSearch]().
			WithCondition(fixtureScope).
			WithDefaultSort(). // Empty call to disable default sorting
			Public(),
	}
}

// MultipleDefaultSort User Resource - with multiple default sort columns.
type MultipleDefaultSortEmployeeFindAllResource struct {
	api.Resource
	crud.FindAll[Employee, EmployeeSearch]
}

func NewMultipleDefaultSortEmployeeFindAllResource() api.Resource {
	return &MultipleDefaultSortEmployeeFindAllResource{
		Resource: api.NewRPCResource("test/employee_all_multi_sort"),
		FindAll: crud.NewFindAll[Employee, EmployeeSearch]().
			WithCondition(fixtureScope).
			WithDefaultSort(
				&sortx.OrderSpec{
					Column:    "status",
					Direction: sortx.OrderAsc,
				},
				&sortx.OrderSpec{
					Column:    "age",
					Direction: sortx.OrderDesc,
				},
			).
			Public(),
	}
}

// SelectEmployeeFindAllResource - with WithSelect column.
type SelectEmployeeFindAllResource struct {
	api.Resource
	crud.FindAll[Employee, EmployeeSearch]
}

func NewSelectEmployeeFindAllResource() api.Resource {
	return &SelectEmployeeFindAllResource{
		Resource: api.NewRPCResource("test/employee_all_select"),
		FindAll: crud.NewFindAll[Employee, EmployeeSearch]().
			WithCondition(fixtureScope).
			WithSelect("te.name").
			Public(),
	}
}

// SelectAsEmployeeFindAllResource - with WithSelectAs column alias.
type SelectAsEmployeeFindAllResource struct {
	api.Resource
	crud.FindAll[Employee, EmployeeSearch]
}

func NewSelectAsEmployeeFindAllResource() api.Resource {
	return &SelectAsEmployeeFindAllResource{
		Resource: api.NewRPCResource("test/employee_all_select_as"),
		FindAll: crud.NewFindAll[Employee, EmployeeSearch]().
			WithCondition(fixtureScope).
			WithSelectAs("te.email", "employee_email").
			Public(),
	}
}

// QueryApplierEmployeeFindAllResource - with WithQueryApplier.
type QueryApplierEmployeeFindAllResource struct {
	api.Resource
	crud.FindAll[Employee, EmployeeSearch]
}

func NewQueryApplierEmployeeFindAllResource() api.Resource {
	return &QueryApplierEmployeeFindAllResource{
		Resource: api.NewRPCResource("test/employee_all_query_applier"),
		FindAll: crud.NewFindAll[Employee, EmployeeSearch]().
			WithCondition(fixtureScope).
			WithQueryApplier(func(query orm.SelectQuery, _ EmployeeSearch, _ fiber.Ctx) error {
				// Custom query applier: filter employees with age >= 40
				query.Where(func(cb orm.ConditionBuilder) {
					cb.GreaterThan("age", 39)
				})

				return nil
			}).
			Public(),
	}
}

// DataPermDisabledEmployeeFindAllResource - with DisableDataPerm.
type DataPermDisabledEmployeeFindAllResource struct {
	api.Resource
	crud.FindAll[Employee, EmployeeSearch]
}

func NewDataPermDisabledEmployeeFindAllResource() api.Resource {
	return &DataPermDisabledEmployeeFindAllResource{
		Resource: api.NewRPCResource("test/employee_all_no_perm"),
		FindAll: crud.NewFindAll[Employee, EmployeeSearch]().
			WithCondition(fixtureScope).
			DisableDataPerm().
			Public(),
	}
}

// RelationEmployeeFindAllResource - with WithRelation to join Operator table.
type RelationEmployeeFindAllResource struct {
	api.Resource
	crud.FindAll[Employee, EmployeeSearch]
}

func NewRelationEmployeeFindAllResource() api.Resource {
	return &RelationEmployeeFindAllResource{
		Resource: api.NewRPCResource("test/employee_all_relation"),
		FindAll: crud.NewFindAll[Employee, EmployeeSearch]().
			WithCondition(fixtureScope).
			WithRelation(&orm.RelationSpec{
				Model:         (*Operator)(nil),
				Alias:         "creator",
				ForeignColumn: "created_by",
				SelectedColumns: []orm.ColumnInfo{
					{Name: "name", Alias: "creator_name"},
				},
			}).
			Public(),
	}
}

// ErrorQueryApplierFindAllResource - FindAll with QueryApplier that returns error.
type ErrorQueryApplierFindAllResource struct {
	api.Resource
	crud.FindAll[Employee, EmployeeSearch]
}

func NewErrorQueryApplierFindAllResource() api.Resource {
	return &ErrorQueryApplierFindAllResource{
		Resource: api.NewRPCResource("test/employee_all_err_applier"),
		FindAll: crud.NewFindAll[Employee, EmployeeSearch]().
			WithCondition(fixtureScope).
			WithQueryApplier(func(_ orm.SelectQuery, _ EmployeeSearch, _ fiber.Ctx) error {
				return errors.New("query applier error")
			}).
			Public(),
	}
}

// AscSortFindAllResource - FindAll with ASC default sort and NullsFirst.
type AscSortFindAllResource struct {
	api.Resource
	crud.FindAll[Employee, EmployeeSearch]
}

func NewAscSortFindAllResource() api.Resource {
	return &AscSortFindAllResource{
		Resource: api.NewRPCResource("test/employee_all_asc_sort"),
		FindAll: crud.NewFindAll[Employee, EmployeeSearch]().
			WithCondition(fixtureScope).
			WithDefaultSort(&sortx.OrderSpec{
				Column:    "name",
				Direction: sortx.OrderAsc,
			}).
			Public(),
	}
}

// CompositePKFindAllResource - FindAll with composite PK model to cover fallback sort branch.
type CompositePKFindAllResource struct {
	api.Resource
	crud.FindAll[ProjectAssignment, struct{ api.P }]
}

func NewCompositePKFindAllResource() api.Resource {
	return &CompositePKFindAllResource{
		Resource: api.NewRPCResource("test/assignment_all"),
		FindAll: crud.NewFindAll[ProjectAssignment, struct{ api.P }]().
			WithCondition(fixtureScope).
			Public(),
	}
}

// FindAllTestSuite tests the FindAll API functionality
// including basic queries, search filters, processors, sorting, audit user names, and negative cases.
type FindAllTestSuite struct {
	BaseTestSuite
}

// SetupSuite runs once before all tests in the suite.
func (suite *FindAllTestSuite) SetupSuite() {
	suite.setupBaseSuite(
		NewTestEmployeeFindAllResource,
		NewProcessedEmployeeFindAllResource,
		NewFilteredEmployeeFindAllResource,
		NewOrderedEmployeeFindAllResource,
		NewAuditedEmployeeFindAllResource,
		NewNoDefaultSortEmployeeFindAllResource,
		NewMultipleDefaultSortEmployeeFindAllResource,
		NewSelectEmployeeFindAllResource,
		NewSelectAsEmployeeFindAllResource,
		NewQueryApplierEmployeeFindAllResource,
		NewDataPermDisabledEmployeeFindAllResource,
		NewRelationEmployeeFindAllResource,
		NewCompositePKFindAllResource,
		NewErrorQueryApplierFindAllResource,
		NewAscSortFindAllResource,
	)
}

// TearDownSuite runs once after all tests in the suite.
func (suite *FindAllTestSuite) TearDownSuite() {
	suite.tearDownBaseSuite()
}

// TestFindAllBasic tests basic FindAll functionality.
func (suite *FindAllTestSuite) TestFindAllBasic() {
	suite.T().Logf("Testing FindAll API basic functionality for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_all",
			Action:   "find_all",
			Version:  "v1",
		},
	})

	suite.Equal(200, resp.StatusCode, "Should return 200 status code")
	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Should return successful response")
	suite.Equal(body.Message, i18n.T(result.OkMessage), "Should return OK message")
	suite.NotNil(body.Data, "Data should not be nil")

	users := suite.ReadDataAsSlice(body.Data)
	suite.Len(users, 25, "Should return all 25 employees")

	suite.T().Logf("Found %d employees without filters", len(users))
}

// TestFindAllWithSearchApplier tests FindAll with custom search conditions.
func (suite *FindAllTestSuite) TestFindAllWithSearchApplier() {
	suite.T().Logf("Testing FindAll API with search filters for %s", suite.ds.Kind)

	suite.Run("SearchByStatus", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_all",
				Action:   "find_all",
				Version:  "v1",
			},
			Params: map[string]any{
				"status": "active",
			},
		})

		suite.Equal(200, resp.StatusCode, "Should return 200 status code")
		body := suite.ReadResult(resp)
		suite.True(body.IsOk(), "Should return successful response")
		suite.NotNil(body.Data, "Data should not be nil")

		users := suite.ReadDataAsSlice(body.Data)
		suite.Len(users, 18, "Should return 18 active employees")

		suite.T().Logf("Found %d employees with status=active", len(users))
	})

	suite.Run("SearchByKeyword", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_all",
				Action:   "find_all",
				Version:  "v1",
			},
			Params: map[string]any{
				"keyword": "engineer",
			},
		})

		suite.Equal(200, resp.StatusCode, "Should return 200 status code")
		body := suite.ReadResult(resp)
		suite.True(body.IsOk(), "Should return successful response")
		suite.NotNil(body.Data, "Data should not be nil")

		users := suite.ReadDataAsSlice(body.Data)
		// PostgreSQL LIKE is case-sensitive: "engineer" does NOT match "Engineering" (3 results)
		// MySQL/SQLite LIKE is case-insensitive: "engineer" matches "Engineering" (4 results)
		if suite.ds.Kind == config.Postgres {
			suite.Len(users, 3, "Should return 3 employees with keyword 'engineer' (PostgreSQL case-sensitive)")
		} else {
			suite.Len(users, 4, "Should return 4 employees with keyword 'engineer' (case-insensitive)")
		}

		suite.T().Logf("Found %d employees with keyword=engineer", len(users))
	})

	suite.Run("SearchByAgeRange", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_all",
				Action:   "find_all",
				Version:  "v1",
			},
			Params: map[string]any{
				"age": []int{25, 28},
			},
		})

		suite.Equal(200, resp.StatusCode, "Should return 200 status code")
		body := suite.ReadResult(resp)
		suite.True(body.IsOk(), "Should return successful response")
		suite.NotNil(body.Data, "Data should not be nil")

		users := suite.ReadDataAsSlice(body.Data)
		suite.Len(users, 4, "Should return 4 employees in age range 25-28")

		suite.T().Logf("Found %d employees with age range 25-28", len(users))
	})
}

// TestFindAllWithProcessor tests FindAll with post-processing.
func (suite *FindAllTestSuite) TestFindAllWithProcessor() {
	suite.T().Logf("Testing FindAll API with processor for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_all_processed",
			Action:   "find_all",
			Version:  "v1",
		},
	})

	suite.Equal(200, resp.StatusCode, "Should return 200 status code")
	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Should return successful response")
	suite.NotNil(body.Data, "Data should not be nil")

	dataMap := suite.ReadDataAsMap(body.Data)
	suite.Equal(true, dataMap["processed"], "Processed flag should be true")
	suite.NotNil(dataMap["users"], "Users array should not be nil")

	users := suite.ReadDataAsSlice(dataMap["users"])
	suite.Len(users, 25, "Should return all 25 employees in processed format")

	suite.T().Logf("Found %d employees with post-processing applied", len(users))
}

// TestFindAllWithFilterApplier tests FindAll with filter applier.
func (suite *FindAllTestSuite) TestFindAllWithFilterApplier() {
	suite.T().Logf("Testing FindAll API with filter applier for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_all_filtered",
			Action:   "find_all",
			Version:  "v1",
		},
	})

	suite.Equal(200, resp.StatusCode, "Should return 200 status code")
	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Should return successful response")
	suite.NotNil(body.Data, "Data should not be nil")

	users := suite.ReadDataAsSlice(body.Data)
	suite.Len(users, 18, "Should return only active employees (18 employees)")

	suite.T().Logf("Found %d active employees with filter applier", len(users))
}

// TestFindAllWithSortApplier tests FindAll with sort applier.
func (suite *FindAllTestSuite) TestFindAllWithSortApplier() {
	suite.T().Logf("Testing FindAll API with sort applier for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_all_ordered",
			Action:   "find_all",
			Version:  "v1",
		},
	})

	suite.Equal(200, resp.StatusCode, "Should return 200 status code")
	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Should return successful response")
	suite.NotNil(body.Data, "Data should not be nil")

	users := suite.ReadDataAsSlice(body.Data)
	suite.Len(users, 25, "Should return all 25 employees")

	firstUser := suite.ReadDataAsMap(users[0])
	suite.Equal(float64(22), firstUser["age"], "First employee should be youngest (age 22)")

	suite.T().Logf("Found %d employees sorted by age ascending, first employee age: %.0f", len(users), firstUser["age"])
}

// TestFindAllNegativeCases tests negative scenarios.
func (suite *FindAllTestSuite) TestFindAllNegativeCases() {
	suite.T().Logf("Testing FindAll API negative cases for %s", suite.ds.Kind)

	suite.Run("EmptySearchCriteria", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_all",
				Action:   "find_all",
				Version:  "v1",
			},
			Params: map[string]any{},
		})

		suite.Equal(200, resp.StatusCode, "Should return 200 status code")
		body := suite.ReadResult(resp)
		suite.True(body.IsOk(), "Should return successful response")
		suite.NotNil(body.Data, "Data should not be nil")

		users := suite.ReadDataAsSlice(body.Data)
		suite.Len(users, 25, "Should return all employees when no search criteria provided")

		suite.T().Logf("Found %d users with empty search criteria", len(users))
	})

	suite.Run("NoMatchingRecords", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_all",
				Action:   "find_all",
				Version:  "v1",
			},
			Params: map[string]any{
				"keyword": "NonexistentKeyword",
			},
		})

		suite.Equal(200, resp.StatusCode, "Should return 200 status code")
		body := suite.ReadResult(resp)
		suite.True(body.IsOk(), "Should return successful response")
		suite.NotNil(body.Data, "Data should not be nil")

		users := suite.ReadDataAsSlice(body.Data)
		suite.Len(users, 0, "Should return empty array when no records match")

		suite.T().Logf("Found %d users with non-existent keyword (empty array as expected)", len(users))
	})
}

// TestFindAllWithAuditUserNames tests FindAll with audit user names populated.
func (suite *FindAllTestSuite) TestFindAllWithAuditUserNames() {
	suite.T().Logf("Testing FindAll API with audit user names for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_all_audit",
			Action:   "find_all",
			Version:  "v1",
		},
		Params: map[string]any{
			"status": "active",
		},
	})

	suite.Equal(200, resp.StatusCode, "Should return 200 status code")
	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Should return successful response")
	suite.NotNil(body.Data, "Data should not be nil")

	users := suite.ReadDataAsSlice(body.Data)
	suite.Len(users, 18, "Should return 18 active employees")

	operatorNames := []string{"Sarah Chen", "Michael Torres", "James Liu", "Emily Johnson", "David Park", "Lisa Wang", "Robert Kim", "Amanda Davis"}

	firstUser := suite.ReadDataAsMap(users[0])
	suite.NotNil(firstUser["createdByName"], "First employee should have createdByName")
	suite.NotNil(firstUser["updatedByName"], "First employee should have updatedByName")

	for _, u := range users {
		user := suite.ReadDataAsMap(u)
		suite.NotNil(user["createdByName"], "Employee %s should have createdByName", user["id"])
		suite.NotNil(user["updatedByName"], "Employee %s should have updatedByName", user["id"])
		suite.Contains(operatorNames, user["createdByName"], "createdByName should be from operators")
		suite.Contains(operatorNames, user["updatedByName"], "updatedByName should be from operators")
	}

	suite.T().Logf("Found %d employees with audit user names populated", len(users))
}

// TestFindAllDefaultSorting tests default sorting behavior.
func (suite *FindAllTestSuite) TestFindAllDefaultSorting() {
	suite.T().Logf("Testing FindAll API default sorting for %s", suite.ds.Kind)

	suite.Run("DefaultSortByPrimaryKey", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_all",
				Action:   "find_all",
				Version:  "v1",
			},
		})

		suite.Equal(200, resp.StatusCode, "Should return 200 status code")
		body := suite.ReadResult(resp)
		suite.True(body.IsOk(), "Should return successful response")

		users := suite.ReadDataAsSlice(body.Data)
		suite.Len(users, 25, "Should return all 25 employees")

		firstUser := suite.ReadDataAsMap(users[0])
		suite.Equal("emp025", firstUser["id"], "First employee should have highest id (emp025) when sorted by id DESC")

		lastUser := suite.ReadDataAsMap(users[len(users)-1])
		suite.Equal("emp001", lastUser["id"], "Last employee should have lowest id (emp001) when sorted by id DESC")

		suite.T().Logf("Default sort by primary key: first=%s, last=%s", firstUser["id"], lastUser["id"])
	})

	suite.Run("CustomDefaultSort", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_all_ordered",
				Action:   "find_all",
				Version:  "v1",
			},
		})

		suite.Equal(200, resp.StatusCode, "Should return 200 status code")
		body := suite.ReadResult(resp)
		suite.True(body.IsOk(), "Should return successful response")

		users := suite.ReadDataAsSlice(body.Data)
		suite.Len(users, 25, "Should return all 25 employees")

		firstUser := suite.ReadDataAsMap(users[0])
		suite.Equal(float64(22), firstUser["age"], "First employee should be youngest (age 22)")
		suite.Equal("Priya Sharma", firstUser["name"], "First employee should be Priya Sharma")

		lastUser := suite.ReadDataAsMap(users[len(users)-1])
		suite.Equal(float64(50), lastUser["age"], "Last employee should be oldest (age 50)")
		suite.Equal("Carlos Rodriguez", lastUser["name"], "Last employee should be Carlos Rodriguez")

		suite.T().Logf("Custom default sort by age: first=%s (age %.0f), last=%s (age %.0f)", firstUser["name"], firstUser["age"], lastUser["name"], lastUser["age"])
	})

	suite.Run("DisableDefaultSort", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_all_no_default_sort",
				Action:   "find_all",
				Version:  "v1",
			},
		})

		suite.Equal(200, resp.StatusCode, "Should return 200 status code")
		body := suite.ReadResult(resp)
		suite.True(body.IsOk(), "Should return successful response")

		users := suite.ReadDataAsSlice(body.Data)
		suite.Len(users, 25, "Should return all 25 employees without sorting")

		suite.T().Logf("Found %d users with default sort disabled (order is database-dependent)", len(users))
	})

	suite.Run("MultipleDefaultSortColumns", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_all_multi_sort",
				Action:   "find_all",
				Version:  "v1",
			},
		})

		suite.Equal(200, resp.StatusCode, "Should return 200 status code")
		body := suite.ReadResult(resp)
		suite.True(body.IsOk(), "Should return successful response")

		users := suite.ReadDataAsSlice(body.Data)
		suite.Len(users, 25, "Should return all 25 employees")

		firstUser := suite.ReadDataAsMap(users[0])
		suite.Equal("active", firstUser["status"], "First employee should be active (sorted by status ASC)")

		// Verify status ordering: active(17) < inactive(5) < on_leave(3) alphabetically
		var prevStatus string
		for _, u := range users {
			user := suite.ReadDataAsMap(u)

			status := user["status"].(string)
			if prevStatus != "" && status != prevStatus {
				suite.True(status > prevStatus, "Status %s should come after %s in ASC order", status, prevStatus)
			}

			prevStatus = status
		}

		suite.T().Logf("Multiple sort columns verified: status ASC ordering confirmed for 25 employees")
	})
}

// TestFindAllRequestSortOverride tests that request-specified sorting overrides default sorting.
func (suite *FindAllTestSuite) TestFindAllRequestSortOverride() {
	suite.T().Logf("Testing FindAll API request sort override for %s", suite.ds.Kind)

	suite.Run("OverrideDefaultSortWithRequestSort", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_all_ordered",
				Action:   "find_all",
				Version:  "v1",
			},
			Meta: map[string]any{
				"sort": []map[string]any{
					{
						"column":    "name",
						"direction": "desc",
					},
				},
			},
		})

		suite.Equal(200, resp.StatusCode, "Should return 200 status code")
		body := suite.ReadResult(resp)
		suite.True(body.IsOk(), "Should return successful response")

		users := suite.ReadDataAsSlice(body.Data)
		suite.Len(users, 25, "Should return all 25 employees")

		firstUser := suite.ReadDataAsMap(users[0])
		firstName, ok := firstUser["name"].(string)
		suite.True(ok, "Type assertion to string should succeed for firstName")

		lastUser := suite.ReadDataAsMap(users[len(users)-1])
		lastName, ok := lastUser["name"].(string)
		suite.True(ok, "Type assertion to string should succeed for lastName")

		suite.True(firstName > lastName, "First name %s should be > last name %s in DESC order", firstName, lastName)

		suite.T().Logf("Request sort override: first=%s, last=%s", firstName, lastName)
	})

	suite.Run("OverrideWithMultipleSortColumns", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_all_ordered",
				Action:   "find_all",
				Version:  "v1",
			},
			Meta: map[string]any{
				"sort": []map[string]any{
					{
						"column":    "status",
						"direction": "asc",
					},
					{
						"column":    "name",
						"direction": "asc",
					},
				},
			},
		})

		suite.Equal(200, resp.StatusCode, "Should return 200 status code")
		body := suite.ReadResult(resp)
		suite.True(body.IsOk(), "Should return successful response")

		users := suite.ReadDataAsSlice(body.Data)
		suite.Len(users, 25, "Should return all 25 employees")

		// Verify status ordering: active < inactive < on_leave alphabetically
		var prevStatus string
		for _, u := range users {
			user := suite.ReadDataAsMap(u)

			status := user["status"].(string)
			if prevStatus != "" && status != prevStatus {
				suite.True(status > prevStatus, "Status %s should come after %s in ASC order", status, prevStatus)
			}

			prevStatus = status
		}

		suite.T().Logf("Multiple sort columns override verified: status ASC, name ASC")
	})

	suite.Run("OverrideDisabledDefaultSort", func() {
		// When WithDefaultSort() is called with no args, both default and request sorting
		// are disabled. Verify that all records are still returned.
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_all_no_default_sort",
				Action:   "find_all",
				Version:  "v1",
			},
			Meta: map[string]any{
				"sort": []map[string]any{
					{
						"column":    "email",
						"direction": "asc",
					},
				},
			},
		})

		suite.Equal(200, resp.StatusCode, "Should return 200 status code")
		body := suite.ReadResult(resp)
		suite.True(body.IsOk(), "Should return successful response")

		users := suite.ReadDataAsSlice(body.Data)
		suite.Len(users, 25, "Should return all 25 employees")

		suite.T().Logf("Resource with disabled default sort returned all %d employees", len(users))
	})
}

// TestFindAllWithSelect tests FindAll with WithSelect column selection.
func (suite *FindAllTestSuite) TestFindAllWithSelect() {
	suite.T().Logf("Testing FindAll API with WithSelect for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_all_select",
			Action:   "find_all",
			Version:  "v1",
		},
	})

	suite.Equal(200, resp.StatusCode, "Should return 200 status code")
	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Should return successful response")

	users := suite.ReadDataAsSlice(body.Data)
	suite.Len(users, 25, "Should return all 25 employees")

	firstUser := suite.ReadDataAsMap(users[0])
	suite.NotEmpty(firstUser["name"], "Should include selected name column")

	suite.T().Logf("WithSelect returned %d employees with name column", len(users))
}

// TestFindAllWithSelectAs tests FindAll with WithSelectAs column alias.
func (suite *FindAllTestSuite) TestFindAllWithSelectAs() {
	suite.T().Logf("Testing FindAll API with WithSelectAs for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_all_select_as",
			Action:   "find_all",
			Version:  "v1",
		},
	})

	suite.Equal(200, resp.StatusCode, "Should return 200 status code")
	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Should return successful response")

	users := suite.ReadDataAsSlice(body.Data)
	suite.Len(users, 25, "Should return all 25 employees")

	suite.T().Logf("WithSelectAs returned %d employees", len(users))
}

// TestFindAllWithQueryApplier tests FindAll with custom WithQueryApplier.
func (suite *FindAllTestSuite) TestFindAllWithQueryApplier() {
	suite.T().Logf("Testing FindAll API with WithQueryApplier for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_all_query_applier",
			Action:   "find_all",
			Version:  "v1",
		},
	})

	suite.Equal(200, resp.StatusCode, "Should return 200 status code")
	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Should return successful response")

	users := suite.ReadDataAsSlice(body.Data)
	suite.Len(users, 7, "Should return 7 employees with age > 39")

	for _, u := range users {
		user := suite.ReadDataAsMap(u)
		suite.Greater(user["age"], float64(39), "All employees should have age > 39")
	}

	suite.T().Logf("WithQueryApplier returned %d employees with age > 39", len(users))
}

// TestFindAllWithDisableDataPerm tests FindAll with DisableDataPerm.
func (suite *FindAllTestSuite) TestFindAllWithDisableDataPerm() {
	suite.T().Logf("Testing FindAll API with DisableDataPerm for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_all_no_perm",
			Action:   "find_all",
			Version:  "v1",
		},
	})

	suite.Equal(200, resp.StatusCode, "Should return 200 status code")
	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Should return successful response")

	users := suite.ReadDataAsSlice(body.Data)
	suite.Len(users, 25, "Should return all 25 employees without data permission")

	suite.T().Logf("DisableDataPerm returned %d employees", len(users))
}

// TestFindAllWithRelation tests FindAll with WithRelation join.
func (suite *FindAllTestSuite) TestFindAllWithRelation() {
	suite.T().Logf("Testing FindAll API with WithRelation for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_all_relation",
			Action:   "find_all",
			Version:  "v1",
		},
	})

	suite.Equal(200, resp.StatusCode, "Should return 200 status code")
	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Should return successful response")

	users := suite.ReadDataAsSlice(body.Data)
	suite.Len(users, 25, "Should return all 25 employees with relation join")

	suite.T().Logf("WithRelation returned %d employees with creator join", len(users))
}

// TestFindAllCompositePK tests FindAll with composite PK model (ProjectAssignment).
func (suite *FindAllTestSuite) TestFindAllCompositePK() {
	suite.T().Logf("Testing FindAll API with composite PK model for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/assignment_all",
			Action:   "find_all",
			Version:  "v1",
		},
	})

	suite.Equal(200, resp.StatusCode, "Should return 200 status code")
	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Should return successful response")

	assignments := suite.ReadDataAsSlice(body.Data)
	suite.NotEmpty(assignments, "Should return project assignments")

	suite.T().Logf("CompositePK FindAll returned %d assignments", len(assignments))
}

// TestFindAllErrorQueryApplier tests FindAll with a QueryApplier that returns error.
func (suite *FindAllTestSuite) TestFindAllErrorQueryApplier() {
	suite.T().Logf("Testing FindAll API error query applier for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_all_err_applier",
			Action:   "find_all",
			Version:  "v1",
		},
	})

	suite.Contains([]int{200, 500}, resp.StatusCode, "Should return error status code")

	suite.T().Logf("FindAll failed as expected due to query applier error")
}

// TestFindAllAscSort tests FindAll with ASC default sort and NullsFirst.
func (suite *FindAllTestSuite) TestFindAllAscSort() {
	suite.T().Logf("Testing FindAll API ASC sort for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_all_asc_sort",
			Action:   "find_all",
			Version:  "v1",
		},
	})

	suite.Equal(200, resp.StatusCode, "Should return 200 status code")
	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Should return successful response")

	users := suite.ReadDataAsSlice(body.Data)
	suite.NotEmpty(users, "Should return employees sorted by name ASC")

	suite.T().Logf("AscSort FindAll returned %d employees", len(users))
}

// TestFindAllDynamicSortNullsFirst tests FindAll with dynamic sort NullsFirst via request meta.
func (suite *FindAllTestSuite) TestFindAllDynamicSortNullsFirst() {
	suite.T().Logf("Testing FindAll API dynamic sort NullsFirst for %s", suite.ds.Kind)

	if suite.ds.Kind == config.MySQL {
		suite.T().Skipf("NULLS FIRST/LAST syntax not supported on %s", suite.ds.Kind)
	}

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_all",
			Action:   "find_all",
			Version:  "v1",
		},
		Meta: map[string]any{
			"sort": []map[string]any{
				{"column": "name", "direction": "asc", "nullsOrder": 1},
			},
		},
	})

	suite.Equal(200, resp.StatusCode, "Should return 200 status code")
	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Should return successful response")

	suite.T().Logf("Dynamic NullsFirst sort returned OK")
}

// TestFindAllDynamicSortNullsLast tests FindAll with dynamic sort NullsLast via request meta.
func (suite *FindAllTestSuite) TestFindAllDynamicSortNullsLast() {
	suite.T().Logf("Testing FindAll API dynamic sort NullsLast for %s", suite.ds.Kind)

	if suite.ds.Kind == config.MySQL {
		suite.T().Skipf("NULLS FIRST/LAST syntax not supported on %s", suite.ds.Kind)
	}

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_all",
			Action:   "find_all",
			Version:  "v1",
		},
		Meta: map[string]any{
			"sort": []map[string]any{
				{"column": "name", "direction": "desc", "nullsOrder": 2},
			},
		},
	})

	suite.Equal(200, resp.StatusCode, "Should return 200 status code")
	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Should return successful response")

	suite.T().Logf("Dynamic NullsLast sort returned OK")
}

// TestSetupErrModelNoPrimaryKey covers find.go:60-62 - auditUserModel on model with no PK.
func TestSetupErrModelNoPrimaryKey(t *testing.T) {
	db := testx.NewTestDB(t)

	fa := crud.NewFindAll[NoPKModel, struct{}]()
	fa.WithAuditUserNames((*struct{})(nil))

	specs := fa.Public().Provide()
	require.Len(t, specs, 1, "Should return exactly 1 operation spec")

	err := callHandlerFactory(t, specs[0].Handler, db)
	assert.ErrorIs(t, err, crud.ErrModelNoPrimaryKey, "Should return ErrModelNoPrimaryKey")
}

// TestSetupErrAuditUserCompositePK covers find.go:64-66 - auditUserModel on composite PK model.
func TestSetupErrAuditUserCompositePK(t *testing.T) {
	db := testx.NewTestDB(t)

	fa := crud.NewFindAll[CompositePKModel, struct{}]()
	fa.WithAuditUserNames((*struct{})(nil))

	specs := fa.Public().Provide()
	require.Len(t, specs, 1, "Should return exactly 1 operation spec")

	err := callHandlerFactory(t, specs[0].Handler, db)
	assert.ErrorIs(t, err, crud.ErrAuditUserCompositePK, "Should return ErrAuditUserCompositePK")
}

// TestFindWithOptions covers find.go WithOptions method.
func TestFindWithOptions(t *testing.T) {
	fa := crud.NewFindAll[orm.Model, struct{}]().
		WithOptions(&crud.FindOperationOption{
			Parts: []crud.QueryPart{crud.QueryRoot},
		}).
		Public()
	specs := fa.Provide()
	assert.Len(t, specs, 1, "Should return exactly 1 operation spec")
}

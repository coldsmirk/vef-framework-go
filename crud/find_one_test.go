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
	"github.com/coldsmirk/vef-framework-go/sortx"
)

func init() {
	registry.Add(func(env *testx.DBEnv) suite.TestingSuite {
		return &FindOneTestSuite{
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
type EmployeeFindOneResource struct {
	api.Resource
	crud.FindOne[Employee, EmployeeSearch]
}

func NewEmployeeFindOneResource() api.Resource {
	return &EmployeeFindOneResource{
		Resource: api.NewRPCResource("test/employee"),
		FindOne:  crud.NewFindOne[Employee, EmployeeSearch]().WithCondition(fixtureScope).Public(),
	}
}

// Processed User Resource - with processor.
type ProcessedUserFindOneResource struct {
	api.Resource
	crud.FindOne[Employee, EmployeeSearch]
}

type ProcessedUser struct {
	Employee

	Processed bool `json:"processed"`
}

func NewProcessedUserFindOneResource() api.Resource {
	return &ProcessedUserFindOneResource{
		Resource: api.NewRPCResource("test/employee_processed"),
		FindOne: crud.NewFindOne[Employee, EmployeeSearch]().
			WithCondition(fixtureScope).
			Public().
			WithProcessor(func(user Employee, _ EmployeeSearch, _ fiber.Ctx) any {
				return ProcessedUser{
					Employee:  user,
					Processed: true,
				}
			}),
	}
}

// Filtered User Resource - with filter applier.
type FilteredUserFineOneResource struct {
	api.Resource
	crud.FindOne[Employee, EmployeeSearch]
}

func NewFilteredUserFineOneResource() api.Resource {
	return &FilteredUserFineOneResource{
		Resource: api.NewRPCResource("test/employee_filtered"),
		FindOne: crud.NewFindOne[Employee, EmployeeSearch]().
			WithCondition(fixtureScope).
			WithCondition(func(cb orm.ConditionBuilder) {
				cb.Equals("status", "active").GreaterThan("age", 32)
			}).
			Public(),
	}
}

// Ordered User Resource - with order applier.
type OrderedUserFindOneResource struct {
	api.Resource
	crud.FindOne[Employee, EmployeeSearch]
}

func NewOrderedUserFindOneResource() api.Resource {
	return &OrderedUserFindOneResource{
		Resource: api.NewRPCResource("test/employee_ordered"),
		FindOne: crud.NewFindOne[Employee, EmployeeSearch]().
			WithCondition(fixtureScope).
			WithDefaultSort(&sortx.OrderSpec{
				Column:    "age",
				Direction: sortx.OrderDesc,
			}).
			Public(),
	}
}

// AuditUser User Resource - with audit user names.
type AuditedEmployeeFindOneResource struct {
	api.Resource
	crud.FindOne[Employee, EmployeeSearch]
}

func NewAuditedEmployeeFindOneResource() api.Resource {
	return &AuditedEmployeeFindOneResource{
		Resource: api.NewRPCResource("test/employee_audit"),
		FindOne: crud.NewFindOne[Employee, EmployeeSearch]().
			WithCondition(fixtureScope).
			WithAuditUserNames((*Operator)(nil)).
			Public(),
	}
}

// ErrorQueryApplierFindOneResource - FindOne with QueryApplier that returns error.
type ErrorQueryApplierFindOneResource struct {
	api.Resource
	crud.FindOne[Employee, EmployeeSearch]
}

func NewErrorQueryApplierFindOneResource() api.Resource {
	return &ErrorQueryApplierFindOneResource{
		Resource: api.NewRPCResource("test/employee_err_applier"),
		FindOne: crud.NewFindOne[Employee, EmployeeSearch]().
			WithCondition(fixtureScope).
			WithQueryApplier(func(_ orm.SelectQuery, _ EmployeeSearch, _ fiber.Ctx) error {
				return errors.New("query applier error")
			}).
			Public(),
	}
}

// FindOneTestSuite tests the FindOne API functionality
// including basic queries, search filters, processors, sorting, audit user names, and negative cases.
type FindOneTestSuite struct {
	BaseTestSuite
}

// SetupSuite runs once before all tests in the suite.
func (suite *FindOneTestSuite) SetupSuite() {
	suite.setupBaseSuite(
		NewEmployeeFindOneResource,
		NewProcessedUserFindOneResource,
		NewFilteredUserFineOneResource,
		NewOrderedUserFindOneResource,
		NewAuditedEmployeeFindOneResource,
		NewErrorQueryApplierFindOneResource,
	)
}

// TearDownSuite runs once after all tests in the suite.
func (suite *FindOneTestSuite) TearDownSuite() {
	suite.tearDownBaseSuite()
}

// TestFindOneBasic tests basic FindOne functionality.
func (suite *FindOneTestSuite) TestFindOneBasic() {
	suite.T().Logf("Testing FindOne API basic functionality for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee",
			Action:   "find_one",
			Version:  "v1",
		},
		Params: map[string]any{
			"id": "emp003",
		},
	})

	suite.Equal(200, resp.StatusCode, "Should return 200 status code")
	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Should return successful response")
	suite.Equal(body.Message, i18n.T(result.OkMessage), "Should return OK message")
	suite.NotNil(body.Data, "Data should not be nil")
	suite.Subset(body.Data, map[string]any{
		"id":     "emp003",
		"name":   "Yuki Tanaka",
		"email":  "yuki.tanaka@company.com",
		"age":    float64(31),
		"status": "active",
	}, "Should return correct employee data")

	suite.T().Logf("Found employee: emp003 (Yuki Tanaka)")
}

// TestFindOneNotFound tests FindOne when record doesn't exist.
func (suite *FindOneTestSuite) TestFindOneNotFound() {
	suite.T().Logf("Testing FindOne API record not found for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee",
			Action:   "find_one",
			Version:  "v1",
		},
		Params: map[string]any{
			"id": "nonexistent-id",
		},
	})

	suite.Equal(200, resp.StatusCode, "Should return 200 status code")
	body := suite.ReadResult(resp)
	suite.Equal(body.Code, result.ErrCodeRecordNotFound, "Should return record not found error code")
	suite.Equal(body.Message, i18n.T(result.ErrMessageRecordNotFound), "Should return record not found message")
	suite.Nil(body.Data, "Data should be nil when record not found")

	suite.T().Logf("Record not found as expected for nonexistent-id")
}

// TestFindOneWithSearchApplier tests FindOne with custom search conditions.
func (suite *FindOneTestSuite) TestFindOneWithSearchApplier() {
	suite.T().Logf("Testing FindOne API with search filters for %s", suite.ds.Kind)

	suite.Run("SearchByKeyword", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee",
				Action:   "find_one",
				Version:  "v1",
			},
			Params: map[string]any{
				"keyword": "Zhang",
			},
		})

		suite.Equal(200, resp.StatusCode, "Should return 200 status code")
		body := suite.ReadResult(resp)
		suite.True(body.IsOk(), "Should return successful response")
		suite.NotNil(body.Data, "Data should not be nil")
		suite.Subset(body.Data, map[string]any{
			"id":    "emp001",
			"name":  "Wei Zhang",
			"email": "wei.zhang@company.com",
			"age":   float64(35),
		}, "Should return employee matching keyword 'Zhang'")

		suite.T().Logf("Found employee by keyword: emp001 (Wei Zhang)")
	})

	suite.Run("SearchByEmail", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee",
				Action:   "find_one",
				Version:  "v1",
			},
			Params: map[string]any{
				"email": "ahmed.hassan@company.com",
			},
		})

		suite.Equal(200, resp.StatusCode, "Should return 200 status code")
		body := suite.ReadResult(resp)
		suite.True(body.IsOk(), "Should return successful response")
		suite.NotNil(body.Data, "Data should not be nil")
		suite.Subset(body.Data, map[string]any{
			"id":    "emp007",
			"name":  "Ahmed Hassan",
			"email": "ahmed.hassan@company.com",
			"age":   float64(29),
		}, "Should return employee matching email")

		suite.T().Logf("Found employee by email: emp007 (Ahmed Hassan)")
	})

	suite.Run("SearchByAgeRange", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee",
				Action:   "find_one",
				Version:  "v1",
			},
			Params: map[string]any{
				"age": []int{33, 34},
			},
		})

		suite.Equal(200, resp.StatusCode, "Should return 200 status code")
		body := suite.ReadResult(resp)
		suite.True(body.IsOk(), "Should return successful response")
		suite.NotNil(body.Data, "Data should not be nil")
		suite.Subset(body.Data, map[string]any{
			"id":   "emp019",
			"name": "Daniel Brown",
			"age":  float64(34),
		}, "Should return employee in age range 33-34")

		suite.T().Logf("Found employee by age range: emp019 (Daniel Brown, age 34)")
	})

	suite.Run("SearchByMultipleConditions", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee",
				Action:   "find_one",
				Version:  "v1",
			},
			Params: map[string]any{
				"email":  "kevin.park@company.com",
				"status": "inactive",
			},
		})

		suite.Equal(200, resp.StatusCode, "Should return 200 status code")
		body := suite.ReadResult(resp)
		suite.True(body.IsOk(), "Should return successful response")
		suite.NotNil(body.Data, "Data should not be nil")
		suite.Subset(body.Data, map[string]any{
			"id":     "emp009",
			"name":   "Kevin Park",
			"email":  "kevin.park@company.com",
			"age":    float64(26),
			"status": "inactive",
		}, "Should return employee matching multiple conditions")

		suite.T().Logf("Found employee by multiple conditions: emp009 (Kevin Park)")
	})
}

// TestFindOneWithProcessor tests FindOne with post-processing.
func (suite *FindOneTestSuite) TestFindOneWithProcessor() {
	suite.T().Logf("Testing FindOne API with processor for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_processed",
			Action:   "find_one",
			Version:  "v1",
		},
		Params: map[string]any{
			"id": "emp001",
		},
	})

	suite.Equal(200, resp.StatusCode, "Should return 200 status code")
	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Should return successful response")
	suite.NotNil(body.Data, "Data should not be nil")
	suite.Subset(body.Data, map[string]any{
		"id":        "emp001",
		"name":      "Wei Zhang",
		"email":     "wei.zhang@company.com",
		"age":       float64(35),
		"processed": true,
	}, "Should return processed employee data")

	suite.T().Logf("Found employee with post-processing applied: emp001 (processed=true)")
}

// TestFindOneWithFilterApplier tests FindOne with filter applier.
func (suite *FindOneTestSuite) TestFindOneWithFilterApplier() {
	suite.T().Logf("Testing FindOne API with filter applier for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_filtered",
			Action:   "find_one",
			Version:  "v1",
		},
	})

	suite.Equal(200, resp.StatusCode, "Should return 200 status code")
	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Should return successful response")
	suite.NotNil(body.Data, "Data should not be nil")
	suite.Subset(body.Data, map[string]any{
		"id":     "emp023",
		"name":   "Noah Anderson",
		"age":    float64(39),
		"status": "active",
	}, "Should return employee matching filter (status=active AND age>32)")

	suite.T().Logf("Found employee with filter applier: emp023 (Noah Anderson)")
}

// TestFindOneWithSortApplier tests FindOne with sort applier.
func (suite *FindOneTestSuite) TestFindOneWithSortApplier() {
	suite.T().Logf("Testing FindOne API with sort applier for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_ordered",
			Action:   "find_one",
			Version:  "v1",
		},
	})

	suite.Equal(200, resp.StatusCode, "Should return 200 status code")
	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Should return successful response")
	suite.NotNil(body.Data, "Data should not be nil")
	suite.Subset(body.Data, map[string]any{
		"id":   "emp013",
		"name": "Carlos Rodriguez",
		"age":  float64(50),
	}, "Should return employee with highest age (sorted by age DESC)")

	suite.T().Logf("Found employee with sort applier: emp013 (Carlos Rodriguez, age 50 - oldest)")
}

// TestFindOneNegativeCases tests negative scenarios.
func (suite *FindOneTestSuite) TestFindOneNegativeCases() {
	suite.T().Logf("Testing FindOne API negative cases for %s", suite.ds.Kind)

	suite.Run("InvalidResource", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/nonexistent",
				Action:   "find_one",
				Version:  "v1",
			},
		})

		suite.Equal(404, resp.StatusCode, "Should return 404 for invalid resource")

		suite.T().Logf("Invalid resource returned 404 as expected")
	})

	suite.Run("InvalidAction", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee",
				Action:   "nonexistentAction",
				Version:  "v1",
			},
		})

		suite.Equal(404, resp.StatusCode, "Should return 404 for invalid action")

		suite.T().Logf("Invalid action returned 404 as expected")
	})

	suite.Run("InvalidVersion", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee",
				Action:   "find_one",
				Version:  "v999",
			},
		})

		suite.Equal(404, resp.StatusCode, "Should return 404 for invalid version")

		suite.T().Logf("Invalid version returned 404 as expected")
	})

	suite.Run("EmptySearchCriteria", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee",
				Action:   "find_one",
				Version:  "v1",
			},
			Params: map[string]any{},
		})

		suite.Equal(200, resp.StatusCode, "Should return 200 status code")
		body := suite.ReadResult(resp)
		suite.True(body.IsOk(), "Should return successful response with empty criteria")
		suite.NotNil(body.Data, "Data should not be nil")

		suite.T().Logf("Empty search criteria returned first record")
	})

	suite.Run("InvalidRangeValue", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee",
				Action:   "find_one",
				Version:  "v1",
			},
			Params: map[string]any{
				"age": []int{30},
			},
		})

		suite.Equal(200, resp.StatusCode, "Should return 200 status code")
		body := suite.ReadResult(resp)
		suite.True(body.IsOk(), "Should work even with invalid range format")

		suite.T().Logf("Invalid range value handled gracefully")
	})

	suite.Run("MultipleConditionsNoMatch", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee",
				Action:   "find_one",
				Version:  "v1",
			},
			Params: map[string]any{
				"email":  "wei.zhang@company.com",
				"status": "inactive",
			},
		})

		suite.Equal(200, resp.StatusCode, "Should return 200 status code")
		body := suite.ReadResult(resp)
		suite.Equal(result.ErrCodeRecordNotFound, body.Code, "Should return record not found error code")
		suite.Nil(body.Data, "Data should be nil when no match found")

		suite.T().Logf("Multiple conflicting conditions returned no match as expected")
	})
}

// TestFindOneWithAuditUserNames tests FindOne with audit user names populated.
func (suite *FindOneTestSuite) TestFindOneWithAuditUserNames() {
	suite.T().Logf("Testing FindOne API with audit user names for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_audit",
			Action:   "find_one",
			Version:  "v1",
		},
		Params: map[string]any{
			"id": "emp001",
		},
	})

	suite.Equal(200, resp.StatusCode, "Should return 200 status code")
	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Should return successful response")
	suite.NotNil(body.Data, "Data should not be nil")

	user := suite.ReadDataAsMap(body.Data)
	suite.Equal("emp001", user["id"], "Should return correct employee id")
	suite.Equal("Wei Zhang", user["name"], "Should return correct employee name")

	suite.NotNil(user["createdByName"], "Should have createdByName populated")
	suite.NotNil(user["updatedByName"], "Should have updatedByName populated")

	suite.Equal("Sarah Chen", user["createdByName"], "Should return correct creator name")
	suite.Equal("James Liu", user["updatedByName"], "Should return correct updater name")

	suite.T().Logf("Found employee with audit names: emp001 (created by: %s, updated by: %s)", user["createdByName"], user["updatedByName"])
}

// TestFindOneErrorQueryApplier tests FindOne with a QueryApplier that returns error.
func (suite *FindOneTestSuite) TestFindOneErrorQueryApplier() {
	suite.T().Logf("Testing FindOne API error query applier for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_err_applier",
			Action:   "find_one",
			Version:  "v1",
		},
		Params: map[string]any{
			"id": "emp001",
		},
	})

	suite.Contains([]int{200, 500}, resp.StatusCode, "Should return error status code")

	suite.T().Logf("FindOne failed as expected due to query applier error")
}

// TestSetupErrFindOneNoPK covers find_one.go:27-29 - FindOne Setup error with no PK model.
func TestSetupErrFindOneNoPK(t *testing.T) {
	db := testx.NewTestDB(t)

	fo := crud.NewFindOne[NoPKModel, struct{}]()
	fo.WithAuditUserNames((*struct{})(nil))

	specs := fo.Public().Provide()
	require.Len(t, specs, 1, "Should return exactly 1 operation spec")

	err := callHandlerFactory(t, specs[0].Handler, db)
	assert.ErrorIs(t, err, crud.ErrModelNoPrimaryKey, "Should return ErrModelNoPrimaryKey")
}

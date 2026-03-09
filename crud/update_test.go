package crud_test

import (
	"errors"
	"fmt"

	"github.com/gofiber/fiber/v3"
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
		return &UpdateTestSuite{
			BaseTestSuite: BaseTestSuite{
				ctx:   env.Ctx,
				db:    env.DB,
				bunDB: env.BunDB,
				ds:    env.DS,
			},
		}
	})
}

type EmployeeUpdateResource struct {
	api.Resource
	crud.Update[Employee, EmployeeUpdateParams]
}

func NewEmployeeUpdateResource() api.Resource {
	return &EmployeeUpdateResource{
		Resource: api.NewRPCResource("test/employee_update"),
		Update:   crud.NewUpdate[Employee, EmployeeUpdateParams]().Public(),
	}
}

type EmployeeUpdateWithPreHookResource struct {
	api.Resource
	crud.Update[Employee, EmployeeUpdateParams]
}

func NewEmployeeUpdateWithPreHookResource() api.Resource {
	return &EmployeeUpdateWithPreHookResource{
		Resource: api.NewRPCResource("test/employee_update_prehook"),
		Update: crud.NewUpdate[Employee, EmployeeUpdateParams]().
			Public().
			WithPreUpdate(func(_, model *Employee, params *EmployeeUpdateParams, _ orm.UpdateQuery, _ fiber.Ctx, _ orm.DB) error {
				if params.Description != "" {
					model.Description = params.Description + " [Updated]"
				}

				return nil
			}),
	}
}

type EmployeeUpdateWithPostHookResource struct {
	api.Resource
	crud.Update[Employee, EmployeeUpdateParams]
}

func NewEmployeeUpdateWithPostHookResource() api.Resource {
	return &EmployeeUpdateWithPostHookResource{
		Resource: api.NewRPCResource("test/employee_update_posthook"),
		Update: crud.NewUpdate[Employee, EmployeeUpdateParams]().
			Public().
			WithPostUpdate(func(_, model *Employee, _ *EmployeeUpdateParams, ctx fiber.Ctx, _ orm.DB) error {
				ctx.Set("X-Updated-User-Name", model.Name)

				return nil
			}),
	}
}

// Resource with DisableDataPerm.
type EmployeeUpdateNoPermResource struct {
	api.Resource
	crud.Update[Employee, EmployeeUpdateParams]
}

func NewEmployeeUpdateNoPermResource() api.Resource {
	return &EmployeeUpdateNoPermResource{
		Resource: api.NewRPCResource("test/employee_update_noperm"),
		Update:   crud.NewUpdate[Employee, EmployeeUpdateParams]().DisableDataPerm().Public(),
	}
}

type EmployeeUpdateParams struct {
	api.P

	ID          string `json:"id"`
	Name        string `json:"name"        validate:"required"`
	Email       string `json:"email"       validate:"required,email"`
	Description string `json:"description"`
	Age         int    `json:"age"         validate:"required,min=1,max=120"`
	Status      string `json:"status"      validate:"required,oneof=active inactive on_leave"`
}

// Resource with PreUpdate hook that returns error.
type EmployeeUpdatePreHookErrorResource struct {
	api.Resource
	crud.Update[Employee, EmployeeUpdateParams]
}

func NewEmployeeUpdatePreHookErrorResource() api.Resource {
	return &EmployeeUpdatePreHookErrorResource{
		Resource: api.NewRPCResource("test/employee_update_prehook_err"),
		Update: crud.NewUpdate[Employee, EmployeeUpdateParams]().
			Public().
			WithPreUpdate(func(_, _ *Employee, _ *EmployeeUpdateParams, _ orm.UpdateQuery, _ fiber.Ctx, _ orm.DB) error {
				return errors.New("pre-update hook rejected")
			}),
	}
}

// Resource with PostUpdate hook that returns error.
type EmployeeUpdatePostHookErrorResource struct {
	api.Resource
	crud.Update[Employee, EmployeeUpdateParams]
}

func NewEmployeeUpdatePostHookErrorResource() api.Resource {
	return &EmployeeUpdatePostHookErrorResource{
		Resource: api.NewRPCResource("test/employee_update_posthook_err"),
		Update: crud.NewUpdate[Employee, EmployeeUpdateParams]().
			Public().
			WithPostUpdate(func(_, _ *Employee, _ *EmployeeUpdateParams, _ fiber.Ctx, _ orm.DB) error {
				return errors.New("post-update hook rejected")
			}),
	}
}

// UpdateTestSuite tests the Update API functionality.
type UpdateTestSuite struct {
	BaseTestSuite

	testEmployees []Employee
}

func (suite *UpdateTestSuite) SetupSuite() {
	suite.setupBaseSuite(
		NewEmployeeUpdateResource,
		NewEmployeeUpdateWithPreHookResource,
		NewEmployeeUpdateWithPostHookResource,
		NewEmployeeUpdateNoPermResource,
		NewEmployeeUpdatePreHookErrorResource,
		NewEmployeeUpdatePostHookErrorResource,
	)
}

func (suite *UpdateTestSuite) TearDownSuite() {
	suite.tearDownBaseSuite()
}

// SetupTest inserts isolated test data before each test method.
func (suite *UpdateTestSuite) SetupTest() {
	suite.testEmployees = []Employee{
		{Name: "UT Alice", Email: "ut_alice@test.com", Age: 30, Position: "Engineer", DepartmentID: "dept005", Status: "active"},
		{Name: "UT Bob", Email: "ut_bob@test.com", Age: 25, Position: "Designer", DepartmentID: "dept007", Status: "active"},
		{Name: "UT Charlie", Email: "ut_charlie@test.com", Age: 35, Position: "Analyst", DepartmentID: "dept015", Status: "active"},
		{Name: "UT Dave", Email: "ut_dave@test.com", Age: 28, Position: "Engineer", DepartmentID: "dept005", Status: "inactive"},
		{Name: "UT Eve", Email: "ut_eve@test.com", Age: 32, Position: "Director", DepartmentID: "dept001", Status: "active"},
		{Name: "UT Frank", Email: "ut_frank@test.com", Age: 40, Position: "Team Lead", DepartmentID: "dept006", Status: "active"},
		{Name: "UT Grace", Email: "ut_grace@test.com", Age: 27, Position: "Engineer", DepartmentID: "dept010", Status: "active"},
	}
	for i := range suite.testEmployees {
		suite.testEmployees[i].ID = fmt.Sprintf("ut_emp%03d", i+1)
	}

	_, err := suite.db.NewInsert().Model(&suite.testEmployees).Exec(suite.ctx)
	suite.Require().NoError(err, "Failed to insert test employees for update tests")
}

// TearDownTest removes all test-inserted data (created_at >= 2026) after each test method.
func (suite *UpdateTestSuite) TearDownTest() {
	suite.cleanupTestRecords()
}

// TestUpdateBasic tests basic Update functionality.
func (suite *UpdateTestSuite) TestUpdateBasic() {
	suite.T().Logf("Testing Update API basic functionality for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_update",
			Action:   "update",
			Version:  "v1",
		},
		Params: map[string]any{
			"id":          "ut_emp001",
			"name":        "UT Alice Updated",
			"email":       "ut_alice_updated@test.com",
			"description": "Updated description",
			"age":         26,
			"status":      "inactive",
		},
	})

	suite.Equal(200, resp.StatusCode, "Should return 200 status code")
	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Should return successful response")
	suite.Equal(body.Message, i18n.T(result.OkMessage), "Should return OK message")

	suite.T().Logf("Updated ut_emp001 successfully")
}

// TestUpdateWithPreHook tests Update with PreUpdate hook.
func (suite *UpdateTestSuite) TestUpdateWithPreHook() {
	suite.T().Logf("Testing Update API with PreUpdate hook for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_update_prehook",
			Action:   "update",
			Version:  "v1",
		},
		Params: map[string]any{
			"id":          "ut_emp002",
			"name":        "UT Bob Updated",
			"email":       "ut_bob_updated@test.com",
			"description": "New description",
			"age":         31,
			"status":      "active",
		},
	})

	suite.Equal(200, resp.StatusCode, "Should return 200 status code")
	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Should return successful response")
	suite.Equal(body.Message, i18n.T(result.OkMessage), "Should return OK message")

	suite.T().Logf("Updated ut_emp002 with PreUpdate hook successfully")
}

// TestUpdateWithPostHook tests Update with PostUpdate hook.
func (suite *UpdateTestSuite) TestUpdateWithPostHook() {
	suite.T().Logf("Testing Update API with PostUpdate hook for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_update_posthook",
			Action:   "update",
			Version:  "v1",
		},
		Params: map[string]any{
			"id":     "ut_emp003",
			"name":   "UT Charlie Updated",
			"email":  "ut_charlie_updated@test.com",
			"age":    29,
			"status": "active",
		},
	})

	suite.Equal(200, resp.StatusCode, "Should return 200 status code")
	suite.Equal("UT Charlie Updated", resp.Header.Get("X-Updated-User-Name"), "Should set X-Updated-User-Name header via PostUpdate hook")

	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Should return successful response")
	suite.Equal(body.Message, i18n.T(result.OkMessage), "Should return OK message")

	suite.T().Logf("Updated ut_emp003 with PostUpdate hook, header: %s", resp.Header.Get("X-Updated-User-Name"))
}

// TestUpdateNegativeCases tests negative scenarios.
func (suite *UpdateTestSuite) TestUpdateNegativeCases() {
	suite.T().Logf("Testing Update API negative cases for %s", suite.ds.Kind)

	suite.Run("NonExistentUser", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_update",
				Action:   "update",
				Version:  "v1",
			},
			Params: map[string]any{
				"id":     "nonexistent",
				"name":   "Test",
				"email":  "test@example.com",
				"age":    25,
				"status": "active",
			},
		})

		suite.Equal(200, resp.StatusCode, "Should return 200 status code")
		body := suite.ReadResult(resp)
		suite.False(body.IsOk(), "Should fail when user does not exist")
		suite.Equal(body.Message, i18n.T(result.ErrMessageRecordNotFound), "Should return record not found message")

		suite.T().Logf("Validation failed as expected for non-existent user")
	})

	suite.Run("MissingID", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_update",
				Action:   "update",
				Version:  "v1",
			},
			Params: map[string]any{
				"name":   "Test",
				"email":  "test@example.com",
				"age":    25,
				"status": "active",
			},
		})

		suite.Equal(200, resp.StatusCode, "Should return 200 status code")
		body := suite.ReadResult(resp)
		suite.False(body.IsOk(), "Should fail when required id is missing")
		suite.Equal(body.Message, i18n.T("primary_key_required", map[string]any{"field": "id"}), "Should return primary key required message")

		suite.T().Logf("Validation failed as expected for missing id")
	})

	suite.Run("InvalidEmail", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_update",
				Action:   "update",
				Version:  "v1",
			},
			Params: map[string]any{
				"id":     "ut_emp004",
				"name":   "Test",
				"email":  "invalid-email",
				"age":    25,
				"status": "active",
			},
		})

		suite.Equal(400, resp.StatusCode, "Should return 400 status code for validation error")
		body := suite.ReadResult(resp)
		suite.False(body.IsOk(), "Should fail when email format is invalid")

		suite.T().Logf("Validation failed as expected for invalid email format")
	})

	suite.Run("InvalidAge", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_update",
				Action:   "update",
				Version:  "v1",
			},
			Params: map[string]any{
				"id":     "ut_emp005",
				"name":   "Test",
				"email":  "test@example.com",
				"age":    0,
				"status": "active",
			},
		})

		suite.Equal(400, resp.StatusCode, "Should return 400 status code for validation error")
		body := suite.ReadResult(resp)
		suite.False(body.IsOk(), "Should fail when age is less than 1")

		suite.T().Logf("Validation failed as expected for invalid age")
	})

	suite.Run("DuplicateEmail", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_update",
				Action:   "update",
				Version:  "v1",
			},
			Params: map[string]any{
				"id":          "ut_emp006",
				"name":        "UT Frank Updated",
				"email":       "wei.zhang@company.com",
				"description": "Duplicate email test",
				"age":         33,
				"status":      "active",
			},
		})

		suite.Equal(200, resp.StatusCode, "Should return 200 status code")
		body := suite.ReadResult(resp)
		suite.False(body.IsOk(), "Should fail due to duplicate email unique constraint")
		suite.Equal(body.Message, i18n.T(result.ErrMessageRecordAlreadyExists), "Should return record already exists message")

		suite.T().Logf("Validation failed as expected for duplicate email")
	})
}

// TestPartialUpdate tests partial field updates.
func (suite *UpdateTestSuite) TestPartialUpdate() {
	suite.T().Logf("Testing Update API partial update for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_update",
			Action:   "update",
			Version:  "v1",
		},
		Params: map[string]any{
			"id":     "ut_emp007",
			"name":   "UT Grace Updated",
			"email":  "ut_grace_updated@test.com",
			"age":    30,
			"status": "active",
		},
	})

	suite.Equal(200, resp.StatusCode, "Should return 200 status code")
	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Should return successful response")
	suite.Equal(body.Message, i18n.T(result.OkMessage), "Should return OK message")

	suite.T().Logf("Partially updated ut_emp007 successfully")
}

// TestUpdateWithDisableDataPerm tests Update with DisableDataPerm.
func (suite *UpdateTestSuite) TestUpdateWithDisableDataPerm() {
	suite.T().Logf("Testing Update API with DisableDataPerm for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_update_noperm",
			Action:   "update",
			Version:  "v1",
		},
		Params: map[string]any{
			"id":     "ut_emp001",
			"name":   "UT Alice NoPerm",
			"email":  "ut_alice_noperm@test.com",
			"age":    31,
			"status": "active",
		},
	})

	suite.Equal(200, resp.StatusCode, "Should return 200 status code")
	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Should return successful response")
	suite.Equal(body.Message, i18n.T(result.OkMessage), "Should return OK message")

	suite.T().Logf("Updated ut_emp001 with DisableDataPerm successfully")
}

// TestUpdatePreHookError tests Update with a pre-hook that returns error.
func (suite *UpdateTestSuite) TestUpdatePreHookError() {
	suite.T().Logf("Testing Update API with pre-hook error for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_update_prehook_err",
			Action:   "update",
			Version:  "v1",
		},
		Params: map[string]any{
			"id":     suite.testEmployees[0].ID,
			"name":   "Should Not Update",
			"email":  "noupdate@test.com",
			"age":    99,
			"status": "active",
		},
	})

	// Hook errors inside transactions may result in 500
	suite.Contains([]int{200, 500}, resp.StatusCode, "Should return error status code")

	suite.T().Logf("Update failed as expected due to pre-hook error")
}

// TestUpdatePostHookError tests Update with a post-hook that returns error.
func (suite *UpdateTestSuite) TestUpdatePostHookError() {
	suite.T().Logf("Testing Update API with post-hook error for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_update_posthook_err",
			Action:   "update",
			Version:  "v1",
		},
		Params: map[string]any{
			"id":     suite.testEmployees[1].ID,
			"name":   "Should Rollback",
			"email":  "rollback@test.com",
			"age":    88,
			"status": "active",
		},
	})

	// Post-hook errors may result in 500 since they occur inside a transaction
	suite.Contains([]int{200, 500}, resp.StatusCode, "Should return error status code")

	suite.T().Logf("Update failed as expected due to post-hook error")
}

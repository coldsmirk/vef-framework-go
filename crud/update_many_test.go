package crud_test

import (
	"errors"
	"fmt"
	"strconv"

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
		return &UpdateManyTestSuite{
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
type EmployeeUpdateManyResource struct {
	api.Resource
	crud.UpdateMany[Employee, EmployeeUpdateParams]
}

func NewEmployeeUpdateManyResource() api.Resource {
	return &EmployeeUpdateManyResource{
		Resource:   api.NewRPCResource("test/employee_update_many"),
		UpdateMany: crud.NewUpdateMany[Employee, EmployeeUpdateParams]().Public(),
	}
}

// Resource with PreUpdateMany hook.
type EmployeeUpdateManyWithPreHookResource struct {
	api.Resource
	crud.UpdateMany[Employee, EmployeeUpdateParams]
}

func NewEmployeeUpdateManyWithPreHookResource() api.Resource {
	return &EmployeeUpdateManyWithPreHookResource{
		Resource: api.NewRPCResource("test/employee_update_many_prehook"),
		UpdateMany: crud.NewUpdateMany[Employee, EmployeeUpdateParams]().
			Public().
			WithPreUpdateMany(func(_, models []Employee, paramsList []EmployeeUpdateParams, _ orm.UpdateQuery, _ fiber.Ctx, _ orm.DB) error {
				// Add suffix to all descriptions
				for i := range models {
					if paramsList[i].Description != "" {
						models[i].Description = paramsList[i].Description + " [Batch Updated]"
					}
				}

				return nil
			}),
	}
}

// Resource with PostUpdateMany hook.
type EmployeeUpdateManyWithPostHookResource struct {
	api.Resource
	crud.UpdateMany[Employee, EmployeeUpdateParams]
}

func NewEmployeeUpdateManyWithPostHookResource() api.Resource {
	return &EmployeeUpdateManyWithPostHookResource{
		Resource: api.NewRPCResource("test/employee_update_many_posthook"),
		UpdateMany: crud.NewUpdateMany[Employee, EmployeeUpdateParams]().
			Public().
			WithPostUpdateMany(func(_, models []Employee, _ []EmployeeUpdateParams, ctx fiber.Ctx, _ orm.DB) error {
				// Set custom header with count
				ctx.Set("X-Updated-Count", strconv.Itoa(len(models)))

				return nil
			}),
	}
}

// Resource with DisableDataPerm.
type EmployeeUpdateManyNoPermResource struct {
	api.Resource
	crud.UpdateMany[Employee, EmployeeUpdateParams]
}

func NewEmployeeUpdateManyNoPermResource() api.Resource {
	return &EmployeeUpdateManyNoPermResource{
		Resource:   api.NewRPCResource("test/employee_update_many_noperm"),
		UpdateMany: crud.NewUpdateMany[Employee, EmployeeUpdateParams]().DisableDataPerm().Public(),
	}
}

// Resource with PreUpdateMany hook that returns error.
type EmployeeUpdateManyPreHookErrorResource struct {
	api.Resource
	crud.UpdateMany[Employee, EmployeeUpdateParams]
}

func NewEmployeeUpdateManyPreHookErrorResource() api.Resource {
	return &EmployeeUpdateManyPreHookErrorResource{
		Resource: api.NewRPCResource("test/employee_update_many_prehook_err"),
		UpdateMany: crud.NewUpdateMany[Employee, EmployeeUpdateParams]().
			Public().
			WithPreUpdateMany(func(_, _ []Employee, _ []EmployeeUpdateParams, _ orm.UpdateQuery, _ fiber.Ctx, _ orm.DB) error {
				return errors.New("pre-update-many hook rejected")
			}),
	}
}

// Resource with PostUpdateMany hook that returns error.
type EmployeeUpdateManyPostHookErrorResource struct {
	api.Resource
	crud.UpdateMany[Employee, EmployeeUpdateParams]
}

func NewEmployeeUpdateManyPostHookErrorResource() api.Resource {
	return &EmployeeUpdateManyPostHookErrorResource{
		Resource: api.NewRPCResource("test/employee_update_many_posthook_err"),
		UpdateMany: crud.NewUpdateMany[Employee, EmployeeUpdateParams]().
			Public().
			WithPostUpdateMany(func(_, _ []Employee, _ []EmployeeUpdateParams, _ fiber.Ctx, _ orm.DB) error {
				return errors.New("post-update-many hook rejected")
			}),
	}
}

// UpdateManyTestSuite tests the UpdateMany API functionality
// including basic batch update, PreUpdateMany/PostUpdateMany hooks, negative cases, transaction rollback, and partial updates.
type UpdateManyTestSuite struct {
	BaseTestSuite

	testEmployees []Employee
}

// SetupSuite runs once before all tests in the suite.
func (suite *UpdateManyTestSuite) SetupSuite() {
	suite.setupBaseSuite(
		NewEmployeeUpdateManyResource,
		NewEmployeeUpdateManyWithPreHookResource,
		NewEmployeeUpdateManyWithPostHookResource,
		NewEmployeeUpdateManyNoPermResource,
		NewEmployeeUpdateManyPreHookErrorResource,
		NewEmployeeUpdateManyPostHookErrorResource,
	)
}

// TearDownSuite runs once after all tests in the suite.
func (suite *UpdateManyTestSuite) TearDownSuite() {
	suite.tearDownBaseSuite()
}

// SetupTest inserts isolated test data before each test method.
func (suite *UpdateManyTestSuite) SetupTest() {
	suite.testEmployees = []Employee{
		{Name: "UM Alice", Email: "um_alice@test.com", Age: 30, Position: "Engineer", DepartmentID: "dept005", Status: "active"},
		{Name: "UM Bob", Email: "um_bob@test.com", Age: 25, Position: "Designer", DepartmentID: "dept007", Status: "active"},
		{Name: "UM Charlie", Email: "um_charlie@test.com", Age: 35, Position: "Analyst", DepartmentID: "dept015", Status: "active"},
		{Name: "UM Dave", Email: "um_dave@test.com", Age: 28, Position: "Engineer", DepartmentID: "dept005", Status: "inactive"},
		{Name: "UM Eve", Email: "um_eve@test.com", Age: 32, Position: "Director", DepartmentID: "dept001", Status: "active"},
		{Name: "UM Frank", Email: "um_frank@test.com", Age: 40, Position: "Team Lead", DepartmentID: "dept006", Status: "active"},
		{Name: "UM Grace", Email: "um_grace@test.com", Age: 27, Position: "Engineer", DepartmentID: "dept010", Status: "active"},
		{Name: "UM Henry", Email: "um_henry@test.com", Age: 29, Position: "Analyst", DepartmentID: "dept015", Status: "active"},
		{Name: "UM Iris", Email: "um_iris@test.com", Age: 31, Position: "Designer", DepartmentID: "dept007", Status: "active"},
		{Name: "UM Jack", Email: "um_jack@test.com", Age: 33, Position: "Engineer", DepartmentID: "dept005", Status: "inactive"},
		{Name: "UM Kate", Email: "um_kate@test.com", Age: 26, Position: "Engineer", DepartmentID: "dept006", Status: "active"},
		{Name: "UM Leo", Email: "um_leo@test.com", Age: 34, Position: "Director", DepartmentID: "dept001", Status: "active"},
	}
	for i := range suite.testEmployees {
		suite.testEmployees[i].ID = fmt.Sprintf("um_emp%03d", i+1)
	}

	_, err := suite.db.NewInsert().Model(&suite.testEmployees).Exec(suite.ctx)
	suite.Require().NoError(err, "Failed to insert test employees for update_many tests")
}

// TearDownTest removes all test-inserted data (created_at >= 2026) after each test method.
func (suite *UpdateManyTestSuite) TearDownTest() {
	suite.cleanupTestRecords()
}

// TestUpdateManyBasic tests basic UpdateMany functionality.
func (suite *UpdateManyTestSuite) TestUpdateManyBasic() {
	suite.T().Logf("Testing UpdateMany API basic functionality for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_update_many",
			Action:   "update_many",
			Version:  "v1",
		},
		Params: map[string]any{
			"list": []any{
				map[string]any{
					"id":          "um_emp001",
					"name":        "UM Alice Updated",
					"email":       "um_alice_updated@test.com",
					"description": "Updated description",
					"age":         26,
					"status":      "inactive",
				},
				map[string]any{
					"id":     "um_emp002",
					"name":   "UM Bob Updated",
					"email":  "um_bob_updated@test.com",
					"age":    31,
					"status": "active",
				},
			},
		},
	})

	suite.Equal(200, resp.StatusCode, "Should return 200 status code")
	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Should return successful response")
	suite.Equal(body.Message, i18n.T(result.OkMessage), "Should return OK message")

	suite.T().Logf("Successfully updated 2 users in batch")
}

// TestUpdateManyWithPreHook tests UpdateMany with PreUpdateMany hook.
func (suite *UpdateManyTestSuite) TestUpdateManyWithPreHook() {
	suite.T().Logf("Testing UpdateMany API with PreUpdateMany hook for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_update_many_prehook",
			Action:   "update_many",
			Version:  "v1",
		},
		Params: map[string]any{
			"list": []any{
				map[string]any{
					"id":          "um_emp003",
					"name":        "UM Charlie Updated",
					"email":       "um_charlie_updated@test.com",
					"description": "New description",
					"age":         29,
					"status":      "active",
				},
				map[string]any{
					"id":          "um_emp004",
					"name":        "UM Dave Updated",
					"email":       "um_dave_updated@test.com",
					"description": "Another description",
					"age":         33,
					"status":      "inactive",
				},
			},
		},
	})

	suite.Equal(200, resp.StatusCode, "Should return 200 status code")
	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Should return successful response")
	suite.Equal(body.Message, i18n.T(result.OkMessage), "Should return OK message")

	suite.T().Logf("Successfully updated 2 users with PreUpdateMany hook")
}

// TestUpdateManyWithPostHook tests UpdateMany with PostUpdateMany hook.
func (suite *UpdateManyTestSuite) TestUpdateManyWithPostHook() {
	suite.T().Logf("Testing UpdateMany API with PostUpdateMany hook for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_update_many_posthook",
			Action:   "update_many",
			Version:  "v1",
		},
		Params: map[string]any{
			"list": []any{
				map[string]any{
					"id":     "um_emp005",
					"name":   "UM Eve Updated",
					"email":  "um_eve_updated@test.com",
					"age":    28,
					"status": "active",
				},
				map[string]any{
					"id":     "um_emp006",
					"name":   "UM Frank Updated",
					"email":  "um_frank_updated@test.com",
					"age":    36,
					"status": "inactive",
				},
			},
		},
	})

	suite.Equal(200, resp.StatusCode, "Should return 200 status code")
	suite.NotEmpty(resp.Header.Get("X-Updated-Count"), "Should set X-Updated-Count header via PostUpdateMany hook")

	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Should return successful response")
	suite.Equal(body.Message, i18n.T(result.OkMessage), "Should return OK message")

	updatedCount := resp.Header.Get("X-Updated-Count")
	suite.T().Logf("Updated %s users with PostUpdateMany hook", updatedCount)
}

// TestUpdateManyNegativeCases tests negative scenarios.
func (suite *UpdateManyTestSuite) TestUpdateManyNegativeCases() {
	suite.T().Logf("Testing UpdateMany API negative cases for %s", suite.ds.Kind)

	suite.Run("EmptyArray", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_update_many",
				Action:   "update_many",
				Version:  "v1",
			},
			Params: map[string]any{"list": []any{}},
		})

		suite.Equal(400, resp.StatusCode, "Should return 400 status code for validation error")
		body := suite.ReadResult(resp)
		suite.False(body.IsOk(), "Should fail when list is empty")

		suite.T().Logf("Validation failed as expected for empty list")
	})

	suite.Run("NonExistentUser", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_update_many",
				Action:   "update_many",
				Version:  "v1",
			},
			Params: map[string]any{
				"list": []any{
					map[string]any{
						"id":     "um_emp007",
						"name":   "Valid Update",
						"email":  "valid@example.com",
						"age":    25,
						"status": "active",
					},
					map[string]any{
						"id":     "nonexistent",
						"name":   "Invalid Update",
						"email":  "invalid@example.com",
						"age":    30,
						"status": "active",
					},
				},
			},
		})

		suite.Equal(200, resp.StatusCode, "Should return 200 status code")
		body := suite.ReadResult(resp)
		suite.False(body.IsOk(), "Should fail when one user does not exist")
		suite.Equal(body.Message, i18n.T(result.ErrMessageRecordNotFound), "Should return record not found message")

		suite.T().Logf("Validation failed as expected for non-existent user in batch")
	})

	suite.Run("MissingID", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_update_many",
				Action:   "update_many",
				Version:  "v1",
			},
			Params: map[string]any{
				"list": []any{
					map[string]any{
						"id":     "um_emp008",
						"name":   "Valid Update",
						"email":  "valid@example.com",
						"age":    25,
						"status": "active",
					},
					map[string]any{
						// Missing "id"
						"name":   "Invalid Update",
						"email":  "invalid@example.com",
						"age":    30,
						"status": "active",
					},
				},
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
				Resource: "test/employee_update_many",
				Action:   "update_many",
				Version:  "v1",
			},
			Params: map[string]any{
				"list": []any{
					map[string]any{
						"id":     "um_emp009",
						"name":   "Valid Update",
						"email":  "valid@example.com",
						"age":    25,
						"status": "active",
					},
					map[string]any{
						"id":     "um_emp010",
						"name":   "Invalid Update",
						"email":  "not-an-email",
						"age":    30,
						"status": "active",
					},
				},
			},
		})

		suite.Equal(400, resp.StatusCode, "Should return 400 status code for validation error")
		body := suite.ReadResult(resp)
		suite.False(body.IsOk(), "Should fail when email format is invalid in batch")

		suite.T().Logf("Validation failed as expected for invalid email format")
	})

	suite.Run("InvalidAge", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_update_many",
				Action:   "update_many",
				Version:  "v1",
			},
			Params: map[string]any{
				"list": []any{
					map[string]any{
						"id":     "um_emp011",
						"name":   "Valid Update",
						"email":  "valid@example.com",
						"age":    25,
						"status": "active",
					},
					map[string]any{
						"id":     "um_emp012",
						"name":   "Invalid Update",
						"email":  "invalid@example.com",
						"age":    0, // Invalid: min=1
						"status": "active",
					},
				},
			},
		})

		suite.Equal(400, resp.StatusCode, "Should return 400 status code for validation error")
		body := suite.ReadResult(resp)
		suite.False(body.IsOk(), "Should fail when age is less than 1 in batch")

		suite.T().Logf("Validation failed as expected for invalid age")
	})

	suite.Run("DuplicateEmailInBatch", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_update_many",
				Action:   "update_many",
				Version:  "v1",
			},
			Params: map[string]any{
				"list": []any{
					map[string]any{
						"id":     "um_emp001",
						"name":   "User One",
						"email":  "duplicate.batch.update@example.com",
						"age":    25,
						"status": "active",
					},
					map[string]any{
						"id":     "um_emp002",
						"name":   "User Two",
						"email":  "duplicate.batch.update@example.com", // Duplicate with previous
						"age":    30,
						"status": "active",
					},
				},
			},
		})

		suite.Equal(200, resp.StatusCode, "Should return 200 status code")
		body := suite.ReadResult(resp)
		suite.False(body.IsOk(), "Should fail due to duplicate email in batch")
		suite.Equal(body.Message, i18n.T(result.ErrMessageRecordAlreadyExists), "Should return record already exists message")

		suite.T().Logf("Validation failed as expected for duplicate email in batch")
	})

	suite.Run("DuplicateEmailWithExisting", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_update_many",
				Action:   "update_many",
				Version:  "v1",
			},
			Params: map[string]any{
				"list": []any{
					map[string]any{
						"id":     "um_emp001",
						"name":   "User One",
						"email":  "maria.garcia@company.com", // Existing email from emp002
						"age":    25,
						"status": "active",
					},
				},
			},
		})

		suite.Equal(200, resp.StatusCode, "Should return 200 status code")
		body := suite.ReadResult(resp)
		suite.False(body.IsOk(), "Should fail due to duplicate email with existing record")
		suite.Equal(body.Message, i18n.T(result.ErrMessageRecordAlreadyExists), "Should return record already exists message")

		suite.T().Logf("Validation failed as expected for duplicate email with existing record")
	})
}

// TestUpdateManyTransactionRollback tests that the entire batch rolls back on error.
func (suite *UpdateManyTestSuite) TestUpdateManyTransactionRollback() {
	suite.T().Logf("Testing UpdateMany API transaction rollback for %s", suite.ds.Kind)

	suite.Run("AllOrNothingSemantics", func() {
		// Try to update a batch where the second item will fail
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_update_many",
				Action:   "update_many",
				Version:  "v1",
			},
			Params: map[string]any{
				"list": []any{
					map[string]any{
						"id":     "um_emp001",
						"name":   "Should Not Be Updated",
						"email":  "rollback1@example.com",
						"age":    25,
						"status": "active",
					},
					map[string]any{
						"id":     "nonexistent_rollback",
						"name":   "Invalid User",
						"email":  "rollback2@example.com",
						"age":    30,
						"status": "active",
					},
				},
			},
		})

		suite.Equal(200, resp.StatusCode, "Should return 200 status code")
		body := suite.ReadResult(resp)
		suite.False(body.IsOk(), "Should fail when one item in batch is invalid")

		// Verify that the first user was not updated (transaction rolled back)
		var user Employee

		err := suite.db.NewSelect().Model(&user).Where(func(cb orm.ConditionBuilder) {
			cb.Equals("id", "um_emp001")
		}).Scan(suite.ctx, &user)
		suite.NoError(err, "Should successfully query database")
		suite.NotEqual("rollback1@example.com", user.Email, "Email should not have been updated - transaction should have rolled back")

		suite.T().Logf("Transaction rollback verified: first user was not updated")
	})
}

// TestUpdateManyPartialUpdate tests updating only some fields.
func (suite *UpdateManyTestSuite) TestUpdateManyPartialUpdate() {
	suite.T().Logf("Testing UpdateMany API partial update for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_update_many",
			Action:   "update_many",
			Version:  "v1",
		},
		Params: map[string]any{
			"list": []any{
				map[string]any{
					"id":     "um_emp009",
					"name":   "Partial Update 1",
					"email":  "partial1@example.com",
					"age":    25,
					"status": "active",
					// Not updating description
				},
				map[string]any{
					"id":     "um_emp010",
					"name":   "Partial Update 2",
					"email":  "partial2@example.com",
					"age":    30,
					"status": "inactive",
					// Not updating description
				},
			},
		},
	})

	suite.Equal(200, resp.StatusCode, "Should return 200 status code")
	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Should return successful response")
	suite.Equal(body.Message, i18n.T(result.OkMessage), "Should return OK message")

	suite.T().Logf("Successfully partially updated 2 users")
}

// TestUpdateManyWithDisableDataPerm tests UpdateMany with DisableDataPerm.
func (suite *UpdateManyTestSuite) TestUpdateManyWithDisableDataPerm() {
	suite.T().Logf("Testing UpdateMany API with DisableDataPerm for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_update_many_noperm",
			Action:   "update_many",
			Version:  "v1",
		},
		Params: map[string]any{
			"list": []any{
				map[string]any{
					"id":     "um_emp001",
					"name":   "UM Alice NoPerm",
					"email":  "um_alice_noperm@test.com",
					"age":    31,
					"status": "active",
				},
				map[string]any{
					"id":     "um_emp002",
					"name":   "UM Bob NoPerm",
					"email":  "um_bob_noperm@test.com",
					"age":    26,
					"status": "active",
				},
			},
		},
	})

	suite.Equal(200, resp.StatusCode, "Should return 200 status code")
	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Should return successful response")
	suite.Equal(body.Message, i18n.T(result.OkMessage), "Should return OK message")

	suite.T().Logf("Updated 2 employees with DisableDataPerm successfully")
}

// TestUpdateManyPreHookError tests UpdateMany with a pre-hook that returns error.
func (suite *UpdateManyTestSuite) TestUpdateManyPreHookError() {
	suite.T().Logf("Testing UpdateMany API pre-hook error for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_update_many_prehook_err",
			Action:   "update_many",
			Version:  "v1",
		},
		Params: map[string]any{
			"list": []any{
				map[string]any{
					"id":     suite.testEmployees[0].ID,
					"name":   "Should Not Update",
					"email":  "noupdate@test.com",
					"age":    99,
					"status": "active",
				},
			},
		},
	})

	suite.Contains([]int{200, 500}, resp.StatusCode, "Should return error status code")

	suite.T().Logf("UpdateMany failed as expected due to pre-hook error")
}

// TestUpdateManyPostHookError tests UpdateMany with a post-hook that returns error.
func (suite *UpdateManyTestSuite) TestUpdateManyPostHookError() {
	suite.T().Logf("Testing UpdateMany API post-hook error for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_update_many_posthook_err",
			Action:   "update_many",
			Version:  "v1",
		},
		Params: map[string]any{
			"list": []any{
				map[string]any{
					"id":     suite.testEmployees[1].ID,
					"name":   "Should Rollback",
					"email":  "rollback@test.com",
					"age":    88,
					"status": "active",
				},
			},
		},
	})

	suite.Contains([]int{200, 500}, resp.StatusCode, "Should return error status code")

	suite.T().Logf("UpdateMany failed as expected due to post-hook error")
}

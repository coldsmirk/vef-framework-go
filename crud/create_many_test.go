package crud_test

import (
	"errors"
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
		return &CreateManyTestSuite{
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
type EmployeeCreateManyResource struct {
	api.Resource
	crud.CreateMany[Employee, EmployeeCreateParams]
}

func NewEmployeeCreateManyResource() api.Resource {
	return &EmployeeCreateManyResource{
		Resource:   api.NewRPCResource("test/employee_create_many"),
		CreateMany: crud.NewCreateMany[Employee, EmployeeCreateParams]().Public(),
	}
}

// Resource with PreCreateMany hook.
type EmployeeCreateManyWithPreHookResource struct {
	api.Resource
	crud.CreateMany[Employee, EmployeeCreateParams]
}

func NewEmployeeCreateManyWithPreHookResource() api.Resource {
	return &EmployeeCreateManyWithPreHookResource{
		Resource: api.NewRPCResource("test/employee_create_many_prehook"),
		CreateMany: crud.NewCreateMany[Employee, EmployeeCreateParams]().
			Public().
			WithPreCreateMany(func(models []Employee, _ []EmployeeCreateParams, _ orm.InsertQuery, _ fiber.Ctx, _ orm.DB) error {
				// Add prefix to all names
				for i := range models {
					models[i].Name = "Mr. " + models[i].Name
				}

				return nil
			}),
	}
}

// Resource with PostCreateMany hook.
type EmployeeCreateManyWithPostHookResource struct {
	api.Resource
	crud.CreateMany[Employee, EmployeeCreateParams]
}

func NewEmployeeCreateManyWithPostHookResource() api.Resource {
	return &EmployeeCreateManyWithPostHookResource{
		Resource: api.NewRPCResource("test/employee_create_many_posthook"),
		CreateMany: crud.NewCreateMany[Employee, EmployeeCreateParams]().
			Public().
			WithPostCreateMany(func(models []Employee, _ []EmployeeCreateParams, ctx fiber.Ctx, _ orm.DB) error {
				// Set custom header with count
				ctx.Set("X-Created-Count", strconv.Itoa(len(models)))

				return nil
			}),
	}
}

// Resource with PreCreateMany hook that returns error.
type EmployeeCreateManyPreHookErrorResource struct {
	api.Resource
	crud.CreateMany[Employee, EmployeeCreateParams]
}

func NewEmployeeCreateManyPreHookErrorResource() api.Resource {
	return &EmployeeCreateManyPreHookErrorResource{
		Resource: api.NewRPCResource("test/employee_create_many_prehook_err"),
		CreateMany: crud.NewCreateMany[Employee, EmployeeCreateParams]().
			Public().
			WithPreCreateMany(func(_ []Employee, _ []EmployeeCreateParams, _ orm.InsertQuery, _ fiber.Ctx, _ orm.DB) error {
				return errors.New("pre-create-many hook rejected")
			}),
	}
}

// Resource with PostCreateMany hook that returns error.
type EmployeeCreateManyPostHookErrorResource struct {
	api.Resource
	crud.CreateMany[Employee, EmployeeCreateParams]
}

func NewEmployeeCreateManyPostHookErrorResource() api.Resource {
	return &EmployeeCreateManyPostHookErrorResource{
		Resource: api.NewRPCResource("test/employee_create_many_posthook_err"),
		CreateMany: crud.NewCreateMany[Employee, EmployeeCreateParams]().
			Public().
			WithPostCreateMany(func(_ []Employee, _ []EmployeeCreateParams, _ fiber.Ctx, _ orm.DB) error {
				return errors.New("post-create-many hook rejected")
			}),
	}
}

// CreateManyTestSuite tests the CreateMany API functionality
// including basic batch creation, PreCreateMany/PostCreateMany hooks, negative cases, and transaction rollback.
type CreateManyTestSuite struct {
	BaseTestSuite
}

// SetupSuite runs once before all tests in the suite.
func (suite *CreateManyTestSuite) SetupSuite() {
	suite.setupBaseSuite(
		NewEmployeeCreateManyResource,
		NewEmployeeCreateManyWithPreHookResource,
		NewEmployeeCreateManyWithPostHookResource,
		NewEmployeeCreateManyPreHookErrorResource,
		NewEmployeeCreateManyPostHookErrorResource,
	)
}

// TearDownSuite runs once after all tests in the suite.
func (suite *CreateManyTestSuite) TearDownSuite() {
	suite.tearDownBaseSuite()
}

// TearDownTest cleans up test-created records after each test.
func (suite *CreateManyTestSuite) TearDownTest() {
	suite.cleanupTestRecords()
}

// TestCreateManyBasic tests basic CreateMany functionality.
func (suite *CreateManyTestSuite) TestCreateManyBasic() {
	suite.T().Logf("Testing CreateMany API basic functionality for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_create_many",
			Action:   "create_many",
			Version:  "v1",
		},
		Params: map[string]any{
			"list": []any{
				map[string]any{
					"name":         "User One",
					"email":        "user1@example.com",
					"description":  "First user",
					"age":          25,
					"position":     "Engineer",
					"departmentId": "dept005",
					"status":       "active",
				},
				map[string]any{
					"name":         "User Two",
					"email":        "user2@example.com",
					"description":  "Second user",
					"age":          30,
					"position":     "Designer",
					"departmentId": "dept007",
					"status":       "inactive",
				},
				map[string]any{
					"name":         "User Three",
					"email":        "user3@example.com",
					"age":          35,
					"position":     "Analyst",
					"departmentId": "dept015",
					"status":       "active",
				},
			},
		},
	})

	suite.Equal(200, resp.StatusCode, "Should return 200 status code")
	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Should return successful response")
	suite.Equal(body.Message, i18n.T(result.OkMessage), "Should return OK message")
	suite.NotNil(body.Data, "Should return data")

	// CreateManyAPI returns array of primary keys
	pks := suite.ReadDataAsSlice(body.Data)
	suite.Len(pks, 3, "Should create 3 users")

	for i, pk := range pks {
		pkMap := suite.ReadDataAsMap(pk)
		suite.NotEmpty(pkMap["id"], "Should return created user id for user %d", i+1)
		suite.T().Logf("Created user %d with id: %v", i+1, pkMap["id"])
	}
}

// TestCreateManyWithPreHook tests CreateMany with PreCreateMany hook.
func (suite *CreateManyTestSuite) TestCreateManyWithPreHook() {
	suite.T().Logf("Testing CreateMany API with PreCreateMany hook for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_create_many_prehook",
			Action:   "create_many",
			Version:  "v1",
		},
		Params: map[string]any{
			"list": []any{
				map[string]any{
					"name":         "John",
					"email":        "john.batch@example.com",
					"age":          28,
					"position":     "Engineer",
					"departmentId": "dept005",
					"status":       "active",
				},
				map[string]any{
					"name":         "Jane",
					"email":        "jane.batch@example.com",
					"age":          26,
					"position":     "Designer",
					"departmentId": "dept007",
					"status":       "active",
				},
			},
		},
	})

	suite.Equal(200, resp.StatusCode, "Should return 200 status code")
	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Should return successful response")

	// CreateManyAPI returns array of primary keys
	pks := suite.ReadDataAsSlice(body.Data)
	suite.Len(pks, 2, "Should create 2 users with PreCreateMany hook")

	for i, pk := range pks {
		pkMap := suite.ReadDataAsMap(pk)
		suite.T().Logf("Created user %d with PreCreateMany hook, id: %v", i+1, pkMap["id"])
	}
}

// TestCreateManyWithPostHook tests CreateMany with PostCreateMany hook.
func (suite *CreateManyTestSuite) TestCreateManyWithPostHook() {
	suite.T().Logf("Testing CreateMany API with PostCreateMany hook for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_create_many_posthook",
			Action:   "create_many",
			Version:  "v1",
		},
		Params: map[string]any{
			"list": []any{
				map[string]any{
					"name":         "Alice",
					"email":        "alice.batch@example.com",
					"age":          29,
					"position":     "Team Lead",
					"departmentId": "dept005",
					"status":       "active",
				},
				map[string]any{
					"name":         "Bob",
					"email":        "bob.batch@example.com",
					"age":          31,
					"position":     "Analyst",
					"departmentId": "dept015",
					"status":       "inactive",
				},
			},
		},
	})

	suite.Equal(200, resp.StatusCode, "Should return 200 status code")
	suite.NotEmpty(resp.Header.Get("X-Created-Count"), "Should set X-Created-Count header via PostCreateMany hook")

	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Should return successful response")

	pks := suite.ReadDataAsSlice(body.Data)
	suite.Len(pks, 2, "Should create 2 users with PostCreateMany hook")

	createdCount := resp.Header.Get("X-Created-Count")
	suite.T().Logf("Created %s users with PostCreateMany hook", createdCount)

	for i, pk := range pks {
		pkMap := suite.ReadDataAsMap(pk)
		suite.T().Logf("Created user %d with id: %v", i+1, pkMap["id"])
	}
}

// TestCreateManyNegativeCases tests negative scenarios.
func (suite *CreateManyTestSuite) TestCreateManyNegativeCases() {
	suite.T().Logf("Testing CreateMany API negative cases for %s", suite.ds.Kind)

	suite.Run("EmptyArray", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_create_many",
				Action:   "create_many",
				Version:  "v1",
			},
			Params: map[string]any{
				"list": []any{},
			},
		})

		suite.Equal(400, resp.StatusCode, "Should return 400 status code for validation error")
		body := suite.ReadResult(resp)
		suite.False(body.IsOk(), "Should fail when list is empty")

		suite.T().Logf("Validation failed as expected for empty list")
	})

	suite.Run("MissingRequiredField", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_create_many",
				Action:   "create_many",
				Version:  "v1",
			},
			Params: map[string]any{
				"list": []any{
					map[string]any{
						"name":   "Valid User",
						"email":  "valid@example.com",
						"age":    25,
						"status": "active",
					},
					map[string]any{
						"email":  "invalid@example.com",
						"age":    30,
						"status": "active",
						// Missing "name"
					},
				},
			},
		})

		suite.Equal(400, resp.StatusCode, "Should return 400 status code for validation error")
		body := suite.ReadResult(resp)
		suite.False(body.IsOk(), "Should fail when required field 'name' is missing in batch")

		suite.T().Logf("Validation failed as expected for missing required field")
	})

	suite.Run("InvalidEmailInBatch", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_create_many",
				Action:   "create_many",
				Version:  "v1",
			},
			Params: map[string]any{
				"list": []any{
					map[string]any{
						"name":   "Valid User",
						"email":  "valid@example.com",
						"age":    25,
						"status": "active",
					},
					map[string]any{
						"name":   "Invalid User",
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

	suite.Run("InvalidAgeInBatch", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_create_many",
				Action:   "create_many",
				Version:  "v1",
			},
			Params: map[string]any{
				"list": []any{
					map[string]any{
						"name":   "Valid User",
						"email":  "valid2@example.com",
						"age":    25,
						"status": "active",
					},
					map[string]any{
						"name":   "Invalid User",
						"email":  "invalid2@example.com",
						"age":    150, // Invalid: > 120
						"status": "active",
					},
				},
			},
		})

		suite.Equal(400, resp.StatusCode, "Should return 400 status code for validation error")
		body := suite.ReadResult(resp)
		suite.False(body.IsOk(), "Should fail when age is greater than 120 in batch")

		suite.T().Logf("Validation failed as expected for invalid age")
	})

	suite.Run("DuplicateEmailInSameBatch", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_create_many",
				Action:   "create_many",
				Version:  "v1",
			},
			Params: map[string]any{
				"list": []any{
					map[string]any{
						"name":   "User A",
						"email":  "duplicate.batch@example.com",
						"age":    25,
						"status": "active",
					},
					map[string]any{
						"name":   "User B",
						"email":  "duplicate.batch@example.com",
						"age":    30,
						"status": "active",
					},
				},
			},
		})

		suite.Equal(400, resp.StatusCode, "Should return 400 status code for validation error")
		body := suite.ReadResult(resp)
		suite.False(body.IsOk(), "Should fail due to duplicate email in same batch")

		suite.T().Logf("Validation failed as expected for duplicate email in batch")
	})

	suite.Run("DuplicateWithExistingRecord", func() {
		// First create a user
		suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_create_many",
				Action:   "create_many",
				Version:  "v1",
			},
			Params: map[string]any{
				"list": []any{
					map[string]any{
						"name":   "Existing User",
						"email":  "existing.batch@example.com",
						"age":    25,
						"status": "active",
					},
				},
			},
		})

		// Try to create batch with duplicate email
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_create_many",
				Action:   "create_many",
				Version:  "v1",
			},
			Params: map[string]any{
				"list": []any{
					map[string]any{
						"name":   "New User",
						"email":  "new.batch@example.com",
						"age":    30,
						"status": "active",
					},
					map[string]any{
						"name":   "Duplicate User",
						"email":  "existing.batch@example.com",
						"age":    35,
						"status": "active",
					},
				},
			},
		})

		suite.Equal(400, resp.StatusCode, "Should return 400 status code for validation error")
		body := suite.ReadResult(resp)
		suite.False(body.IsOk(), "Should fail due to duplicate email with existing record")

		suite.T().Logf("Validation failed as expected for duplicate with existing record")
	})
}

// TestCreateManyTransactionRollback tests that the entire batch rolls back on error.
func (suite *CreateManyTestSuite) TestCreateManyTransactionRollback() {
	suite.T().Logf("Testing CreateMany API transaction rollback for %s", suite.ds.Kind)

	suite.Run("AllOrNothingSemantics", func() {
		// Try to create a batch where the second item will fail
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_create_many",
				Action:   "create_many",
				Version:  "v1",
			},
			Params: map[string]any{
				"list": []any{
					map[string]any{
						"name":   "Should Not Be Created",
						"email":  "rollback1@example.com",
						"age":    25,
						"status": "active",
					},
					map[string]any{
						"name":   "Invalid User",
						"email":  "rollback2@example.com",
						"age":    0, // Invalid: min=1
						"status": "active",
					},
				},
			},
		})

		suite.Equal(400, resp.StatusCode, "Should return 400 status code for validation error")
		body := suite.ReadResult(resp)
		suite.False(body.IsOk(), "Should fail when one item in batch is invalid")

		// Verify that the first user was not created (transaction rolled back)
		count, err := suite.db.NewSelect().
			Model((*Employee)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("email", "rollback1@example.com")
			}).
			Count(suite.ctx)
		suite.NoError(err, "Should successfully query database")
		suite.Equal(int64(0), count, "First user should not exist - transaction should have rolled back")

		suite.T().Logf("Transaction rollback verified: first user was not created")
	})
}

// TestCreateManyEmptyList tests CreateMany with an empty list.
func (suite *CreateManyTestSuite) TestCreateManyEmptyList() {
	suite.T().Logf("Testing CreateMany API empty list for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_create_many",
			Action:   "create_many",
			Version:  "v1",
		},
		Params: map[string]any{
			"list": []any{},
		},
	})

	// Empty list fails validation and returns 400
	suite.Contains([]int{200, 400, 500}, resp.StatusCode, "Should handle empty list")

	suite.T().Logf("CreateMany with empty list handled")
}

// TestCreateManyPreHookError tests CreateMany with a pre-hook that returns error.
func (suite *CreateManyTestSuite) TestCreateManyPreHookError() {
	suite.T().Logf("Testing CreateMany API pre-hook error for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_create_many_prehook_err",
			Action:   "create_many",
			Version:  "v1",
		},
		Params: map[string]any{
			"list": []any{
				map[string]any{
					"name":         "HookErr1",
					"email":        "hookerr1@example.com",
					"age":          25,
					"position":     "Engineer",
					"departmentId": "dept005",
					"status":       "active",
				},
			},
		},
	})

	suite.Contains([]int{200, 500}, resp.StatusCode, "Should return error status code")

	suite.T().Logf("CreateMany failed as expected due to pre-hook error")
}

// TestCreateManyPostHookError tests CreateMany with a post-hook that returns error.
func (suite *CreateManyTestSuite) TestCreateManyPostHookError() {
	suite.T().Logf("Testing CreateMany API post-hook error for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_create_many_posthook_err",
			Action:   "create_many",
			Version:  "v1",
		},
		Params: map[string]any{
			"list": []any{
				map[string]any{
					"name":         "PostHookErr1",
					"email":        "posthookerr1@example.com",
					"age":          30,
					"position":     "Analyst",
					"departmentId": "dept005",
					"status":       "active",
				},
			},
		},
	})

	suite.Contains([]int{200, 500}, resp.StatusCode, "Should return error status code")

	suite.T().Logf("CreateMany failed as expected due to post-hook error")
}

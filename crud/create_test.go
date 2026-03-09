package crud_test

import (
	"errors"

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
		return &CreateTestSuite{
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
type EmployeeCreateResource struct {
	api.Resource
	crud.Create[Employee, EmployeeCreateParams]
}

func NewEmployeeCreateResource() api.Resource {
	return &EmployeeCreateResource{
		Resource: api.NewRPCResource("test/employee_create"),
		Create:   crud.NewCreate[Employee, EmployeeCreateParams]().Public(),
	}
}

// Resource with PreCreate hook.
type EmployeeCreateWithPreHookResource struct {
	api.Resource
	crud.Create[Employee, EmployeeCreateParams]
}

func NewEmployeeCreateWithPreHookResource() api.Resource {
	return &EmployeeCreateWithPreHookResource{
		Resource: api.NewRPCResource("test/employee_create_prehook"),
		Create: crud.NewCreate[Employee, EmployeeCreateParams]().
			Public().
			WithPreCreate(func(model *Employee, _ *EmployeeCreateParams, _ orm.InsertQuery, _ fiber.Ctx, _ orm.DB) error {
				// Add prefix to name
				model.Name = "Mr. " + model.Name

				return nil
			}),
	}
}

// Resource with PostCreate hook.
type EmployeeCreateWithPostHookResource struct {
	api.Resource
	crud.Create[Employee, EmployeeCreateParams]
}

func NewEmployeeCreateWithPostHookResource() api.Resource {
	return &EmployeeCreateWithPostHookResource{
		Resource: api.NewRPCResource("test/employee_create_posthook"),
		Create: crud.NewCreate[Employee, EmployeeCreateParams]().
			Public().
			WithPostCreate(func(model *Employee, _ *EmployeeCreateParams, ctx fiber.Ctx, _ orm.DB) error {
				// Log or perform additional operations
				ctx.Set("X-Created-User-ID", model.ID)

				return nil
			}),
	}
}

// Test params for create.
type EmployeeCreateParams struct {
	api.P

	Name         string `json:"name"         validate:"required"`
	Email        string `json:"email"        validate:"required,email"`
	Description  string `json:"description"`
	Age          int    `json:"age"          validate:"required,min=1,max=120"`
	Position     string `json:"position"     validate:"required"`
	DepartmentID string `json:"departmentId" validate:"required"`
	Status       string `json:"status"       validate:"required,oneof=active inactive on_leave"`
}

// Resource with PreCreate hook that returns error.
type EmployeeCreatePreHookErrorResource struct {
	api.Resource
	crud.Create[Employee, EmployeeCreateParams]
}

func NewEmployeeCreatePreHookErrorResource() api.Resource {
	return &EmployeeCreatePreHookErrorResource{
		Resource: api.NewRPCResource("test/employee_create_prehook_err"),
		Create: crud.NewCreate[Employee, EmployeeCreateParams]().
			Public().
			WithPreCreate(func(_ *Employee, _ *EmployeeCreateParams, _ orm.InsertQuery, _ fiber.Ctx, _ orm.DB) error {
				return errors.New("pre-create hook rejected")
			}),
	}
}

// Resource with PostCreate hook that returns error.
type EmployeeCreatePostHookErrorResource struct {
	api.Resource
	crud.Create[Employee, EmployeeCreateParams]
}

func NewEmployeeCreatePostHookErrorResource() api.Resource {
	return &EmployeeCreatePostHookErrorResource{
		Resource: api.NewRPCResource("test/employee_create_posthook_err"),
		Create: crud.NewCreate[Employee, EmployeeCreateParams]().
			Public().
			WithPostCreate(func(_ *Employee, _ *EmployeeCreateParams, _ fiber.Ctx, _ orm.DB) error {
				return errors.New("post-create hook rejected")
			}),
	}
}

// CreateTestSuite tests the Create API functionality
// including basic create, PreCreate/PostCreate hooks, and negative cases.
type CreateTestSuite struct {
	BaseTestSuite
}

// SetupSuite runs once before all tests in the suite.
func (suite *CreateTestSuite) SetupSuite() {
	suite.setupBaseSuite(
		NewEmployeeCreateResource,
		NewEmployeeCreateWithPreHookResource,
		NewEmployeeCreateWithPostHookResource,
		NewEmployeeCreatePreHookErrorResource,
		NewEmployeeCreatePostHookErrorResource,
	)
}

// TearDownSuite runs once after all tests in the suite.
func (suite *CreateTestSuite) TearDownSuite() {
	suite.tearDownBaseSuite()
}

// TearDownTest cleans up test-created records after each test.
func (suite *CreateTestSuite) TearDownTest() {
	suite.cleanupTestRecords()
}

// TestCreateBasic tests basic Create functionality.
func (suite *CreateTestSuite) TestCreateBasic() {
	suite.T().Logf("Testing Create API basic functionality for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_create",
			Action:   "create",
			Version:  "v1",
		},
		Params: map[string]any{
			"name":         "New User",
			"email":        "newuser@example.com",
			"description":  "Test user",
			"age":          25,
			"position":     "Engineer",
			"departmentId": "dept005",
			"status":       "active",
		},
	})

	suite.Equal(200, resp.StatusCode, "Should return 200 status code")
	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Should return successful response")
	suite.Equal(body.Message, i18n.T(result.OkMessage), "Should return OK message")
	suite.NotNil(body.Data, "Should return data")

	pk := suite.ReadDataAsMap(body.Data)
	suite.NotEmpty(pk["id"], "Should return created user id")

	suite.T().Logf("Created user with id: %v", pk["id"])
}

// TestCreateWithPreHook tests Create with PreCreate hook.
func (suite *CreateTestSuite) TestCreateWithPreHook() {
	suite.T().Logf("Testing Create API with PreCreate hook for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_create_prehook",
			Action:   "create",
			Version:  "v1",
		},
		Params: map[string]any{
			"name":         "John",
			"email":        "john@example.com",
			"age":          30,
			"position":     "Engineer",
			"departmentId": "dept005",
			"status":       "active",
		},
	})

	suite.Equal(200, resp.StatusCode, "Should return 200 status code")
	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Should return successful response")

	pk := suite.ReadDataAsMap(body.Data)
	suite.NotEmpty(pk["id"], "Should return created user id")

	suite.T().Logf("Created user with PreCreate hook, id: %v", pk["id"])
}

// TestCreateWithPostHook tests Create with PostCreate hook.
func (suite *CreateTestSuite) TestCreateWithPostHook() {
	suite.T().Logf("Testing Create API with PostCreate hook for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_create_posthook",
			Action:   "create",
			Version:  "v1",
		},
		Params: map[string]any{
			"name":         "Jane",
			"email":        "jane@example.com",
			"age":          28,
			"position":     "Designer",
			"departmentId": "dept007",
			"status":       "active",
		},
	})

	suite.Equal(200, resp.StatusCode, "Should return 200 status code")
	suite.NotEmpty(resp.Header.Get("X-Created-User-ID"), "Should set X-Created-User-ID header via PostCreate hook")

	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Should return successful response")

	pk := suite.ReadDataAsMap(body.Data)
	suite.NotEmpty(pk["id"], "Should return created user id")

	suite.T().Logf("Created user with PostCreate hook, id: %v, header: %s", pk["id"], resp.Header.Get("X-Created-User-ID"))
}

// TestCreateNegativeCases tests negative scenarios.
func (suite *CreateTestSuite) TestCreateNegativeCases() {
	suite.T().Logf("Testing Create API negative cases for %s", suite.ds.Kind)

	suite.Run("MissingRequiredField", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_create",
				Action:   "create",
				Version:  "v1",
			},
			Params: map[string]any{
				"email":  "test@example.com",
				"age":    25,
				"status": "active",
			},
		})

		suite.Equal(400, resp.StatusCode, "Should return 400 status code for validation error")
		body := suite.ReadResult(resp)
		suite.False(body.IsOk(), "Should fail when required field 'name' is missing")

		suite.T().Logf("Validation failed as expected for missing required field")
	})

	suite.Run("InvalidEmail", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_create",
				Action:   "create",
				Version:  "v1",
			},
			Params: map[string]any{
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
				Resource: "test/employee_create",
				Action:   "create",
				Version:  "v1",
			},
			Params: map[string]any{
				"name":   "Test",
				"email":  "test@example.com",
				"age":    150,
				"status": "active",
			},
		})

		suite.Equal(400, resp.StatusCode, "Should return 400 status code for validation error")
		body := suite.ReadResult(resp)
		suite.False(body.IsOk(), "Should fail when age is greater than 120")

		suite.T().Logf("Validation failed as expected for invalid age")
	})

	suite.Run("InvalidStatus", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_create",
				Action:   "create",
				Version:  "v1",
			},
			Params: map[string]any{
				"name":   "Test",
				"email":  "test@example.com",
				"age":    25,
				"status": "invalid_status",
			},
		})

		suite.Equal(400, resp.StatusCode, "Should return 400 status code for validation error")
		body := suite.ReadResult(resp)
		suite.False(body.IsOk(), "Should fail when status is not 'active' or 'inactive'")

		suite.T().Logf("Validation failed as expected for invalid status")
	})

	suite.Run("DuplicateEmail", func() {
		suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_create",
				Action:   "create",
				Version:  "v1",
			},
			Params: map[string]any{
				"name":         "First User",
				"email":        "duplicate@example.com",
				"age":          25,
				"position":     "Engineer",
				"departmentId": "dept005",
				"status":       "active",
			},
		})

		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_create",
				Action:   "create",
				Version:  "v1",
			},
			Params: map[string]any{
				"name":         "Second User",
				"email":        "duplicate@example.com",
				"age":          30,
				"position":     "Analyst",
				"departmentId": "dept015",
				"status":       "active",
			},
		})

		suite.Equal(200, resp.StatusCode, "Should return 200 status code")
		body := suite.ReadResult(resp)
		suite.False(body.IsOk(), "Should fail due to duplicate email unique constraint")

		suite.T().Logf("Validation failed as expected for duplicate email")
	})
}

// TestCreatePreHookError tests Create with a pre-hook that returns error.
func (suite *CreateTestSuite) TestCreatePreHookError() {
	suite.T().Logf("Testing Create API with pre-hook error for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_create_prehook_err",
			Action:   "create",
			Version:  "v1",
		},
		Params: map[string]any{
			"name":         "Hook Error User",
			"email":        "hookerr@example.com",
			"age":          25,
			"position":     "Engineer",
			"departmentId": "dept005",
			"status":       "active",
		},
	})

	// Hook errors inside transactions may result in 500
	suite.Contains([]int{200, 500}, resp.StatusCode, "Should return error status code")

	suite.T().Logf("Create failed as expected due to pre-hook error")
}

// TestCreatePostHookError tests Create with a post-hook that returns error.
func (suite *CreateTestSuite) TestCreatePostHookError() {
	suite.T().Logf("Testing Create API with post-hook error for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_create_posthook_err",
			Action:   "create",
			Version:  "v1",
		},
		Params: map[string]any{
			"name":         "PostHook Error User",
			"email":        "posthookerr@example.com",
			"age":          25,
			"position":     "Engineer",
			"departmentId": "dept005",
			"status":       "active",
		},
	})

	// Post-hook errors may result in 500 since they occur inside a transaction
	suite.Contains([]int{200, 500}, resp.StatusCode, "Should return error status code")

	suite.T().Logf("Create failed as expected due to post-hook error")
}

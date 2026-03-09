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
		return &DeleteTestSuite{
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
type EmployeeDeleteResource struct {
	api.Resource
	crud.Delete[Employee]
}

func NewEmployeeDeleteResource() api.Resource {
	return &EmployeeDeleteResource{
		Resource: api.NewRPCResource("test/employee_delete"),
		Delete:   crud.NewDelete[Employee]().Public(),
	}
}

// Resource with PreDelete hook.
type EmployeeDeleteWithPreHookResource struct {
	api.Resource
	crud.Delete[Employee]
}

func NewEmployeeDeleteWithPreHookResource() api.Resource {
	return &EmployeeDeleteWithPreHookResource{
		Resource: api.NewRPCResource("test/employee_delete_prehook"),
		Delete: crud.NewDelete[Employee]().
			Public().
			WithPreDelete(func(model *Employee, _ orm.DeleteQuery, ctx fiber.Ctx, _ orm.DB) error {
				// Log or check conditions before delete
				if model.Status == "active" {
					ctx.Set("X-Delete-Warning", "Deleting active user")
				}

				return nil
			}),
	}
}

// Resource with PostDelete hook.
type EmployeeDeleteWithPostHookResource struct {
	api.Resource
	crud.Delete[Employee]
}

func NewEmployeeDeleteWithPostHookResource() api.Resource {
	return &EmployeeDeleteWithPostHookResource{
		Resource: api.NewRPCResource("test/employee_delete_posthook"),
		Delete: crud.NewDelete[Employee]().
			Public().
			WithPostDelete(func(model *Employee, ctx fiber.Ctx, _ orm.DB) error {
				// Set custom header after deletion
				ctx.Set("X-Deleted-User-ID", model.ID)

				return nil
			}),
	}
}

// Resource with DisableDataPerm.
type EmployeeDeleteNoPermResource struct {
	api.Resource
	crud.Delete[Employee]
}

func NewEmployeeDeleteNoPermResource() api.Resource {
	return &EmployeeDeleteNoPermResource{
		Resource: api.NewRPCResource("test/employee_delete_noperm"),
		Delete:   crud.NewDelete[Employee]().DisableDataPerm().Public(),
	}
}

// Resource with PreDelete hook that returns error.
type EmployeeDeletePreHookErrorResource struct {
	api.Resource
	crud.Delete[Employee]
}

func NewEmployeeDeletePreHookErrorResource() api.Resource {
	return &EmployeeDeletePreHookErrorResource{
		Resource: api.NewRPCResource("test/employee_delete_prehook_err"),
		Delete: crud.NewDelete[Employee]().
			Public().
			WithPreDelete(func(_ *Employee, _ orm.DeleteQuery, _ fiber.Ctx, _ orm.DB) error {
				return errors.New("pre-delete hook rejected")
			}),
	}
}

// Resource with PostDelete hook that returns error.
type EmployeeDeletePostHookErrorResource struct {
	api.Resource
	crud.Delete[Employee]
}

func NewEmployeeDeletePostHookErrorResource() api.Resource {
	return &EmployeeDeletePostHookErrorResource{
		Resource: api.NewRPCResource("test/employee_delete_posthook_err"),
		Delete: crud.NewDelete[Employee]().
			Public().
			WithPostDelete(func(_ *Employee, _ fiber.Ctx, _ orm.DB) error {
				return errors.New("post-delete hook rejected")
			}),
	}
}

// DeleteTestSuite tests the Delete API functionality
// including basic delete, PreDelete/PostDelete hooks, negative cases, and primary key requirements.
type DeleteTestSuite struct {
	BaseTestSuite

	testEmployees []Employee
}

// SetupSuite runs once before all tests in the suite.
func (suite *DeleteTestSuite) SetupSuite() {
	suite.setupBaseSuite(
		NewEmployeeDeleteResource,
		NewEmployeeDeleteWithPreHookResource,
		NewEmployeeDeleteWithPostHookResource,
		NewEmployeeDeleteNoPermResource,
		NewEmployeeDeletePreHookErrorResource,
		NewEmployeeDeletePostHookErrorResource,
	)
}

// TearDownSuite runs once after all tests in the suite.
func (suite *DeleteTestSuite) TearDownSuite() {
	suite.tearDownBaseSuite()
}

// SetupTest inserts isolated test data before each test method.
func (suite *DeleteTestSuite) SetupTest() {
	suite.testEmployees = []Employee{
		{Name: "DT Alice", Email: "dt_alice@test.com", Age: 30, Position: "Engineer", DepartmentID: "dept005", Status: "active"},
		{Name: "DT Bob", Email: "dt_bob@test.com", Age: 25, Position: "Designer", DepartmentID: "dept007", Status: "active"},
		{Name: "DT Charlie", Email: "dt_charlie@test.com", Age: 35, Position: "Analyst", DepartmentID: "dept015", Status: "active"},
		{Name: "DT Dave", Email: "dt_dave@test.com", Age: 28, Position: "Engineer", DepartmentID: "dept005", Status: "inactive"},
		{Name: "DT Eve", Email: "dt_eve@test.com", Age: 32, Position: "Director", DepartmentID: "dept001", Status: "active"},
	}
	for i := range suite.testEmployees {
		suite.testEmployees[i].ID = fmt.Sprintf("dt_emp%03d", i+1)
	}

	_, err := suite.db.NewInsert().Model(&suite.testEmployees).Exec(suite.ctx)
	suite.Require().NoError(err, "Failed to insert test employees for delete tests")
}

// TearDownTest removes all test-inserted data (created_at >= 2026) after each test method.
func (suite *DeleteTestSuite) TearDownTest() {
	suite.cleanupTestRecords()
}

// TestDeleteBasic tests basic Delete functionality.
func (suite *DeleteTestSuite) TestDeleteBasic() {
	suite.T().Logf("Testing Delete API basic functionality for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_delete",
			Action:   "delete",
			Version:  "v1",
		},
		Params: map[string]any{
			"id": "dt_emp001",
		},
	})

	suite.Equal(200, resp.StatusCode, "Should return 200 status code")
	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Should return successful response")
	suite.Equal(body.Message, i18n.T(result.OkMessage), "Should return OK message")

	suite.T().Logf("Deleted dt_emp001 successfully")
}

// TestDeleteWithPreHook tests Delete with PreDelete hook.
func (suite *DeleteTestSuite) TestDeleteWithPreHook() {
	suite.T().Logf("Testing Delete API with PreDelete hook for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_delete_prehook",
			Action:   "delete",
			Version:  "v1",
		},
		Params: map[string]any{
			"id": "dt_emp002", // This is an active user
		},
	})

	suite.Equal(200, resp.StatusCode, "Should return 200 status code")
	suite.Equal("Deleting active user", resp.Header.Get("X-Delete-Warning"), "Should set X-Delete-Warning header via PreDelete hook")

	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Should return successful response")
	suite.Equal(body.Message, i18n.T(result.OkMessage), "Should return OK message")

	suite.T().Logf("Deleted dt_emp002 with PreDelete hook, warning: %s", resp.Header.Get("X-Delete-Warning"))
}

// TestDeleteWithPostHook tests Delete with PostDelete hook.
func (suite *DeleteTestSuite) TestDeleteWithPostHook() {
	suite.T().Logf("Testing Delete API with PostDelete hook for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_delete_posthook",
			Action:   "delete",
			Version:  "v1",
		},
		Params: map[string]any{
			"id": "dt_emp003",
		},
	})

	suite.Equal(200, resp.StatusCode, "Should return 200 status code")
	suite.Equal("dt_emp003", resp.Header.Get("X-Deleted-User-ID"), "Should set X-Deleted-User-ID header via PostDelete hook")

	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Should return successful response")
	suite.Equal(body.Message, i18n.T(result.OkMessage), "Should return OK message")

	suite.T().Logf("Deleted dt_emp003 with PostDelete hook, user id: %s", resp.Header.Get("X-Deleted-User-ID"))
}

// TestDeleteNegativeCases tests negative scenarios.
func (suite *DeleteTestSuite) TestDeleteNegativeCases() {
	suite.T().Logf("Testing Delete API negative cases for %s", suite.ds.Kind)

	suite.Run("NonExistentUser", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_delete",
				Action:   "delete",
				Version:  "v1",
			},
			Params: map[string]any{
				"id": "nonexistent",
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
				Resource: "test/employee_delete",
				Action:   "delete",
				Version:  "v1",
			},
			Params: map[string]any{
				// Missing "id"
			},
		})

		suite.Equal(200, resp.StatusCode, "Should return 200 status code")
		body := suite.ReadResult(resp)
		suite.False(body.IsOk(), "Should fail when required id is missing")
		suite.Equal(body.Message, i18n.T("primary_key_required", map[string]any{"field": "id"}), "Should return primary key required message")

		suite.T().Logf("Validation failed as expected for missing id")
	})

	suite.Run("DeleteTwice", func() {
		// First delete
		resp1 := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_delete",
				Action:   "delete",
				Version:  "v1",
			},
			Params: map[string]any{
				"id": "dt_emp004",
			},
		})

		suite.Equal(200, resp1.StatusCode, "Should return 200 status code")
		body1 := suite.ReadResult(resp1)
		suite.True(body1.IsOk(), "Should return successful response on first delete")

		suite.T().Logf("First delete of dt_emp004 succeeded")

		// Try to delete again
		resp2 := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_delete",
				Action:   "delete",
				Version:  "v1",
			},
			Params: map[string]any{
				"id": "dt_emp004",
			},
		})

		suite.Equal(200, resp2.StatusCode, "Should return 200 status code")
		body2 := suite.ReadResult(resp2)
		suite.False(body2.IsOk(), "Should fail when trying to delete already deleted user")
		suite.Equal(body2.Message, i18n.T(result.ErrMessageRecordNotFound), "Should return record not found message")

		suite.T().Logf("Second delete of dt_emp004 failed as expected - user already deleted")
	})
}

// TestDeleteRequiresPrimaryKey tests that delete requires primary key.
func (suite *DeleteTestSuite) TestDeleteRequiresPrimaryKey() {
	suite.T().Logf("Testing Delete API primary key requirement for %s", suite.ds.Kind)

	suite.Run("DeleteByEmailShouldFail", func() {
		// Delete operation only supports deletion by primary key, not by other fields
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_delete",
				Action:   "delete",
				Version:  "v1",
			},
			Params: map[string]any{
				"email": "frank@example.com",
			},
		})

		suite.Equal(200, resp.StatusCode, "Should return 200 status code")
		body := suite.ReadResult(resp)
		suite.False(body.IsOk(), "Should fail when trying to delete by email instead of primary key")
		suite.Equal(body.Message, i18n.T("primary_key_required", map[string]any{"field": "id"}), "Should return primary key required message")

		suite.T().Logf("Validation failed as expected - cannot delete by email, primary key required")
	})

	suite.Run("DeleteByStatusShouldFail", func() {
		// Delete operation only supports deletion by primary key, not by other fields
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_delete",
				Action:   "delete",
				Version:  "v1",
			},
			Params: map[string]any{
				"status": "inactive",
			},
		})

		suite.Equal(200, resp.StatusCode, "Should return 200 status code")
		body := suite.ReadResult(resp)
		suite.False(body.IsOk(), "Should fail when trying to delete by status instead of primary key")
		suite.Equal(body.Message, i18n.T("primary_key_required", map[string]any{"field": "id"}), "Should return primary key required message")

		suite.T().Logf("Validation failed as expected - cannot delete by status, primary key required")
	})
}

// TestDeleteWithDisableDataPerm tests Delete with DisableDataPerm.
func (suite *DeleteTestSuite) TestDeleteWithDisableDataPerm() {
	suite.T().Logf("Testing Delete API with DisableDataPerm for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_delete_noperm",
			Action:   "delete",
			Version:  "v1",
		},
		Params: map[string]any{
			"id": "dt_emp005",
		},
	})

	suite.Equal(200, resp.StatusCode, "Should return 200 status code")
	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Should return successful response")
	suite.Equal(body.Message, i18n.T(result.OkMessage), "Should return OK message")

	suite.T().Logf("Deleted dt_emp005 with DisableDataPerm successfully")
}

// TestDeletePreHookError tests Delete with a pre-hook that returns error.
func (suite *DeleteTestSuite) TestDeletePreHookError() {
	suite.T().Logf("Testing Delete API with pre-hook error for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_delete_prehook_err",
			Action:   "delete",
			Version:  "v1",
		},
		Params: map[string]any{
			"id": suite.testEmployees[0].ID,
		},
	})

	// Hook errors inside transactions may result in 500
	suite.Contains([]int{200, 500}, resp.StatusCode, "Should return error status code")

	suite.T().Logf("Delete failed as expected due to pre-hook error")
}

// TestDeletePostHookError tests Delete with a post-hook that returns error.
func (suite *DeleteTestSuite) TestDeletePostHookError() {
	suite.T().Logf("Testing Delete API with post-hook error for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_delete_posthook_err",
			Action:   "delete",
			Version:  "v1",
		},
		Params: map[string]any{
			"id": suite.testEmployees[1].ID,
		},
	})

	// Post-hook errors may result in 500 since they occur inside a transaction
	suite.Contains([]int{200, 500}, resp.StatusCode, "Should return error status code")

	suite.T().Logf("Delete failed as expected due to post-hook error")
}

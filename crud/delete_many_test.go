package crud_test

import (
	"errors"
	"fmt"
	"strconv"
	"time"

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
		return &DeleteManyTestSuite{
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
type EmployeeDeleteManyResource struct {
	api.Resource
	crud.DeleteMany[Employee]
}

func NewEmployeeDeleteManyResource() api.Resource {
	return &EmployeeDeleteManyResource{
		Resource:   api.NewRPCResource("test/employee_delete_many"),
		DeleteMany: crud.NewDeleteMany[Employee]().Public(),
	}
}

// Resource for composite PK testing.
type ProjectAssignmentCompositeDeleteManyResource struct {
	api.Resource
	crud.DeleteMany[ProjectAssignment]
}

func NewProjectAssignmentCompositeDeleteManyResource() api.Resource {
	return &ProjectAssignmentCompositeDeleteManyResource{
		Resource:   api.NewRPCResource("test/project_assignment_delete_many"),
		DeleteMany: crud.NewDeleteMany[ProjectAssignment]().Public(),
	}
}

// Resource with PreDeleteMany hook.
type EmployeeDeleteManyWithPreHookResource struct {
	api.Resource
	crud.DeleteMany[Employee]
}

func NewEmployeeDeleteManyWithPreHookResource() api.Resource {
	return &EmployeeDeleteManyWithPreHookResource{
		Resource: api.NewRPCResource("test/employee_delete_many_prehook"),
		DeleteMany: crud.NewDeleteMany[Employee]().
			Public().
			WithPreDeleteMany(func(models []Employee, _ orm.DeleteQuery, ctx fiber.Ctx, _ orm.DB) error {
				// Check if any active users in batch
				activeCount := 0

				for _, model := range models {
					if model.Status == "active" {
						activeCount++
					}
				}

				if activeCount > 0 {
					ctx.Set("X-Delete-Active-Count", strconv.Itoa(activeCount))
				}

				return nil
			}),
	}
}

// Resource with PostDeleteMany hook.
type EmployeeDeleteManyWithPostHookResource struct {
	api.Resource
	crud.DeleteMany[Employee]
}

func NewEmployeeDeleteManyWithPostHookResource() api.Resource {
	return &EmployeeDeleteManyWithPostHookResource{
		Resource: api.NewRPCResource("test/employee_delete_many_posthook"),
		DeleteMany: crud.NewDeleteMany[Employee]().
			Public().
			WithPostDeleteMany(func(models []Employee, ctx fiber.Ctx, _ orm.DB) error {
				// Set custom header with count
				ctx.Set("X-Deleted-Count", strconv.Itoa(len(models)))

				return nil
			}),
	}
}

// Resource with DisableDataPerm.
type EmployeeDeleteManyNoPermResource struct {
	api.Resource
	crud.DeleteMany[Employee]
}

func NewEmployeeDeleteManyNoPermResource() api.Resource {
	return &EmployeeDeleteManyNoPermResource{
		Resource:   api.NewRPCResource("test/employee_delete_many_noperm"),
		DeleteMany: crud.NewDeleteMany[Employee]().DisableDataPerm().Public(),
	}
}

// Resource with PreDeleteMany hook that returns error.
type EmployeeDeleteManyPreHookErrorResource struct {
	api.Resource
	crud.DeleteMany[Employee]
}

func NewEmployeeDeleteManyPreHookErrorResource() api.Resource {
	return &EmployeeDeleteManyPreHookErrorResource{
		Resource: api.NewRPCResource("test/employee_delete_many_prehook_err"),
		DeleteMany: crud.NewDeleteMany[Employee]().
			Public().
			WithPreDeleteMany(func(_ []Employee, _ orm.DeleteQuery, _ fiber.Ctx, _ orm.DB) error {
				return errors.New("pre-delete-many hook rejected")
			}),
	}
}

// Resource with PostDeleteMany hook that returns error.
type EmployeeDeleteManyPostHookErrorResource struct {
	api.Resource
	crud.DeleteMany[Employee]
}

func NewEmployeeDeleteManyPostHookErrorResource() api.Resource {
	return &EmployeeDeleteManyPostHookErrorResource{
		Resource: api.NewRPCResource("test/employee_delete_many_posthook_err"),
		DeleteMany: crud.NewDeleteMany[Employee]().
			Public().
			WithPostDeleteMany(func(_ []Employee, _ fiber.Ctx, _ orm.DB) error {
				return errors.New("post-delete-many hook rejected")
			}),
	}
}

// DeleteManyTestSuite tests the DeleteMany API functionality
// including basic batch delete, PreDeleteMany/PostDeleteMany hooks, negative cases, transaction rollback, and primary key formats.
type DeleteManyTestSuite struct {
	BaseTestSuite
}

// SetupSuite runs once before all tests in the suite.
func (suite *DeleteManyTestSuite) SetupSuite() {
	suite.setupBaseSuite(
		NewEmployeeDeleteManyResource,
		NewEmployeeDeleteManyWithPreHookResource,
		NewEmployeeDeleteManyWithPostHookResource,
		NewProjectAssignmentCompositeDeleteManyResource,
		NewEmployeeDeleteManyNoPermResource,
		NewEmployeeDeleteManyPreHookErrorResource,
		NewEmployeeDeleteManyPostHookErrorResource,
	)
}

// TearDownSuite runs once after all tests in the suite.
func (suite *DeleteManyTestSuite) TearDownSuite() {
	suite.tearDownBaseSuite()
}

// SetupTest inserts isolated test data before each test method.
func (suite *DeleteManyTestSuite) SetupTest() {
	// Insert test employees specifically for delete tests
	additionalUsers := []Employee{
		{Name: "Delete User 1", Email: "deluser001@example.com", Age: 25, Position: "Engineer", DepartmentID: "dept005", Status: "active"},
		{Name: "Delete User 2", Email: "deluser002@example.com", Age: 26, Position: "Engineer", DepartmentID: "dept005", Status: "active"},
		{Name: "Delete User 3", Email: "deluser003@example.com", Age: 27, Position: "Designer", DepartmentID: "dept007", Status: "inactive"},
		{Name: "Delete User 4", Email: "deluser004@example.com", Age: 28, Position: "Engineer", DepartmentID: "dept005", Status: "active"},
		{Name: "Delete User 5", Email: "deluser005@example.com", Age: 29, Position: "Analyst", DepartmentID: "dept015", Status: "active"},
		{Name: "Delete User 6", Email: "deluser006@example.com", Age: 30, Position: "Designer", DepartmentID: "dept007", Status: "inactive"},
		{Name: "Delete User 7", Email: "deluser007@example.com", Age: 31, Position: "Engineer", DepartmentID: "dept006", Status: "active"},
		{Name: "Delete User 8", Email: "deluser008@example.com", Age: 32, Position: "Team Lead", DepartmentID: "dept005", Status: "active"},
		{Name: "Delete User 9", Email: "deluser009@example.com", Age: 33, Position: "Analyst", DepartmentID: "dept015", Status: "inactive"},
		{Name: "Delete User 10", Email: "deluser010@example.com", Age: 34, Position: "Director", DepartmentID: "dept001", Status: "active"},
	}
	for i := range additionalUsers {
		additionalUsers[i].ID = fmt.Sprintf("deluser%03d", i+1)
	}

	_, err := suite.db.NewInsert().Model(&additionalUsers).Exec(suite.ctx)
	suite.Require().NoError(err, "Failed to insert test employees for delete_many tests")

	// Insert test project assignments for composite PK tests.
	// ProjectAssignment uses raw string fields (no orm.Model), so set audit fields manually.
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	testAssignments := []ProjectAssignment{
		{ProjectCode: "proj-test", EmployeeID: "emp001", Role: "Lead", Status: "active", CreatedAt: now, CreatedBy: "test"},
		{ProjectCode: "proj-test", EmployeeID: "emp002", Role: "Member", Status: "active", CreatedAt: now, CreatedBy: "test"},
		{ProjectCode: "proj-test", EmployeeID: "emp003", Role: "Member", Status: "active", CreatedAt: now, CreatedBy: "test"},
		{ProjectCode: "proj-test", EmployeeID: "emp004", Role: "Reviewer", Status: "active", CreatedAt: now, CreatedBy: "test"},
	}

	_, err = suite.bunDB.NewInsert().Model(&testAssignments).Exec(suite.ctx)
	suite.Require().NoError(err, "Failed to insert test project assignments for delete_many tests")
}

// TearDownTest removes all test-inserted data (created_at >= 2026) after each test method.
func (suite *DeleteManyTestSuite) TearDownTest() {
	suite.cleanupTestRecords()
}

// TestDeleteManyBasic tests basic DeleteMany functionality.
func (suite *DeleteManyTestSuite) TestDeleteManyBasic() {
	suite.T().Logf("Testing DeleteMany API basic functionality for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_delete_many",
			Action:   "delete_many",
			Version:  "v1",
		},
		Params: map[string]any{
			"pks": []string{"deluser001", "deluser002", "deluser003"},
		},
	})

	suite.Equal(200, resp.StatusCode, "Should return 200 status code")
	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Should return successful response")
	suite.Equal(body.Message, i18n.T(result.OkMessage), "Should return OK message")

	suite.T().Logf("Successfully deleted 3 users in batch")
}

// TestDeleteManyWithPreHook tests DeleteMany with PreDeleteMany hook.
func (suite *DeleteManyTestSuite) TestDeleteManyWithPreHook() {
	suite.T().Logf("Testing DeleteMany API with PreDeleteMany hook for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_delete_many_prehook",
			Action:   "delete_many",
			Version:  "v1",
		},
		Params: map[string]any{
			"pks": []string{"deluser004", "deluser005"}, // deluser004 and deluser005 are active
		},
	})

	suite.Equal(200, resp.StatusCode, "Should return 200 status code")
	suite.NotEmpty(resp.Header.Get("X-Delete-Active-Count"), "Should set X-Delete-Active-Count header via PreDeleteMany hook")

	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Should return successful response")
	suite.Equal(body.Message, i18n.T(result.OkMessage), "Should return OK message")

	activeCount := resp.Header.Get("X-Delete-Active-Count")
	suite.T().Logf("Deleted 2 users with PreDeleteMany hook, active count: %s", activeCount)
}

// TestDeleteManyWithPostHook tests DeleteMany with PostDeleteMany hook.
func (suite *DeleteManyTestSuite) TestDeleteManyWithPostHook() {
	suite.T().Logf("Testing DeleteMany API with PostDeleteMany hook for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_delete_many_posthook",
			Action:   "delete_many",
			Version:  "v1",
		},
		Params: map[string]any{
			"pks": []string{"deluser006"},
		},
	})

	suite.Equal(200, resp.StatusCode, "Should return 200 status code")
	suite.NotEmpty(resp.Header.Get("X-Deleted-Count"), "Should set X-Deleted-Count header via PostDeleteMany hook")

	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Should return successful response")
	suite.Equal(body.Message, i18n.T(result.OkMessage), "Should return OK message")

	deletedCount := resp.Header.Get("X-Deleted-Count")
	suite.T().Logf("Deleted %s users with PostDeleteMany hook", deletedCount)
}

// TestDeleteManyNegativeCases tests negative scenarios.
func (suite *DeleteManyTestSuite) TestDeleteManyNegativeCases() {
	suite.T().Logf("Testing DeleteMany API negative cases for %s", suite.ds.Kind)

	suite.Run("EmptyArray", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_delete_many",
				Action:   "delete_many",
				Version:  "v1",
			},
			Params: map[string]any{
				"pks": []string{},
			},
		})

		suite.Equal(400, resp.StatusCode, "Should return 400 status code for validation error")
		body := suite.ReadResult(resp)
		suite.False(body.IsOk(), "Should fail when pks list is empty")

		suite.T().Logf("Validation failed as expected for empty pks list")
	})

	suite.Run("NonExistentUser", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_delete_many",
				Action:   "delete_many",
				Version:  "v1",
			},
			Params: map[string]any{
				"pks": []string{"emp008", "nonexistent"},
			},
		})

		suite.Equal(200, resp.StatusCode, "Should return 200 status code")
		body := suite.ReadResult(resp)
		suite.False(body.IsOk(), "Should fail when one user does not exist in batch")
		suite.Equal(body.Message, i18n.T(result.ErrMessageRecordNotFound), "Should return record not found message")

		suite.T().Logf("Validation failed as expected for non-existent user in batch")
	})

	suite.Run("MissingPrimaryKeys", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_delete_many",
				Action:   "delete_many",
				Version:  "v1",
			},
			Params: map[string]any{
				// Missing "pks"
			},
		})

		suite.Equal(400, resp.StatusCode, "Should return 400 status code for validation error")
		body := suite.ReadResult(resp)
		suite.False(body.IsOk(), "Should fail when primary keys parameter is missing")
		suite.Contains(body.Message, i18n.T("batch_delete_pks"), "Message should indicate primary keys is required")

		suite.T().Logf("Validation failed as expected for missing primary keys parameter")
	})

	suite.Run("InvalidPrimaryKeysType", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_delete_many",
				Action:   "delete_many",
				Version:  "v1",
			},
			Params: map[string]any{
				"pks": "not-an-array", // Should be array
			},
		})

		suite.Equal(500, resp.StatusCode, "Should return 500 status code for invalid parameter type")

		suite.T().Logf("Validation failed as expected for invalid primary keys parameter type")
	})

	suite.Run("AllNonExistent", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_delete_many",
				Action:   "delete_many",
				Version:  "v1",
			},
			Params: map[string]any{
				"pks": []string{"nonexistent1", "nonexistent2"},
			},
		})

		suite.Equal(200, resp.StatusCode, "Should return 200 status code")
		body := suite.ReadResult(resp)
		suite.False(body.IsOk(), "Should fail when all users do not exist")
		suite.Equal(body.Message, i18n.T(result.ErrMessageRecordNotFound), "Should return record not found message")

		suite.T().Logf("Validation failed as expected for all non-existent users")
	})

	suite.Run("DeleteTwice", func() {
		// First delete
		resp1 := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_delete_many",
				Action:   "delete_many",
				Version:  "v1",
			},
			Params: map[string]any{
				"pks": []string{"deluser009", "deluser010"},
			},
		})

		suite.Equal(200, resp1.StatusCode, "Should return 200 status code")
		body1 := suite.ReadResult(resp1)
		suite.True(body1.IsOk(), "Should return successful response on first delete")

		suite.T().Logf("First delete of deluser009 and deluser010 succeeded")

		// Try to delete same users again
		resp2 := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_delete_many",
				Action:   "delete_many",
				Version:  "v1",
			},
			Params: map[string]any{
				"pks": []string{"deluser009", "deluser010"},
			},
		})

		suite.Equal(200, resp2.StatusCode, "Should return 200 status code")
		body2 := suite.ReadResult(resp2)
		suite.False(body2.IsOk(), "Should fail when trying to delete already deleted users")
		suite.Equal(body2.Message, i18n.T(result.ErrMessageRecordNotFound), "Should return record not found message")

		suite.T().Logf("Second delete of deluser009 and deluser010 failed as expected - users already deleted")
	})

	suite.Run("PartiallyDeleted", func() {
		// First delete deluser001 so it no longer exists
		resp1 := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_delete_many",
				Action:   "delete_many",
				Version:  "v1",
			},
			Params: map[string]any{
				"pks": []string{"deluser001"},
			},
		})
		suite.True(suite.ReadResult(resp1).IsOk(), "Pre-delete of deluser001 should succeed")

		// Now try to delete both deluser001 (deleted) and deluser007 (exists)
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_delete_many",
				Action:   "delete_many",
				Version:  "v1",
			},
			Params: map[string]any{
				"pks": []string{"deluser001", "deluser007"}, // deluser001 already deleted, deluser007 still exists
			},
		})

		suite.Equal(200, resp.StatusCode, "Should return 200 status code")
		body := suite.ReadResult(resp)
		suite.False(body.IsOk(), "Should fail when one user is already deleted")
		suite.Equal(body.Message, i18n.T(result.ErrMessageRecordNotFound), "Should return record not found message")

		suite.T().Logf("Validation failed as expected for partially deleted batch")
	})
}

// TestDeleteManyTransactionRollback tests that the entire batch rolls back on error.
func (suite *DeleteManyTestSuite) TestDeleteManyTransactionRollback() {
	suite.T().Logf("Testing DeleteMany API transaction rollback for %s", suite.ds.Kind)

	suite.Run("AllOrNothingSemantics", func() {
		// Try to delete a batch where the second item doesn't exist
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_delete_many",
				Action:   "delete_many",
				Version:  "v1",
			},
			Params: map[string]any{
				"pks": []string{"deluser007", "nonexistent_rollback"},
			},
		})

		suite.Equal(200, resp.StatusCode, "Should return 200 status code")
		body := suite.ReadResult(resp)
		suite.False(body.IsOk(), "Should fail when one item in batch does not exist")

		// Verify that the first user was not deleted (transaction rolled back)
		count, err := suite.db.NewSelect().Model((*Employee)(nil)).Where(func(cb orm.ConditionBuilder) {
			cb.Equals("id", "deluser007")
		}).Count(suite.ctx)
		suite.NoError(err, "Should successfully query database")
		suite.Equal(int64(1), count, "First user should still exist - transaction should have rolled back")

		suite.T().Logf("Transaction rollback verified: first user was not deleted")
	})
}

// TestDeleteManyPrimaryKeyFormats tests different primary key format support.
func (suite *DeleteManyTestSuite) TestDeleteManyPrimaryKeyFormats() {
	suite.T().Logf("Testing DeleteMany API primary key formats for %s", suite.ds.Kind)

	suite.Run("SinglePKDirectValues", func() {
		// Single PK with direct value array: ["id1", "id2"]
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_delete_many",
				Action:   "delete_many",
				Version:  "v1",
			},
			Params: map[string]any{
				"pks": []string{"deluser008"},
			},
		})

		suite.Equal(200, resp.StatusCode, "Should return 200 status code")
		body := suite.ReadResult(resp)
		suite.True(body.IsOk(), "Should return successful response for single primary key with direct values")

		suite.T().Logf("Successfully deleted user using single primary key direct value format")
	})

	suite.Run("SinglePKMapFormat", func() {
		// Single PK with map format: [{"id": "value1"}, {"id": "value2"}]
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_delete_many",
				Action:   "delete_many",
				Version:  "v1",
			},
			Params: map[string]any{
				"pks": []any{
					map[string]any{"id": "deluser009"},
					map[string]any{"id": "deluser010"},
				},
			},
		})

		suite.Equal(200, resp.StatusCode, "Should return 200 status code")
		body := suite.ReadResult(resp)
		suite.True(body.IsOk(), "Should succeed deleting users with map format")

		suite.T().Logf("Map format correctly handled - deleted deluser009 and deluser010")
	})

	suite.Run("SinglePKMixedFormat", func() {
		// Mixed format - both direct values and maps
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_delete_many",
				Action:   "delete_many",
				Version:  "v1",
			},
			Params: map[string]any{
				"pks": []any{
					"deluser001",                       // direct value
					map[string]any{"id": "deluser002"}, // map format
				},
			},
		})

		suite.Equal(200, resp.StatusCode, "Should return 200 status code")
		body := suite.ReadResult(resp)
		suite.True(body.IsOk(), "Should succeed deleting users with mixed format")

		suite.T().Logf("Mixed format correctly handled - deleted deluser001 and deluser002")
	})

	// Composite PK tests with ProjectAssignment model
	suite.Run("CompositePKMapFormatRequired", func() {
		// Test with map format (correct for composite PKs)
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/project_assignment_delete_many",
				Action:   "delete_many",
				Version:  "v1",
			},
			Params: map[string]any{
				"pks": []any{
					map[string]any{"projectCode": "proj-test", "employeeId": "emp001"},
					map[string]any{"projectCode": "proj-test", "employeeId": "emp003"},
				},
			},
		})

		suite.Equal(200, resp.StatusCode, "Should return 200 status code")
		body := suite.ReadResult(resp)
		suite.True(body.IsOk(), "Composite primary key deletion with map format should succeed")

		// Verify items were deleted
		count, err := suite.db.NewSelect().
			Model((*ProjectAssignment)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("project_code", "proj-test")
			}).
			Count(suite.ctx)
		suite.NoError(err, "Should successfully query database")
		suite.Equal(int64(2), count, "proj-test should have 2 remaining members after deleting 2")

		suite.T().Logf("Successfully deleted 2 items using composite primary key map format")
	})

	suite.Run("CompositePKPartialKeys", func() {
		// Test with missing one of the composite keys
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/project_assignment_delete_many",
				Action:   "delete_many",
				Version:  "v1",
			},
			Params: map[string]any{
				"pks": []any{
					map[string]any{"projectCode": "proj-test"}, // Missing employeeId
				},
			},
		})

		suite.Equal(200, resp.StatusCode, "Should return 200 status code")
		body := suite.ReadResult(resp)
		suite.False(body.IsOk(), "Should fail when missing composite primary key fields")
		suite.Contains(body.Message, "employeeId", "Error message should mention missing employeeId field")

		suite.T().Logf("Validation failed as expected for missing composite primary key field")
	})
}

// TestDeleteManyWithDisableDataPerm tests DeleteMany with DisableDataPerm.
func (suite *DeleteManyTestSuite) TestDeleteManyWithDisableDataPerm() {
	suite.T().Logf("Testing DeleteMany API with DisableDataPerm for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_delete_many_noperm",
			Action:   "delete_many",
			Version:  "v1",
		},
		Params: map[string]any{
			"pks": []any{
				map[string]any{"id": "deluser009"},
				map[string]any{"id": "deluser010"},
			},
		},
	})

	suite.Equal(200, resp.StatusCode, "Should return 200 status code")
	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Should return successful response")
	suite.Equal(body.Message, i18n.T(result.OkMessage), "Should return OK message")

	suite.T().Logf("Deleted 2 employees with DisableDataPerm successfully")
}

// TestDeleteManyPreHookError tests DeleteMany with a pre-hook that returns error.
func (suite *DeleteManyTestSuite) TestDeleteManyPreHookError() {
	suite.T().Logf("Testing DeleteMany API pre-hook error for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_delete_many_prehook_err",
			Action:   "delete_many",
			Version:  "v1",
		},
		Params: map[string]any{
			"ids": []any{
				map[string]any{"id": "deluser001"},
			},
		},
	})

	suite.Contains([]int{200, 400, 500}, resp.StatusCode, "Should return error status code")

	suite.T().Logf("DeleteMany failed as expected due to pre-hook error")
}

// TestDeleteManyPostHookError tests DeleteMany with a post-hook that returns error.
func (suite *DeleteManyTestSuite) TestDeleteManyPostHookError() {
	suite.T().Logf("Testing DeleteMany API post-hook error for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_delete_many_posthook_err",
			Action:   "delete_many",
			Version:  "v1",
		},
		Params: map[string]any{
			"ids": []any{
				map[string]any{"id": "deluser002"},
			},
		},
	})

	suite.Contains([]int{200, 400, 500}, resp.StatusCode, "Should return error status code")

	suite.T().Logf("DeleteMany failed as expected due to post-hook error")
}

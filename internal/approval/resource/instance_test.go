package resource_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/internal/apptest"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/security"
)

// adminRPCCall sends an RPC request to the approval/admin resource and returns the result.
func (s *InstanceResourceTestSuite) adminRPCCall(action string, params map[string]any, token ...string) result.Result {
	t := s.token
	if len(token) > 0 {
		t = token[0]
	}

	resp := s.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{Resource: "approval/admin", Action: action, Version: "v1"},
		Params:     params,
	}, t)
	s.Require().Equal(http.StatusOK, resp.StatusCode, "Should match expected value")

	return s.ReadResult(resp)
}

// FlowSetup holds references to a deployed and published flow for E2E testing.
type FlowSetup struct {
	FlowID   string
	FlowCode string
}

// InstanceResourceTestSuite tests instance lifecycle and query operations via HTTP.
type InstanceResourceTestSuite struct {
	apptest.Suite

	ctx        context.Context
	db         orm.DB
	token      string
	categoryID string

	// Flow setups
	simple   FlowSetup // Start → End
	approval FlowSetup // Start → Approval(rollback/assignee/cc enabled) → End
	complex  FlowSetup // Start → Condition → [Approval/Handle] → End
	parallel FlowSetup // Start → Approval(parallel, passAll) → End
	noCC     FlowSetup // Start → Approval(no manual CC) → End

	// Multi-user tokens
	approver2Token string // approver-2
	manager1Token  string // manager-1
	handler1Token  string // handler-1
	otherUserToken string // other-user (permission denied tests)
}

func TestInstanceResource(t *testing.T) {
	suite.Run(t, new(InstanceResourceTestSuite))
}

func (s *InstanceResourceTestSuite) SetupSuite() {
	s.ctx = context.Background()
	s.db, s.token = setupResourceApp(&s.Suite)

	// Create shared category
	cat := &approval.FlowCategory{
		TenantID: "default",
		Code:     "inst-test-cat",
		Name:     "Instance Test Category",
		IsActive: true,
	}
	_, err := s.db.NewInsert().Model(cat).Exec(s.ctx)
	s.Require().NoError(err, "Should insert test category")
	s.categoryID = cat.ID

	// Pre-build flows
	s.simple = s.createAndPublishFlow("res-simple", "Simple Flow", simpleFlowDef())
	s.approval = s.createAndPublishFlow("res-approval", "Approval Flow", approvalFlowDef())
	s.complex = s.createAndPublishFlow("res-complex", "Complex Flow", complexFlowDef())
	s.parallel = s.createAndPublishFlow("res-parallel", "Parallel Flow", parallelApprovalFlowDef())
	s.noCC = s.createAndPublishFlow("res-no-cc", "No CC Flow", noManualCCFlowDef())

	// Generate tokens for multiple users
	s.approver2Token = s.GenerateToken(security.NewUser("approver-2", "Approver 2"))
	s.manager1Token = s.GenerateToken(security.NewUser("manager-1", "Manager 1"))
	s.handler1Token = s.GenerateToken(security.NewUser("handler-1", "Handler 1"))
	s.otherUserToken = s.GenerateToken(security.NewUser("other-user", "Other User"))
}

func (s *InstanceResourceTestSuite) TearDownSuite() {
	cleanAllApprovalData(s.ctx, s.db)
	s.TearDownApp()
}

func (s *InstanceResourceTestSuite) TearDownTest() {
	cleanRuntimeData(s.ctx, s.db)
}

// --- Suite-local RPC helpers ---

// createAndPublishFlow creates, deploys, and publishes a flow via RPC calls.
func (s *InstanceResourceTestSuite) createAndPublishFlow(code, name string, def approval.FlowDefinition) FlowSetup {
	// Create flow
	resp := s.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{Resource: "approval/flow", Action: "create", Version: "v1"},
		Params: map[string]any{
			"tenantId":               "default",
			"code":                   code,
			"name":                   name,
			"categoryId":             s.categoryID,
			"bindingMode":            "standalone",
			"isAllInitiationAllowed": true,
			"instanceTitleTemplate":  name + " {{.instanceNo}}",
		},
	}, s.token)
	s.Require().Equal(http.StatusOK, resp.StatusCode, "Should match expected value")
	res := s.ReadResult(resp)
	s.Require().True(res.IsOk(), "Should create flow: "+code)
	flowData := s.ReadDataAsMap(res.Data)
	flowID := flowData["id"].(string)

	// Deploy
	resp = s.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{Resource: "approval/flow", Action: "deploy", Version: "v1"},
		Params: map[string]any{
			"flowId":         flowID,
			"flowDefinition": toMap(def),
		},
	}, s.token)
	s.Require().Equal(http.StatusOK, resp.StatusCode, "Should match expected value")
	res = s.ReadResult(resp)
	s.Require().True(res.IsOk(), "Should deploy flow: "+code)
	versionData := s.ReadDataAsMap(res.Data)
	versionID := versionData["id"].(string)

	// Publish
	resp = s.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{Resource: "approval/flow", Action: "publish_version", Version: "v1"},
		Params:     map[string]any{"versionId": versionID},
	}, s.token)
	s.Require().Equal(http.StatusOK, resp.StatusCode, "Should match expected value")
	res = s.ReadResult(resp)
	s.Require().True(res.IsOk(), "Should publish flow: "+code)

	return FlowSetup{FlowID: flowID, FlowCode: code}
}

// rpcCall sends an RPC request to the approval/instance resource and returns the result.
// If token is provided, it overrides the default token.
func (s *InstanceResourceTestSuite) rpcCall(action string, params map[string]any, token ...string) result.Result {
	t := s.token
	if len(token) > 0 {
		t = token[0]
	}

	resp := s.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{Resource: "approval/instance", Action: action, Version: "v1"},
		Params:     params,
	}, t)
	s.Require().Equal(http.StatusOK, resp.StatusCode, "Should match expected value")

	return s.ReadResult(resp)
}

// startInstance starts an instance via RPC and returns the response data map.
func (s *InstanceResourceTestSuite) startInstance(flowCode string, formData map[string]any) map[string]any {
	res := s.rpcCall("start", map[string]any{
		"tenantId": "default",
		"flowCode": flowCode,
		"formData": formData,
	})
	s.Require().True(res.IsOk(), "Should start instance for flow: "+flowCode)

	return s.ReadDataAsMap(res.Data)
}

// findPendingTasks queries pending tasks for the given instance via the admin resource.
// It normalizes admin.Task fields (taskId→id, assigneeId) so callers can use t["id"].
func (s *InstanceResourceTestSuite) findPendingTasks(instanceID string) []map[string]any {
	res := s.adminRPCCall("find_tasks", map[string]any{
		"instanceId": instanceID,
		"status":     "pending",
		"page":       1,
		"pageSize":   50,
	})
	s.Require().True(res.IsOk(), "Should find tasks")

	data := s.ReadDataAsMap(res.Data)

	items, ok := data["items"].([]any)
	if !ok || len(items) == 0 {
		return nil
	}

	tasks := make([]map[string]any, len(items))
	for i, item := range items {
		m := item.(map[string]any)
		// Normalize: admin.Task uses "taskId" but helpers expect "id"
		if tid, exists := m["taskId"]; exists {
			m["id"] = tid
		}

		tasks[i] = m
	}

	return tasks
}

// processTask sends a process_task RPC and returns the result.
func (s *InstanceResourceTestSuite) processTask(taskID, action, opinion string, token ...string) result.Result {
	return s.rpcCall("process_task", map[string]any{
		"taskId":  taskID,
		"action":  action,
		"opinion": opinion,
	}, token...)
}

// assertErrorCode asserts that the result is an error with the expected code.
func (s *InstanceResourceTestSuite) assertErrorCode(res result.Result, code int, msgAndArgs ...any) {
	s.Assert().False(res.IsOk(), msgAndArgs...)
	s.Assert().Equal(code, res.Code, msgAndArgs...)
}

// loadInstance loads an instance from the database by ID.
func (s *InstanceResourceTestSuite) loadInstance(instanceID string) approval.Instance {
	var instance approval.Instance

	instance.ID = instanceID
	s.Require().NoError(s.db.NewSelect().Model(&instance).WherePK().Scan(s.ctx), "Should not return error")

	return instance
}

// loadTask loads a task from the database by ID.
func (s *InstanceResourceTestSuite) loadTask(taskID string) approval.Task {
	var task approval.Task

	task.ID = taskID
	s.Require().NoError(s.db.NewSelect().Model(&task).WherePK().Scan(s.ctx), "Should not return error")

	return task
}

// findNodeIDByKind returns the database ID of the first flow node with the given kind
// in the specified flow version.
func (s *InstanceResourceTestSuite) findNodeIDByKind(flowVersionID string, kind approval.NodeKind) string {
	var node approval.FlowNode
	s.Require().NoError(
		s.db.NewSelect().Model(&node).
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("flow_version_id", flowVersionID)
				cb.Equals("kind", kind)
			}).
			Limit(1).
			Scan(s.ctx), "Should not return error",
	)

	return node.ID
}

// --- Test methods ---

func (s *InstanceResourceTestSuite) TestStart() {
	s.Run("SimpleFlowAutoComplete", func() {
		data := s.startInstance(s.simple.FlowCode, nil)
		instanceID := data["id"].(string)
		s.Assert().NotEmpty(instanceID, "Should not be empty")

		instance := s.loadInstance(instanceID)
		s.Assert().Equal(approval.InstanceApproved, instance.Status, "Simple flow should auto-complete")
	})

	s.Run("ApprovalFlowRunning", func() {
		data := s.startInstance(s.approval.FlowCode, nil)
		instanceID := data["id"].(string)

		instance := s.loadInstance(instanceID)
		s.Assert().Equal(approval.InstanceRunning, instance.Status, "Approval flow should be running")

		tasks := s.findPendingTasks(instanceID)
		s.Assert().NotEmpty(tasks, "Should have pending tasks")
	})

	s.Run("FlowNotFound", func() {
		res := s.rpcCall("start", map[string]any{
			"tenantId": "default",
			"flowCode": "non-existent-flow",
		})
		s.Assert().False(res.IsOk(), "Should fail for non-existent flow")
	})

	s.Run("WithFormData", func() {
		formData := map[string]any{"title": "Test Request", "amount": 100}
		data := s.startInstance(s.approval.FlowCode, formData)
		instanceID := data["id"].(string)

		instance := s.loadInstance(instanceID)
		s.Assert().Equal(approval.InstanceRunning, instance.Status, "Should match expected value")
		s.Assert().NotNil(instance.FormData, "Form data should be stored")
	})
}

func (s *InstanceResourceTestSuite) TestProcessTask() {
	s.Run("Approve", func() {
		data := s.startInstance(s.approval.FlowCode, nil)
		instanceID := data["id"].(string)

		for range 10 {
			tasks := s.findPendingTasks(instanceID)
			if len(tasks) == 0 {
				break
			}

			res := s.processTask(tasks[0]["id"].(string), "approve", "Approved")
			s.Assert().True(res.IsOk(), "Should approve task")
		}

		instance := s.loadInstance(instanceID)
		s.Assert().Equal(approval.InstanceApproved, instance.Status, "Instance should be approved after all tasks approved")
	})

	s.Run("Reject", func() {
		data := s.startInstance(s.approval.FlowCode, nil)
		instanceID := data["id"].(string)

		tasks := s.findPendingTasks(instanceID)
		s.Require().NotEmpty(tasks, "Should not be empty")

		res := s.processTask(tasks[0]["id"].(string), "reject", "Rejected")
		s.Assert().True(res.IsOk(), "Should reject task")

		instance := s.loadInstance(instanceID)
		s.Assert().Equal(approval.InstanceRejected, instance.Status, "Instance should be rejected")
	})

	s.Run("TransferAndNewTask", func() {
		data := s.startInstance(s.approval.FlowCode, nil)
		instanceID := data["id"].(string)

		tasks := s.findPendingTasks(instanceID)
		s.Require().NotEmpty(tasks, "Should not be empty")
		originalTaskID := tasks[0]["id"].(string)

		res := s.rpcCall("process_task", map[string]any{
			"taskId":       originalTaskID,
			"action":       "transfer",
			"opinion":      "Please handle this",
			"transferToId": "new-assignee-1",
		})
		s.Assert().True(res.IsOk(), "Should transfer task")

		// Verify original task is transferred
		original := s.loadTask(originalTaskID)
		s.Assert().Equal(approval.TaskTransferred, original.Status, "Original task should be transferred")

		// Verify new task exists and is pending
		newTasks := s.findPendingTasks(instanceID)
		s.Assert().NotEmpty(newTasks, "Should have new pending task after transfer")
	})

	s.Run("TransferInvalidTarget", func() {
		data := s.startInstance(s.approval.FlowCode, nil)
		instanceID := data["id"].(string)

		tasks := s.findPendingTasks(instanceID)
		s.Require().NotEmpty(tasks, "Should have pending tasks to transfer")
		taskID := tasks[0]["id"].(string)

		res := s.rpcCall("process_task", map[string]any{
			"taskId":       taskID,
			"action":       "transfer",
			"opinion":      "invalid transfer target",
			"transferToId": "   ",
		})
		s.assertErrorCode(res, shared.ErrCodeInvalidTransferTarget, "Should return invalid transfer target error code for blank transferee")
	})

	s.Run("HandleOnComplexFlow", func() {
		// amount:500 routes to handle branch
		data := s.startInstance(s.complex.FlowCode, map[string]any{"amount": 500})
		instanceID := data["id"].(string)

		tasks := s.findPendingTasks(instanceID)
		s.Require().NotEmpty(tasks, "Should have pending handle task")

		// Use handler1Token to handle
		res := s.processTask(tasks[0]["id"].(string), "handle", "Handled", s.handler1Token)
		s.Assert().True(res.IsOk(), "Should handle task")

		task := s.loadTask(tasks[0]["id"].(string))
		s.Assert().Equal(approval.TaskHandled, task.Status, "Task should be handled")

		instance := s.loadInstance(instanceID)
		s.Assert().Equal(approval.InstanceApproved, instance.Status, "Instance should be approved after handle")
	})

	s.Run("AlreadyProcessed", func() {
		// Use parallel flow so instance stays running after first approval
		data := s.startInstance(s.parallel.FlowCode, nil)
		instanceID := data["id"].(string)

		tasks := s.findPendingTasks(instanceID)
		s.Require().NotEmpty(tasks, "Should not be empty")

		// Find test-admin's task
		var taskID string
		for _, t := range tasks {
			if t["assigneeId"] == "test-admin" {
				taskID = t["id"].(string)

				break
			}
		}

		s.Require().NotEmpty(taskID, "Should not be empty")

		// Approve first
		res := s.processTask(taskID, "approve", "OK")
		s.Require().True(res.IsOk(), "Condition should be true")

		// Try to approve again — task is no longer pending
		res = s.processTask(taskID, "approve", "OK again")
		s.assertErrorCode(res, shared.ErrCodeTaskNotPending, "Should fail on already processed task")
	})

	s.Run("NotAssignee", func() {
		data := s.startInstance(s.approval.FlowCode, nil)
		instanceID := data["id"].(string)

		tasks := s.findPendingTasks(instanceID)
		s.Require().NotEmpty(tasks, "Should not be empty")

		// Try with other-user who is not the assignee
		res := s.processTask(tasks[0]["id"].(string), "approve", "Trying", s.otherUserToken)
		s.assertErrorCode(res, shared.ErrCodeNotAssignee, "Should fail for non-assignee")
	})

	s.Run("OpinionRequired", func() {
		data := s.startInstance(s.approval.FlowCode, nil)
		instanceID := data["id"].(string)

		tasks := s.findPendingTasks(instanceID)
		s.Require().NotEmpty(tasks, "Should not be empty")

		// Submit with empty opinion (approval flow requires opinion)
		res := s.processTask(tasks[0]["id"].(string), "approve", "")
		s.assertErrorCode(res, shared.ErrCodeOpinionRequired, "Should fail when opinion is required but empty")
	})
}

func (s *InstanceResourceTestSuite) TestRollback() {
	s.Run("ToPrevious", func() {
		data := s.startInstance(s.approval.FlowCode, nil)
		instanceID := data["id"].(string)

		tasks := s.findPendingTasks(instanceID)
		s.Require().NotEmpty(tasks, "Should not be empty")
		taskID := tasks[0]["id"].(string)

		// Get the flow version ID and find start node ID for rollback target
		instance := s.loadInstance(instanceID)
		startNodeID := s.findNodeIDByKind(instance.FlowVersionID, approval.NodeStart)

		// Rollback to start node
		res := s.rpcCall("process_task", map[string]any{
			"taskId":       taskID,
			"action":       "rollback",
			"opinion":      "Rolling back",
			"targetNodeId": startNodeID,
		})
		s.Assert().True(res.IsOk(), "Should rollback task")

		// Verify task is rolled back
		task := s.loadTask(taskID)
		s.Assert().Equal(approval.TaskRolledBack, task.Status, "Task should be rolled_back")

		// Verify instance is returned
		instance = s.loadInstance(instanceID)
		s.Assert().Equal(approval.InstanceReturned, instance.Status, "Instance should be returned after rollback to start")
	})

	s.Run("NotAllowed", func() {
		// Complex flow handle node has no rollback configured
		data := s.startInstance(s.complex.FlowCode, map[string]any{"amount": 500})
		instanceID := data["id"].(string)

		tasks := s.findPendingTasks(instanceID)
		s.Require().NotEmpty(tasks, "Should not be empty")

		instance := s.loadInstance(instanceID)
		startNodeID := s.findNodeIDByKind(instance.FlowVersionID, approval.NodeStart)

		res := s.rpcCall("process_task", map[string]any{
			"taskId":       tasks[0]["id"].(string),
			"action":       "rollback",
			"opinion":      "Try rollback",
			"targetNodeId": startNodeID,
		}, s.handler1Token)
		s.assertErrorCode(res, shared.ErrCodeRollbackNotAllowed, "Should not allow rollback on handle node")
	})

	s.Run("InvalidTarget", func() {
		data := s.startInstance(s.approval.FlowCode, nil)
		instanceID := data["id"].(string)

		tasks := s.findPendingTasks(instanceID)
		s.Require().NotEmpty(tasks, "Should not be empty")

		instance := s.loadInstance(instanceID)
		endNodeID := s.findNodeIDByKind(instance.FlowVersionID, approval.NodeEnd)

		res := s.rpcCall("process_task", map[string]any{
			"taskId":       tasks[0]["id"].(string),
			"action":       "rollback",
			"opinion":      "Rolling back to end",
			"targetNodeId": endNodeID,
		})
		s.assertErrorCode(res, shared.ErrCodeInvalidRollbackTarget, "Should not allow rollback to end node")
	})

	s.Run("TargetRequired", func() {
		data := s.startInstance(s.approval.FlowCode, nil)
		instanceID := data["id"].(string)

		tasks := s.findPendingTasks(instanceID)
		s.Require().NotEmpty(tasks, "Should have pending tasks to rollback")

		res := s.rpcCall("process_task", map[string]any{
			"taskId":       tasks[0]["id"].(string),
			"action":       "rollback",
			"opinion":      "Rollback without target",
			"targetNodeId": "  ",
		})
		s.assertErrorCode(res, shared.ErrCodeInvalidRollbackTarget, "Should return invalid rollback target error code when target node is blank")
	})
}

func (s *InstanceResourceTestSuite) TestWithdraw() {
	s.Run("HappyPath", func() {
		data := s.startInstance(s.approval.FlowCode, nil)
		instanceID := data["id"].(string)

		res := s.rpcCall("withdraw", map[string]any{
			"instanceId": instanceID,
			"reason":     "Changed my mind",
		})
		s.Assert().True(res.IsOk(), "Should withdraw instance")

		instance := s.loadInstance(instanceID)
		s.Assert().Equal(approval.InstanceWithdrawn, instance.Status, "Instance should be withdrawn")
	})

	s.Run("AlreadyWithdrawn", func() {
		data := s.startInstance(s.approval.FlowCode, nil)
		instanceID := data["id"].(string)

		// Withdraw first
		res := s.rpcCall("withdraw", map[string]any{
			"instanceId": instanceID,
			"reason":     "First withdraw",
		})
		s.Require().True(res.IsOk(), "Condition should be true")

		// Try again
		res = s.rpcCall("withdraw", map[string]any{
			"instanceId": instanceID,
			"reason":     "Second withdraw",
		})
		s.Assert().False(res.IsOk(), "Should not allow withdrawing an already withdrawn instance")
	})

	s.Run("NotApplicant", func() {
		data := s.startInstance(s.approval.FlowCode, nil)
		instanceID := data["id"].(string)

		res := s.rpcCall("withdraw", map[string]any{
			"instanceId": instanceID,
			"reason":     "Not my instance",
		}, s.otherUserToken)
		s.assertErrorCode(res, shared.ErrCodeNotApplicant, "Should fail for non-applicant")
	})
}

func (s *InstanceResourceTestSuite) TestResubmit() {
	s.Run("AfterWithdrawFullLifecycle", func() {
		data := s.startInstance(s.approval.FlowCode, nil)
		instanceID := data["id"].(string)

		// Withdraw
		res := s.rpcCall("withdraw", map[string]any{
			"instanceId": instanceID,
			"reason":     "Need to revise",
		})
		s.Require().True(res.IsOk(), "Condition should be true")

		// Resubmit
		res = s.rpcCall("resubmit", map[string]any{
			"instanceId": instanceID,
			"formData":   map[string]any{"revised": true},
		})
		s.Assert().True(res.IsOk(), "Should resubmit instance")

		instance := s.loadInstance(instanceID)
		s.Assert().Equal(approval.InstanceRunning, instance.Status, "Instance should be running after resubmit")

		// Approve to complete the lifecycle
		for range 10 {
			tasks := s.findPendingTasks(instanceID)
			if len(tasks) == 0 {
				break
			}

			approveRes := s.processTask(tasks[0]["id"].(string), "approve", "Approved after resubmit")
			s.Assert().True(approveRes.IsOk(), "Condition should be true")
		}

		instance = s.loadInstance(instanceID)
		s.Assert().Equal(approval.InstanceApproved, instance.Status, "Instance should be approved after resubmit and approval")
	})

	s.Run("NotAllowedFromRunning", func() {
		data := s.startInstance(s.approval.FlowCode, nil)
		instanceID := data["id"].(string)

		// Try to resubmit a running instance
		res := s.rpcCall("resubmit", map[string]any{
			"instanceId": instanceID,
			"formData":   map[string]any{"test": true},
		})
		s.assertErrorCode(res, shared.ErrCodeResubmitNotAllowed, "Should not allow resubmit from running state")
	})
}

func (s *InstanceResourceTestSuite) TestAddAssignee() {
	s.Run("Before", func() {
		data := s.startInstance(s.approval.FlowCode, nil)
		instanceID := data["id"].(string)

		tasks := s.findPendingTasks(instanceID)
		s.Require().NotEmpty(tasks, "Should not be empty")
		originalTaskID := tasks[0]["id"].(string)

		res := s.rpcCall("add_assignee", map[string]any{
			"taskId":  originalTaskID,
			"userIds": []string{"before-user"},
			"addType": "before",
		})
		s.Assert().True(res.IsOk(), "Should add assignee before")

		// Original task should become waiting
		original := s.loadTask(originalTaskID)
		s.Assert().Equal(approval.TaskWaiting, original.Status, "Original task should be waiting after before-add")

		// New task should be pending
		newTasks := s.findPendingTasks(instanceID)
		s.Assert().NotEmpty(newTasks, "Should have new pending task")
	})

	s.Run("After", func() {
		data := s.startInstance(s.approval.FlowCode, nil)
		instanceID := data["id"].(string)

		tasks := s.findPendingTasks(instanceID)
		s.Require().NotEmpty(tasks, "Should not be empty")
		originalTaskID := tasks[0]["id"].(string)

		res := s.rpcCall("add_assignee", map[string]any{
			"taskId":  originalTaskID,
			"userIds": []string{"after-user"},
			"addType": "after",
		})
		s.Assert().True(res.IsOk(), "Should add assignee after")

		// Original task should still be pending
		original := s.loadTask(originalTaskID)
		s.Assert().Equal(approval.TaskPending, original.Status, "Original task should remain pending")

		// The after-added task should be waiting
		var waitingTasks []approval.Task
		s.Require().NoError(
			s.db.NewSelect().Model(&waitingTasks).
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("instance_id", instanceID)
					cb.Equals("status", approval.TaskWaiting)
				}).
				Scan(s.ctx), "Should not return error",
		)
		s.Assert().NotEmpty(waitingTasks, "Should have waiting task for after-added assignee")
	})

	s.Run("Parallel", func() {
		data := s.startInstance(s.approval.FlowCode, nil)
		instanceID := data["id"].(string)

		tasks := s.findPendingTasks(instanceID)
		s.Require().NotEmpty(tasks, "Should not be empty")
		originalTaskID := tasks[0]["id"].(string)

		res := s.rpcCall("add_assignee", map[string]any{
			"taskId":  originalTaskID,
			"userIds": []string{"parallel-user"},
			"addType": "parallel",
		})
		s.Assert().True(res.IsOk(), "Should add assignee parallel")

		// Original task should still be pending
		original := s.loadTask(originalTaskID)
		s.Assert().Equal(approval.TaskPending, original.Status, "Original task should remain pending")

		// New parallel task should also be pending
		pendingTasks := s.findPendingTasks(instanceID)
		s.Assert().GreaterOrEqual(len(pendingTasks), 2, "Should have at least 2 pending tasks")
	})

	s.Run("NotAllowed", func() {
		// Complex flow handle node doesn't have add_assignee enabled
		data := s.startInstance(s.complex.FlowCode, map[string]any{"amount": 500})
		instanceID := data["id"].(string)

		tasks := s.findPendingTasks(instanceID)
		s.Require().NotEmpty(tasks, "Should not be empty")

		res := s.rpcCall("add_assignee", map[string]any{
			"taskId":  tasks[0]["id"].(string),
			"userIds": []string{"extra-user"},
			"addType": "before",
		}, s.handler1Token)
		s.assertErrorCode(res, shared.ErrCodeAddAssigneeNotAllowed, "Should not allow add assignee on handle node")
	})
}

func (s *InstanceResourceTestSuite) TestRemoveAssignee() {
	s.Run("HappyPath", func() {
		data := s.startInstance(s.approval.FlowCode, nil)
		instanceID := data["id"].(string)

		tasks := s.findPendingTasks(instanceID)
		s.Require().NotEmpty(tasks, "Should not be empty")
		originalTaskID := tasks[0]["id"].(string)

		// First add a parallel assignee so the node has 2+ tasks
		res := s.rpcCall("add_assignee", map[string]any{
			"taskId":  originalTaskID,
			"userIds": []string{"extra-user"},
			"addType": "parallel",
		})
		s.Require().True(res.IsOk(), "Should add parallel assignee")

		// Find the newly added task
		pendingTasks := s.findPendingTasks(instanceID)
		s.Require().GreaterOrEqual(len(pendingTasks), 2, "Should have at least 2 pending tasks")

		// Find the task for extra-user and remove it
		var removeTaskID string
		for _, t := range pendingTasks {
			if t["assigneeId"] == "extra-user" {
				removeTaskID = t["id"].(string)

				break
			}
		}

		s.Require().NotEmpty(removeTaskID, "Should find task for extra-user")

		res = s.rpcCall("remove_assignee", map[string]any{
			"taskId": removeTaskID,
		})
		s.Assert().True(res.IsOk(), "Should remove assignee")

		task := s.loadTask(removeTaskID)
		s.Assert().Equal(approval.TaskRemoved, task.Status, "Removed task should have removed status")
	})

	s.Run("NotAllowed", func() {
		// Complex flow handle node doesn't have remove_assignee enabled
		data := s.startInstance(s.complex.FlowCode, map[string]any{"amount": 500})
		instanceID := data["id"].(string)

		tasks := s.findPendingTasks(instanceID)
		s.Require().NotEmpty(tasks, "Should not be empty")

		res := s.rpcCall("remove_assignee", map[string]any{
			"taskId": tasks[0]["id"].(string),
		}, s.handler1Token)
		s.assertErrorCode(res, shared.ErrCodeRemoveAssigneeNotAllowed, "Should not allow remove assignee on handle node")
	})
}

func (s *InstanceResourceTestSuite) TestCC() {
	s.Run("AddCC", func() {
		data := s.startInstance(s.approval.FlowCode, nil)
		instanceID := data["id"].(string)

		res := s.rpcCall("add_cc", map[string]any{
			"instanceId": instanceID,
			"ccUserIds":  []string{"cc-user-1", "cc-user-2"},
		})
		s.Assert().True(res.IsOk(), "Should add CC")

		count, err := s.db.NewSelect().Model((*approval.CCRecord)(nil)).
			Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", instanceID) }).Count(s.ctx)
		s.Require().NoError(err, "Should not return error")
		s.Assert().GreaterOrEqual(count, int64(2), "Should have CC records")
	})

	s.Run("MarkCCRead", func() {
		data := s.startInstance(s.approval.FlowCode, nil)
		instanceID := data["id"].(string)

		// Add CC first
		addRes := s.rpcCall("add_cc", map[string]any{
			"instanceId": instanceID,
			"ccUserIds":  []string{"test-admin"},
		})
		s.Require().True(addRes.IsOk(), "Condition should be true")

		// Mark as read
		res := s.rpcCall("mark_cc_read", map[string]any{
			"instanceId": instanceID,
		})
		s.Assert().True(res.IsOk(), "Should mark CC as read")
	})

	s.Run("ManualCCNotAllowed", func() {
		data := s.startInstance(s.noCC.FlowCode, nil)
		instanceID := data["id"].(string)

		res := s.rpcCall("add_cc", map[string]any{
			"instanceId": instanceID,
			"ccUserIds":  []string{"cc-user-1"},
		})
		s.assertErrorCode(res, shared.ErrCodeManualCcNotAllowed, "Should not allow manual CC")
	})
}

func (s *InstanceResourceTestSuite) TestUrgeTask() {
	data := s.startInstance(s.approval.FlowCode, nil)
	instanceID := data["id"].(string)

	tasks := s.findPendingTasks(instanceID)
	s.Require().NotEmpty(tasks, "Should have pending tasks")
	taskID := tasks[0]["id"].(string)

	res := s.rpcCall("urge_task", map[string]any{
		"taskId":  taskID,
		"message": "Please review ASAP",
	})
	s.Assert().True(res.IsOk(), "Should urge task")

	count, err := s.db.NewSelect().Model((*approval.UrgeRecord)(nil)).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("task_id", taskID) }).Count(s.ctx)
	s.Require().NoError(err, "Should not return error")
	s.Assert().Equal(int64(1), count, "Should have one urge record")
}

func (s *InstanceResourceTestSuite) TestQuery() {
	s.Run("FindInstances", func() {
		_ = s.startInstance(s.approval.FlowCode, nil)
		_ = s.startInstance(s.approval.FlowCode, nil)

		res := s.adminRPCCall("find_instances", map[string]any{
			"tenantId": "default",
			"page":     1,
			"pageSize": 10,
		})
		s.Assert().True(res.IsOk(), "Should find instances")

		data := s.ReadDataAsMap(res.Data)
		total := data["total"].(float64)
		s.Assert().GreaterOrEqual(int(total), 2, "Should find at least 2 instances")
	})

	s.Run("FindTasks", func() {
		data := s.startInstance(s.approval.FlowCode, nil)
		instanceID := data["id"].(string)

		res := s.adminRPCCall("find_tasks", map[string]any{
			"instanceId": instanceID,
			"page":       1,
			"pageSize":   10,
		})
		s.Assert().True(res.IsOk(), "Should find tasks")

		resData := s.ReadDataAsMap(res.Data)
		total := resData["total"].(float64)
		s.Assert().GreaterOrEqual(int(total), 1, "Should find at least 1 task")
	})

	s.Run("GetInstanceDetail", func() {
		data := s.startInstance(s.approval.FlowCode, nil)
		instanceID := data["id"].(string)

		res := s.adminRPCCall("get_instance_detail", map[string]any{
			"instanceId": instanceID,
		})
		s.Assert().True(res.IsOk(), "Should get instance detail")

		detail := s.ReadDataAsMap(res.Data)
		s.Assert().NotNil(detail["instance"], "Detail should contain instance")
		s.Assert().NotNil(detail["tasks"], "Detail should contain tasks")
	})

	s.Run("FindActionLogs", func() {
		data := s.startInstance(s.approval.FlowCode, nil)
		instanceID := data["id"].(string)

		// Approve a task to generate action logs
		tasks := s.findPendingTasks(instanceID)
		if len(tasks) > 0 {
			s.processTask(tasks[0]["id"].(string), "approve", "Looks good")
		}

		res := s.adminRPCCall("find_action_logs", map[string]any{
			"instanceId": instanceID,
			"page":       1,
			"pageSize":   10,
		})
		s.Assert().True(res.IsOk(), "Should get action logs")
	})
}

func (s *InstanceResourceTestSuite) TestConditionRouting() {
	s.Run("HighAmountFullLifecycle", func() {
		data := s.startInstance(s.complex.FlowCode, map[string]any{"amount": 2000})
		instanceID := data["id"].(string)

		instance := s.loadInstance(instanceID)
		s.Assert().Equal(approval.InstanceRunning, instance.Status, "High amount should route to approval and be running")

		tasks := s.findPendingTasks(instanceID)
		s.Require().NotEmpty(tasks, "Should have pending tasks for high-amount branch")

		// Approve with manager-1 to complete lifecycle
		res := s.processTask(tasks[0]["id"].(string), "approve", "Approved high amount", s.manager1Token)
		s.Assert().True(res.IsOk(), "Should approve high amount task")

		instance = s.loadInstance(instanceID)
		s.Assert().Equal(approval.InstanceApproved, instance.Status, "Instance should be approved after manager approval")
	})

	s.Run("LowAmountFullLifecycle", func() {
		data := s.startInstance(s.complex.FlowCode, map[string]any{"amount": 500})
		instanceID := data["id"].(string)

		instance := s.loadInstance(instanceID)
		s.Assert().Equal(approval.InstanceRunning, instance.Status, "Low amount should route to handle and be running")

		tasks := s.findPendingTasks(instanceID)
		s.Require().NotEmpty(tasks, "Should have pending tasks for default branch")

		// Handle with handler-1 to complete lifecycle
		res := s.processTask(tasks[0]["id"].(string), "handle", "Handled low amount", s.handler1Token)
		s.Assert().True(res.IsOk(), "Should handle low amount task")

		instance = s.loadInstance(instanceID)
		s.Assert().Equal(approval.InstanceApproved, instance.Status, "Instance should be approved after handler completion")
	})
}

func (s *InstanceResourceTestSuite) TestParallelApproval() {
	s.Run("BothApproveCompletes", func() {
		data := s.startInstance(s.parallel.FlowCode, nil)
		instanceID := data["id"].(string)

		instance := s.loadInstance(instanceID)
		s.Assert().Equal(approval.InstanceRunning, instance.Status, "Should match expected value")

		// Should have 2 pending tasks (test-admin + approver-2)
		tasks := s.findPendingTasks(instanceID)
		s.Require().Len(tasks, 2, "Should have 2 pending tasks for parallel approval")

		// test-admin approves first
		var testAdminTaskID string
		for _, t := range tasks {
			if t["assigneeId"] == "test-admin" {
				testAdminTaskID = t["id"].(string)

				break
			}
		}

		s.Require().NotEmpty(testAdminTaskID, "Should find test-admin task")

		res := s.processTask(testAdminTaskID, "approve", "Admin approved")
		s.Assert().True(res.IsOk(), "Condition should be true")

		// Instance should still be running (waiting for approver-2)
		instance = s.loadInstance(instanceID)
		s.Assert().Equal(approval.InstanceRunning, instance.Status, "Should still be running after first approval")

		// approver-2 approves
		var approver2TaskID string
		for _, t := range tasks {
			if t["assigneeId"] == "approver-2" {
				approver2TaskID = t["id"].(string)

				break
			}
		}

		s.Require().NotEmpty(approver2TaskID, "Should find approver-2 task")

		res = s.processTask(approver2TaskID, "approve", "Approver 2 approved", s.approver2Token)
		s.Assert().True(res.IsOk(), "Condition should be true")

		// Instance should be approved
		instance = s.loadInstance(instanceID)
		s.Assert().Equal(approval.InstanceApproved, instance.Status, "Should be approved after both approve")
	})

	s.Run("OneRejectRejects", func() {
		data := s.startInstance(s.parallel.FlowCode, nil)
		instanceID := data["id"].(string)

		tasks := s.findPendingTasks(instanceID)
		s.Require().Len(tasks, 2, "Should have 2 pending tasks")

		// test-admin rejects
		var testAdminTaskID string
		for _, t := range tasks {
			if t["assigneeId"] == "test-admin" {
				testAdminTaskID = t["id"].(string)

				break
			}
		}

		s.Require().NotEmpty(testAdminTaskID, "Should not be empty")

		res := s.processTask(testAdminTaskID, "reject", "Rejected by admin")
		s.Assert().True(res.IsOk(), "Condition should be true")

		// Instance should be rejected immediately (PassAll rule: any rejection terminates)
		instance := s.loadInstance(instanceID)
		s.Assert().Equal(approval.InstanceRejected, instance.Status, "Should be rejected after one rejection under PassAll")
	})
}

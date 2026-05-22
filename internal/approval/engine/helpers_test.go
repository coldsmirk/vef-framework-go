package engine

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/decimal"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// TestDeduplicateAssignees tests deduplicate assignees scenarios.
func TestDeduplicateAssignees(t *testing.T) {
	tests := []struct {
		name      string
		assignees []approval.ResolvedAssignee
		expected  []approval.ResolvedAssignee
	}{
		{
			name: "RemoveDuplicates",
			assignees: []approval.ResolvedAssignee{
				{UserID: "u1"}, {UserID: "u2"}, {UserID: "u1"}, {UserID: "u3"},
			},
			expected: []approval.ResolvedAssignee{
				{UserID: "u1"}, {UserID: "u2"}, {UserID: "u3"},
			},
		},
		{
			name: "NoDuplicates",
			assignees: []approval.ResolvedAssignee{
				{UserID: "u1"}, {UserID: "u2"}, {UserID: "u3"},
			},
			expected: []approval.ResolvedAssignee{
				{UserID: "u1"}, {UserID: "u2"}, {UserID: "u3"},
			},
		},
		{
			name:     "EmptySlice",
			expected: []approval.ResolvedAssignee{},
		},
		{
			name: "AllSame",
			assignees: []approval.ResolvedAssignee{
				{UserID: "u1"}, {UserID: "u1"}, {UserID: "u1"},
			},
			expected: []approval.ResolvedAssignee{
				{UserID: "u1"},
			},
		},
		{
			name: "IgnoreEmptyUserID",
			assignees: []approval.ResolvedAssignee{
				{UserID: ""}, {UserID: "u1"}, {UserID: ""},
			},
			expected: []approval.ResolvedAssignee{
				{UserID: "u1"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deduplicateAssignees(tt.assignees)
			assert.Equal(t, tt.expected, got, "Should return expected assignees")
		})
	}
}

// TestMatchDelegation tests match delegation scenarios.
func TestMatchDelegation(t *testing.T) {
	now := timex.DateTime(time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC))
	past := now.Add(-24 * time.Hour)
	future := now.Add(24 * time.Hour)

	tests := []struct {
		name           string
		delegations    []approval.Delegation
		flowID         string
		flowCategoryID string
		expectedID     string // DelegateeID of expected match, empty if nil
	}{
		{
			name: "FlowSpecificMatch",
			delegations: []approval.Delegation{
				{DelegateeID: "d1", FlowID: new("flow1")},
			},
			flowID:     "flow1",
			expectedID: "d1",
		},
		{
			name: "CategoryMatch",
			delegations: []approval.Delegation{
				{DelegateeID: "d1", FlowCategoryID: new("cat1")},
			},
			flowCategoryID: "cat1",
			expectedID:     "d1",
		},
		{
			name: "GlobalMatch",
			delegations: []approval.Delegation{
				{DelegateeID: "d1"},
			},
			expectedID: "d1",
		},
		{
			name: "FlowOverCategory",
			delegations: []approval.Delegation{
				{DelegateeID: "cat-match", FlowCategoryID: new("cat1")},
				{DelegateeID: "flow-match", FlowID: new("flow1")},
			},
			flowID:         "flow1",
			flowCategoryID: "cat1",
			expectedID:     "flow-match",
		},
		{
			name: "CategoryOverGlobal",
			delegations: []approval.Delegation{
				{DelegateeID: "global-match"},
				{DelegateeID: "cat-match", FlowCategoryID: new("cat1")},
			},
			flowCategoryID: "cat1",
			expectedID:     "cat-match",
		},
		{
			name: "ExpiredDelegation",
			delegations: []approval.Delegation{
				{DelegateeID: "d1", EndTime: past},
			},
			expectedID: "",
		},
		{
			name: "NotStartedDelegation",
			delegations: []approval.Delegation{
				{DelegateeID: "d1", StartTime: future},
			},
			expectedID: "",
		},
		{
			name: "NoMatch",
			delegations: []approval.Delegation{
				{DelegateeID: "d1", FlowID: new("other-flow")},
			},
			flowID:     "flow1",
			expectedID: "",
		},
		{
			name:       "EmptyList",
			expectedID: "",
		},
		{
			name: "WrongFlowID",
			delegations: []approval.Delegation{
				{DelegateeID: "d1", FlowID: new("wrong-flow")},
			},
			flowID:     "flow1",
			expectedID: "",
		},
		{
			name: "WrongCategoryID",
			delegations: []approval.Delegation{
				{DelegateeID: "d1", FlowCategoryID: new("wrong-cat")},
			},
			flowCategoryID: "cat1",
			expectedID:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchDelegation(tt.delegations, now, tt.flowID, tt.flowCategoryID)
			if tt.expectedID == "" {
				assert.Nil(t, got, "Should not match any delegation")
			} else {
				assert.NotNil(t, got, "Should match a delegation")
				assert.Equal(t, tt.expectedID, got.DelegateeID, "Should match the expected delegatee")
			}
		})
	}
}

// TestComputeDeadline tests compute deadline scenarios.
func TestComputeDeadline(t *testing.T) {
	t.Run("NilNode", func(t *testing.T) {
		assert.Nil(t, computeDeadline(nil), "Should return nil for nil node")
	})

	t.Run("ZeroTimeout", func(t *testing.T) {
		node := &approval.FlowNode{TimeoutHours: 0}
		assert.Nil(t, computeDeadline(node), "Should return nil when timeout is zero")
	})

	t.Run("NegativeTimeout", func(t *testing.T) {
		node := &approval.FlowNode{TimeoutHours: -1}
		assert.Nil(t, computeDeadline(node), "Should return nil when timeout is negative")
	})

	t.Run("PositiveTimeout", func(t *testing.T) {
		node := &approval.FlowNode{TimeoutHours: 24}
		before := time.Now()
		deadline := computeDeadline(node)
		after := time.Now()

		require.NotNil(t, deadline, "Should return non-nil deadline")
		d := deadline.Unwrap()
		assert.True(t, d.After(before.Add(23*time.Hour+59*time.Minute)), "Deadline should be approximately 24 hours from now")
		assert.True(t, d.Before(after.Add(24*time.Hour+time.Minute)), "Deadline should be approximately 24 hours from now")
	})
}

// MockAssigneeService is a mock implementation of approval.AssigneeService for testing.
type MockAssigneeService struct {
	mock.Mock
}

func (m *MockAssigneeService) GetSuperior(ctx context.Context, userID string) (*approval.UserInfo, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*approval.UserInfo), args.Error(1)
}

func (m *MockAssigneeService) GetDepartmentLeaders(ctx context.Context, departmentID string) ([]approval.UserInfo, error) {
	args := m.Called(ctx, departmentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).([]approval.UserInfo), args.Error(1)
}

func (m *MockAssigneeService) GetRoleUsers(ctx context.Context, roleID string) ([]approval.UserInfo, error) {
	args := m.Called(ctx, roleID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).([]approval.UserInfo), args.Error(1)
}

// TestGetSuperior tests get superior scenarios.
func TestGetSuperior(t *testing.T) {
	t.Run("NilOrgService", func(t *testing.T) {
		_, err := getSuperior(t.Context(), nil, "user1")
		assert.ErrorIs(t, err, ErrAssigneeServiceNotConfigured, "Should return ErrAssigneeServiceNotConfigured when OrganizationService is nil")
	})

	t.Run("WithService", func(t *testing.T) {
		svc := new(MockAssigneeService)
		svc.On("GetSuperior", mock.Anything, "user1").
			Return(&approval.UserInfo{ID: "superior1", Name: "Superior"}, nil).
			Once()

		info, err := getSuperior(t.Context(), svc, "user1")
		require.NoError(t, err, "Should not return error")
		assert.Equal(t, "superior1", info.ID, "Should return superior ID from service")

		svc.AssertExpectations(t)
	})

	t.Run("WithServiceError", func(t *testing.T) {
		svc := new(MockAssigneeService)
		svc.On("GetSuperior", mock.Anything, "user1").
			Return((*approval.UserInfo)(nil), assert.AnError).
			Once()

		_, err := getSuperior(t.Context(), svc, "user1")
		assert.ErrorIs(t, err, assert.AnError, "Should propagate service error")

		svc.AssertExpectations(t)
	})
}

// TestBuildPassRuleContext tests buildPassRuleContext scenarios.
func TestBuildPassRuleContext(t *testing.T) {
	t.Run("EmptyTasks", func(t *testing.T) {
		node := &approval.FlowNode{PassRatio: decimal.NewFromInt(50)}
		prc := buildPassRuleContext(node, nil)

		assert.Equal(t, 0, prc.TotalCount, "Should have zero total")
		assert.Equal(t, 0, prc.ApprovedCount, "Should have zero approved")
		assert.Equal(t, 0, prc.RejectedCount, "Should have zero rejected")
		assert.InDelta(t, 50.0, prc.PassRatio, 0.001, "Should normalize ratio")
	})

	t.Run("CountsActionableTasks", func(t *testing.T) {
		node := &approval.FlowNode{PassRatio: decimal.NewFromFloat(0.8)}
		tasks := []approval.Task{
			{Status: approval.TaskApproved},
			{Status: approval.TaskRejected},
			{Status: approval.TaskPending},
			{Status: approval.TaskHandled},
		}

		prc := buildPassRuleContext(node, tasks)
		assert.Equal(t, 4, prc.TotalCount, "Should count all actionable tasks")
		assert.Equal(t, 2, prc.ApprovedCount, "Should count approved + handled")
		assert.Equal(t, 1, prc.RejectedCount, "Should count rejected")
		assert.InDelta(t, 80.0, prc.PassRatio, 0.001, "Should normalize 0.8 to 80")
	})

	t.Run("ExcludesNonActionable", func(t *testing.T) {
		node := &approval.FlowNode{PassRatio: decimal.NewFromInt(0)}
		tasks := []approval.Task{
			{Status: approval.TaskApproved},
			{Status: approval.TaskTransferred},
			{Status: approval.TaskCanceled},
			{Status: approval.TaskRemoved},
			{Status: approval.TaskSkipped},
			{Status: approval.TaskRolledBack},
		}

		prc := buildPassRuleContext(node, tasks)
		assert.Equal(t, 1, prc.TotalCount, "Should only count actionable task")
		assert.Equal(t, 1, prc.ApprovedCount, "Should count the one approved task")
	})
}

// TestPublishEventsNilPublisher tests publishEvents with nil publisher.
func TestPublishEventsNilPublisher(t *testing.T) {
	eng := NewFlowEngine(nil, nil, nil, nil, nil, nil)

	t.Run("NilPublisherNoEvents", func(t *testing.T) {
		err := eng.publishEvents(t.Context(), nil)
		assert.NoError(t, err, "Should not error with nil publisher and no events")
	})

	t.Run("NilPublisherWithEvents", func(t *testing.T) {
		err := eng.publishEvents(t.Context(), nil, approval.NewInstanceCompletedEvent("inst-1", "tenant-1", approval.InstanceApproved))
		assert.NoError(t, err, "Should not error with nil publisher even with events")
	})
}

// newProcessContextForEvent constructs the minimum ProcessContext needed
// to exercise the TaskCreatedEvent factory helpers without touching the DB.
func newProcessContextForEvent(instanceID, nodeID string) *ProcessContext {
	inst := &approval.Instance{TenantID: "tenant-1"}
	inst.ID = instanceID

	node := &approval.FlowNode{}
	node.ID = nodeID

	return &ProcessContext{Instance: inst, Node: node}
}

// TestNewTaskCreatedEvent verifies the factory copies the full task payload
// onto the event, including the conditional deadline used by subscribers to
// distinguish actionable tasks from sequential-queue tasks.
func TestNewTaskCreatedEvent(t *testing.T) {
	t.Run("MapsAllPayloadFields", func(t *testing.T) {
		pc := newProcessContextForEvent("inst-1", "node-1")
		deadline := timex.DateTime(time.Date(2026, 5, 22, 9, 0, 0, 0, time.UTC))

		task := &approval.Task{
			TenantID:     "tenant-1",
			AssigneeID:   "user-7",
			AssigneeName: "测试用户",
			Deadline:     &deadline,
		}
		task.ID = "task-99"

		domainEvt := newTaskCreatedEvent(pc, task)
		require.NotNil(t, domainEvt, "Factory should return a non-nil event")

		evt, ok := domainEvt.(*approval.TaskCreatedEvent)
		require.True(t, ok, "Event should be *TaskCreatedEvent")

		assert.Equal(t, approval.EventTypeTaskCreated, evt.EventType(),
			"Event type should be approval.task.created")
		assert.Equal(t, "task-99", evt.TaskID, "Event should map Task.ID")
		assert.Equal(t, "tenant-1", evt.TenantID, "Event should map Task.TenantID")
		assert.Equal(t, "inst-1", evt.InstanceID, "Event should map ProcessContext.Instance.ID")
		assert.Equal(t, "node-1", evt.NodeID, "Event should map ProcessContext.Node.ID")
		assert.Equal(t, "user-7", evt.AssigneeID, "Event should map Task.AssigneeID")
		assert.Equal(t, "测试用户", evt.AssigneeName, "Event should map Task.AssigneeName")
		require.NotNil(t, evt.Deadline, "Pending task should include deadline")
		assert.True(t, evt.Deadline.Equal(deadline), "Deadline should preserve the original value")
		assert.False(t, evt.OccurredTime.IsZero(), "OccurredTime should be populated")
	})

	t.Run("LeavesDeadlineNilWhenTaskHasNone", func(t *testing.T) {
		pc := newProcessContextForEvent("inst-1", "node-1")
		task := &approval.Task{AssigneeID: "u", AssigneeName: "n", Deadline: nil}
		task.ID = "task-waiting"

		evt := newTaskCreatedEvent(pc, task).(*approval.TaskCreatedEvent)
		assert.Nil(t, evt.Deadline,
			"Waiting task should keep deadline nil to distinguish inactive work")
	})
}

// TestTaskCreatedEventsFor verifies the batch helper preserves input order
// — order matters because some downstream subscribers reconstruct task
// ordering (e.g. sequential approval queues) from the event stream.
func TestTaskCreatedEventsFor(t *testing.T) {
	t.Run("PreservesOrder", func(t *testing.T) {
		pc := newProcessContextForEvent("inst-batch", "node-batch")

		ids := []string{"t-a", "t-b", "t-c"}
		tasks := make([]*approval.Task, len(ids))

		for i, id := range ids {
			tk := &approval.Task{AssigneeID: "u", AssigneeName: "n"}
			tk.ID = id
			tasks[i] = tk
		}

		events := taskCreatedEventsFor(pc, tasks)
		require.Len(t, events, len(ids), "Batch helper should create one event per task")

		for i, want := range ids {
			got := events[i].(*approval.TaskCreatedEvent).TaskID
			assert.Equal(t, want, got, "Event #%d should match input task #%d", i, i)
		}
	})

	t.Run("EmptyInputReturnsEmptyOutput", func(t *testing.T) {
		pc := newProcessContextForEvent("inst-empty", "node-empty")

		events := taskCreatedEventsFor(pc, nil)
		assert.Empty(t, events, "Empty input should return an empty slice without panic")
	})
}

package approval_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coldsmirk/vef-framework-go/approval"
)

// TestAddAssigneeTypeIsValid tests AddAssigneeType IsValid scenarios.
func TestAddAssigneeTypeIsValid(t *testing.T) {
	tests := []struct {
		name     string
		value    approval.AddAssigneeType
		expected bool
	}{
		{"Before", approval.AddAssigneeBefore, true},
		{"After", approval.AddAssigneeAfter, true},
		{"Parallel", approval.AddAssigneeParallel, true},
		{"InvalidEmpty", approval.AddAssigneeType(""), false},
		{"InvalidRandom", approval.AddAssigneeType("invalid"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.value.IsValid(), "%s: IsValid should report %v", tt.name, tt.expected)
		})
	}
}

// TestAddAssigneeTypeUnmarshalJSON tests AddAssigneeType JSON decoding validation.
func TestAddAssigneeTypeUnmarshalJSON(t *testing.T) {
	t.Run("Valid", func(t *testing.T) {
		var value approval.AddAssigneeType

		err := json.Unmarshal([]byte(`"before"`), &value)

		assert.NoError(t, err, "Should decode valid add assignee type")
		assert.Equal(t, approval.AddAssigneeBefore, value, "Should decode to typed value")
	})

	t.Run("Invalid", func(t *testing.T) {
		var value approval.AddAssigneeType

		err := json.Unmarshal([]byte(`"invalid"`), &value)

		assert.Error(t, err, "Should reject invalid add assignee type")
	})
}

// TestInstanceStatusString tests InstanceStatus String scenarios.
func TestInstanceStatusString(t *testing.T) {
	tests := []struct {
		name     string
		status   approval.InstanceStatus
		expected string
	}{
		{"Running", approval.InstanceRunning, "running"},
		{"Approved", approval.InstanceApproved, "approved"},
		{"Rejected", approval.InstanceRejected, "rejected"},
		{"Withdrawn", approval.InstanceWithdrawn, "withdrawn"},
		{"Returned", approval.InstanceReturned, "returned"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.status.String(), "%s: String() should return %q", tt.name, tt.expected)
		})
	}
}

// TestInstanceStatusIsFinal tests InstanceStatus IsFinal scenarios.
func TestInstanceStatusIsFinal(t *testing.T) {
	tests := []struct {
		name     string
		status   approval.InstanceStatus
		expected bool
	}{
		{"Approved", approval.InstanceApproved, true},
		{"Rejected", approval.InstanceRejected, true},
		{"Withdrawn", approval.InstanceWithdrawn, false},
		{"Running", approval.InstanceRunning, false},
		{"Returned", approval.InstanceReturned, false},
		{"Terminated", approval.InstanceTerminated, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.status.IsFinal(), "%s: IsFinal should report %v", tt.name, tt.expected)
		})
	}
}

// TestTaskStatusString tests TaskStatus String scenarios.
func TestTaskStatusString(t *testing.T) {
	tests := []struct {
		name     string
		status   approval.TaskStatus
		expected string
	}{
		{"Waiting", approval.TaskWaiting, "waiting"},
		{"Pending", approval.TaskPending, "pending"},
		{"Approved", approval.TaskApproved, "approved"},
		{"Rejected", approval.TaskRejected, "rejected"},
		{"Handled", approval.TaskHandled, "handled"},
		{"Transferred", approval.TaskTransferred, "transferred"},
		{"Rollback", approval.TaskRolledBack, "rolled_back"},
		{"Canceled", approval.TaskCanceled, "canceled"},
		{"Removed", approval.TaskRemoved, "removed"},
		{"Skipped", approval.TaskSkipped, "skipped"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.status.String(), "%s: String() should return %q", tt.name, tt.expected)
		})
	}
}

// TestTaskStatusIsFinal tests TaskStatus IsFinal scenarios.
func TestTaskStatusIsFinal(t *testing.T) {
	tests := []struct {
		name     string
		status   approval.TaskStatus
		expected bool
	}{
		{"Approved", approval.TaskApproved, true},
		{"Rejected", approval.TaskRejected, true},
		{"Handled", approval.TaskHandled, true},
		{"Transferred", approval.TaskTransferred, true},
		{"Rollback", approval.TaskRolledBack, true},
		{"Canceled", approval.TaskCanceled, true},
		{"Removed", approval.TaskRemoved, true},
		{"Skipped", approval.TaskSkipped, true},
		{"Waiting", approval.TaskWaiting, false},
		{"Pending", approval.TaskPending, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.status.IsFinal(), "%s: IsFinal should report %v", tt.name, tt.expected)
		})
	}
}

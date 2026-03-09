package approval

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestAddAssigneeTypeIsValid tests AddAssigneeType IsValid scenarios.
func TestAddAssigneeTypeIsValid(t *testing.T) {
	tests := []struct {
		name     string
		value    AddAssigneeType
		expected bool
	}{
		{"Before", AddAssigneeBefore, true},
		{"After", AddAssigneeAfter, true},
		{"Parallel", AddAssigneeParallel, true},
		{"InvalidEmpty", AddAssigneeType(""), false},
		{"InvalidRandom", AddAssigneeType("invalid"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.value.IsValid(), "Should equal expected value")
		})
	}
}

// TestAddAssigneeTypeUnmarshalJSON tests AddAssigneeType JSON decoding validation.
func TestAddAssigneeTypeUnmarshalJSON(t *testing.T) {
	t.Run("Valid", func(t *testing.T) {
		var value AddAssigneeType

		err := json.Unmarshal([]byte(`"before"`), &value)

		assert.NoError(t, err, "Should decode valid add assignee type")
		assert.Equal(t, AddAssigneeBefore, value, "Should decode to typed value")
	})

	t.Run("Invalid", func(t *testing.T) {
		var value AddAssigneeType

		err := json.Unmarshal([]byte(`"invalid"`), &value)

		assert.Error(t, err, "Should reject invalid add assignee type")
	})
}

// TestInstanceStatusString tests InstanceStatus String scenarios.
func TestInstanceStatusString(t *testing.T) {
	tests := []struct {
		name     string
		status   InstanceStatus
		expected string
	}{
		{"Running", InstanceRunning, "running"},
		{"Approved", InstanceApproved, "approved"},
		{"Rejected", InstanceRejected, "rejected"},
		{"Withdrawn", InstanceWithdrawn, "withdrawn"},
		{"Returned", InstanceReturned, "returned"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.status.String(), "Should equal expected value")
		})
	}
}

// TestInstanceStatusIsFinal tests InstanceStatus IsFinal scenarios.
func TestInstanceStatusIsFinal(t *testing.T) {
	tests := []struct {
		name     string
		status   InstanceStatus
		expected bool
	}{
		{"Approved", InstanceApproved, true},
		{"Rejected", InstanceRejected, true},
		{"Withdrawn", InstanceWithdrawn, false},
		{"Running", InstanceRunning, false},
		{"Returned", InstanceReturned, false},
		{"Terminated", InstanceTerminated, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.status.IsFinal(), "Should equal expected value")
		})
	}
}

// TestTaskStatusString tests TaskStatus String scenarios.
func TestTaskStatusString(t *testing.T) {
	tests := []struct {
		name     string
		status   TaskStatus
		expected string
	}{
		{"Waiting", TaskWaiting, "waiting"},
		{"Pending", TaskPending, "pending"},
		{"Approved", TaskApproved, "approved"},
		{"Rejected", TaskRejected, "rejected"},
		{"Handled", TaskHandled, "handled"},
		{"Transferred", TaskTransferred, "transferred"},
		{"Rollback", TaskRolledBack, "rolled_back"},
		{"Canceled", TaskCanceled, "canceled"},
		{"Removed", TaskRemoved, "removed"},
		{"Skipped", TaskSkipped, "skipped"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.status.String(), "Should equal expected value")
		})
	}
}

// TestTaskStatusIsFinal tests TaskStatus IsFinal scenarios.
func TestTaskStatusIsFinal(t *testing.T) {
	tests := []struct {
		name     string
		status   TaskStatus
		expected bool
	}{
		{"Approved", TaskApproved, true},
		{"Rejected", TaskRejected, true},
		{"Handled", TaskHandled, true},
		{"Transferred", TaskTransferred, true},
		{"Rollback", TaskRolledBack, true},
		{"Canceled", TaskCanceled, true},
		{"Removed", TaskRemoved, true},
		{"Skipped", TaskSkipped, true},
		{"Waiting", TaskWaiting, false},
		{"Pending", TaskPending, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.status.IsFinal(), "Should equal expected value")
		})
	}
}

package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	collections "github.com/coldsmirk/go-collections"

	"github.com/coldsmirk/vef-framework-go/approval"
)

// --- Test Helpers ---

// TestState is a mock state type for testing the generic StateMachine independently.
type TestState string

const (
	TestStateA     TestState = "a"
	TestStateB     TestState = "b"
	TestStateC     TestState = "c"
	TestStateFinal TestState = "final"
)

func (s TestState) String() string { return string(s) }
func (s TestState) IsFinal() bool  { return s == TestStateFinal }

// --- Generic StateMachine ---

// TestStateMachine tests the generic StateMachine type.
func TestStateMachine(t *testing.T) {
	t.Run("NewEmpty", func(t *testing.T) {
		sm := NewStateMachine[TestState]("test")
		require.NotNil(t, sm, "NewStateMachine should return a non-nil value")
		assert.Equal(t, "test", sm.name, "NewStateMachine should store the provided name")
		assert.Empty(t, sm.transitions, "NewStateMachine should start with an empty transition table")
	})

	t.Run("CanTransitionReturnsFalseOnEmpty", func(t *testing.T) {
		sm := NewStateMachine[TestState]("test")
		assert.False(t, sm.CanTransition(TestStateA, TestStateB), "CanTransition should return false on an empty state machine")
	})

	t.Run("ChainingReturnsSameInstance", func(t *testing.T) {
		sm := NewStateMachine[TestState]("test")
		returned := sm.AddTransition(TestStateA, TestStateB)
		assert.Same(t, sm, returned, "AddTransition should return the same StateMachine instance for chaining")
	})

	t.Run("MultipleChainedCalls", func(t *testing.T) {
		sm := NewStateMachine[TestState]("test").
			AddTransition(TestStateA, TestStateB).
			AddTransition(TestStateB, TestStateC).
			AddTransition(TestStateC, TestStateFinal)

		assert.True(t, sm.CanTransition(TestStateA, TestStateB), "Should allow a to b")
		assert.True(t, sm.CanTransition(TestStateB, TestStateC), "Should allow b to c")
		assert.True(t, sm.CanTransition(TestStateC, TestStateFinal), "Should allow c to final")
		assert.False(t, sm.CanTransition(TestStateA, TestStateC), "Should not allow a to c (not directly registered)")
	})

	t.Run("Overwrite", func(t *testing.T) {
		sm := NewStateMachine[TestState]("test")
		sm.AddTransition(TestStateA, TestStateB)
		sm.AddTransition(TestStateA, TestStateB)

		assert.True(t, sm.CanTransition(TestStateA, TestStateB), "AddTransition called twice for same pair should still allow the transition")
	})

	t.Run("UnregisteredFrom", func(t *testing.T) {
		sm := NewStateMachine[TestState]("test")
		sm.AddTransition(TestStateA, TestStateB)
		assert.False(t, sm.CanTransition(TestStateC, TestStateA), "CanTransition should return false for an unregistered from-state")
	})

	t.Run("UnregisteredTo", func(t *testing.T) {
		sm := NewStateMachine[TestState]("test")
		sm.AddTransition(TestStateA, TestStateB)
		assert.False(t, sm.CanTransition(TestStateA, TestStateC), "CanTransition should return false for an unregistered to-state")
	})

	t.Run("FinalStateAsFrom", func(t *testing.T) {
		sm := NewStateMachine[TestState]("test")
		sm.AddTransition(TestStateA, TestStateB)
		assert.False(t, sm.CanTransition(TestStateFinal, TestStateA), "CanTransition should return false when using a final state as the from-state")
	})
}

// --- Instance StateMachine ---

// TestInstanceStateMachine tests the instance state machine.
func TestInstanceStateMachine(t *testing.T) {
	t.Run("ValidTransitions", func(t *testing.T) {
		tests := []struct {
			name string
			from approval.InstanceStatus
			to   approval.InstanceStatus
		}{
			{"RunningToApproved", approval.InstanceRunning, approval.InstanceApproved},
			{"RunningToRejected", approval.InstanceRunning, approval.InstanceRejected},
			{"RunningToWithdrawn", approval.InstanceRunning, approval.InstanceWithdrawn},
			{"RunningToReturned", approval.InstanceRunning, approval.InstanceReturned},
			{"ReturnedToRunning", approval.InstanceReturned, approval.InstanceRunning},
			{"WithdrawnToRunning", approval.InstanceWithdrawn, approval.InstanceRunning},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				assert.True(t, InstanceStateMachine.CanTransition(tt.from, tt.to), "CanTransition should return true for a valid instance transition")
			})
		}
	})

	t.Run("InvalidTransitions", func(t *testing.T) {
		tests := []struct {
			name string
			from approval.InstanceStatus
			to   approval.InstanceStatus
		}{
			{"ApprovedToRunning", approval.InstanceApproved, approval.InstanceRunning},
			{"RejectedToApproved", approval.InstanceRejected, approval.InstanceApproved},
			{"ApprovedToRejected", approval.InstanceApproved, approval.InstanceRejected},
			{"RejectedToRunning", approval.InstanceRejected, approval.InstanceRunning},
			{"ReturnedToApproved", approval.InstanceReturned, approval.InstanceApproved},
			{"RunningToRunning", approval.InstanceRunning, approval.InstanceRunning},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				assert.False(t, InstanceStateMachine.CanTransition(tt.from, tt.to), "CanTransition should return false for an invalid instance transition")
			})
		}
	})

	t.Run("AvailableFromRunning", func(t *testing.T) {
		targets := make([]approval.InstanceStatus, 0)
		for _, to := range []approval.InstanceStatus{
			approval.InstanceApproved,
			approval.InstanceRejected,
			approval.InstanceWithdrawn,
			approval.InstanceReturned,
			approval.InstanceTerminated,
		} {
			if InstanceStateMachine.CanTransition(approval.InstanceRunning, to) {
				targets = append(targets, to)
			}
		}

		require.Len(t, targets, 5, "InstanceStateMachine should have 5 valid targets from running")

		targetSet := collections.NewHashSetFrom(targets...)
		assert.True(t, targetSet.Contains(approval.InstanceApproved), "Should include approved")
		assert.True(t, targetSet.Contains(approval.InstanceRejected), "Should include rejected")
		assert.True(t, targetSet.Contains(approval.InstanceWithdrawn), "Should include withdrawn")
		assert.True(t, targetSet.Contains(approval.InstanceReturned), "Should include returned")
		assert.True(t, targetSet.Contains(approval.InstanceTerminated), "Should include terminated")
	})

	t.Run("TerminalStatesBlockAll", func(t *testing.T) {
		allStatuses := []approval.InstanceStatus{
			approval.InstanceRunning,
			approval.InstanceApproved,
			approval.InstanceRejected,
			approval.InstanceWithdrawn,
			approval.InstanceReturned,
		}

		terminalStates := []struct {
			name   string
			status approval.InstanceStatus
		}{
			{"Approved", approval.InstanceApproved},
			{"Rejected", approval.InstanceRejected},
		}

		for _, ts := range terminalStates {
			t.Run(ts.name, func(t *testing.T) {
				for _, target := range allStatuses {
					t.Run("To"+string(target), func(t *testing.T) {
						assert.False(t, InstanceStateMachine.CanTransition(ts.status, target), "CanTransition should not allow transition from a terminal instance state")
					})
				}
			})
		}
	})
}

// --- Task StateMachine ---

// TestTaskStateMachine tests the task state machine.
func TestTaskStateMachine(t *testing.T) {
	t.Run("ValidTransitions", func(t *testing.T) {
		tests := []struct {
			name string
			from approval.TaskStatus
			to   approval.TaskStatus
		}{
			{"WaitingToPending", approval.TaskWaiting, approval.TaskPending},
			{"WaitingToCanceled", approval.TaskWaiting, approval.TaskCanceled},
			{"WaitingToSkipped", approval.TaskWaiting, approval.TaskSkipped},
			{"WaitingToRemoved", approval.TaskWaiting, approval.TaskRemoved},
			{"PendingToApproved", approval.TaskPending, approval.TaskApproved},
			{"PendingToHandled", approval.TaskPending, approval.TaskHandled},
			{"PendingToRejected", approval.TaskPending, approval.TaskRejected},
			{"PendingToTransferred", approval.TaskPending, approval.TaskTransferred},
			{"PendingToRollback", approval.TaskPending, approval.TaskRolledBack},
			{"PendingToCanceled", approval.TaskPending, approval.TaskCanceled},
			{"PendingToWaiting", approval.TaskPending, approval.TaskWaiting},
			{"PendingToRemoved", approval.TaskPending, approval.TaskRemoved},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				assert.True(t, TaskStateMachine.CanTransition(tt.from, tt.to), "CanTransition should return true for a valid task transition")
			})
		}
	})

	t.Run("InvalidTransitions", func(t *testing.T) {
		tests := []struct {
			name string
			from approval.TaskStatus
			to   approval.TaskStatus
		}{
			{"ApprovedToPending", approval.TaskApproved, approval.TaskPending},
			{"RejectedToApproved", approval.TaskRejected, approval.TaskApproved},
			{"CanceledToPending", approval.TaskCanceled, approval.TaskPending},
			{"TransferredToPending", approval.TaskTransferred, approval.TaskPending},
			{"RemovedToPending", approval.TaskRemoved, approval.TaskPending},
			{"SkippedToPending", approval.TaskSkipped, approval.TaskPending},
			{"WaitingToApproved", approval.TaskWaiting, approval.TaskApproved},
			{"WaitingToRejected", approval.TaskWaiting, approval.TaskRejected},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				assert.False(t, TaskStateMachine.CanTransition(tt.from, tt.to), "CanTransition should return false for an invalid task transition")
			})
		}
	})

	t.Run("PendingTransitionCount", func(t *testing.T) {
		pending := []approval.TaskStatus{
			approval.TaskApproved, approval.TaskHandled, approval.TaskRejected,
			approval.TaskTransferred, approval.TaskRolledBack, approval.TaskCanceled,
			approval.TaskWaiting, approval.TaskRemoved,
		}

		count := 0
		for _, to := range pending {
			if TaskStateMachine.CanTransition(approval.TaskPending, to) {
				count++
			}
		}

		assert.Equal(t, 8, count, "TaskStateMachine should have 8 valid targets from pending")
	})

	t.Run("WaitingTransitionCount", func(t *testing.T) {
		waiting := []approval.TaskStatus{
			approval.TaskPending, approval.TaskCanceled, approval.TaskSkipped, approval.TaskRemoved,
		}

		count := 0
		for _, to := range waiting {
			if TaskStateMachine.CanTransition(approval.TaskWaiting, to) {
				count++
			}
		}

		assert.Equal(t, 4, count, "TaskStateMachine should have 4 valid targets from waiting")
	})

	t.Run("TerminalStatesBlockAll", func(t *testing.T) {
		allStatuses := []approval.TaskStatus{
			approval.TaskWaiting,
			approval.TaskPending,
			approval.TaskApproved,
			approval.TaskRejected,
			approval.TaskHandled,
			approval.TaskTransferred,
			approval.TaskRolledBack,
			approval.TaskCanceled,
			approval.TaskRemoved,
			approval.TaskSkipped,
		}

		terminalStates := []struct {
			name   string
			status approval.TaskStatus
		}{
			{"Approved", approval.TaskApproved},
			{"Rejected", approval.TaskRejected},
			{"Handled", approval.TaskHandled},
			{"Transferred", approval.TaskTransferred},
			{"Rollback", approval.TaskRolledBack},
			{"Canceled", approval.TaskCanceled},
			{"Removed", approval.TaskRemoved},
			{"Skipped", approval.TaskSkipped},
		}

		for _, ts := range terminalStates {
			t.Run(ts.name, func(t *testing.T) {
				for _, target := range allStatuses {
					t.Run("To"+string(target), func(t *testing.T) {
						assert.False(t, TaskStateMachine.CanTransition(ts.status, target), "CanTransition should not allow transition from a terminal task state")
					})
				}
			})
		}
	})
}

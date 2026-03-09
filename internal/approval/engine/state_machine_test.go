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
		assert.NotNil(t, sm, "Should not be nil")
		assert.Equal(t, "test", sm.name, "Should equal expected value")
		assert.Empty(t, sm.transitions, "Should be empty")
	})

	t.Run("CanTransitionReturnsFalseOnEmpty", func(t *testing.T) {
		sm := NewStateMachine[TestState]("test")
		assert.False(t, sm.CanTransition(TestStateA, TestStateB), "Should return false")
	})

	t.Run("AvailableTransitionsReturnsNilOnEmpty", func(t *testing.T) {
		sm := NewStateMachine[TestState]("test")
		assert.Nil(t, sm.AvailableTransitions(TestStateA), "Should return nil")
	})

	t.Run("TransitionReturnsErrorOnEmpty", func(t *testing.T) {
		sm := NewStateMachine[TestState]("test")
		err := sm.Transition(TestStateA, TestStateB)
		require.Error(t, err, "Should return error")
		assert.Contains(t, err.Error(), "invalid state transition", "Should contain expected value")
	})

	t.Run("ChainingReturnsSameInstance", func(t *testing.T) {
		sm := NewStateMachine[TestState]("test")
		returned := sm.AddTransition(TestStateA, TestStateB, "go_b")
		assert.Same(t, sm, returned, "Should return the same instance")
	})

	t.Run("MultipleChainedCalls", func(t *testing.T) {
		sm := NewStateMachine[TestState]("test").
			AddTransition(TestStateA, TestStateB, "a_to_b").
			AddTransition(TestStateB, TestStateC, "b_to_c").
			AddTransition(TestStateC, TestStateFinal, "c_to_final")

		assert.True(t, sm.CanTransition(TestStateA, TestStateB), "Should allow a to b")
		assert.True(t, sm.CanTransition(TestStateB, TestStateC), "Should allow b to c")
		assert.True(t, sm.CanTransition(TestStateC, TestStateFinal), "Should allow c to final")
		assert.False(t, sm.CanTransition(TestStateA, TestStateC), "Should not allow a to c")
	})

	t.Run("Overwrite", func(t *testing.T) {
		sm := NewStateMachine[TestState]("test")
		sm.AddTransition(TestStateA, TestStateB, "original_event")
		sm.AddTransition(TestStateA, TestStateB, "overwritten_event")

		tr := sm.transitions[TestStateA][TestStateB]
		require.NotNil(t, tr, "Should not be nil")
		assert.Equal(t, "overwritten_event", tr.Event, "Should equal expected value")
	})

	t.Run("UnregisteredFrom", func(t *testing.T) {
		sm := NewStateMachine[TestState]("test")
		sm.AddTransition(TestStateA, TestStateB, "a_to_b")
		assert.False(t, sm.CanTransition(TestStateC, TestStateA), "Should return false for unregistered from state")
	})

	t.Run("UnregisteredTo", func(t *testing.T) {
		sm := NewStateMachine[TestState]("test")
		sm.AddTransition(TestStateA, TestStateB, "a_to_b")
		assert.False(t, sm.CanTransition(TestStateA, TestStateC), "Should return false for unregistered to state")
	})

	t.Run("FinalStateAsFrom", func(t *testing.T) {
		sm := NewStateMachine[TestState]("test")
		sm.AddTransition(TestStateA, TestStateB, "a_to_b")
		assert.False(t, sm.CanTransition(TestStateFinal, TestStateA), "Should return false for final state as from")
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
				assert.True(t, InstanceStateMachine.CanTransition(tt.from, tt.to), "Condition should be true")
				assert.NoError(t, InstanceStateMachine.Transition(tt.from, tt.to), "Should not return error")
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
				assert.False(t, InstanceStateMachine.CanTransition(tt.from, tt.to), "Condition should be false")
				assert.Error(t, InstanceStateMachine.Transition(tt.from, tt.to), "Should return error")
			})
		}
	})

	t.Run("AvailableTransitions", func(t *testing.T) {
		t.Run("RunningState", func(t *testing.T) {
			targets := InstanceStateMachine.AvailableTransitions(approval.InstanceRunning)
			require.Len(t, targets, 5, "Should have 5 available transitions from running")

			targetSet := collections.NewHashSetFrom(targets...)
			assert.True(t, targetSet.Contains(approval.InstanceApproved), "Should include approved")
			assert.True(t, targetSet.Contains(approval.InstanceRejected), "Should include rejected")
			assert.True(t, targetSet.Contains(approval.InstanceWithdrawn), "Should include withdrawn")
			assert.True(t, targetSet.Contains(approval.InstanceReturned), "Should include returned")
			assert.True(t, targetSet.Contains(approval.InstanceTerminated), "Should include terminated")
		})

		t.Run("ReturnedState", func(t *testing.T) {
			targets := InstanceStateMachine.AvailableTransitions(approval.InstanceReturned)
			require.Len(t, targets, 1, "Should have 1 available transition from returned")
			assert.Equal(t, approval.InstanceRunning, targets[0], "Should equal expected value")
		})

		t.Run("WithdrawnState", func(t *testing.T) {
			targets := InstanceStateMachine.AvailableTransitions(approval.InstanceWithdrawn)
			require.Len(t, targets, 1, "Should have 1 available transition from withdrawn")
			assert.Equal(t, approval.InstanceRunning, targets[0], "Should equal expected value")
		})

		t.Run("ApprovedState", func(t *testing.T) {
			assert.Nil(t, InstanceStateMachine.AvailableTransitions(approval.InstanceApproved), "Should have no transitions from approved")
		})

		t.Run("RejectedState", func(t *testing.T) {
			assert.Nil(t, InstanceStateMachine.AvailableTransitions(approval.InstanceRejected), "Should have no transitions from rejected")
		})
	})

	t.Run("TransitionError", func(t *testing.T) {
		err := InstanceStateMachine.Transition(approval.InstanceApproved, approval.InstanceRunning)
		require.Error(t, err, "Should return error for invalid transition")
		assert.Contains(t, err.Error(), "invalid state transition", "Should contain expected value")
		assert.Contains(t, err.Error(), "approved", "Should contain expected value")
		assert.Contains(t, err.Error(), "running", "Should contain expected value")
	})

	t.Run("EventValues", func(t *testing.T) {
		tests := []struct {
			name  string
			from  approval.InstanceStatus
			to    approval.InstanceStatus
			event string
		}{
			{"RunningToApproved", approval.InstanceRunning, approval.InstanceApproved, "complete_approved"},
			{"RunningToRejected", approval.InstanceRunning, approval.InstanceRejected, "complete_rejected"},
			{"RunningToWithdrawn", approval.InstanceRunning, approval.InstanceWithdrawn, "withdraw"},
			{"RunningToReturned", approval.InstanceRunning, approval.InstanceReturned, "return_to_initiator"},
			{"ReturnedToRunning", approval.InstanceReturned, approval.InstanceRunning, "resubmit"},
			{"WithdrawnToRunning", approval.InstanceWithdrawn, approval.InstanceRunning, "resubmit"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				tr := InstanceStateMachine.transitions[tt.from][tt.to]
				require.NotNil(t, tr, "Should not be nil")
				assert.Equal(t, tt.event, tr.Event, "Should equal expected value")
				assert.Equal(t, tt.from, tr.From, "Should equal expected value")
				assert.Equal(t, tt.to, tr.To, "Should equal expected value")
			})
		}
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
						assert.False(t, InstanceStateMachine.CanTransition(ts.status, target), "Should not allow transition from terminal state")
						assert.Error(t, InstanceStateMachine.Transition(ts.status, target), "Should return error")
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
				assert.True(t, TaskStateMachine.CanTransition(tt.from, tt.to), "Condition should be true")
				assert.NoError(t, TaskStateMachine.Transition(tt.from, tt.to), "Should not return error")
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
				assert.False(t, TaskStateMachine.CanTransition(tt.from, tt.to), "Condition should be false")
				assert.Error(t, TaskStateMachine.Transition(tt.from, tt.to), "Should return error")
			})
		}
	})

	t.Run("AvailableTransitions", func(t *testing.T) {
		t.Run("PendingState", func(t *testing.T) {
			pending := TaskStateMachine.AvailableTransitions(approval.TaskPending)
			assert.Len(t, pending, 8, "Should have 8 available transitions from pending")

			targetSet := collections.NewHashSetFrom(pending...)
			assert.True(t, targetSet.Contains(approval.TaskHandled), "Should include handled in pending transitions")
		})

		t.Run("WaitingState", func(t *testing.T) {
			waiting := TaskStateMachine.AvailableTransitions(approval.TaskWaiting)
			assert.Len(t, waiting, 4, "Should have 4 available transitions from waiting")
		})

		t.Run("TerminalStates", func(t *testing.T) {
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

			for _, tt := range terminalStates {
				t.Run(tt.name, func(t *testing.T) {
					assert.Nil(t, TaskStateMachine.AvailableTransitions(tt.status), "Should have no transitions from terminal state")
				})
			}
		})
	})

	t.Run("TransitionError", func(t *testing.T) {
		err := TaskStateMachine.Transition(approval.TaskApproved, approval.TaskPending)
		require.Error(t, err, "Should return error for invalid transition")
		assert.Contains(t, err.Error(), "invalid state transition", "Should contain expected value")
		assert.Contains(t, err.Error(), "approved", "Should contain expected value")
		assert.Contains(t, err.Error(), "pending", "Should contain expected value")
	})

	t.Run("EventValues", func(t *testing.T) {
		tests := []struct {
			name  string
			from  approval.TaskStatus
			to    approval.TaskStatus
			event string
		}{
			{"WaitingToPending", approval.TaskWaiting, approval.TaskPending, "activate"},
			{"WaitingToCanceled", approval.TaskWaiting, approval.TaskCanceled, "cancel"},
			{"WaitingToSkipped", approval.TaskWaiting, approval.TaskSkipped, "skip"},
			{"WaitingToRemoved", approval.TaskWaiting, approval.TaskRemoved, "remove"},
			{"PendingToApproved", approval.TaskPending, approval.TaskApproved, "approve"},
			{"PendingToHandled", approval.TaskPending, approval.TaskHandled, "handle"},
			{"PendingToRejected", approval.TaskPending, approval.TaskRejected, "reject"},
			{"PendingToTransferred", approval.TaskPending, approval.TaskTransferred, "transfer"},
			{"PendingToRollback", approval.TaskPending, approval.TaskRolledBack, "rollback"},
			{"PendingToCanceled", approval.TaskPending, approval.TaskCanceled, "cancel"},
			{"PendingToWaiting", approval.TaskPending, approval.TaskWaiting, "wait_for_before"},
			{"PendingToRemoved", approval.TaskPending, approval.TaskRemoved, "remove"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				tr := TaskStateMachine.transitions[tt.from][tt.to]
				require.NotNil(t, tr, "Should not be nil")
				assert.Equal(t, tt.event, tr.Event, "Should equal expected value")
				assert.Equal(t, tt.from, tr.From, "Should equal expected value")
				assert.Equal(t, tt.to, tr.To, "Should equal expected value")
			})
		}
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
						assert.False(t, TaskStateMachine.CanTransition(ts.status, target), "Should not allow transition from terminal state")
						assert.Error(t, TaskStateMachine.Transition(ts.status, target), "Should return error")
					})
				}
			})
		}
	})
}

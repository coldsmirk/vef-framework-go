package engine

import (
	"github.com/coldsmirk/vef-framework-go/approval"
)

// State represents a state that can be used in a state machine.
type State interface {
	comparable

	// String returns the string representation of this state.
	String() string
	// IsFinal returns true if this is a terminal state with no further transitions.
	IsFinal() bool
}

// StateMachine manages state transitions.
type StateMachine[S State] struct {
	name        string
	transitions map[S]map[S]struct{}
}

// NewStateMachine creates a new state machine with the given name.
func NewStateMachine[S State](name string) *StateMachine[S] {
	return &StateMachine[S]{
		name:        name,
		transitions: make(map[S]map[S]struct{}),
	}
}

// AddTransition registers a valid state transition.
func (sm *StateMachine[S]) AddTransition(from, to S) *StateMachine[S] {
	if sm.transitions[from] == nil {
		sm.transitions[from] = make(map[S]struct{})
	}

	sm.transitions[from][to] = struct{}{}

	return sm
}

// CanTransition checks if a transition from one state to another is valid.
func (sm *StateMachine[S]) CanTransition(from, to S) bool {
	targets, ok := sm.transitions[from]
	if !ok {
		return false
	}

	_, ok = targets[to]

	return ok
}

// InstanceStateMachine defines valid instance state transitions.
var InstanceStateMachine = buildInstanceStateMachine()

func buildInstanceStateMachine() *StateMachine[approval.InstanceStatus] {
	sm := NewStateMachine[approval.InstanceStatus]("instance")
	sm.AddTransition(approval.InstanceRunning, approval.InstanceApproved)
	sm.AddTransition(approval.InstanceRunning, approval.InstanceRejected)
	sm.AddTransition(approval.InstanceRunning, approval.InstanceWithdrawn)
	sm.AddTransition(approval.InstanceRunning, approval.InstanceTerminated)
	sm.AddTransition(approval.InstanceRunning, approval.InstanceReturned)
	sm.AddTransition(approval.InstanceReturned, approval.InstanceRunning)
	sm.AddTransition(approval.InstanceWithdrawn, approval.InstanceRunning)

	return sm
}

// TaskStateMachine defines valid task state transitions.
var TaskStateMachine = buildTaskStateMachine()

func buildTaskStateMachine() *StateMachine[approval.TaskStatus] {
	sm := NewStateMachine[approval.TaskStatus]("task")
	sm.AddTransition(approval.TaskWaiting, approval.TaskPending)
	sm.AddTransition(approval.TaskWaiting, approval.TaskCanceled)
	sm.AddTransition(approval.TaskWaiting, approval.TaskSkipped)
	sm.AddTransition(approval.TaskWaiting, approval.TaskRemoved)
	sm.AddTransition(approval.TaskPending, approval.TaskApproved)
	sm.AddTransition(approval.TaskPending, approval.TaskHandled)
	sm.AddTransition(approval.TaskPending, approval.TaskRejected)
	sm.AddTransition(approval.TaskPending, approval.TaskTransferred)
	sm.AddTransition(approval.TaskPending, approval.TaskRolledBack)
	sm.AddTransition(approval.TaskPending, approval.TaskCanceled)
	sm.AddTransition(approval.TaskPending, approval.TaskWaiting)
	sm.AddTransition(approval.TaskPending, approval.TaskRemoved)

	return sm
}

package engine

import (
	"fmt"

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

// Transition defines a state transition.
type Transition[S State] struct {
	From  S
	To    S
	Event string
}

// StateMachine manages state transitions.
type StateMachine[S State] struct {
	name        string
	transitions map[S]map[S]*Transition[S]
}

// NewStateMachine creates a new state machine with the given name.
func NewStateMachine[S State](name string) *StateMachine[S] {
	return &StateMachine[S]{
		name:        name,
		transitions: make(map[S]map[S]*Transition[S]),
	}
}

// AddTransition registers a valid state transition.
func (sm *StateMachine[S]) AddTransition(from, to S, event string) *StateMachine[S] {
	if sm.transitions[from] == nil {
		sm.transitions[from] = make(map[S]*Transition[S])
	}

	sm.transitions[from][to] = &Transition[S]{From: from, To: to, Event: event}

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

// Transition performs a state transition, returning an error if invalid.
func (sm *StateMachine[S]) Transition(from, to S) error {
	if !sm.CanTransition(from, to) {
		return fmt.Errorf("%w: %s from %s to %s", errInvalidTransition, sm.name, from, to)
	}

	return nil
}

// AvailableTransitions returns all valid target states from the given state.
func (sm *StateMachine[S]) AvailableTransitions(from S) []S {
	targets, ok := sm.transitions[from]
	if !ok {
		return nil
	}

	result := make([]S, 0, len(targets))
	for to := range targets {
		result = append(result, to)
	}

	return result
}

// InstanceStateMachine defines valid instance state transitions.
var InstanceStateMachine = buildInstanceStateMachine()

func buildInstanceStateMachine() *StateMachine[approval.InstanceStatus] {
	sm := NewStateMachine[approval.InstanceStatus]("instance")
	sm.AddTransition(approval.InstanceRunning, approval.InstanceApproved, "complete_approved")
	sm.AddTransition(approval.InstanceRunning, approval.InstanceRejected, "complete_rejected")
	sm.AddTransition(approval.InstanceRunning, approval.InstanceWithdrawn, "withdraw")
	sm.AddTransition(approval.InstanceRunning, approval.InstanceTerminated, "terminate")
	sm.AddTransition(approval.InstanceRunning, approval.InstanceReturned, "return_to_initiator")
	sm.AddTransition(approval.InstanceReturned, approval.InstanceRunning, "resubmit")
	sm.AddTransition(approval.InstanceWithdrawn, approval.InstanceRunning, "resubmit")

	return sm
}

// TaskStateMachine defines valid task state transitions.
var TaskStateMachine = buildTaskStateMachine()

func buildTaskStateMachine() *StateMachine[approval.TaskStatus] {
	sm := NewStateMachine[approval.TaskStatus]("task")
	sm.AddTransition(approval.TaskWaiting, approval.TaskPending, "activate")
	sm.AddTransition(approval.TaskWaiting, approval.TaskCanceled, "cancel")
	sm.AddTransition(approval.TaskWaiting, approval.TaskSkipped, "skip")
	sm.AddTransition(approval.TaskWaiting, approval.TaskRemoved, "remove")
	sm.AddTransition(approval.TaskPending, approval.TaskApproved, "approve")
	sm.AddTransition(approval.TaskPending, approval.TaskHandled, "handle")
	sm.AddTransition(approval.TaskPending, approval.TaskRejected, "reject")
	sm.AddTransition(approval.TaskPending, approval.TaskTransferred, "transfer")
	sm.AddTransition(approval.TaskPending, approval.TaskRolledBack, "rollback")
	sm.AddTransition(approval.TaskPending, approval.TaskCanceled, "cancel")
	sm.AddTransition(approval.TaskPending, approval.TaskWaiting, "wait_for_before")
	sm.AddTransition(approval.TaskPending, approval.TaskRemoved, "remove")

	return sm
}

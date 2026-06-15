package approval

import (
	"reflect"

	"github.com/coldsmirk/vef-framework-go/timex"
)

// PayloadOccurredAt extracts the OccurredTime field from a DomainEvent
// payload. Every in-tree approval event struct carries this field;
// publishers use the value to project business time onto
// Envelope.OccurredAt via event.WithOccurredAt. Returns the zero DateTime
// for payloads that lack the field (defensive — should never happen for
// in-tree events).
//
// Exposed as a package-level helper rather than a method on DomainEvent so
// the interface can stay minimal (EventType only) while still letting
// transports / behaviors project business time.
func PayloadOccurredAt(e DomainEvent) timex.DateTime {
	v := reflect.Indirect(reflect.ValueOf(e))
	if !v.IsValid() || v.Kind() != reflect.Struct {
		return timex.DateTime{}
	}

	f := v.FieldByName("OccurredTime")
	if !f.IsValid() || f.Type() != reflect.TypeFor[timex.DateTime]() {
		return timex.DateTime{}
	}

	t, _ := f.Interface().(timex.DateTime)

	return t
}

// stringPtrOrNil returns nil for empty strings, or a pointer to the string value.
func stringPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}

	return &s
}

// DomainEvent is the contract every approval domain event satisfies.
// EventType matches the framework's event.Event surface so domain
// events can be published through the event Bus without adaptation.
// Business time is carried as OccurredTime in the concrete payload and
// projected onto Envelope.OccurredAt via event.WithOccurredAt at publish time.
//
// Tenant scope: every instance/task/node/cc-level event carries TenantID so
// subscribers can route on tenancy without re-querying. Flow-level events
// (created/updated/etc.) already include TenantID on the payload.
type DomainEvent interface {
	// EventType returns the unique event identifier (e.g., "approval.instance.created").
	EventType() string
}

// Approval event type identifiers. Exposed as constants so framework
// callers (route inspection, subscription filters, metrics labels) can
// reference them by symbol rather than risking a typo on the wire string.
const (
	EventTypeInstanceCreated       = "approval.instance.created"
	EventTypeInstanceCompleted     = "approval.instance.completed"
	EventTypeInstanceWithdrawn     = "approval.instance.withdrawn"
	EventTypeInstanceRolledBack    = "approval.instance.rolled_back"
	EventTypeInstanceReturned      = "approval.instance.returned"
	EventTypeInstanceResubmitted   = "approval.instance.resubmitted"
	EventTypeInstanceBindingFailed = "approval.instance.binding_failed"

	EventTypeNodeAutoPassed = "approval.node.auto_passed"

	EventTypeTaskCreated         = "approval.task.created"
	EventTypeTaskApproved        = "approval.task.approved"
	EventTypeTaskHandled         = "approval.task.handled"
	EventTypeTaskRejected        = "approval.task.rejected"
	EventTypeTaskCanceled        = "approval.task.canceled"
	EventTypeTaskTransferred     = "approval.task.transferred"
	EventTypeTaskReassigned      = "approval.task.reassigned"
	EventTypeTaskTimedOut        = "approval.task.timed_out"
	EventTypeAssigneesAdded      = "approval.task.assignees_added"
	EventTypeAssigneesRemoved    = "approval.task.assignees_removed"
	EventTypeTaskDeadlineWarning = "approval.task.deadline_warning"
	EventTypeTaskUrged           = "approval.task.urged"

	EventTypeCCNotified = "approval.cc.notified"

	EventTypeFlowCreated   = "approval.flow.created"
	EventTypeFlowUpdated   = "approval.flow.updated"
	EventTypeFlowDeployed  = "approval.flow.deployed"
	EventTypeFlowToggled   = "approval.flow.toggled"
	EventTypeFlowPublished = "approval.flow.published"
)

// AllEventTypes returns every approval domain event type identifier. It is
// the canonical, enumerable registry of the event surface: callers that must
// stay exhaustive over the event set (e.g. the start-up routing check, which
// asserts each transactional event resolves to a transactional transport)
// derive from this list so adding a new event constant cannot silently slip
// past them. Keep this in sync with the EventType* constants above — the only
// reason a new constant would be omitted is a deliberate, reviewed decision.
func AllEventTypes() []string {
	return []string{
		EventTypeInstanceCreated,
		EventTypeInstanceCompleted,
		EventTypeInstanceWithdrawn,
		EventTypeInstanceRolledBack,
		EventTypeInstanceReturned,
		EventTypeInstanceResubmitted,
		EventTypeInstanceBindingFailed,
		EventTypeNodeAutoPassed,
		EventTypeTaskCreated,
		EventTypeTaskApproved,
		EventTypeTaskHandled,
		EventTypeTaskRejected,
		EventTypeTaskCanceled,
		EventTypeTaskTransferred,
		EventTypeTaskReassigned,
		EventTypeTaskTimedOut,
		EventTypeAssigneesAdded,
		EventTypeAssigneesRemoved,
		EventTypeTaskDeadlineWarning,
		EventTypeTaskUrged,
		EventTypeCCNotified,
		EventTypeFlowCreated,
		EventTypeFlowUpdated,
		EventTypeFlowDeployed,
		EventTypeFlowToggled,
		EventTypeFlowPublished,
	}
}

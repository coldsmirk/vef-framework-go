package approval_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// EventInnerWithoutOccurredTime is nested in EventWithoutOccurredTime so the
// reflection fallback can see a struct with no OccurredTime field.
type EventInnerWithoutOccurredTime struct{ Ignored string }

// EventWithoutOccurredTime is a DomainEvent without an OccurredTime field, used
// to drive the reflection fall-through branch of PayloadOccurredAt.
type EventWithoutOccurredTime struct {
	Inner any
}

func (*EventWithoutOccurredTime) EventType() string { return "fake.event" }

func TestPayloadOccurredAt(t *testing.T) {
	t.Parallel()

	t.Run("InstanceCreated", func(t *testing.T) {
		t.Parallel()

		evt := approval.NewInstanceCreatedEvent("i1", "t1", "f1", "title", "u1", "Alice")
		got := approval.PayloadOccurredAt(evt)
		assert.False(t, got.IsZero(), "InstanceCreatedEvent should carry OccurredTime")
		assert.WithinDuration(t, time.Now(), got.Unwrap(), 2*time.Second, "OccurredTime should be wall-clock close to now")
	})

	t.Run("TaskApproved", func(t *testing.T) {
		t.Parallel()

		evt := approval.NewTaskApprovedEvent("ta1", "t1", "i1", "n1", "u1", "ok")
		got := approval.PayloadOccurredAt(evt)
		assert.False(t, got.IsZero(), "TaskApprovedEvent should carry OccurredTime")
	})

	t.Run("NilPayloadReturnsZero", func(t *testing.T) {
		t.Parallel()

		var evt approval.DomainEvent

		got := approval.PayloadOccurredAt(evt)
		assert.True(t, got.IsZero(), "Nil DomainEvent should return zero DateTime")
	})

	t.Run("PointerStructWithoutField", func(t *testing.T) {
		t.Parallel()

		var typed approval.DomainEvent = &EventWithoutOccurredTime{Inner: EventInnerWithoutOccurredTime{Ignored: "x"}}

		got := approval.PayloadOccurredAt(typed)
		assert.True(t, got.IsZero(), "Struct without OccurredTime field should return zero DateTime")
	})

	t.Run("ZeroDateTimeIsZero", func(t *testing.T) {
		t.Parallel()

		evt := &approval.InstanceCompletedEvent{
			InstanceID:   "i1",
			TenantID:     "t1",
			FinalStatus:  approval.InstanceRejected,
			OccurredTime: timex.DateTime{},
		}
		got := approval.PayloadOccurredAt(evt)
		assert.True(t, got.IsZero(), "Explicit zero OccurredTime should report zero")
	})
}

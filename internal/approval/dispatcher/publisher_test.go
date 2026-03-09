package dispatcher

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/mapx"
	"github.com/coldsmirk/vef-framework-go/timex"
)

func TestEventNames(t *testing.T) {
	tests := []struct {
		name     string
		event    approval.DomainEvent
		expected string
	}{
		{"InstanceCreated", approval.NewInstanceCreatedEvent("i1", "f1", "title", "u1", ""), "approval.instance.created"},
		{"InstanceCompleted", approval.NewInstanceCompletedEvent("i1", approval.InstanceApproved), "approval.instance.completed"},
		{"InstanceWithdrawn", approval.NewInstanceWithdrawnEvent("i1", "u1"), "approval.instance.withdrawn"},
		{"InstanceRolledBack", approval.NewInstanceRolledBackEvent("i1", "n1", "n2", "u1"), "approval.instance.rolled_back"},
		{"NodeEntered", approval.NewNodeEnteredEvent("i1", "n1", "Node"), "approval.node.entered"},
		{"NodeAutoPassed", approval.NewNodeAutoPassedEvent("i1", "n1", "reason"), "approval.node.auto_passed"},
		{"ParallelJoined", approval.NewParallelJoinedEvent("i1", "n1"), "approval.node.parallel_joined"},
		{"TaskCreated", approval.NewTaskCreatedEvent("t1", "i1", "n1", "u1", "", nil), "approval.task.created"},
		{"TaskApproved", approval.NewTaskApprovedEvent("t1", "i1", "n1", "u1", "ok"), "approval.task.approved"},
		{"TaskRejected", approval.NewTaskRejectedEvent("t1", "i1", "n1", "u1", "no"), "approval.task.rejected"},
		{"TaskTransferred", approval.NewTaskTransferredEvent("t1", "i1", "n1", "u1", "", "u2", "", "reason"), "approval.task.transferred"},
		{"TaskTimeout", approval.NewTaskTimeoutEvent("t1", "i1", "n1", "u1", "", timex.Now()), "approval.task.timeout"},
		{"AssigneesAdded", approval.NewAssigneesAddedEvent("i1", "n1", "t1", approval.AddAssigneeBefore, []string{"u1"}, nil), "approval.task.assignees_added"},
		{"AssigneesRemoved", approval.NewAssigneesRemovedEvent("i1", "n1", "t1", []string{"u1"}, nil), "approval.task.assignees_removed"},
		{"CcNotified", approval.NewCCNotifiedEvent("i1", "n1", []string{"u1"}, nil, true), "approval.cc.notified"},
		{"FlowPublished", approval.NewFlowPublishedEvent("f1", "v1"), "approval.flow.published"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.event.EventName(), "Should return correct event name")
		})
	}
}

func TestEventConstructors(t *testing.T) {
	t.Run("InstanceCreatedEvent", func(t *testing.T) {
		evt := approval.NewInstanceCreatedEvent("inst1", "flow1", "My Title", "user1", "")
		require.NotNil(t, evt, "Should create event")
		assert.Equal(t, "inst1", evt.InstanceID, "Should set InstanceID")
		assert.Equal(t, "flow1", evt.FlowID, "Should set FlowID")
		assert.Equal(t, "My Title", evt.Title, "Should set Title")
		assert.Equal(t, "user1", evt.ApplicantID, "Should set ApplicantID")
	})

	t.Run("InstanceCompletedEvent", func(t *testing.T) {
		evt := approval.NewInstanceCompletedEvent("inst1", approval.InstanceRejected)
		require.NotNil(t, evt, "Should create event")
		assert.Equal(t, "inst1", evt.InstanceID, "Should set InstanceID")
		assert.Equal(t, approval.InstanceRejected, evt.FinalStatus, "Should set FinalStatus")
		assert.False(t, evt.FinishedAt.IsZero(), "Should have FinishedAt set")
	})

	t.Run("InstanceWithdrawnEvent", func(t *testing.T) {
		evt := approval.NewInstanceWithdrawnEvent("inst1", "user1")
		require.NotNil(t, evt, "Should create event")
		assert.Equal(t, "inst1", evt.InstanceID, "Should set InstanceID")
		assert.Equal(t, "user1", evt.OperatorID, "Should set OperatorID")
	})

	t.Run("InstanceRolledBackEvent", func(t *testing.T) {
		evt := approval.NewInstanceRolledBackEvent("i1", "from_node", "to_node", "op1")
		require.NotNil(t, evt, "Should create event")
		assert.Equal(t, "i1", evt.InstanceID, "Should set InstanceID")
		assert.Equal(t, "from_node", evt.FromNodeID, "Should set FromNodeID")
		assert.Equal(t, "to_node", evt.ToNodeID, "Should set ToNodeID")
		assert.Equal(t, "op1", evt.OperatorID, "Should set OperatorID")
	})

	t.Run("NodeEnteredEvent", func(t *testing.T) {
		evt := approval.NewNodeEnteredEvent("i1", "n1", "Approval Node")
		require.NotNil(t, evt, "Should create event")
		assert.Equal(t, "i1", evt.InstanceID, "Should set InstanceID")
		assert.Equal(t, "n1", evt.NodeID, "Should set NodeID")
		assert.Equal(t, "Approval Node", evt.NodeName, "Should set NodeName")
	})

	t.Run("NodeAutoPassedEvent", func(t *testing.T) {
		evt := approval.NewNodeAutoPassedEvent("i1", "n1", "no assignees")
		require.NotNil(t, evt, "Should create event")
		assert.Equal(t, "i1", evt.InstanceID, "Should set InstanceID")
		assert.Equal(t, "n1", evt.NodeID, "Should set NodeID")
		assert.Equal(t, "no assignees", evt.Reason, "Should set Reason")
	})

	t.Run("ParallelJoinedEvent", func(t *testing.T) {
		evt := approval.NewParallelJoinedEvent("i1", "n1")
		require.NotNil(t, evt, "Should create event")
		assert.Equal(t, "i1", evt.InstanceID, "Should set InstanceID")
		assert.Equal(t, "n1", evt.NodeID, "Should set NodeID")
	})

	t.Run("TaskCreatedWithDeadline", func(t *testing.T) {
		deadline := timex.Now().Add(24 * time.Hour)
		evt := approval.NewTaskCreatedEvent("t1", "i1", "n1", "u1", "", &deadline)
		require.NotNil(t, evt, "Should create event")
		assert.Equal(t, "t1", evt.TaskID, "Should set TaskID")
		assert.Equal(t, "i1", evt.InstanceID, "Should set InstanceID")
		assert.Equal(t, "n1", evt.NodeID, "Should set NodeID")
		assert.Equal(t, "u1", evt.AssigneeID, "Should set AssigneeID")
		require.NotNil(t, evt.Deadline, "Should have deadline set")
		assert.Equal(t, deadline, *evt.Deadline, "Should set correct deadline value")
	})

	t.Run("TaskCreatedWithoutDeadline", func(t *testing.T) {
		evt := approval.NewTaskCreatedEvent("t1", "i1", "n1", "u1", "", nil)
		require.NotNil(t, evt, "Should create event")
		assert.Nil(t, evt.Deadline, "Should have nil deadline")
	})

	t.Run("TaskApprovedEvent", func(t *testing.T) {
		evt := approval.NewTaskApprovedEvent("t1", "i1", "n1", "op1", "looks good")
		require.NotNil(t, evt, "Should create event")
		assert.Equal(t, "t1", evt.TaskID, "Should set TaskID")
		assert.Equal(t, "i1", evt.InstanceID, "Should set InstanceID")
		assert.Equal(t, "n1", evt.NodeID, "Should set NodeID")
		assert.Equal(t, "op1", evt.OperatorID, "Should set OperatorID")
		require.NotNil(t, evt.Opinion, "Should set Opinion")
		assert.Equal(t, "looks good", *evt.Opinion, "Should set Opinion")
	})

	t.Run("TaskRejectedEvent", func(t *testing.T) {
		evt := approval.NewTaskRejectedEvent("t1", "i1", "n1", "op1", "not acceptable")
		require.NotNil(t, evt, "Should create event")
		assert.Equal(t, "t1", evt.TaskID, "Should set TaskID")
		assert.Equal(t, "i1", evt.InstanceID, "Should set InstanceID")
		assert.Equal(t, "n1", evt.NodeID, "Should set NodeID")
		assert.Equal(t, "op1", evt.OperatorID, "Should set OperatorID")
		require.NotNil(t, evt.Opinion, "Should set Opinion")
		assert.Equal(t, "not acceptable", *evt.Opinion, "Should set Opinion")
	})

	t.Run("TaskTransferredEvent", func(t *testing.T) {
		evt := approval.NewTaskTransferredEvent("t1", "i1", "n1", "from", "", "to", "", "reason")
		require.NotNil(t, evt, "Should create event")
		assert.Equal(t, "t1", evt.TaskID, "Should set TaskID")
		assert.Equal(t, "i1", evt.InstanceID, "Should set InstanceID")
		assert.Equal(t, "n1", evt.NodeID, "Should set NodeID")
		assert.Equal(t, "from", evt.FromUserID, "Should set FromUserID")
		assert.Equal(t, "to", evt.ToUserID, "Should set ToUserID")
		require.NotNil(t, evt.Reason, "Should set Reason")
		assert.Equal(t, "reason", *evt.Reason, "Should set Reason")
	})

	t.Run("TaskTimeoutEvent", func(t *testing.T) {
		deadline := timex.Now().Add(-time.Hour)
		evt := approval.NewTaskTimeoutEvent("t1", "i1", "n1", "u1", "", deadline)
		require.NotNil(t, evt, "Should create event")
		assert.Equal(t, "t1", evt.TaskID, "Should set TaskID")
		assert.Equal(t, "i1", evt.InstanceID, "Should set InstanceID")
		assert.Equal(t, "n1", evt.NodeID, "Should set NodeID")
		assert.Equal(t, "u1", evt.AssigneeID, "Should set AssigneeID")
		assert.Equal(t, deadline, evt.Deadline, "Should set Deadline")
	})

	t.Run("AssigneesAddedEvent", func(t *testing.T) {
		evt := approval.NewAssigneesAddedEvent("i1", "n1", "t1", approval.AddAssigneeBefore, []string{"u1", "u2"}, nil)
		require.NotNil(t, evt, "Should create event")
		assert.Equal(t, "i1", evt.InstanceID, "Should set InstanceID")
		assert.Equal(t, "n1", evt.NodeID, "Should set NodeID")
		assert.Equal(t, "t1", evt.TaskID, "Should set TaskID")
		assert.Equal(t, approval.AddAssigneeBefore, evt.AddType, "Should set AddType")
		assert.Equal(t, []string{"u1", "u2"}, evt.AssigneeIDs, "Should set AssigneeIDs")
	})

	t.Run("AssigneesRemovedEvent", func(t *testing.T) {
		evt := approval.NewAssigneesRemovedEvent("i1", "n1", "t1", []string{"u3"}, nil)
		require.NotNil(t, evt, "Should create event")
		assert.Equal(t, "i1", evt.InstanceID, "Should set InstanceID")
		assert.Equal(t, "n1", evt.NodeID, "Should set NodeID")
		assert.Equal(t, "t1", evt.TaskID, "Should set TaskID")
		assert.Equal(t, []string{"u3"}, evt.AssigneeIDs, "Should set AssigneeIDs")
	})

	t.Run("CcNotifiedManual", func(t *testing.T) {
		evt := approval.NewCCNotifiedEvent("i1", "n1", []string{"u1", "u2"}, nil, true)
		require.NotNil(t, evt, "Should create event")
		assert.Equal(t, "i1", evt.InstanceID, "Should set InstanceID")
		assert.Equal(t, "n1", evt.NodeID, "Should set NodeID")
		assert.Equal(t, []string{"u1", "u2"}, evt.CCUserIDs, "Should set CCUserIDs")
		assert.True(t, evt.IsManual, "Should be manual CC")
	})

	t.Run("CcNotifiedAutomatic", func(t *testing.T) {
		evt := approval.NewCCNotifiedEvent("i1", "n1", []string{"u1"}, nil, false)
		require.NotNil(t, evt, "Should create event")
		assert.False(t, evt.IsManual, "Should be automatic CC")
	})

	t.Run("FlowPublishedEvent", func(t *testing.T) {
		evt := approval.NewFlowPublishedEvent("f1", "v1")
		require.NotNil(t, evt, "Should create event")
		assert.Equal(t, "f1", evt.FlowID, "Should set FlowID")
		assert.Equal(t, "v1", evt.VersionID, "Should set VersionID")
	})
}

func TestEventOccurredAtAll(t *testing.T) {
	before := timex.Now().Add(-time.Second)
	deadline := timex.Now()

	events := []approval.DomainEvent{
		approval.NewInstanceCreatedEvent("i1", "f1", "title", "u1", ""),
		approval.NewInstanceCompletedEvent("i1", approval.InstanceApproved),
		approval.NewInstanceWithdrawnEvent("i1", "u1"),
		approval.NewInstanceRolledBackEvent("i1", "n1", "n2", "u1"),
		approval.NewNodeEnteredEvent("i1", "n1", "Node"),
		approval.NewNodeAutoPassedEvent("i1", "n1", "reason"),
		approval.NewParallelJoinedEvent("i1", "n1"),
		approval.NewTaskCreatedEvent("t1", "i1", "n1", "u1", "", nil),
		approval.NewTaskApprovedEvent("t1", "i1", "n1", "u1", "ok"),
		approval.NewTaskRejectedEvent("t1", "i1", "n1", "u1", "no"),
		approval.NewTaskTransferredEvent("t1", "i1", "n1", "u1", "", "u2", "", "r"),
		approval.NewTaskTimeoutEvent("t1", "i1", "n1", "u1", "", deadline),
		approval.NewAssigneesAddedEvent("i1", "n1", "t1", approval.AddAssigneeBefore, []string{"u1"}, nil),
		approval.NewAssigneesRemovedEvent("i1", "n1", "t1", []string{"u1"}, nil),
		approval.NewCCNotifiedEvent("i1", "n1", []string{"u1"}, nil, false),
		approval.NewFlowPublishedEvent("f1", "v1"),
	}

	for _, evt := range events {
		assert.True(t, evt.OccurredAt().After(before), "Should have recent OccurredAt for %s", evt.EventName())
	}
}

func TestToMap(t *testing.T) {
	t.Run("ValidStruct", func(t *testing.T) {
		m, err := mapx.ToMap(approval.NewFlowPublishedEvent("f1", "v1"))
		require.NoError(t, err, "Should marshal valid struct")
		assert.Equal(t, "f1", m["flowId"], "Should contain flowId")
		assert.Equal(t, "v1", m["versionId"], "Should contain versionId")
		assert.Contains(t, m, "occurredTime", "Should contain occurredTime")
	})

	t.Run("StructWithAllFields", func(t *testing.T) {
		m, err := mapx.ToMap(approval.NewTaskTransferredEvent("t1", "i1", "n1", "from", "", "to", "", "reason"))
		require.NoError(t, err, "Should marshal struct with all fields")
		assert.Equal(t, "t1", m["taskId"], "Should contain taskId")
		assert.Equal(t, "i1", m["instanceId"], "Should contain instanceId")
		assert.Equal(t, "n1", m["nodeId"], "Should contain nodeId")
		assert.Equal(t, "from", m["fromUserId"], "Should contain fromUserId")
		assert.Equal(t, "to", m["toUserId"], "Should contain toUserId")
		require.IsType(t, (*string)(nil), m["reason"], "Should contain reason as *string")
		assert.Equal(t, "reason", *m["reason"].(*string), "Should contain reason")
	})

	t.Run("NonStructValue", func(t *testing.T) {
		_, err := mapx.ToMap("not a struct")
		assert.ErrorIs(t, err, mapx.ErrInvalidToMapValue, "Should fail for non-struct type")
	})
}

// BadEvent is a non-struct DomainEvent that causes mapx.ToMap to fail.
type BadEvent string

func (BadEvent) EventName() string          { return "bad.event" }
func (BadEvent) OccurredAt() timex.DateTime { return timex.Now() }

func TestNewEventPublisher(t *testing.T) {
	pub := NewEventPublisher()
	require.NotNil(t, pub, "Should create publisher")
}

func TestPublishAll(t *testing.T) {
	ctx := context.Background()
	pub := NewEventPublisher()

	t.Run("NilEvents", func(t *testing.T) {
		err := pub.PublishAll(ctx, nil, nil)
		require.NoError(t, err, "Should return nil for nil events slice")
	})

	t.Run("EmptySlice", func(t *testing.T) {
		err := pub.PublishAll(ctx, nil, []approval.DomainEvent{})
		require.NoError(t, err, "Should return nil for zero-length events slice")
	})

	t.Run("InsertsSingleEvent", func(t *testing.T) {
		db := testx.NewTestDB(t)
		_, err := db.NewCreateTable().Model((*approval.EventOutbox)(nil)).IfNotExists().Exec(ctx)
		require.NoError(t, err, "Should create table")

		evt := approval.NewFlowPublishedEvent("f1", "v1")
		err = pub.PublishAll(ctx, db, []approval.DomainEvent{evt})
		require.NoError(t, err, "Should insert event without error")

		var records []approval.EventOutbox

		err = db.NewSelect().Model(&records).Scan(ctx)
		require.NoError(t, err, "Should query records")
		require.Len(t, records, 1, "Should insert exactly one record")

		rec := records[0]
		assert.NotEmpty(t, rec.EventID, "Should generate EventID")
		assert.Len(t, rec.EventID, 36, "EventID should be UUID format")
		assert.Equal(t, "approval.flow.published", rec.EventType, "Should set EventType from event name")
		assert.Equal(t, approval.EventOutboxPending, rec.Status, "Should set status to pending")
		assert.Equal(t, "f1", rec.Payload["flowId"], "Should include flowId in payload")
		assert.Equal(t, "v1", rec.Payload["versionId"], "Should include versionId in payload")
	})

	t.Run("InsertsBatchEvents", func(t *testing.T) {
		db := testx.NewTestDB(t)
		_, err := db.NewCreateTable().Model((*approval.EventOutbox)(nil)).IfNotExists().Exec(ctx)
		require.NoError(t, err, "Should create table")

		events := []approval.DomainEvent{
			approval.NewInstanceCreatedEvent("i1", "f1", "Title", "u1", ""),
			approval.NewTaskApprovedEvent("t1", "i1", "n1", "op1", "ok"),
			approval.NewFlowPublishedEvent("f2", "v2"),
		}
		err = pub.PublishAll(ctx, db, events)
		require.NoError(t, err, "Should insert batch events without error")

		var records []approval.EventOutbox

		err = db.NewSelect().Model(&records).OrderBy("event_type").Scan(ctx)
		require.NoError(t, err, "Should query records")
		require.Len(t, records, 3, "Should insert all three records")

		assert.Equal(t, "approval.flow.published", records[0].EventType, "Should set correct event type")
		assert.Equal(t, "approval.instance.created", records[1].EventType, "Should set correct event type")
		assert.Equal(t, "approval.task.approved", records[2].EventType, "Should set correct event type")

		for _, rec := range records {
			assert.Equal(t, approval.EventOutboxPending, rec.Status, "Should set status to pending for all records")
			assert.NotEmpty(t, rec.EventID, "Should generate EventID for all records")
		}
	})

	t.Run("ReturnsErrorForNonStructEvent", func(t *testing.T) {
		db := testx.NewTestDB(t)
		_, err := db.NewCreateTable().Model((*approval.EventOutbox)(nil)).IfNotExists().Exec(ctx)
		require.NoError(t, err, "Should create table")

		err = pub.PublishAll(ctx, db, []approval.DomainEvent{BadEvent("test")})
		require.Error(t, err, "Should return error for non-struct event")
		assert.Contains(t, err.Error(), "bad.event", "Should include event name in error message")
	})
}

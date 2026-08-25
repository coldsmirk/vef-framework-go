package approval_test

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// newEventFixtures builds the instance / task / node trio event constructors
// snapshot from.
func newEventFixtures() (*approval.Instance, *approval.Task, *approval.FlowNode) {
	ref := "ord-1"
	dept := "d1"
	deptName := "Finance"

	instance := &approval.Instance{
		TenantID:                "t1",
		FlowID:                  "f1",
		FlowCode:                "expense",
		FlowVersionID:           "fv1",
		Title:                   "Trip reimbursement",
		InstanceNo:              "APV-001",
		ApplicantID:             "u1",
		ApplicantName:           "Alice",
		ApplicantDepartmentID:   &dept,
		ApplicantDepartmentName: &deptName,
		Status:                  approval.InstanceRunning,
		BusinessRef:             &ref,
	}
	instance.ID = "i1"

	task := &approval.Task{
		TenantID:     "t1",
		InstanceID:   "i1",
		NodeID:       "n1",
		VisitID:      "v1",
		AssigneeID:   "u2",
		AssigneeName: "Bob",
		Status:       approval.TaskPending,
	}
	task.ID = "ta1"

	node := &approval.FlowNode{
		FlowVersionID: "fv1",
		Key:           "approve1",
		Kind:          approval.NodeApproval,
		Name:          "Manager approval",
	}
	node.ID = "n1"

	return instance, task, node
}

// TestInstanceEventBaseEnvelope pins the shared envelope contract: the base
// snapshots the instance verbatim and serializes flat (embedded fields are
// promoted, never nested under a struct key).
func TestInstanceEventBaseEnvelope(t *testing.T) {
	t.Parallel()

	instance, _, _ := newEventFixtures()
	evt := approval.NewInstanceCreatedEvent(instance)

	assert.Equal(t, "i1", evt.InstanceID, "InstanceID should snapshot the instance id")
	assert.Equal(t, "APV-001", evt.InstanceNo, "InstanceNo should snapshot")
	assert.Equal(t, "t1", evt.TenantID, "TenantID should snapshot")
	assert.Equal(t, "Trip reimbursement", evt.Title, "Title should snapshot")
	assert.Equal(t, "f1", evt.FlowID, "FlowID should snapshot")
	assert.Equal(t, "expense", evt.FlowCode, "FlowCode should snapshot")
	require.NotNil(t, evt.BusinessRef, "BusinessRef should carry over when bound")
	assert.Equal(t, "ord-1", *evt.BusinessRef, "BusinessRef should snapshot")
	assert.Equal(t, instance.Applicant(), evt.Applicant, "Applicant should be the full person snapshot")
	assert.False(t, evt.OccurredTime.IsZero(), "OccurredTime should be stamped")

	raw, err := json.Marshal(evt)
	require.NoError(t, err, "event should marshal")

	var flat map[string]any

	require.NoError(t, json.Unmarshal(raw, &flat), "event JSON should be an object")
	assert.Contains(t, flat, "instanceId", "envelope fields must serialize flat at the top level")
	assert.Contains(t, flat, "flowCode", "envelope fields must serialize flat at the top level")
	assert.NotContains(t, string(raw), "InstanceEventBase", "the embedded base must not appear as a nested key")

	applicant, ok := flat["applicant"].(map[string]any)
	require.True(t, ok, "applicant should be a nested UserInfo object")
	assert.Equal(t, "Alice", applicant["name"], "applicant should carry the person snapshot")
}

// TestTaskEventBaseEnvelope pins the task-level envelope: instance envelope
// plus task/node coordinates.
func TestTaskEventBaseEnvelope(t *testing.T) {
	t.Parallel()

	instance, task, node := newEventFixtures()
	evt := approval.NewTaskCreatedEvent(instance, task, node)

	assert.Equal(t, "ta1", evt.TaskID, "TaskID should snapshot")
	assert.Equal(t, "n1", evt.NodeID, "NodeID should come from the node")
	assert.Equal(t, "Manager approval", evt.NodeName, "NodeName should come from the node")
	assert.Equal(t, "expense", evt.FlowCode, "task events should inherit the instance envelope")
	assert.Equal(t, task.Assignee(), evt.Assignee, "assignee should be the full person snapshot")
}

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

		instance, _, _ := newEventFixtures()
		evt := approval.NewInstanceCreatedEvent(instance)
		got := approval.PayloadOccurredAt(evt)
		assert.False(t, got.IsZero(), "InstanceCreatedEvent should carry OccurredTime")
		assert.WithinDuration(t, time.Now(), got.Unwrap(), 2*time.Second, "OccurredTime should be wall-clock close to now")
	})

	t.Run("TaskApproved", func(t *testing.T) {
		t.Parallel()

		instance, task, node := newEventFixtures()
		evt := approval.NewTaskApprovedEvent(instance, task, node, approval.UserInfo{ID: "u1", Name: "Alice"}, "ok")
		got := approval.PayloadOccurredAt(evt)
		assert.False(t, got.IsZero(), "TaskApprovedEvent should carry OccurredTime — embedded base fields must stay reachable by reflection")
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
			OccurredTime: timex.DateTime{},
			FinalStatus:  approval.InstanceRejected,
		}
		got := approval.PayloadOccurredAt(evt)
		assert.True(t, got.IsZero(), "Explicit zero OccurredTime should report zero")
	})
}

// eventTypeConstsFromSource parses events.go and returns the string values of
// every top-level `EventType*` constant declared in it, by reflecting over the
// real source AST rather than a hand-maintained list. Because EventType* are
// untyped string constants (not an enumerable type), AST parsing — not reflect
// — is the only way to recover the full declared set, mirroring how
// instance_dispatch_test.go reflects over the live dispatch source.
func eventTypeConstsFromSource(t *testing.T) []string {
	t.Helper()

	fset := token.NewFileSet()

	file, err := parser.ParseFile(fset, "events.go", nil, 0)
	require.NoError(t, err, "parsing events.go should succeed")

	var got []string

	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.CONST {
			continue
		}

		for _, spec := range genDecl.Specs {
			valueSpec, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}

			for i, name := range valueSpec.Names {
				if !strings.HasPrefix(name.Name, "EventType") {
					continue
				}

				require.Less(t, i, len(valueSpec.Values),
					"EventType const %q must have an explicit value", name.Name)

				lit, ok := valueSpec.Values[i].(*ast.BasicLit)
				require.True(t, ok && lit.Kind == token.STRING,
					"EventType const %q must be assigned a string literal", name.Name)

				unquoted, unquoteErr := strconv.Unquote(lit.Value)
				require.NoError(t, unquoteErr, "unquoting %q literal should succeed", name.Name)

				got = append(got, unquoted)
			}
		}
	}

	return got
}

// TestAllEventTypesMatchesSourceConstants is the real drift guard for
// AllEventTypes(): it asserts the slice returned by AllEventTypes() equals
// EXACTLY the set of EventType* string constants declared in events.go — no
// missing, no extra, no duplicates. The companion test in
// internal/approval/module_test.go derives its expectation from
// AllEventTypes() itself, so it cannot catch a new EventType* constant that a
// developer forgets to append to the AllEventTypes() slice literal; such an
// event would silently bypass the start-up verifyEventRouting check and only
// fail at runtime with event.ErrTxRequired (rolling back a business tx). This
// test closes that gap by comparing against the source of truth (the consts).
func TestAllEventTypesMatchesSourceConstants(t *testing.T) {
	declared := eventTypeConstsFromSource(t)
	require.NotEmpty(t, declared, "events.go must declare at least one EventType* constant")

	returned := approval.AllEventTypes()

	t.Run("NoDuplicatesInAllEventTypes", func(t *testing.T) {
		seen := make(map[string]struct{}, len(returned))
		for _, et := range returned {
			_, dup := seen[et]
			assert.False(t, dup, "AllEventTypes() must not contain the duplicate %q", et)
			seen[et] = struct{}{}
		}
	})

	t.Run("ExactlyMatchesDeclaredConstants", func(t *testing.T) {
		wantSorted := slices.Sorted(slices.Values(declared))
		gotSorted := slices.Sorted(slices.Values(returned))

		assert.Equal(t, wantSorted, gotSorted,
			"AllEventTypes() must enumerate exactly the EventType* constants declared in events.go; "+
				"a new EventType* const must be appended to the AllEventTypes() slice literal, "+
				"otherwise the new event silently bypasses the start-up routing check")
	})
}

func TestInstanceLifecycleEventOpinions(t *testing.T) {
	instance, _, node := newEventFixtures()
	operator := approval.UserInfo{ID: "op1", Name: "Olivia"}
	opinion := "needs a revised quote"

	withdrawn := approval.NewInstanceWithdrawnEvent(instance, operator, &opinion)
	require.Equal(t, &opinion, withdrawn.Reason, "Withdrawn event should carry the applicant reason")

	rolled := approval.NewInstanceRolledBackEvent(instance, node, node, operator, &opinion)
	require.Equal(t, &opinion, rolled.Opinion, "RolledBack event should carry the operator opinion")

	returned := approval.NewInstanceReturnedEvent(instance, node, node, operator, nil)
	require.Nil(t, returned.Opinion, "Returned event opinion should stay nil when none was provided")

	completed := approval.NewInstanceCompletedEvent(instance, approval.InstanceTerminated)
	require.Nil(t, completed.Reason, "Completed event reason defaults to nil; only the terminate path sets it")

	payload, err := json.Marshal(withdrawn)
	require.NoError(t, err, "Withdrawn event should marshal")
	require.Contains(t, string(payload), `"reason":"needs a revised quote"`, "Reason should serialize on the wire")

	payload, err = json.Marshal(returned)
	require.NoError(t, err, "Returned event should marshal")
	require.NotContains(t, string(payload), `"opinion"`, "Nil opinion should be omitted from the wire")
}

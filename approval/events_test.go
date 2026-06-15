package approval_test

import (
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

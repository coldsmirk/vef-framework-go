package approval

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/event"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// SubscribeSpyBus captures the single subscription SubscribeInstance makes so
// tests can inspect the resolved group / concurrency and replay events
// through the registered handler.
type SubscribeSpyBus struct {
	eventType    string
	group        string
	concurrency  int
	handler      event.Handler
	subscribeErr error
	unsubscribed bool
}

func (b *SubscribeSpyBus) Subscribe(eventType string, h event.Handler, opts ...event.SubscribeOption) (event.Unsubscribe, error) {
	if b.subscribeErr != nil {
		return nil, b.subscribeErr
	}

	cfg := event.ApplySubscribeOptions(opts)
	b.eventType = eventType
	b.group = cfg.Group
	b.concurrency = cfg.Concurrency
	b.handler = h

	return func() { b.unsubscribed = true }, nil
}

func (*SubscribeSpyBus) Publish(context.Context, event.Event, ...event.PublishOption) error {
	return nil
}

func (*SubscribeSpyBus) PublishBatch(context.Context, []event.Event, ...event.PublishOption) error {
	return nil
}

// emit replays an event through the captured handler the way an in-process
// delivery would (payload carries the typed event directly).
func (b *SubscribeSpyBus) emit(t *testing.T, evt event.Event) error {
	t.Helper()
	require.NotNil(t, b.handler, "emit requires a captured subscription handler")

	return b.handler(t.Context(), event.Envelope{ID: "envelope-1", Type: evt.EventType(), Payload: evt})
}

// SubscribeSpySvc is the method-value subscriber used to exercise group
// derivation — its runtime identity is stable and assertable.
type SubscribeSpySvc struct {
	calls []*InstanceCompletedEvent
	envs  []event.Envelope
}

func (s *SubscribeSpySvc) Handle(_ context.Context, evt *InstanceCompletedEvent, env event.Envelope) error {
	s.calls = append(s.calls, evt)
	s.envs = append(s.envs, env)

	return nil
}

func namedInstanceHandler(context.Context, *InstanceCompletedEvent, event.Envelope) error {
	return nil
}

func completedEvent(flowCode, tenantID string) *InstanceCompletedEvent {
	return &InstanceCompletedEvent{
		InstanceEventBase: InstanceEventBase{FlowCode: flowCode, TenantID: tenantID},
		FinalStatus:       InstanceApproved,
	}
}

func TestDeriveGroup(t *testing.T) {
	t.Run("MethodValue", func(t *testing.T) {
		svc := new(SubscribeSpySvc)

		group, err := deriveGroup(svc.Handle)
		require.NoError(t, err, "A method value has a stable runtime identity")
		assert.Equal(t, "vef:sub:approval.SubscribeSpySvc.Handle", group,
			"Derived name should be the vef:sub: prefix plus module-relative package, type, and method")
	})

	t.Run("NamedFunction", func(t *testing.T) {
		group, err := deriveGroup(namedInstanceHandler)
		require.NoError(t, err, "A package-level named function has a stable runtime identity")
		assert.Equal(t, "vef:sub:approval.namedInstanceHandler", group,
			"Named functions derive without a type segment")
	})

	t.Run("AnonymousRejected", func(t *testing.T) {
		_, err := deriveGroup(func(context.Context, *InstanceCompletedEvent, event.Envelope) error { return nil })
		assert.ErrorIs(t, err, ErrAnonymousSubscriberGroup,
			"Anonymous functions carry positional counters and must not derive a group")
	})
}

func TestTrimModulePrefix(t *testing.T) {
	tests := []struct {
		name       string
		pkgPath    string
		modulePath string
		want       string
	}{
		{"ModuleRootCollapsesToBase", "example.com/foo", "example.com/foo", "foo"},
		{"SubPackageTrimmed", "example.com/foo/internal/mms", "example.com/foo", "internal/mms"},
		{"SiblingModuleNotSwallowed", "example.com/foobar/pkg", "example.com/foo", "example.com/foobar/pkg"},
		{"UnrelatedModuleKept", "other.org/lib", "example.com/foo", "other.org/lib"},
		{"EmptyModuleKeepsPath", "example.com/foo/pkg", "", "example.com/foo/pkg"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, trimModulePrefix(tt.pkgPath, tt.modulePath),
				"Trim must strip only at path-segment boundaries")
		})
	}
}

func TestInstanceFilterMatches(t *testing.T) {
	tests := []struct {
		name     string
		filter   InstanceFilter
		flowCode string
		tenantID string
		want     bool
	}{
		{"EmptyFilterMatchesEverything", InstanceFilter{}, "any", "any", true},
		{"FlowCodeInList", ForFlows("a", "b"), "b", "t1", true},
		{"FlowCodeNotInList", ForFlows("a", "b"), "c", "t1", false},
		{"TenantInList", ForTenants("t1"), "any", "t1", true},
		{"TenantNotInList", ForTenants("t1"), "any", "t2", false},
		{"BothDimensionsMustMatch", InstanceFilter{FlowCodes: []string{"a"}, TenantIDs: []string{"t1"}}, "a", "t2", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.filter.Matches(tt.flowCode, tt.tenantID),
				"Filter match verdict should follow the OR-within / AND-across contract")
		})
	}
}

func TestSubscribeInstance(t *testing.T) {
	t.Run("DerivedGroupAndFiltering", func(t *testing.T) {
		bus := &SubscribeSpyBus{}
		svc := new(SubscribeSpySvc)

		unsubscribe, err := SubscribeInstance(bus, svc.Handle, ForFlows("right_application"))
		require.NoError(t, err, "Method-value subscription should derive its group")

		t.Cleanup(unsubscribe)

		assert.Equal(t, "approval.instance.completed", bus.eventType,
			"Event type should be deduced from the handler's event parameter")
		assert.Equal(t, "vef:sub:approval.SubscribeSpySvc.Handle", bus.group,
			"Derived group should reach the underlying subscription")

		require.NoError(t, bus.emit(t, completedEvent("other_flow", "t1")),
			"A filtered-out event must be acknowledged, not errored")
		assert.Empty(t, svc.calls, "Handler must not see events of other flows")

		require.NoError(t, bus.emit(t, completedEvent("right_application", "t1")),
			"A matching event should be handled")
		require.Len(t, svc.calls, 1, "Handler should see exactly the matching event")
		assert.Equal(t, "right_application", svc.calls[0].FlowCode, "Handler should receive the typed event")
		require.Len(t, svc.envs, 1, "Handler should receive the delivery envelope")
		assert.Equal(t, "envelope-1", svc.envs[0].ID,
			"Envelope.ID must reach the handler as the manual idempotency key")
	})

	t.Run("ExplicitGroupBypassesDerivation", func(t *testing.T) {
		bus := &SubscribeSpyBus{}

		unsubscribe, err := SubscribeInstance(bus,
			func(context.Context, *InstanceCompletedEvent, event.Envelope) error { return nil },
			WithGroup("smp:right-application"))
		require.NoError(t, err, "An explicit group must make anonymous handlers legal")

		t.Cleanup(unsubscribe)

		assert.Equal(t, "smp:right-application", bus.group, "Explicit group should pass through verbatim")
	})

	t.Run("AnonymousWithoutGroupFails", func(t *testing.T) {
		bus := &SubscribeSpyBus{}

		_, err := SubscribeInstance(bus, func(context.Context, *InstanceCompletedEvent, event.Envelope) error { return nil })
		assert.ErrorIs(t, err, ErrAnonymousSubscriberGroup,
			"An anonymous handler without WithGroup must fail fast")
	})

	t.Run("DuplicateDerivedGroupConflicts", func(t *testing.T) {
		bus := &SubscribeSpyBus{}
		svc := new(SubscribeSpySvc)

		unsubscribe, err := SubscribeInstance(bus, svc.Handle)
		require.NoError(t, err, "First subscription should succeed")

		_, err = SubscribeInstance(&SubscribeSpyBus{}, svc.Handle)
		assert.ErrorIs(t, err, ErrDerivedGroupConflict,
			"The same method subscribed twice in one process must fail fast")

		unsubscribe()
		assert.True(t, bus.unsubscribed, "Wrapped unsubscribe should release the inner subscription")

		retried, err := SubscribeInstance(&SubscribeSpyBus{}, svc.Handle)
		require.NoError(t, err, "Unsubscribing should release the derived name for reuse")

		t.Cleanup(retried)
	})

	t.Run("UnsubscribeIsIdempotent", func(t *testing.T) {
		svc := new(SubscribeSpySvc)

		unsubscribe, err := SubscribeInstance(new(SubscribeSpyBus), svc.Handle)
		require.NoError(t, err, "First subscription should succeed")
		unsubscribe()

		rebound, err := SubscribeInstance(new(SubscribeSpyBus), svc.Handle)
		require.NoError(t, err, "Released group should be reusable")

		t.Cleanup(rebound)

		unsubscribe()

		_, err = SubscribeInstance(new(SubscribeSpyBus), svc.Handle)
		assert.ErrorIs(t, err, ErrDerivedGroupConflict,
			"A stale unsubscribe must not delete the newer subscription's group claim")
	})

	t.Run("SubscribeErrorReleasesDerivedName", func(t *testing.T) {
		svc := new(SubscribeSpySvc)
		failing := &SubscribeSpyBus{subscribeErr: errors.New("route not subscribable")}

		_, err := SubscribeInstance(failing, svc.Handle)
		require.Error(t, err, "Underlying subscribe errors should propagate")

		unsubscribe, err := SubscribeInstance(&SubscribeSpyBus{}, svc.Handle)
		require.NoError(t, err, "A failed subscription must not leak its derived name claim")

		t.Cleanup(unsubscribe)
	})

	t.Run("ConcurrencyForwarded", func(t *testing.T) {
		bus := &SubscribeSpyBus{}

		unsubscribe, err := SubscribeInstance(bus,
			func(context.Context, *InstanceCompletedEvent, event.Envelope) error { return nil },
			WithGroup("g"), WithConcurrency(4))
		require.NoError(t, err, "Subscription with concurrency should succeed")

		t.Cleanup(unsubscribe)

		assert.Equal(t, 4, bus.concurrency, "Concurrency should reach the underlying subscription")
	})
}

// RecordingLifecycleHook counts invocations so filter behavior is observable.
type RecordingLifecycleHook struct {
	created     int
	transitions int
}

func (h *RecordingLifecycleHook) OnInstanceCreated(context.Context, orm.DB, *Instance) error {
	h.created++

	return nil
}

func (h *RecordingLifecycleHook) OnInstanceTransition(context.Context, orm.DB, *Instance, InstanceStatus, InstanceStatus) error {
	h.transitions++

	return nil
}

func TestNewFilteredLifecycleHook(t *testing.T) {
	instance := func(flowCode, tenantID string) *Instance {
		return &Instance{FlowCode: flowCode, TenantID: tenantID}
	}

	t.Run("NoFiltersReturnsHookUnchanged", func(t *testing.T) {
		inner := new(RecordingLifecycleHook)
		assert.Same(t, InstanceLifecycleHook(inner), NewFilteredLifecycleHook(inner),
			"Zero filters should not add a wrapper layer")
	})

	t.Run("MatchingInstancePassesThrough", func(t *testing.T) {
		inner := new(RecordingLifecycleHook)
		hook := NewFilteredLifecycleHook(inner, ForFlows("leave"), ForTenants("t1"))

		require.NoError(t, hook.OnInstanceCreated(t.Context(), nil, instance("leave", "t1")),
			"Matching instance should reach the inner hook")
		require.NoError(t, hook.OnInstanceTransition(t.Context(), nil, instance("leave", "t1"), InstanceRunning, InstanceApproved),
			"Matching instance should reach the inner hook on transition")
		assert.Equal(t, 1, inner.created, "Created hook should fire once")
		assert.Equal(t, 1, inner.transitions, "Transition hook should fire once")
	})

	t.Run("NonMatchingInstanceIsNoOp", func(t *testing.T) {
		inner := new(RecordingLifecycleHook)
		hook := NewFilteredLifecycleHook(inner, ForFlows("leave"))

		require.NoError(t, hook.OnInstanceCreated(t.Context(), nil, instance("expense", "t1")),
			"Non-matching instance should no-op without error")
		require.NoError(t, hook.OnInstanceTransition(t.Context(), nil, instance("expense", "t1"), InstanceRunning, InstanceApproved),
			"Non-matching instance should no-op on transition")
		assert.Zero(t, inner.created, "Created hook must not fire for other flows")
		assert.Zero(t, inner.transitions, "Transition hook must not fire for other flows")
	})
}

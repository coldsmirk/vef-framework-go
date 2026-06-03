package memory

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/event/transport"
	"github.com/coldsmirk/vef-framework-go/event/transport/memory"
)

// TestEnqueueReturnsSubscriptionStoppedWhenStopped pins the sentinel that
// distinguishes a per-subscription teardown from a transport-level stop:
// enqueue on a subscription whose stopCh is closed must report
// errSubscriptionStopped, not the exported ErrBusStopped.
func TestEnqueueReturnsSubscriptionStoppedWhenStopped(t *testing.T) {
	sub := &subscription{
		queue:      make(chan transport.Frame), // unbuffered: always "full"
		stopCh:     make(chan struct{}),
		fullPolicy: memory.FullPolicyBlock,
	}
	close(sub.stopCh)

	err := sub.enqueue(context.Background(), transport.Frame{})
	require.ErrorIs(t, err, errSubscriptionStopped, "enqueue on a stopped subscription must return errSubscriptionStopped")
	require.NotErrorIs(t, err, ErrBusStopped, "a per-subscription stop must not masquerade as a transport stop")
}

// TestPublishSkipsStoppedSubscription verifies the fan-out loop treats a
// concurrently-stopped subscription as a skip rather than surfacing
// errSubscriptionStopped to the publisher.
func TestPublishSkipsStoppedSubscription(t *testing.T) {
	tp := New(memory.Config{FullPolicy: memory.FullPolicyBlock})

	stopped := &subscription{
		id:         "stopped",
		eventType:  "evt",
		queue:      make(chan transport.Frame), // unbuffered: always "full"
		stopCh:     make(chan struct{}),
		fullPolicy: memory.FullPolicyBlock,
		ctx:        context.Background(),
	}
	close(stopped.stopCh)

	tp.mu.Lock()
	tp.subs["evt"] = map[string]*subscription{"stopped": stopped}
	tp.mu.Unlock()

	require.NoError(t, tp.Publish(context.Background(), []transport.Frame{{Type: "evt"}}),
		"Publish must skip a stopped subscription rather than surface errSubscriptionStopped")
}

package push

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/push"
)

// ReceiveEnvelope reads the next enqueued envelope off a registry-only
// connection's send queue.
func ReceiveEnvelope(t *testing.T, conn *connection) push.Message {
	t.Helper()

	select {
	case payload := <-conn.send:
		var message push.Message
		require.NoError(t, json.Unmarshal(payload, &message), "The queued frame should carry the JSON envelope")

		return message

	case <-time.After(3 * time.Second):
		require.FailNow(t, "Expected an envelope on the connection queue")

		return push.Message{}
	}
}

func TestNewRelay(t *testing.T) {
	hub := NewHub(new(config.PushConfig))

	assert.Nil(t, NewRelay(hub, new(config.PushConfig), new(config.AppConfig), nil),
		"A disabled endpoint should build no relay")
	assert.Nil(t, NewRelay(hub, EnabledConfig(), new(config.AppConfig), nil),
		"Without a Redis client the hub pushes node-locally")
}

func TestRelayChannelFor(t *testing.T) {
	assert.Equal(t, "vef:push:relay", relayChannelFor(""), "An unnamed application uses the bare channel")
	assert.Equal(t, "vef:push:relay:crm", relayChannelFor("crm"),
		"The application name namespaces the channel — Pub/Sub is not isolated by the Redis database number")
}

func TestRelayAcrossNodes(t *testing.T) {
	ctx := context.Background()
	container := testx.NewRedisContainer(ctx, t)

	client := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%d", container.Redis.Host, container.Redis.Port),
		DB:   int(container.Redis.Database),
	})
	require.NoError(t, client.Ping(ctx).Err(), "Should connect to Redis")
	t.Cleanup(func() { _ = client.Close() })

	newNode := func(appName string) (*Hub, *Relay) {
		hub := NewHub(EnabledConfig())
		relay := NewRelay(hub, EnabledConfig(), &config.AppConfig{Name: appName}, client)
		require.NotNil(t, relay, "Relay should build with an enabled endpoint and a client")

		relay.start()
		t.Cleanup(relay.stop)

		return hub, relay
	}

	awaitSubscribers := func(channel string, count int64) {
		require.Eventually(t, func() bool {
			counts, err := client.PubSubNumSub(ctx, channel).Result()

			return err == nil && counts[channel] >= count
		}, 5*time.Second, 50*time.Millisecond, "Relays should be subscribed on %s", channel)
	}

	hubA, relayA := newNode("app")
	hubB, _ := newNode("app")
	awaitSubscribers(relayA.channel, 2)

	t.Run("FansOutMessagesToEveryNode", func(t *testing.T) {
		local := NewTestConnection("alice", "")
		require.NoError(t, hubA.register(local), "Local connection should register")

		remote := NewTestConnection("alice", "")
		require.NoError(t, hubB.register(remote), "Remote connection should register")

		require.NoError(t, relayA.Push(ctx, push.NewMessage("cross.node", "v"), push.ToUsers("alice")),
			"Relay push should succeed")

		assert.Equal(t, "cross.node", ReceiveEnvelope(t, local).Type, "The publishing node should deliver locally")
		assert.Equal(t, "cross.node", ReceiveEnvelope(t, remote).Type, "The other node should deliver via the relay")

		select {
		case <-local.send:
			assert.Fail(t, "The publishing node must skip its own relayed frame (origin dedupe)")
		case <-time.After(300 * time.Millisecond):
		}

		hubA.unregister(local)
		hubB.unregister(remote)
	})

	t.Run("KicksSessionsOnEveryNode", func(t *testing.T) {
		remote := NewTestConnection("bob", "hash-b")
		remote.sessionID = "s9"
		require.NoError(t, hubB.register(remote), "Remote connection should register")

		// The kick rides a revocation call path whose context ends with the
		// request — the detached publish must still reach the other nodes.
		canceled, cancel := context.WithCancel(ctx)
		cancel()
		relayA.KickSessions(canceled, []string{"s9"})

		require.Eventually(t, func() bool { return ConnectionClosing(remote) }, 3*time.Second, 20*time.Millisecond,
			"The kick should reach the other node")
		assert.Equal(t, push.CloseSessionInvalid, remote.closeCode, "The kick should use the session-invalid close code")
	})

	t.Run("IsolatesApplicationsOnASharedRedis", func(t *testing.T) {
		hubOther, relayOther := newNode("other-app")
		awaitSubscribers(relayOther.channel, 1)

		foreign := NewTestConnection("alice", "")
		require.NoError(t, hubOther.register(foreign), "The other application's connection should register")

		local := NewTestConnection("alice", "")
		require.NoError(t, hubB.register(local), "The same application's connection should register")

		require.NoError(t, relayA.Push(ctx, push.NewMessage("tenant.only", nil), push.Broadcast()),
			"Relay push should succeed")

		assert.Equal(t, "tenant.only", ReceiveEnvelope(t, local).Type, "The same application's node should deliver")

		select {
		case <-foreign.send:
			assert.Fail(t, "A broadcast must never cross into another application's channel")
		case <-time.After(300 * time.Millisecond):
		}

		hubB.unregister(local)
		hubOther.unregister(foreign)
	})

	t.Run("ValidatesLikeTheHub", func(t *testing.T) {
		require.ErrorIs(t, relayA.Push(ctx, push.NewMessage("t", nil)), push.ErrNoTarget,
			"Relay push shares the hub's validation")
	})
}

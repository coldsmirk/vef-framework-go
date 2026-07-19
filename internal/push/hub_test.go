package push

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/push"
	"github.com/coldsmirk/vef-framework-go/security"
)

// NewTestConnection builds a registry-only connection (no socket); safe for
// every hub operation that stays off the wire.
func NewTestConnection(userID, tokenHash string, roles ...string) *connection {
	return newConnection(nil, &security.Principal{ID: userID, Roles: roles}, tokenHash, 4)
}

func TestHubRegister(t *testing.T) {
	t.Run("EnforcesPerUserCap", func(t *testing.T) {
		hub := NewHub(&config.PushConfig{MaxConnectionsPerUser: 1})

		require.NoError(t, hub.register(NewTestConnection("alice", "")), "First connection should be admitted")
		require.ErrorIs(t, hub.register(NewTestConnection("alice", "")), errTooManyConnections,
			"Second connection should exceed the cap")
		require.NoError(t, hub.register(NewTestConnection("bob", "")), "The cap is per user, not global")
	})

	t.Run("UnregisterReleasesCapacity", func(t *testing.T) {
		hub := NewHub(&config.PushConfig{MaxConnectionsPerUser: 1})
		conn := NewTestConnection("alice", "")

		require.NoError(t, hub.register(conn), "Connection should be admitted")
		hub.unregister(conn)

		assert.Empty(t, hub.byUser, "An emptied user entry should be removed")
		require.NoError(t, hub.register(NewTestConnection("alice", "")), "Capacity should be released on unregister")
	})

	t.Run("RefusesAfterShutdown", func(t *testing.T) {
		hub := NewHub(new(config.PushConfig))
		hub.Shutdown()

		require.ErrorIs(t, hub.register(NewTestConnection("alice", "")), errHubClosed,
			"A shut-down hub should refuse registrations")
	})
}

func TestHubSelectRecipients(t *testing.T) {
	hub := NewHub(new(config.PushConfig))
	alice1 := NewTestConnection("alice", "", "admin")
	alice2 := NewTestConnection("alice", "")
	bob := NewTestConnection("bob", "", "auditor")

	for _, conn := range []*connection{alice1, alice2, bob} {
		require.NoError(t, hub.register(conn), "Fixture connections should register")
	}

	tests := []struct {
		name    string
		targets []push.Target
		want    []*connection
	}{
		{"ByUser", []push.Target{push.ToUsers("alice")}, []*connection{alice1, alice2}},
		{"ByUnknownUser", []push.Target{push.ToUsers("nobody")}, nil},
		{"ByRole", []push.Target{push.ToRoles("admin")}, []*connection{alice1}},
		{"ByAnyRole", []push.Target{push.ToRoles("admin", "auditor")}, []*connection{alice1, bob}},
		{"Broadcast", []push.Target{push.Broadcast()}, []*connection{alice1, alice2, bob}},
		{"UnionDeduplicates", []push.Target{push.ToUsers("alice"), push.ToRoles("admin")}, []*connection{alice1, alice2}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.ElementsMatch(t, tt.want, hub.selectRecipients(tt.targets),
				"Recipient selection should match the target semantics")
		})
	}
}

func TestHubPushValidation(t *testing.T) {
	hub := NewHub(new(config.PushConfig))
	ctx := context.Background()

	require.ErrorIs(t, hub.Push(ctx, push.Message{}, push.Broadcast()), push.ErrTypeRequired,
		"A message without a type should be rejected")
	require.ErrorIs(t, hub.Push(ctx, push.NewMessage("t", nil)), push.ErrNoTarget,
		"A push without targets should be rejected")
	require.ErrorIs(t, hub.Push(ctx, push.NewMessage("t", nil), push.Target{Kind: "bogus"}), push.ErrUnknownTargetKind,
		"A hand-built target with an unknown kind should be rejected")
}

func TestHubOpaqueConnections(t *testing.T) {
	hub := NewHub(new(config.PushConfig))
	tab1 := NewTestConnection("alice", "hash-a")
	tab2 := NewTestConnection("alice", "hash-a")
	bob := NewTestConnection("bob", "hash-b")
	jwt := NewTestConnection("carol", "")

	for _, conn := range []*connection{tab1, tab2, bob, jwt} {
		require.NoError(t, hub.register(conn), "Fixture connections should register")
	}

	groups := hub.opaqueConnections()

	require.Len(t, groups, 2, "Only opaque connections should be grouped")
	assert.ElementsMatch(t, []*connection{tab1, tab2}, groups["hash-a"], "Tabs sharing a login should share one group")
	assert.ElementsMatch(t, []*connection{bob}, groups["hash-b"], "Each token hash should form its own group")
}

func TestPushDropsSlowClient(t *testing.T) {
	auth := &FakeAuthManager{Principals: map[string]*security.Principal{"good": UserPrincipal("alice")}}
	cfg := EnabledConfig()
	cfg.SendBuffer = 1
	server := StartTestServer(t, cfg, new(config.SecurityConfig), auth)

	// The client never reads: large frames jam the socket, the one-slot queue
	// fills, and the next push must drop the connection instead of blocking.
	Dial(t, server, "good")
	require.Eventually(t, func() bool { return HubSize(server.Hub) == 1 }, time.Second, 10*time.Millisecond,
		"The connection should register")

	payload := strings.Repeat("x", 1<<20)
	for range 8 {
		require.NoError(t, server.Hub.Push(context.Background(), push.NewMessage("bulk", payload), push.ToUsers("alice")),
			"Push must never fail because of a slow recipient")
	}

	assert.Eventually(t, func() bool { return HubSize(server.Hub) == 0 }, 3*time.Second, 20*time.Millisecond,
		"The slow connection should be dropped")
}

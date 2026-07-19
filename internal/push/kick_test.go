package push

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/push"
	"github.com/coldsmirk/vef-framework-go/security"
)

func TestRevocationListener(t *testing.T) {
	t.Run("DisabledContributesNothing", func(t *testing.T) {
		listener := newRevocationListener(NewHub(new(config.PushConfig)), nil, new(config.PushConfig))

		assert.Nil(t, listener, "A disabled endpoint should register no listener")
	})

	t.Run("KicksRevokedSessionsLocally", func(t *testing.T) {
		hub := NewHub(new(config.PushConfig))

		kicked := NewTestConnection("alice", "hash-a")
		kicked.sessionID = "s1"
		survivor := NewTestConnection("alice", "hash-b")
		survivor.sessionID = "s2"

		for _, conn := range []*connection{kicked, survivor} {
			require.NoError(t, hub.register(conn), "Fixture connections should register")
		}

		listener := newRevocationListener(hub, nil, EnabledConfig())
		listener.OnSessionsRevoked(context.Background(),
			[]security.SessionRevocation{{SessionID: "s1", UserID: "alice"}})

		require.True(t, ConnectionClosing(kicked), "The revoked session's connection must close")
		assert.Equal(t, push.CloseSessionInvalid, kicked.closeCode, "The kick should use the session-invalid close code")
		assert.False(t, ConnectionClosing(survivor), "Other sessions of the same user must stay open")
	})
}

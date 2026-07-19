package push

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/security"
)

// revocationListener bridges security's session revocations into the push
// channel: a revoked session's connections close immediately instead of
// waiting for the periodic sweep. With a relay the kick reaches every node;
// without one it covers this node (the sweep remains the cross-node net).
type revocationListener struct {
	hub   *Hub
	relay *Relay
}

func newRevocationListener(hub *Hub, relay *Relay, cfg *config.PushConfig) security.SessionRevocationListener {
	if !cfg.Enabled {
		return nil
	}

	return &revocationListener{hub: hub, relay: relay}
}

func (l *revocationListener) OnSessionsRevoked(_ context.Context, revocations []security.SessionRevocation) {
	sessionIDs := make([]string, 0, len(revocations))
	for _, revocation := range revocations {
		sessionIDs = append(sessionIDs, revocation.SessionID)
	}

	if l.relay != nil {
		l.relay.KickSessions(sessionIDs)

		return
	}

	l.hub.closeSessions(sessionIDs)
}

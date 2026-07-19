package push

import (
	"context"
	"time"

	"github.com/coldsmirk/vef-framework-go/push"
	"github.com/coldsmirk/vef-framework-go/security"
)

// sessionSweeper periodically revalidates opaque-token connections against
// the session store: a revoked or expired session closes its connections with
// CloseSessionInvalid. It is deliberately a presence check via Lookup — never
// the authenticator, whose sliding renewal would turn an idle open tab into a
// session keepalive.
type sessionSweeper struct {
	hub      *Hub
	store    security.SessionStore
	interval time.Duration
	ctx      context.Context
	cancel   context.CancelFunc
	stopped  chan struct{}
}

func newSessionSweeper(hub *Hub, store security.SessionStore, interval time.Duration) *sessionSweeper {
	// The sweeper owns a cancelable context for its store calls: shutdown
	// cancels it before waiting, so an in-flight lookup against a slow or
	// blocking store can never stall the application stop.
	ctx, cancel := context.WithCancel(context.Background())

	return &sessionSweeper{
		hub:      hub,
		store:    store,
		interval: interval,
		ctx:      ctx,
		cancel:   cancel,
		stopped:  make(chan struct{}),
	}
}

func (s *sessionSweeper) start() {
	go s.run()
}

func (s *sessionSweeper) run() {
	defer close(s.stopped)

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.sweep(s.ctx)
		case <-s.ctx.Done():
			return
		}
	}
}

func (s *sessionSweeper) shutdown() {
	s.cancel()
	<-s.stopped
}

// sweep closes every connection whose session is gone. Store errors fail open
// (mirroring the login guard): the sweep is a revocation net, not an
// availability dependency.
func (s *sessionSweeper) sweep(ctx context.Context) {
	for tokenHash, conns := range s.hub.opaqueConnections() {
		if ctx.Err() != nil {
			return
		}

		session, err := s.store.Lookup(ctx, tokenHash)
		if err != nil {
			logger.Warnf("Push session sweep lookup failed: %v", err)

			continue
		}

		if session != nil {
			continue
		}

		for _, conn := range conns {
			conn.close(push.CloseSessionInvalid, "session invalid")
		}
	}
}

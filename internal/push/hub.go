package push

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"sync"
	"time"

	"github.com/coldsmirk/go-collections"
	"github.com/gofiber/contrib/v3/websocket"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/id"
	"github.com/coldsmirk/vef-framework-go/push"
)

// Hub is the per-node connection registry and the push.Notifier
// implementation: it indexes live connections by user and fans each message
// out to the selected recipients through their writer queues.
type Hub struct {
	maxPerUser int

	mu     sync.RWMutex
	conns  map[*connection]struct{}
	byUser map[string]map[*connection]struct{}
	closed bool
}

func NewHub(cfg *config.PushConfig) *Hub {
	return &Hub{
		maxPerUser: cfg.MaxConnectionsPerUser,
		conns:      make(map[*connection]struct{}),
		byUser:     make(map[string]map[*connection]struct{}),
	}
}

// Push implements push.Notifier for single-node deployments (the relay wraps
// deliver with cross-node publishing).
func (h *Hub) Push(_ context.Context, message push.Message, targets ...push.Target) error {
	payload, err := encodeMessage(&message, targets)
	if err != nil {
		return err
	}

	h.deliver(payload, targets)

	return nil
}

// encodeMessage validates the push, fills the generated envelope fields, and
// marshals the wire envelope once — every recipient (on every node) receives
// the same bytes.
func encodeMessage(message *push.Message, targets []push.Target) ([]byte, error) {
	if message.Type == "" {
		return nil, push.ErrTypeRequired
	}

	if len(targets) == 0 {
		return nil, push.ErrNoTarget
	}

	for _, target := range targets {
		switch target.Kind {
		case push.TargetUsers, push.TargetRoles:
			// An empty selector would silently deliver to nobody — the exact
			// caller bug ErrNoTarget exists to surface.
			if len(target.Values) == 0 {
				return nil, fmt.Errorf("%w: empty %s selector", push.ErrNoTarget, target.Kind)
			}
		case push.TargetBroadcast:
		default:
			return nil, fmt.Errorf("%w: %q", push.ErrUnknownTargetKind, target.Kind)
		}
	}

	if message.ID == "" {
		message.ID = id.Generate()
	}

	if message.Time.IsZero() {
		message.Time = time.Now()
	}

	payload, err := json.Marshal(*message)
	if err != nil {
		return nil, fmt.Errorf("marshal push message: %w", err)
	}

	return payload, nil
}

// deliver fans the marshaled envelope out to the selected local recipients; a
// recipient whose queue is full is a slow client and is dropped rather than
// allowed to block or backlog the fan-out.
func (h *Hub) deliver(payload []byte, targets []push.Target) {
	for _, conn := range h.selectRecipients(targets) {
		if !conn.enqueue(payload) {
			logger.Warnf("Dropping slow push connection of user %s", conn.userID)
			conn.terminate()
		}
	}
}

// closeSessions closes every local connection bound to one of the sessions —
// the instant revocation kick.
func (h *Hub) closeSessions(sessionIDs []string) {
	if len(sessionIDs) == 0 {
		return
	}

	ids := collections.NewHashSetFrom(sessionIDs...)

	h.mu.RLock()

	var kicked []*connection

	for conn := range h.conns {
		if conn.sessionID != "" && ids.Contains(conn.sessionID) {
			kicked = append(kicked, conn)
		}
	}

	h.mu.RUnlock()

	for _, conn := range kicked {
		conn.close(push.CloseSessionInvalid, "session revoked")
	}
}

// selectRecipients resolves the target union to a deduplicated connection
// set: every connection receives one copy no matter how many targets match it.
func (h *Hub) selectRecipients(targets []push.Target) []*connection {
	h.mu.RLock()
	defer h.mu.RUnlock()

	selected := make(map[*connection]struct{})

	for _, target := range targets {
		switch target.Kind {
		case push.TargetBroadcast:
			maps.Copy(selected, h.conns)
		case push.TargetUsers:
			for _, userID := range target.Values {
				maps.Copy(selected, h.byUser[userID])
			}
		case push.TargetRoles:
			roles := collections.NewHashSetFrom(target.Values...)
			for conn := range h.conns {
				if slices.ContainsFunc(conn.roles, roles.Contains) {
					selected[conn] = struct{}{}
				}
			}
		}
	}

	// Pending connections are quarantined until their session recheck passes:
	// kickable and counted toward the cap, but never a recipient.
	for conn := range selected {
		if conn.pending {
			delete(selected, conn)
		}
	}

	return slices.Collect(maps.Keys(selected))
}

// activate lifts a connection out of its pending quarantine; only after the
// post-register session recheck has passed does it become a push recipient.
func (h *Hub) activate(conn *connection) {
	h.mu.Lock()
	defer h.mu.Unlock()

	conn.pending = false
}

// register admits a connection, enforcing the per-user cap on this node.
func (h *Hub) register(conn *connection) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.closed {
		return errHubClosed
	}

	if h.maxPerUser > 0 && len(h.byUser[conn.userID]) >= h.maxPerUser {
		return errTooManyConnections
	}

	h.conns[conn] = struct{}{}

	users := h.byUser[conn.userID]
	if users == nil {
		users = make(map[*connection]struct{})
		h.byUser[conn.userID] = users
	}

	users[conn] = struct{}{}

	return nil
}

func (h *Hub) unregister(conn *connection) {
	h.mu.Lock()
	defer h.mu.Unlock()

	delete(h.conns, conn)

	if users := h.byUser[conn.userID]; users != nil {
		delete(users, conn)

		if len(users) == 0 {
			delete(h.byUser, conn.userID)
		}
	}
}

// opaqueConnections groups the opaque-token connections by token hash for the
// session sweep; one store lookup covers every tab sharing a login.
func (h *Hub) opaqueConnections() map[string][]*connection {
	h.mu.RLock()
	defer h.mu.RUnlock()

	groups := make(map[string][]*connection)

	for conn := range h.conns {
		if conn.tokenHash != "" {
			groups[conn.tokenHash] = append(groups[conn.tokenHash], conn)
		}
	}

	return groups
}

// Shutdown refuses new registrations and closes every live connection with a
// going-away frame. Connection goroutines unregister themselves as their
// sockets wind down.
func (h *Hub) Shutdown() {
	h.mu.Lock()
	h.closed = true
	conns := slices.Collect(maps.Keys(h.conns))
	h.mu.Unlock()

	for _, conn := range conns {
		conn.close(websocket.CloseGoingAway, "server shutting down")
	}
}

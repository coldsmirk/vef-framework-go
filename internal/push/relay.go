package push

import (
	"context"
	"encoding/json"

	"github.com/redis/go-redis/v9"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/id"
	"github.com/coldsmirk/vef-framework-go/push"
)

// relayChannel is the Redis Pub/Sub channel carrying push frames between
// nodes. Pub/Sub (not Streams) is deliberate: a frame must reach every node,
// needs no persistence or consumer groups, and a frame missed while a node is
// disconnected is worthless later — exactly the ephemeral fan-out contract.
const relayChannel = "vef:push:relay"

// Relay frame kinds.
const (
	frameMessage      = "message"
	frameKickSessions = "kick_sessions"
)

// relayFrame is the wire format between nodes: a pre-encoded message envelope
// with its targets, or a session kick.
type relayFrame struct {
	Kind       string          `json:"kind"`
	Origin     string          `json:"origin"`
	Envelope   json.RawMessage `json:"envelope,omitempty"`
	Targets    []push.Target   `json:"targets,omitempty"`
	SessionIDs []string        `json:"sessionIds,omitempty"`
}

// Relay implements push.Notifier for multi-node deployments: every push and
// revocation kick is applied to the local hub first, then published so the
// other nodes apply it to their own connections (own frames are recognized by
// origin and skipped). Local delivery never depends on Redis health — a
// publish failure degrades to node-local delivery under the best-effort
// contract.
type Relay struct {
	hub    *Hub
	client *redis.Client
	nodeID string

	pubsub  *redis.PubSub
	stopped chan struct{}
}

// NewRelay builds the cross-node relay; nil when the endpoint is disabled or
// no Redis client is available (single-node deployments push through the hub
// directly).
func NewRelay(hub *Hub, cfg *config.PushConfig, client *redis.Client) *Relay {
	if !cfg.Enabled || client == nil {
		return nil
	}

	return &Relay{
		hub:     hub,
		client:  client,
		nodeID:  id.Generate(),
		stopped: make(chan struct{}),
	}
}

// Push implements push.Notifier.
func (r *Relay) Push(ctx context.Context, message push.Message, targets ...push.Target) error {
	payload, err := encodeMessage(&message, targets)
	if err != nil {
		return err
	}

	r.hub.deliver(payload, targets)
	r.publish(ctx, relayFrame{Kind: frameMessage, Origin: r.nodeID, Envelope: payload, Targets: targets})

	return nil
}

// KickSessions closes the sessions' connections on every node.
func (r *Relay) KickSessions(ctx context.Context, sessionIDs []string) {
	r.hub.closeSessions(sessionIDs)
	r.publish(ctx, relayFrame{Kind: frameKickSessions, Origin: r.nodeID, SessionIDs: sessionIDs})
}

func (r *Relay) publish(ctx context.Context, frame relayFrame) {
	payload, err := json.Marshal(frame)
	if err != nil {
		logger.Errorf("Failed to marshal push relay frame: %v", err)

		return
	}

	if err := r.client.Publish(ctx, relayChannel, payload).Err(); err != nil {
		logger.Warnf("Push relay publish failed; delivery stays node-local: %v", err)
	}
}

// start opens the subscription and runs the apply loop. go-redis re-subscribes
// automatically after connection loss; frames missed meanwhile are lost
// (best-effort).
func (r *Relay) start() {
	r.pubsub = r.client.Subscribe(context.Background(), relayChannel)

	go r.run()
}

func (r *Relay) run() {
	defer close(r.stopped)

	for message := range r.pubsub.Channel() {
		r.apply([]byte(message.Payload))
	}
}

// stop closes the subscription and waits for the apply loop to exit.
func (r *Relay) stop() {
	_ = r.pubsub.Close()
	<-r.stopped
}

// apply replays a frame published by another node onto the local hub.
func (r *Relay) apply(payload []byte) {
	var frame relayFrame
	if err := json.Unmarshal(payload, &frame); err != nil {
		logger.Warnf("Dropping malformed push relay frame: %v", err)

		return
	}

	if frame.Origin == r.nodeID {
		return
	}

	switch frame.Kind {
	case frameMessage:
		r.hub.deliver(frame.Envelope, frame.Targets)
	case frameKickSessions:
		r.hub.closeSessions(frame.SessionIDs)
	default:
		logger.Warnf("Dropping push relay frame of unknown kind %q", frame.Kind)
	}
}

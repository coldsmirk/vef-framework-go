package push

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/redis/go-redis/v9"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/id"
	"github.com/coldsmirk/vef-framework-go/push"
)

// relayChannelPrefix roots the Redis Pub/Sub channel carrying push frames
// between nodes. Pub/Sub (not Streams) is deliberate: a frame must reach
// every node, needs no persistence or consumer groups, and a frame missed
// while a node is disconnected is worthless later — exactly the ephemeral
// fan-out contract.
const relayChannelPrefix = "vef:push:relay"

// relayChannelFor derives the relay channel from the configured Redis
// database and the application name. Pub/Sub channels are server-global —
// unlike keys, they are NOT isolated by the Redis database number — so the
// channel carries both dimensions explicitly: the database number mirrors the
// keyspace isolation operators already rely on to separate environments, and
// the application name separates co-tenant applications within one database.
func relayChannelFor(database uint8, appName string) string {
	channel := relayChannelPrefix + ":" + strconv.Itoa(int(database))
	if appName != "" {
		channel += ":" + appName
	}

	return channel
}

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
// kickQueueCapacity bounds the pending cross-node kick publishes. Kicks are
// low-frequency, so a full queue means Redis has been failing for a while;
// a dropped frame degrades to the remote nodes' periodic sweep.
const kickQueueCapacity = 256

type Relay struct {
	hub     *Hub
	client  *redis.Client
	nodeID  string
	channel string

	ctx    context.Context
	cancel context.CancelFunc
	kicks  chan []string

	pubsub      *redis.PubSub
	stopped     chan struct{}
	kickStopped chan struct{}
}

// NewRelay builds the cross-node relay; nil when the endpoint is disabled or
// no Redis client is available (single-node deployments push through the hub
// directly).
func NewRelay(hub *Hub, cfg *config.PushConfig, appCfg *config.AppConfig, redisCfg *config.RedisConfig, client *redis.Client) *Relay {
	if !cfg.Enabled || client == nil {
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Relay{
		hub:         hub,
		client:      client,
		nodeID:      id.Generate(),
		channel:     relayChannelFor(redisCfg.Database, appCfg.Name),
		ctx:         ctx,
		cancel:      cancel,
		kicks:       make(chan []string, kickQueueCapacity),
		stopped:     make(chan struct{}),
		kickStopped: make(chan struct{}),
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

// KickSessions closes the sessions' connections on every node. The publish is
// handed to the relay's bounded worker: kicks arrive through revocation call
// paths (logout, concurrent-login eviction) whose listener contract forbids
// blocking on Redis health, and a goroutine per kick would grow without bound
// under a Redis outage. A full queue drops the frame — the remote nodes'
// periodic sweep remains the net.
func (r *Relay) KickSessions(sessionIDs []string) {
	r.hub.closeSessions(sessionIDs)

	select {
	case r.kicks <- sessionIDs:
	default:
		logger.Warnf("Push relay kick queue is full; remote nodes fall back to the session sweep")
	}
}

func (r *Relay) publish(ctx context.Context, frame relayFrame) {
	payload, err := json.Marshal(frame)
	if err != nil {
		logger.Errorf("Failed to marshal push relay frame: %v", err)

		return
	}

	if err := r.client.Publish(ctx, r.channel, payload).Err(); err != nil {
		logger.Warnf("Push relay publish failed; delivery stays node-local: %v", err)
	}
}

// start opens the subscription and runs the apply loop. go-redis re-subscribes
// automatically after connection loss; frames missed meanwhile are lost
// (best-effort).
func (r *Relay) start() {
	r.pubsub = r.client.Subscribe(context.Background(), r.channel)

	go r.run()
	go r.kickPump()
}

// kickPump is the single worker draining queued kicks into publishes; its
// context is canceled at stop, so an in-flight publish never delays shutdown.
func (r *Relay) kickPump() {
	defer close(r.kickStopped)

	for {
		select {
		case sessionIDs := <-r.kicks:
			r.publish(r.ctx, relayFrame{Kind: frameKickSessions, Origin: r.nodeID, SessionIDs: sessionIDs})
		case <-r.ctx.Done():
			return
		}
	}
}

func (r *Relay) run() {
	defer close(r.stopped)

	for message := range r.pubsub.Channel() {
		r.apply([]byte(message.Payload))
	}
}

// stop cancels the kick worker, closes the subscription, and waits for both
// loops to exit; kicks still queued are dropped (best-effort — remote nodes
// still sweep).
func (r *Relay) stop() {
	r.cancel()
	<-r.kickStopped

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

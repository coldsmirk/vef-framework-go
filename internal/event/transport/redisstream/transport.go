// Package redisstream implements the cross-process Transport backed
// by Redis Streams (XADD / XREADGROUP / XACK / XCLAIM). Each event
// type maps to a single stream key composed of the configured prefix
// + event type. Consumer groups deliver each message at-least-once
// to one consumer within the group; a reaper claims orphaned pending
// messages after a configurable idle threshold so a crashed consumer
// does not block the partition.
package redisstream

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/coldsmirk/vef-framework-go/event/transport"
	pubredisstream "github.com/coldsmirk/vef-framework-go/event/transport/redisstream"
	"github.com/coldsmirk/vef-framework-go/id"
	"github.com/coldsmirk/vef-framework-go/internal/logx"
	publogx "github.com/coldsmirk/vef-framework-go/logx"
)

// maxFrameBytes caps inbound Redis Stream frames so a hostile or
// misconfigured publisher cannot OOM the consumer process. Frames
// exceeding the limit are XACKed and discarded with a log line.
const maxFrameBytes = 1 << 20 // 1 MiB

// eventTypePattern is the alphabet allowed in event types when used
// to compose Redis Stream keys and DLQ topics. Validation keeps stream
// keys deterministic and safe from accidental wildcard injection.
var eventTypePattern = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

// errFrameTooLarge wraps the per-message size check failure so callers
// can detect it with errors.Is.
var errFrameTooLarge = errors.New("redisstream: frame body exceeds size limit")

// errTransportNotStarted indicates Subscribe was called before Start
// (or after Stop).
var errTransportNotStarted = errors.New("redisstream: transport not started")

// errInvalidEventType is returned when an event type contains
// characters outside the eventTypePattern allowlist.
var errInvalidEventType = errors.New("redisstream: invalid event type")

// Transport implements transport.Transport over Redis Streams.
type Transport struct {
	client *goredis.Client
	cfg    pubredisstream.Config
	logger publogx.Logger

	mu      sync.Mutex
	subs    []*subscription
	started atomic.Bool
	stopCh  chan struct{}
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

type subscription struct {
	stream   string
	group    string
	consumer string
	fn       transport.ConsumeFunc
	cfg      transport.SubscribeConfig
	stopCh   chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup
}

// New constructs a Transport. A nil logger is replaced with logx.Discard.
func New(client *goredis.Client, cfg pubredisstream.Config, log publogx.Logger) *Transport {
	if log == nil {
		log = logx.Discard()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Transport{
		client: client,
		cfg:    cfg,
		logger: log,
		stopCh: make(chan struct{}),
		ctx:    ctx,
		cancel: cancel,
	}
}

// Name implements transport.Transport.
func (*Transport) Name() string { return pubredisstream.Name }

// Capabilities advertises cross-process durable at-least-once delivery
// with consumer groups.
func (*Transport) Capabilities() transport.Capabilities {
	return transport.Capabilities{
		Durable:        true,
		Transactional:  false,
		Ordered:        true,
		AtLeastOnce:    true,
		SupportsGroups: true,
	}
}

// Start verifies the redis connection and spins up the reaper goroutine.
func (t *Transport) Start(ctx context.Context) error {
	if !t.started.CompareAndSwap(false, true) {
		return nil
	}

	if err := t.client.Ping(ctx).Err(); err != nil {
		t.started.Store(false)

		return fmt.Errorf("redisstream: ping: %w", err)
	}

	t.wg.Add(1)
	go t.reaperLoop()

	return nil
}

// Stop drains worker goroutines and closes the stop channel.
func (t *Transport) Stop(ctx context.Context) error {
	if !t.started.CompareAndSwap(true, false) {
		return nil
	}

	close(t.stopCh)
	t.cancel()

	t.mu.Lock()
	subs := t.subs
	t.subs = nil
	t.mu.Unlock()

	for _, s := range subs {
		s.stop()
	}

	done := make(chan struct{})
	go func() {
		t.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Publish writes frames to their respective streams via XADD MAXLEN ~.
func (t *Transport) Publish(ctx context.Context, frames []transport.Frame) error {
	for _, frame := range frames {
		if err := validateEventType(frame.Type); err != nil {
			return err
		}

		body, err := json.Marshal(frame)
		if err != nil {
			return fmt.Errorf("redisstream: encode frame %s: %w", frame.ID, err)
		}

		if len(body) > maxFrameBytes {
			return fmt.Errorf("%w: frame %s is %d bytes (max %d)",
				errFrameTooLarge, frame.ID, len(body), maxFrameBytes)
		}

		args := &goredis.XAddArgs{
			Stream: t.cfg.StreamKey(frame.Type),
			Values: map[string]any{"frame": body},
		}
		if t.cfg.MaxLenApprox > 0 {
			args.MaxLen = t.cfg.MaxLenApprox
			args.Approx = true
		}

		if _, err := t.client.XAdd(ctx, args).Result(); err != nil {
			return fmt.Errorf("redisstream: xadd %s: %w", frame.Type, err)
		}
	}

	return nil
}

// Subscribe attaches a consumer to the supplied event type within a
// consumer group. A blank group falls back to a per-consumer group so
// the subscription receives every message in fan-out fashion.
func (t *Transport) Subscribe(eventType, group string, fn transport.ConsumeFunc, cfg transport.SubscribeConfig) (transport.Unsubscribe, error) {
	if !t.started.Load() {
		return nil, errTransportNotStarted
	}

	if err := validateEventType(eventType); err != nil {
		return nil, err
	}

	if group == "" {
		group = "vef:default:" + id.GenerateUUID()
	}

	stream := t.cfg.StreamKey(eventType)
	// XGroupCreateMkStream is idempotent via BUSYGROUP error. We use
	// the configured StartID (default "0") so a group created after
	// some messages have been published still observes them.
	if err := t.client.XGroupCreateMkStream(t.ctx, stream, group, t.cfg.EffectiveStartID()).Err(); err != nil &&
		!isBusyGroup(err) {
		return nil, fmt.Errorf("redisstream: create group %s on %s: %w", group, stream, err)
	}

	consumer := t.cfg.ConsumerID
	if consumer == "" {
		consumer = "vef-" + id.GenerateUUID()
	}

	sub := &subscription{
		stream:   stream,
		group:    group,
		consumer: consumer,
		fn:       fn,
		cfg:      cfg,
		stopCh:   make(chan struct{}),
	}

	concurrency := cfg.Concurrency
	if concurrency <= 0 {
		concurrency = 1
	}

	for range concurrency {
		sub.wg.Add(1)

		t.wg.Add(1)
		go t.consumerLoop(sub)
	}

	t.mu.Lock()
	t.subs = append(t.subs, sub)
	t.mu.Unlock()

	return func() {
		sub.stop()
		t.mu.Lock()
		defer t.mu.Unlock()

		for i, existing := range t.subs {
			if existing == sub {
				t.subs = append(t.subs[:i], t.subs[i+1:]...)

				return
			}
		}
	}, nil
}

func (t *Transport) consumerLoop(sub *subscription) {
	defer sub.wg.Done()
	defer t.wg.Done()

	for {
		select {
		case <-sub.stopCh:
			return
		case <-t.stopCh:
			return
		default:
		}

		res, err := t.client.XReadGroup(t.ctx, &goredis.XReadGroupArgs{
			Group:    sub.group,
			Consumer: sub.consumer,
			Streams:  []string{sub.stream, ">"},
			Count:    1,
			Block:    t.cfg.EffectiveBlockTimeout(),
		}).Result()
		if err != nil {
			if errors.Is(err, goredis.Nil) {
				continue
			}

			// Transport ctx canceled (Stop in progress) — exit instead of
			// busy-spinning until stopCh is observed on the next iteration.
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return
			}

			t.logger.Warnf("redisstream: XREADGROUP on %s: %v", sub.stream, err)
			t.sleepOrStop(sub, time.Second)

			continue
		}

		for _, stream := range res {
			for _, msg := range stream.Messages {
				t.deliver(t.ctx, sub, msg)
			}
		}
	}
}

func (t *Transport) deliver(ctx context.Context, sub *subscription, msg goredis.XMessage) {
	rawFrame, ok := msg.Values["frame"].(string)
	if !ok {
		t.logger.Errorf("redisstream: frame missing or non-string on %s id=%s", sub.stream, msg.ID)
		_, _ = t.client.XAck(ctx, sub.stream, sub.group, msg.ID).Result()

		return
	}

	if len(rawFrame) > maxFrameBytes {
		t.logger.Errorf("redisstream: frame %s on %s exceeds %d bytes, dropping", msg.ID, sub.stream, maxFrameBytes)
		_, _ = t.client.XAck(ctx, sub.stream, sub.group, msg.ID).Result()

		return
	}

	var frame transport.Frame
	if err := json.Unmarshal([]byte(rawFrame), &frame); err != nil {
		t.logger.Errorf("redisstream: decode frame %s on %s: %v", msg.ID, sub.stream, err)
		_, _ = t.client.XAck(ctx, sub.stream, sub.group, msg.ID).Result()

		return
	}

	delivery := &streamDelivery{frame: frame, attempt: 1, msgID: msg.ID}
	if err := sub.fn(ctx, delivery); err != nil {
		t.logger.Warnf("redisstream: handler returned error on %s id=%s: %v — leaving pending for retry", sub.stream, msg.ID, err)

		return
	}

	if _, err := t.client.XAck(ctx, sub.stream, sub.group, msg.ID).Result(); err != nil {
		t.logger.Warnf("redisstream: XACK %s id=%s: %v", sub.stream, msg.ID, err)
	}
}

func (t *Transport) sleepOrStop(sub *subscription, d time.Duration) {
	select {
	case <-time.After(d):
	case <-sub.stopCh:
	case <-t.stopCh:
	}
}

func (s *subscription) stop() {
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
	s.wg.Wait()
}

// isBusyGroup detects the BUSYGROUP error returned when a consumer
// group already exists, so XGroupCreateMkStream remains idempotent.
func isBusyGroup(err error) bool {
	if err == nil {
		return false
	}

	return strings.Contains(err.Error(), "BUSYGROUP")
}

// validateEventType ensures the event type only contains characters
// safe for stream keys and DLQ topics. Returning a typed error lets
// callers (bus, contract suite, applications) check via errors.Is.
func validateEventType(t string) error {
	if t == "" || !eventTypePattern.MatchString(t) {
		return fmt.Errorf("%w: %q (allowed: %s)",
			errInvalidEventType, t, eventTypePattern.String())
	}

	return nil
}

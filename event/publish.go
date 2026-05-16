package event

import (
	"maps"
	"time"

	"github.com/coldsmirk/vef-framework-go/orm"
)

// PublishOption mutates publish-time configuration. Options compose
// left-to-right; later options override earlier ones for the same field.
type PublishOption func(*PublishConfig)

// PublishConfig carries the resolved per-publish settings. It is
// exported for transport implementations and middleware; business code
// should compose options instead of building it directly.
type PublishConfig struct {
	// Tx, when non-nil, forces routing through a TxTransport using the
	// supplied transaction handle. Returns ErrTxRequired at publish
	// time when the resolved route contains no TxTransport.
	Tx orm.DB
	// Async, when true, enqueues the publish on the bus's fan-in queue
	// and returns immediately. Errors flow to the bus ErrorSink, not
	// the caller. Mutually exclusive with Tx (a transactional publish
	// must complete before the transaction commits).
	Async bool
	// Source overrides Envelope.Source.
	Source string
	// OccurredAt overrides Envelope.OccurredAt.
	OccurredAt time.Time
	// CorrelationID stamps the envelope with a caller-controlled key.
	CorrelationID string
	// Headers are merged into the envelope; later keys override earlier.
	Headers map[string]string
}

// ApplyPublishOptions returns a fully resolved PublishConfig from the
// supplied options. Useful for middleware that needs to inspect the
// effective configuration without re-implementing the fold.
func ApplyPublishOptions(opts []PublishOption) PublishConfig {
	var cfg PublishConfig
	for _, opt := range opts {
		opt(&cfg)
	}

	return cfg
}

// WithTx routes the publish through a TxTransport using the supplied
// transaction handle. Events become visible iff the caller commits.
func WithTx(tx orm.DB) PublishOption {
	return func(c *PublishConfig) { c.Tx = tx }
}

// WithAsync enqueues the publish on the bus's fan-in queue and returns
// immediately. Designed for hot-path events (audit, login) where
// back-pressure must not affect request latency.
func WithAsync() PublishOption {
	return func(c *PublishConfig) { c.Async = true }
}

// WithSource overrides Envelope.Source. Defaults to the application
// name configured under vef.app.
func WithSource(src string) PublishOption {
	return func(c *PublishConfig) { c.Source = src }
}

// WithOccurredAt overrides Envelope.OccurredAt. Defaults to time.Now.
func WithOccurredAt(t time.Time) PublishOption {
	return func(c *PublishConfig) { c.OccurredAt = t }
}

// WithCorrelationID stamps the envelope with a caller-controlled
// correlation key, opaque to the framework.
func WithCorrelationID(id string) PublishOption {
	return func(c *PublishConfig) { c.CorrelationID = id }
}

// WithHeaders merges arbitrary headers into the envelope; later keys
// override earlier ones.
func WithHeaders(h map[string]string) PublishOption {
	return func(c *PublishConfig) {
		if c.Headers == nil {
			c.Headers = make(map[string]string, len(h))
		}

		maps.Copy(c.Headers, h)
	}
}

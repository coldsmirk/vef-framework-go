package config

import "time"

// PushConfig defines the server push (WebSocket) endpoint settings under
// vef.push. The endpoint is opt-in; the push.Notifier stays available while
// disabled, with deliveries silently dropped (no connections exist).
type PushConfig struct {
	// Enabled turns the WebSocket endpoint on. Default: false.
	Enabled bool `config:"enabled"`
	// Path is the endpoint path. Default: /ws.
	Path string `config:"path"`
	// AllowedOrigins whitelists browser origins for the handshake; empty allows
	// every origin. The handshake is token-authenticated, so a cross-site page
	// cannot connect without a token — the whitelist is defense in depth.
	AllowedOrigins []string `config:"allowed_origins"`
	// PingInterval is the server heartbeat period; a connection that misses two
	// consecutive pongs is dropped. Default: 30s.
	PingInterval time.Duration `config:"ping_interval"`
	// WriteTimeout bounds a single outbound frame write. Default: 10s.
	WriteTimeout time.Duration `config:"write_timeout"`
	// SendBuffer is the per-connection outbound queue length; a client too slow
	// to drain it is disconnected. Default: 32.
	SendBuffer int `config:"send_buffer"`
	// MaxConnectionsPerUser caps concurrent sockets per user on one node;
	// 0 is unlimited.
	MaxConnectionsPerUser int `config:"max_connections_per_user"`
	// SessionRecheckInterval is how often opaque-token connections are
	// revalidated against the session store, closing connections whose session
	// was revoked or expired. Default: 60s.
	SessionRecheckInterval time.Duration `config:"session_recheck_interval"`
}

// EffectivePath returns Path or its default (/ws).
func (c *PushConfig) EffectivePath() string {
	if c.Path == "" {
		return "/ws"
	}

	return c.Path
}

// EffectivePingInterval returns PingInterval or its default (30s).
func (c *PushConfig) EffectivePingInterval() time.Duration {
	return coalescePositive(c.PingInterval, 30*time.Second)
}

// EffectiveWriteTimeout returns WriteTimeout or its default (10s).
func (c *PushConfig) EffectiveWriteTimeout() time.Duration {
	return coalescePositive(c.WriteTimeout, 10*time.Second)
}

// EffectiveSendBuffer returns SendBuffer or its default (32).
func (c *PushConfig) EffectiveSendBuffer() int {
	return coalescePositive(c.SendBuffer, 32)
}

// EffectiveSessionRecheckInterval returns SessionRecheckInterval or its
// default (60s).
func (c *PushConfig) EffectiveSessionRecheckInterval() time.Duration {
	return coalescePositive(c.SessionRecheckInterval, 60*time.Second)
}

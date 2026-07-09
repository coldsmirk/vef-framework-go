package security

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"time"
)

// opaqueTokenBytes is the entropy of an opaque session token before encoding.
const opaqueTokenBytes = 32

// SessionExceedPolicy selects what happens when a login would exceed the
// per-account concurrent-session limit.
type SessionExceedPolicy string

const (
	// SessionExceedReject denies the new login once the limit is reached.
	SessionExceedReject SessionExceedPolicy = "reject"
	// SessionExceedEvictOldest revokes the oldest session(s) to admit the new
	// login (the "kick the earliest device offline" behavior).
	SessionExceedEvictOldest SessionExceedPolicy = "evict_oldest"
)

// SessionMeta carries the client context captured when a token is issued. A
// stateless generator (JWT) ignores it; the opaque generator records it on the
// session so active devices can be listed and audited.
type SessionMeta struct {
	ClientIP  string
	UserAgent string
}

// Session is a server-side login session backing an opaque token. It never
// carries the token or its hash; the store keys sessions by the token hash
// internally and exposes only the public ID for administration.
type Session struct {
	// ID is a random public identifier, safe to surface in "active devices" UIs.
	ID string
	// UserID is the owning principal's id.
	UserID string
	// Principal is the snapshot returned to callers on authentication.
	Principal *Principal
	// ClientIP and UserAgent describe where the session was opened.
	ClientIP  string
	UserAgent string
	// CreatedAt, LastSeenAt and ExpiresAt track the session's lifecycle.
	CreatedAt  time.Time
	LastSeenAt time.Time
	ExpiresAt  time.Time
}

// SessionPolicy is the resolved session behavior a store and the opaque
// generator/authenticator enforce, built from configuration.
type SessionPolicy struct {
	// MaxConcurrent bounds simultaneous sessions per account; 0 is unlimited.
	MaxConcurrent int
	// OnExceed selects reject vs. evict-oldest when the limit is hit.
	OnExceed SessionExceedPolicy
	// IdleTTL is how long a session survives without activity and the sliding
	// renewal window applied on each authenticated request.
	IdleTTL time.Duration
	// MaxLifetime caps a session's total age regardless of renewal; 0 is uncapped.
	MaxLifetime time.Duration
	// Sliding enables idle-timeout renewal on each authenticated request.
	Sliding bool
}

// SessionStore persists opaque-token sessions. Implementations must be safe for
// concurrent use. Lookup and Renew address a session by the presented token's
// hash (the per-request path); Revoke and administration address it by public
// ID. A single-node deployment can use MemorySessionStore; a multi-node one must
// use RedisSessionStore so sessions are shared across nodes.
type SessionStore interface {
	// Create stores a new session for tokenHash with the given time-to-live.
	Create(ctx context.Context, tokenHash string, session Session, ttl time.Duration) error
	// Lookup returns the session for a presented token's hash, or nil when it is
	// absent or expired.
	Lookup(ctx context.Context, tokenHash string) (*Session, error)
	// Renew extends the session for tokenHash to expiresAt with a fresh ttl.
	Renew(ctx context.Context, tokenHash string, expiresAt time.Time, ttl time.Duration) error
	// Revoke removes the session identified by its public ID.
	Revoke(ctx context.Context, id string) error
	// ListByUser returns a user's live sessions, newest activity first.
	ListByUser(ctx context.Context, userID string) ([]Session, error)
	// RevokeUser removes every session belonging to userID (force-logout).
	RevokeUser(ctx context.Context, userID string) error
}

// SessionInspector is an optional capability a SessionStore may implement to
// support cross-user administration and monitoring — for example an "all online
// sessions" dashboard. It is kept out of SessionStore because the
// authentication mechanism never needs it, so custom stores are not forced to
// implement it; callers type-assert for it (mirroring event.StreamInspector).
//
// ListAll enumerates every live session and is O(all sessions); it is intended
// for infrequent administrative views, not a request-path call. Deployments
// large enough to need pagination should build it on their own store.
type SessionInspector interface {
	// ListAll returns every live session across all users, newest activity first.
	ListAll(ctx context.Context) ([]Session, error)
}

// GenerateOpaqueToken returns a new high-entropy, URL-safe opaque token.
func GenerateOpaqueToken() (string, error) {
	buf := make([]byte, opaqueTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// HashOpaqueToken returns the lookup key for a token. Sessions are stored under
// this hash, never under the raw token, so a store leak cannot yield live
// credentials (the raw token is unrecoverable from its SHA-256 hash).
func HashOpaqueToken(token string) string {
	sum := sha256.Sum256([]byte(token))

	return hex.EncodeToString(sum[:])
}

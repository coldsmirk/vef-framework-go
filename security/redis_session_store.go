package security

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	redisSessionPrefix  = "vef:security:session:"
	redisSessionByID    = redisSessionPrefix + "id:"
	redisSessionByToken = redisSessionPrefix + "token:"
	redisSessionByUser  = redisSessionPrefix + "user:"
	redisSessionAll     = redisSessionPrefix + "all"
)

// RedisSessionStore implements SessionStore on Redis so sessions are shared
// across nodes. Session and token keys carry a TTL that Redis expires
// automatically; the per-user and global id sets are pruned lazily on read and
// on revoke. Every multi-key mutation runs in a MULTI/EXEC transaction so a
// reader never observes a half-written or half-deleted session.
type RedisSessionStore struct {
	client *redis.Client
}

// NewRedisSessionStore creates a Redis-backed session store.
func NewRedisSessionStore(client *redis.Client) SessionStore {
	return &RedisSessionStore{client: client}
}

type redisSessionRecord struct {
	Session   Session `json:"session"`
	TokenHash string  `json:"tokenHash"`
}

func (*RedisSessionStore) idKey(id string) string           { return redisSessionByID + id }
func (*RedisSessionStore) tokenKey(tokenHash string) string { return redisSessionByToken + tokenHash }
func (*RedisSessionStore) userKey(userID string) string     { return redisSessionByUser + userID }

func (s *RedisSessionStore) Create(ctx context.Context, tokenHash string, session Session, ttl time.Duration) error {
	payload, err := json.Marshal(redisSessionRecord{Session: session, TokenHash: tokenHash})
	if err != nil {
		return err
	}

	// Write the record, token index, and both id sets atomically so a lookup can
	// never resolve a token to a session that is not yet fully indexed.
	pipe := s.client.TxPipeline()
	pipe.Set(ctx, s.idKey(session.ID), payload, ttl)
	pipe.Set(ctx, s.tokenKey(tokenHash), session.ID, ttl)
	pipe.SAdd(ctx, s.userKey(session.UserID), session.ID)
	pipe.SAdd(ctx, redisSessionAll, session.ID)
	_, err = pipe.Exec(ctx)

	return err
}

func (s *RedisSessionStore) Lookup(ctx context.Context, tokenHash string) (*Session, error) {
	id, err := s.client.Get(ctx, s.tokenKey(tokenHash)).Result()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	record, err := s.load(ctx, id)
	if err != nil || record == nil {
		return nil, err
	}

	session := record.Session

	return &session, nil
}

func (s *RedisSessionStore) Renew(ctx context.Context, tokenHash string, expiresAt time.Time, ttl time.Duration) error {
	id, err := s.client.Get(ctx, s.tokenKey(tokenHash)).Result()
	if errors.Is(err, redis.Nil) {
		return nil
	}

	if err != nil {
		return err
	}

	record, err := s.load(ctx, id)
	if err != nil || record == nil {
		return err
	}

	record.Session.ExpiresAt = expiresAt
	record.Session.LastSeenAt = time.Now()

	payload, err := json.Marshal(record)
	if err != nil {
		return err
	}

	// SET ... XX rewrites the record only if it still exists, so a renewal that
	// races a concurrent Revoke can never resurrect a just-deleted session. The
	// token TTL is refreshed in the same transaction.
	pipe := s.client.TxPipeline()
	pipe.SetArgs(ctx, s.idKey(id), payload, redis.SetArgs{Mode: "XX", TTL: ttl})
	pipe.Expire(ctx, s.tokenKey(tokenHash), ttl)

	if _, err := pipe.Exec(ctx); err != nil && !errors.Is(err, redis.Nil) {
		return err
	}

	return nil
}

func (s *RedisSessionStore) Revoke(ctx context.Context, id string) error {
	record, err := s.load(ctx, id)
	if err != nil || record == nil {
		return err
	}

	return s.deleteRecord(ctx, record)
}

func (s *RedisSessionStore) ListByUser(ctx context.Context, userID string) ([]Session, error) {
	sessions, stale, err := s.collect(ctx, s.userKey(userID))
	if err != nil {
		return nil, err
	}

	// Drop expired ids from both the user set and the global set.
	if len(stale) > 0 {
		pipe := s.client.Pipeline()
		pipe.SRem(ctx, s.userKey(userID), stale)
		pipe.SRem(ctx, redisSessionAll, stale)
		_, _ = pipe.Exec(ctx)
	}

	return sessions, nil
}

func (s *RedisSessionStore) ListAll(ctx context.Context) ([]Session, error) {
	sessions, stale, err := s.collect(ctx, redisSessionAll)
	if err != nil {
		return nil, err
	}

	if len(stale) > 0 {
		s.client.SRem(ctx, redisSessionAll, stale)
	}

	return sessions, nil
}

func (s *RedisSessionStore) RevokeUser(ctx context.Context, userID string) error {
	ids, err := s.client.SMembers(ctx, s.userKey(userID)).Result()
	if err != nil {
		return err
	}

	keys := make([]string, 0, len(ids)*2)
	for _, id := range ids {
		record, err := s.load(ctx, id)
		if err != nil {
			return err
		}

		keys = append(keys, s.idKey(id))
		if record != nil {
			keys = append(keys, s.tokenKey(record.TokenHash))
		}
	}

	// Delete every record and token key, drop all ids from the global set, and
	// remove the user set itself in one transaction.
	pipe := s.client.TxPipeline()
	if len(keys) > 0 {
		pipe.Del(ctx, keys...)
	}

	if len(ids) > 0 {
		pipe.SRem(ctx, redisSessionAll, ids)
	}

	pipe.Del(ctx, s.userKey(userID))
	_, err = pipe.Exec(ctx)

	return err
}

// collect loads every session id in setKey, returning the live sessions (newest
// activity first) and the ids whose records have expired.
func (s *RedisSessionStore) collect(ctx context.Context, setKey string) (sessions []Session, stale []string, err error) {
	ids, err := s.client.SMembers(ctx, setKey).Result()
	if err != nil {
		return nil, nil, err
	}

	for _, id := range ids {
		record, err := s.load(ctx, id)
		if err != nil {
			return nil, nil, err
		}

		if record == nil {
			stale = append(stale, id)

			continue
		}

		sessions = append(sessions, record.Session)
	}

	slices.SortFunc(sessions, func(a, b Session) int {
		return b.LastSeenAt.Compare(a.LastSeenAt)
	})

	return sessions, stale, nil
}

// load reads and decodes a session record by id, returning nil when the key has
// expired or was revoked.
func (s *RedisSessionStore) load(ctx context.Context, id string) (*redisSessionRecord, error) {
	payload, err := s.client.Get(ctx, s.idKey(id)).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	var record redisSessionRecord
	if err := json.Unmarshal(payload, &record); err != nil {
		return nil, err
	}

	return &record, nil
}

// deleteRecord removes a record from every key and set in one transaction.
func (s *RedisSessionStore) deleteRecord(ctx context.Context, record *redisSessionRecord) error {
	pipe := s.client.TxPipeline()
	pipe.Del(ctx, s.idKey(record.Session.ID), s.tokenKey(record.TokenHash))
	pipe.SRem(ctx, s.userKey(record.Session.UserID), record.Session.ID)
	pipe.SRem(ctx, redisSessionAll, record.Session.ID)
	_, err := pipe.Exec(ctx)

	return err
}

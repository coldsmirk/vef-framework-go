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
)

// RedisSessionStore implements SessionStore on Redis so sessions are shared
// across nodes. Session and token keys carry a TTL that Redis expires
// automatically; the per-user id set is pruned lazily on read and on revoke.
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

	if err := s.client.Set(ctx, s.idKey(session.ID), payload, ttl).Err(); err != nil {
		return err
	}

	if err := s.client.Set(ctx, s.tokenKey(tokenHash), session.ID, ttl).Err(); err != nil {
		return err
	}

	return s.client.SAdd(ctx, s.userKey(session.UserID), session.ID).Err()
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

	if err := s.client.Set(ctx, s.idKey(id), payload, ttl).Err(); err != nil {
		return err
	}

	return s.client.Expire(ctx, s.tokenKey(tokenHash), ttl).Err()
}

func (s *RedisSessionStore) Revoke(ctx context.Context, id string) error {
	record, err := s.load(ctx, id)
	if err != nil || record == nil {
		return err
	}

	return s.deleteRecord(ctx, record)
}

func (s *RedisSessionStore) ListByUser(ctx context.Context, userID string) ([]Session, error) {
	ids, err := s.client.SMembers(ctx, s.userKey(userID)).Result()
	if err != nil {
		return nil, err
	}

	var (
		sessions []Session
		stale    []string
	)

	for _, id := range ids {
		record, err := s.load(ctx, id)
		if err != nil {
			return nil, err
		}

		if record == nil {
			stale = append(stale, id)

			continue
		}

		sessions = append(sessions, record.Session)
	}

	if len(stale) > 0 {
		s.client.SRem(ctx, s.userKey(userID), stale)
	}

	slices.SortFunc(sessions, func(a, b Session) int {
		return b.LastSeenAt.Compare(a.LastSeenAt)
	})

	return sessions, nil
}

func (s *RedisSessionStore) RevokeUser(ctx context.Context, userID string) error {
	ids, err := s.client.SMembers(ctx, s.userKey(userID)).Result()
	if err != nil {
		return err
	}

	for _, id := range ids {
		record, err := s.load(ctx, id)
		if err != nil {
			return err
		}

		if record != nil {
			if err := s.client.Del(ctx, s.idKey(id), s.tokenKey(record.TokenHash)).Err(); err != nil {
				return err
			}
		}
	}

	return s.client.Del(ctx, s.userKey(userID)).Err()
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

// deleteRecord removes a record from all keys.
func (s *RedisSessionStore) deleteRecord(ctx context.Context, record *redisSessionRecord) error {
	if err := s.client.Del(ctx, s.idKey(record.Session.ID), s.tokenKey(record.TokenHash)).Err(); err != nil {
		return err
	}

	return s.client.SRem(ctx, s.userKey(record.Session.UserID), record.Session.ID).Err()
}

package sequence

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/redis/go-redis/v9"

	"github.com/coldsmirk/vef-framework-go/timex"
)

const redisSequencePrefix = "vef:sequence:"

// RedisStore implements Store using Redis for distributed deployments.
type RedisStore struct {
	client *redis.Client
}

// NewRedisStore creates a new Redis-backed sequence store.
func NewRedisStore(client *redis.Client) Store {
	return &RedisStore{client: client}
}

func (s *RedisStore) Reserve(ctx context.Context, key string, count int, now timex.DateTime) (*Rule, int, error) {
	rKey := redisSequencePrefix + key

	for {
		var (
			reservedRule *Rule
			newValue     int
		)

		err := s.client.Watch(ctx, func(tx *redis.Tx) error {
			fields, err := tx.HGetAll(ctx, rKey).Result()
			if err != nil {
				return err
			}

			if len(fields) == 0 {
				return ErrRuleNotFound
			}

			rule, err := parseRedisRule(fields)
			if err != nil {
				return fmt.Errorf("failed to parse sequence rule %q from redis: %w", key, err)
			}

			if !rule.IsActive {
				return ErrRuleNotFound
			}

			resetNeeded, err := evaluateReserve(rule, count, now)
			if err != nil {
				return err
			}

			if resetNeeded {
				resetAt := now
				rule.CurrentValue = rule.StartValue
				rule.LastResetAt = &resetAt
			}

			newValue = rule.CurrentValue + rule.SeqStep*count

			if _, err := tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
				updates := []any{"current_value", newValue}
				if resetNeeded {
					updates = append(updates, "last_reset_at", now.String())
				}

				pipe.HSet(ctx, rKey, updates...)

				return nil
			}); err != nil {
				return err
			}

			rule.CurrentValue = newValue
			reservedRule = rule

			return nil
		}, rKey)
		if errors.Is(err, redis.TxFailedErr) {
			continue
		}

		if err != nil {
			return nil, 0, err
		}

		return reservedRule, newValue, nil
	}
}

// RegisterRule stores a rule in Redis as a hash.
// This is a helper for setting up rules in Redis.
func (s *RedisStore) RegisterRule(ctx context.Context, rule *Rule) error {
	rKey := redisSequencePrefix + rule.Key

	fields := map[string]any{
		"key":               rule.Key,
		"name":              rule.Name,
		"prefix":            rule.Prefix,
		"suffix":            rule.Suffix,
		"date_format":       rule.DateFormat,
		"seq_length":        rule.SeqLength,
		"seq_step":          rule.SeqStep,
		"start_value":       rule.StartValue,
		"max_value":         rule.MaxValue,
		"overflow_strategy": string(rule.OverflowStrategy),
		"reset_cycle":       string(rule.ResetCycle),
		"current_value":     rule.CurrentValue,
		"is_active":         strconv.FormatBool(rule.IsActive),
	}

	if rule.LastResetAt != nil {
		fields["last_reset_at"] = rule.LastResetAt.String()
	}

	return s.client.HSet(ctx, rKey, fields).Err()
}

func parseRedisRule(fields map[string]string) (*Rule, error) {
	isActive, err := parseRedisBoolField(fields, "is_active")
	if err != nil {
		return nil, err
	}

	seqLength, err := parseRedisIntField(fields, "seq_length")
	if err != nil {
		return nil, err
	}

	seqStep, err := parseRedisIntField(fields, "seq_step")
	if err != nil {
		return nil, err
	}

	startValue, err := parseRedisIntField(fields, "start_value")
	if err != nil {
		return nil, err
	}

	maxValue, err := parseRedisIntField(fields, "max_value")
	if err != nil {
		return nil, err
	}

	currentValue, err := parseRedisIntField(fields, "current_value")
	if err != nil {
		return nil, err
	}

	rule := &Rule{
		Key:              fields["key"],
		Name:             fields["name"],
		Prefix:           fields["prefix"],
		Suffix:           fields["suffix"],
		DateFormat:       fields["date_format"],
		SeqLength:        seqLength,
		SeqStep:          seqStep,
		StartValue:       startValue,
		MaxValue:         maxValue,
		OverflowStrategy: OverflowStrategy(fields["overflow_strategy"]),
		ResetCycle:       ResetCycle(fields["reset_cycle"]),
		CurrentValue:     currentValue,
		IsActive:         isActive,
	}

	if lastReset := fields["last_reset_at"]; lastReset != "" {
		dt, err := timex.Parse(lastReset)
		if err != nil {
			return nil, fmt.Errorf("invalid field %q=%q: %w", "last_reset_at", lastReset, err)
		}

		rule.LastResetAt = &dt
	}

	return rule, nil
}

func parseRedisIntField(fields map[string]string, key string) (int, error) {
	value, ok := fields[key]
	if !ok {
		return 0, fmt.Errorf("%w: %q", errMissingField, key)
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("invalid field %q=%q: %w", key, value, err)
	}

	return parsed, nil
}

func parseRedisBoolField(fields map[string]string, key string) (bool, error) {
	value, ok := fields[key]
	if !ok {
		return false, fmt.Errorf("%w: %q", errMissingField, key)
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("invalid field %q=%q: %w", key, value, err)
	}

	return parsed, nil
}

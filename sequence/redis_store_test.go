package sequence

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/coldsmirk/go-collections"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/timex"
)

func baseRedisRuleFields() map[string]string {
	return map[string]string{
		"key":               "parse-rule",
		"name":              "Parse Rule",
		"prefix":            "ORD-",
		"suffix":            "-CN",
		"date_format":       "yyyyMMdd",
		"seq_length":        "4",
		"seq_step":          "1",
		"start_value":       "0",
		"max_value":         "0",
		"overflow_strategy": string(OverflowError),
		"reset_cycle":       string(ResetNone),
		"current_value":     "10",
		"is_active":         "true",
	}
}

func TestParseRedisIntField(t *testing.T) {
	t.Run("ValidValue", func(t *testing.T) {
		got, err := parseRedisIntField(map[string]string{"n": "42"}, "n")
		require.NoError(t, err, "Valid integer field should parse successfully")
		assert.Equal(t, 42, got, "Parsed integer should match stored value")
	})

	t.Run("MissingField", func(t *testing.T) {
		_, err := parseRedisIntField(map[string]string{}, "missing")
		require.Error(t, err, "Missing integer field should return an error")
		assert.ErrorContains(t, err, "missing field", "Error should indicate missing field")
	})

	t.Run("InvalidValue", func(t *testing.T) {
		_, err := parseRedisIntField(map[string]string{"n": "bad"}, "n")
		require.Error(t, err, "Invalid integer field should return an error")
		assert.ErrorContains(t, err, "invalid field", "Error should indicate invalid field value")
	})
}

func TestParseRedisBoolField(t *testing.T) {
	t.Run("ValidValue", func(t *testing.T) {
		got, err := parseRedisBoolField(map[string]string{"flag": "true"}, "flag")
		require.NoError(t, err, "Valid bool field should parse successfully")
		assert.True(t, got, "Parsed bool should match stored value")
	})

	t.Run("MissingField", func(t *testing.T) {
		_, err := parseRedisBoolField(map[string]string{}, "missing")
		require.Error(t, err, "Missing bool field should return an error")
		assert.ErrorContains(t, err, "missing field", "Error should indicate missing field")
	})

	t.Run("InvalidValue", func(t *testing.T) {
		_, err := parseRedisBoolField(map[string]string{"flag": "not-bool"}, "flag")
		require.Error(t, err, "Invalid bool field should return an error")
		assert.ErrorContains(t, err, "invalid field", "Error should indicate invalid field value")
	})
}

func TestParseRedisRule(t *testing.T) {
	t.Run("ValidRule", func(t *testing.T) {
		fields := baseRedisRuleFields()
		rule, err := parseRedisRule(fields)
		require.NoError(t, err, "Valid redis hash fields should parse into a rule")
		assert.Equal(t, "parse-rule", rule.Key, "Rule key should be parsed from hash")
		assert.Equal(t, "Parse Rule", rule.Name, "Rule name should be parsed from hash")
		assert.Equal(t, "ORD-", rule.Prefix, "Rule prefix should be parsed from hash")
		assert.Equal(t, "-CN", rule.Suffix, "Rule suffix should be parsed from hash")
		assert.Equal(t, "yyyyMMdd", rule.DateFormat, "Rule date format should be parsed from hash")
		assert.Equal(t, 4, rule.SeqLength, "Rule sequence length should be parsed from hash")
		assert.Equal(t, 1, rule.SeqStep, "Rule sequence step should be parsed from hash")
		assert.Equal(t, 10, rule.CurrentValue, "Rule current value should be parsed from hash")
		assert.True(t, rule.IsActive, "Rule active state should be parsed from hash")
		assert.Nil(t, rule.LastResetAt, "Rule last reset should be nil when field is missing")
	})

	t.Run("ValidRuleWithLastResetAt", func(t *testing.T) {
		fields := baseRedisRuleFields()
		lastResetAt := timex.Now().String()
		fields["last_reset_at"] = lastResetAt

		rule, err := parseRedisRule(fields)
		require.NoError(t, err, "Valid last_reset_at should be parsed into DateTime")
		require.NotNil(t, rule.LastResetAt, "Rule last reset should be present after parse")
		assert.Equal(t, lastResetAt, rule.LastResetAt.String(), "Parsed last reset should match stored value")
	})

	t.Run("MissingField", func(t *testing.T) {
		fields := baseRedisRuleFields()
		delete(fields, "is_active")

		_, err := parseRedisRule(fields)
		require.Error(t, err, "Missing required field should return parse error")
		assert.ErrorContains(t, err, "missing field", "Error should indicate missing required field")
	})

	t.Run("InvalidIntField", func(t *testing.T) {
		fields := baseRedisRuleFields()
		fields["seq_length"] = "bad-int"

		_, err := parseRedisRule(fields)
		require.Error(t, err, "Invalid numeric field should return parse error")
		assert.ErrorContains(t, err, "invalid field", "Error should indicate invalid numeric field")
	})

	t.Run("InvalidBoolField", func(t *testing.T) {
		fields := baseRedisRuleFields()
		fields["is_active"] = "not-bool"

		_, err := parseRedisRule(fields)
		require.Error(t, err, "Invalid bool field should return parse error")
		assert.ErrorContains(t, err, "invalid field", "Error should indicate invalid bool field")
	})

	t.Run("InvalidLastResetAt", func(t *testing.T) {
		fields := baseRedisRuleFields()
		fields["last_reset_at"] = "invalid-time"

		_, err := parseRedisRule(fields)
		require.Error(t, err, "Invalid last_reset_at should return parse error")
		assert.ErrorContains(t, err, "last_reset_at", "Error should indicate invalid last_reset_at field")
	})
}

type RedisStoreTestSuite struct {
	suite.Suite

	container *testx.RedisContainer
	client    *redis.Client
	store     *RedisStore
}

func TestRedisStore(t *testing.T) {
	suite.Run(t, new(RedisStoreTestSuite))
}

func (s *RedisStoreTestSuite) SetupSuite() {
	ctx := context.Background()
	s.container = testx.NewRedisContainer(ctx, s.T())

	s.client = redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%d", s.container.Redis.Host, s.container.Redis.Port),
		DB:   int(s.container.Redis.Database),
	})

	s.Require().NoError(s.client.Ping(ctx).Err(), "Should connect to Redis")
	s.store = NewRedisStore(s.client).(*RedisStore)
}

func (s *RedisStoreTestSuite) TearDownSuite() {
	if s.client != nil {
		s.client.Close()
	}
}

func (s *RedisStoreTestSuite) SetupTest() {
	s.client.FlushDB(context.Background())
}

func (*RedisStoreTestSuite) TestImplementsStoreInterface() {
	var _ Store = (*RedisStore)(nil)
}

func (s *RedisStoreTestSuite) TestReserve() {
	ctx := context.Background()

	s.Run("RuleNotFound", func() {
		_, _, err := s.store.Reserve(ctx, "reserve-missing", 1, timex.Now())
		s.ErrorIs(err, ErrRuleNotFound, "Missing rule should return ErrRuleNotFound")
	})

	s.Run("InactiveRule", func() {
		rule := &Rule{
			Key:      "reserve-inactive",
			Name:     "Reserve Inactive",
			SeqStep:  1,
			IsActive: false,
		}
		s.Require().NoError(s.store.RegisterRule(ctx, rule), "Should register inactive rule")

		_, _, err := s.store.Reserve(ctx, "reserve-inactive", 1, timex.Now())
		s.ErrorIs(err, ErrRuleNotFound, "Inactive rule should return ErrRuleNotFound")
	})

	s.Run("BasicReserve", func() {
		rule := &Rule{
			Key:      "reserve-basic",
			Name:     "Reserve Basic",
			SeqStep:  1,
			IsActive: true,
		}
		s.Require().NoError(s.store.RegisterRule(ctx, rule), "Should register rule")

		reservedRule, newValue, err := s.store.Reserve(ctx, "reserve-basic", 3, timex.Now())
		s.Require().NoError(err, "Reserve should succeed")
		s.Equal(3, newValue, "New value should equal step * count")
		s.Equal("reserve-basic", reservedRule.Key, "Returned rule key should match")
	})

	s.Run("OverflowErrorShouldNotAdvanceCounter", func() {
		rule := &Rule{
			Key:              "reserve-overflow-error",
			Name:             "Reserve Overflow Error",
			SeqStep:          1,
			CurrentValue:     9,
			MaxValue:         9,
			OverflowStrategy: OverflowError,
			ResetCycle:       ResetNone,
			IsActive:         true,
		}
		s.Require().NoError(s.store.RegisterRule(ctx, rule), "Should register rule")

		_, _, err := s.store.Reserve(ctx, "reserve-overflow-error", 1, timex.Now())
		s.ErrorIs(err, ErrSequenceOverflow, "Overflow error strategy should return ErrSequenceOverflow")

		raw, err := s.client.HGet(ctx, redisSequencePrefix+"reserve-overflow-error", "current_value").Result()
		s.Require().NoError(err, "Should read current_value from redis")
		s.Equal("9", raw, "Counter should remain unchanged after overflow error")
	})

	s.Run("InvalidStoredRuleData", func() {
		rKey := redisSequencePrefix + "reserve-invalid-data"
		s.Require().NoError(s.client.HSet(ctx, rKey, map[string]any{
			"key":               "reserve-invalid-data",
			"name":              "Reserve Invalid Data",
			"prefix":            "",
			"suffix":            "",
			"date_format":       "",
			"seq_length":        "4",
			"seq_step":          "bad-int",
			"start_value":       "0",
			"max_value":         "0",
			"overflow_strategy": string(OverflowError),
			"reset_cycle":       string(ResetNone),
			"current_value":     "0",
			"is_active":         "true",
		}).Err(), "Should seed invalid redis rule payload")

		_, _, err := s.store.Reserve(ctx, "reserve-invalid-data", 1, timex.Now())
		s.Error(err, "Malformed redis rule data should return an error")
		s.ErrorContains(err, "failed to parse sequence rule", "Reserve should wrap parse failure with context")
	})

	s.Run("CycleReset", func() {
		now := timex.Now()
		yesterday := timex.DateTime(time.Time(now).AddDate(0, 0, -1))
		rule := &Rule{
			Key:          "reserve-cycle-reset",
			Name:         "Reserve Cycle Reset",
			SeqStep:      1,
			StartValue:   0,
			CurrentValue: 100,
			ResetCycle:   ResetDaily,
			LastResetAt:  &yesterday,
			IsActive:     true,
		}
		s.Require().NoError(s.store.RegisterRule(ctx, rule), "Should register rule")

		_, newValue, err := s.store.Reserve(ctx, "reserve-cycle-reset", 1, now)
		s.Require().NoError(err, "Cycle reset reserve should succeed")
		s.Equal(1, newValue, "Cycle reset should restart counter from start value")

		raw, err := s.client.HGet(ctx, redisSequencePrefix+"reserve-cycle-reset", "current_value").Result()
		s.Require().NoError(err, "Should read current_value from redis")
		s.Equal("1", raw, "Persisted counter should be reset then incremented")
	})
}

func (s *RedisStoreTestSuite) TestConcurrentReserve() {
	ctx := context.Background()
	rule := &Rule{
		Key:      "reserve-concurrent",
		Name:     "Reserve Concurrent",
		SeqStep:  1,
		IsActive: true,
	}
	s.Require().NoError(s.store.RegisterRule(ctx, rule), "Should register rule")

	numGoroutines := 100
	values := make([]int, numGoroutines)
	errs := make([]error, numGoroutines)

	var wg sync.WaitGroup

	for i := range numGoroutines {
		wg.Go(func() {
			_, values[i], errs[i] = s.store.Reserve(ctx, "reserve-concurrent", 1, timex.Now())
		})
	}

	wg.Wait()

	for i := range numGoroutines {
		s.NoError(errs[i], "Concurrent reserve should not error")
	}

	unique := collections.NewHashSetFrom(values...)
	s.Equal(numGoroutines, unique.Size(), "Concurrent reserve should return unique values")

	raw, err := s.client.HGet(ctx, redisSequencePrefix+"reserve-concurrent", "current_value").Result()
	s.Require().NoError(err, "Should read final counter from redis")

	finalValue, err := strconv.Atoi(raw)
	s.Require().NoError(err, "Final counter should be a valid integer")
	s.Equal(numGoroutines, finalValue, "Final counter should equal successful reserve count")
}

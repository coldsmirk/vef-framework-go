package sequence

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/coldsmirk/go-collections"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/id"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/sequence"
	"github.com/coldsmirk/vef-framework-go/timex"
)

func newTestRule(key string, opts ...func(*sequence.Rule)) *sequence.Rule {
	rule := &sequence.Rule{
		Key:              key,
		Name:             "Test Rule",
		SeqLength:        4,
		SeqStep:          1,
		OverflowStrategy: sequence.OverflowError,
		ResetCycle:       sequence.ResetNone,
		IsActive:         true,
	}
	for _, opt := range opts {
		opt(rule)
	}

	return rule
}

type ScriptedReserveStore struct {
	mu             sync.Mutex
	rule           *sequence.Rule
	reserveResults []int
	reserveCalls   int
}

func newScriptedReserveStore(rule *sequence.Rule, reserveResults ...int) *ScriptedReserveStore {
	return &ScriptedReserveStore{
		rule:           rule.Clone(),
		reserveResults: reserveResults,
	}
}

func (s *ScriptedReserveStore) Reserve(_ context.Context, key string, _ int, _ timex.DateTime) (*sequence.Rule, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.rule.Key != key || !s.rule.IsActive {
		return nil, 0, sequence.ErrRuleNotFound
	}

	s.reserveCalls++
	if len(s.reserveResults) == 0 {
		return nil, 0, fmt.Errorf("unexpected reserve call #%d", s.reserveCalls)
	}

	result := s.reserveResults[0]
	s.reserveResults = s.reserveResults[1:]
	reservedRule := s.rule.Clone()
	reservedRule.CurrentValue = result

	return reservedRule, result, nil
}

func (s *ScriptedReserveStore) ReserveCallCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.reserveCalls
}

func TestGenerateConcurrentResetShouldProduceUniqueNumbers(t *testing.T) {
	now := timex.Now()
	yesterday := timex.DateTime(time.Time(now).AddDate(0, 0, -1))
	key := "race-reset-unique"

	store := sequence.NewMemoryStore().(*sequence.MemoryStore)
	store.Register(newTestRule(key, func(rule *sequence.Rule) {
		rule.ResetCycle = sequence.ResetDaily
		rule.StartValue = 0
		rule.LastResetAt = &yesterday
	}))
	generator := NewGenerator(store)

	numGoroutines := 50
	results := make([]string, numGoroutines)
	errors := make([]error, numGoroutines)

	var wg sync.WaitGroup

	for i := range numGoroutines {
		wg.Go(func() {
			results[i], errors[i] = generator.Generate(context.Background(), key)
		})
	}

	wg.Wait()

	for i := range numGoroutines {
		require.NoError(t, errors[i], "Concurrent generation should succeed for each goroutine")
	}

	unique := collections.NewHashSetFrom(results...)
	assert.Equal(t, numGoroutines, unique.Size(), "Concurrent reset boundary generation should not return duplicate serial numbers")
}

func TestGenerateOverflowErrorShouldNotAdvanceCounter(t *testing.T) {
	ctx := context.Background()
	key := "overflow-error-no-state-change"

	store := sequence.NewMemoryStore().(*sequence.MemoryStore)
	store.Register(newTestRule(key, func(r *sequence.Rule) {
		r.CurrentValue = 9
		r.MaxValue = 9
		r.OverflowStrategy = sequence.OverflowError
	}))
	generator := NewGenerator(store)

	_, err := generator.Generate(ctx, key)
	require.ErrorIs(t, err, sequence.ErrSequenceOverflow, "Overflow with error strategy should return ErrSequenceOverflow")
	_, err = generator.Generate(ctx, key)
	require.ErrorIs(t, err, sequence.ErrSequenceOverflow, "Counter should not advance after overflow error")
}

func TestGenerateOverflowResetShouldUseSingleIncrementCall(t *testing.T) {
	key := "overflow-reset-single-reserve"
	store := newScriptedReserveStore(newTestRule(key, func(rule *sequence.Rule) {
		rule.CurrentValue = 9
		rule.MaxValue = 10
		rule.StartValue = 0
		rule.OverflowStrategy = sequence.OverflowReset
	}), 1)
	generator := NewGenerator(store)

	_, err := generator.Generate(context.Background(), key)
	require.NoError(t, err, "Overflow reset path should still return a generated serial number")
	assert.Equal(t, 1, store.ReserveCallCount(), "Generation should complete with a single atomic reserve call")
}

// ---------------------------------------------------------------------------
// EngineTestSuite — base suite with all common Engine test methods.
// Concrete suites embed this and configure generator/registerRule in SetupTest.
// ---------------------------------------------------------------------------

type EngineTestSuite struct {
	suite.Suite

	generator    sequence.Generator
	ctx          context.Context
	registerRule func(rule *sequence.Rule)
}

func (s *EngineTestSuite) TestGenerate() {
	s.Run("BasicGeneration", func() {
		s.registerRule(newTestRule("gen-basic"))

		result, err := s.generator.Generate(s.ctx, "gen-basic")

		s.Require().NoError(err, "Should generate without error")
		s.Equal("0001", result, "First generated number should be 0001")
	})

	s.Run("ConsecutiveGeneration", func() {
		s.registerRule(newTestRule("gen-consec"))

		r1, err := s.generator.Generate(s.ctx, "gen-consec")
		s.Require().NoError(err, "First generation should succeed")
		s.Equal("0001", r1, "First number should be 0001")

		r2, err := s.generator.Generate(s.ctx, "gen-consec")
		s.Require().NoError(err, "Second generation should succeed")
		s.Equal("0002", r2, "Second number should be 0002")

		r3, err := s.generator.Generate(s.ctx, "gen-consec")
		s.Require().NoError(err, "Third generation should succeed")
		s.Equal("0003", r3, "Third number should be 0003")
	})

	s.Run("WithPrefix", func() {
		s.registerRule(newTestRule("gen-prefix", func(rule *sequence.Rule) {
			rule.Prefix = "ORD-"
		}))

		result, err := s.generator.Generate(s.ctx, "gen-prefix")

		s.Require().NoError(err, "Should generate with prefix")
		s.Equal("ORD-0001", result, "Result should include prefix")
	})

	s.Run("WithSuffix", func() {
		s.registerRule(newTestRule("gen-suffix", func(rule *sequence.Rule) {
			rule.Suffix = "-SH"
		}))

		result, err := s.generator.Generate(s.ctx, "gen-suffix")

		s.Require().NoError(err, "Should generate with suffix")
		s.Equal("0001-SH", result, "Result should include suffix")
	})

	s.Run("WithPrefixAndSuffix", func() {
		s.registerRule(newTestRule("gen-both", func(rule *sequence.Rule) {
			rule.Prefix = "INV-"
			rule.Suffix = "-CN"
		}))

		result, err := s.generator.Generate(s.ctx, "gen-both")

		s.Require().NoError(err, "Should generate with prefix and suffix")
		s.Equal("INV-0001-CN", result, "Result should include both prefix and suffix")
	})

	s.Run("WithDateFormat", func() {
		s.registerRule(newTestRule("gen-date", func(rule *sequence.Rule) {
			rule.Prefix = "ORD"
			rule.DateFormat = "yyyyMMdd"
		}))

		result, err := s.generator.Generate(s.ctx, "gen-date")

		s.Require().NoError(err, "Should generate with date format")
		s.Contains(result, "ORD", "Result should contain prefix")
		s.Len(result, 3+8+4, "Length should be prefix(3) + date(8) + seq(4)")
	})

	s.Run("EmptyPrefixSuffixDate", func() {
		s.registerRule(newTestRule("gen-empty"))

		result, err := s.generator.Generate(s.ctx, "gen-empty")

		s.Require().NoError(err, "Should generate with no prefix/suffix/date")
		s.Equal("0001", result, "Result should be pure sequence number")
	})

	s.Run("SeqLengthOne", func() {
		s.registerRule(newTestRule("gen-len1", func(rule *sequence.Rule) {
			rule.SeqLength = 1
		}))

		result, err := s.generator.Generate(s.ctx, "gen-len1")

		s.Require().NoError(err, "Should generate with SeqLength=1")
		s.Equal("1", result, "Result should be single digit")
	})

	s.Run("StepGreaterThanOne", func() {
		s.registerRule(newTestRule("gen-step5", func(rule *sequence.Rule) {
			rule.SeqStep = 5
		}))

		r1, err := s.generator.Generate(s.ctx, "gen-step5")
		s.Require().NoError(err, "First generation should succeed")
		s.Equal("0005", r1, "First value should be step value")

		r2, err := s.generator.Generate(s.ctx, "gen-step5")
		s.Require().NoError(err, "Second generation should succeed")
		s.Equal("0010", r2, "Second value should be 2 * step")
	})

	s.Run("RuleNotFound", func() {
		_, err := s.generator.Generate(s.ctx, "non-existent")

		s.ErrorIs(err, sequence.ErrRuleNotFound, "Should return ErrRuleNotFound for missing key")
	})
}

func (s *EngineTestSuite) TestGenerateN() {
	s.Run("BatchOfThree", func() {
		s.registerRule(newTestRule("batch-three"))

		results, err := s.generator.GenerateN(s.ctx, "batch-three", 3)

		s.Require().NoError(err, "Should generate batch without error")
		s.Require().Len(results, 3, "Should return 3 results")
		s.Equal("0001", results[0], "First result should be 0001")
		s.Equal("0002", results[1], "Second result should be 0002")
		s.Equal("0003", results[2], "Third result should be 0003")
	})

	s.Run("BatchOfOne", func() {
		s.registerRule(newTestRule("batch-one"))

		results, err := s.generator.GenerateN(s.ctx, "batch-one", 1)

		s.Require().NoError(err, "Should generate single-item batch")
		s.Require().Len(results, 1, "Should return 1 result")
		s.Equal("0001", results[0], "Result should be 0001")
	})

	s.Run("InvalidCountZero", func() {
		s.registerRule(newTestRule("batch-zero"))

		_, err := s.generator.GenerateN(s.ctx, "batch-zero", 0)

		s.ErrorIs(err, sequence.ErrInvalidCount, "Should reject count=0")
	})

	s.Run("InvalidCountNegative", func() {
		s.registerRule(newTestRule("batch-neg"))

		_, err := s.generator.GenerateN(s.ctx, "batch-neg", -1)

		s.ErrorIs(err, sequence.ErrInvalidCount, "Should reject negative count")
	})

	s.Run("BatchWithStep", func() {
		s.registerRule(newTestRule("batch-step", func(rule *sequence.Rule) {
			rule.SeqStep = 2
		}))

		results, err := s.generator.GenerateN(s.ctx, "batch-step", 3)

		s.Require().NoError(err, "Should generate batch with step=2")
		s.Require().Len(results, 3, "Should return 3 results")
		s.Equal("0002", results[0], "First result should be 0002")
		s.Equal("0004", results[1], "Second result should be 0004")
		s.Equal("0006", results[2], "Third result should be 0006")
	})

	s.Run("ConsecutiveBatches", func() {
		s.registerRule(newTestRule("batch-consec"))

		batch1, err := s.generator.GenerateN(s.ctx, "batch-consec", 2)
		s.Require().NoError(err, "First batch should succeed")
		s.Equal("0001", batch1[0], "First batch start should be 0001")
		s.Equal("0002", batch1[1], "First batch end should be 0002")

		batch2, err := s.generator.GenerateN(s.ctx, "batch-consec", 2)
		s.Require().NoError(err, "Second batch should succeed")
		s.Equal("0003", batch2[0], "Second batch should continue from 0003")
		s.Equal("0004", batch2[1], "Second batch end should be 0004")
	})
}

func (s *EngineTestSuite) TestOverflow() {
	s.Run("NoMaxValue", func() {
		s.registerRule(newTestRule("ovf-nomax", func(rule *sequence.Rule) {
			rule.CurrentValue = 9999
			rule.MaxValue = 0 // unlimited
		}))

		result, err := s.generator.Generate(s.ctx, "ovf-nomax")

		s.Require().NoError(err, "Should allow unlimited growth")
		s.Equal("10000", result, "Value should exceed SeqLength when unlimited")
	})

	s.Run("ErrorStrategy", func() {
		s.registerRule(newTestRule("ovf-error", func(rule *sequence.Rule) {
			rule.CurrentValue = 9999
			rule.MaxValue = 9999
			rule.OverflowStrategy = sequence.OverflowError
		}))

		_, err := s.generator.Generate(s.ctx, "ovf-error")

		s.ErrorIs(err, sequence.ErrSequenceOverflow, "Should return overflow error")
	})

	s.Run("ResetStrategy", func() {
		s.registerRule(newTestRule("ovf-reset", func(rule *sequence.Rule) {
			rule.CurrentValue = 9999
			rule.MaxValue = 9999
			rule.OverflowStrategy = sequence.OverflowReset
			rule.StartValue = 0
		}))

		result, err := s.generator.Generate(s.ctx, "ovf-reset")

		s.Require().NoError(err, "Should reset on overflow")
		s.Equal("0001", result, "Value should restart from StartValue+Step")
	})

	s.Run("ExtendStrategy", func() {
		s.registerRule(newTestRule("ovf-extend", func(rule *sequence.Rule) {
			rule.CurrentValue = 9999
			rule.MaxValue = 9999
			rule.OverflowStrategy = sequence.OverflowExtend
		}))

		result, err := s.generator.Generate(s.ctx, "ovf-extend")

		s.Require().NoError(err, "Should extend on overflow")
		s.Equal("10000", result, "Value should exceed SeqLength")
	})

	s.Run("ResetWithStartValue", func() {
		s.registerRule(newTestRule("ovf-reset-sv", func(rule *sequence.Rule) {
			rule.CurrentValue = 9999
			rule.MaxValue = 9999
			rule.OverflowStrategy = sequence.OverflowReset
			rule.StartValue = 100
		}))

		result, err := s.generator.Generate(s.ctx, "ovf-reset-sv")

		s.Require().NoError(err, "Should reset to StartValue on overflow")
		s.Equal("0101", result, "Value should be StartValue(100)+Step(1)=101")
	})

	s.Run("WithinMaxValue", func() {
		s.registerRule(newTestRule("ovf-within", func(rule *sequence.Rule) {
			rule.CurrentValue = 0
			rule.MaxValue = 9999
		}))

		result, err := s.generator.Generate(s.ctx, "ovf-within")

		s.Require().NoError(err, "Should generate normally within max")
		s.Equal("0001", result, "Value should be 0001 when within limit")
	})
}

func (s *EngineTestSuite) TestStartValue() {
	s.Run("NonZeroStartValue", func() {
		s.registerRule(newTestRule("sv-nonzero", func(rule *sequence.Rule) {
			rule.StartValue = 100
			rule.CurrentValue = 100
		}))

		result, err := s.generator.Generate(s.ctx, "sv-nonzero")

		s.Require().NoError(err, "Should generate from non-zero start")
		s.Equal("0101", result, "First value should be StartValue(100)+Step(1)=101")
	})
}

// ---------------------------------------------------------------------------
// MemoryEngineTestSuite — Engine backed by MemoryStore.
// ---------------------------------------------------------------------------

type MemoryEngineTestSuite struct {
	EngineTestSuite
}

func TestMemoryEngine(t *testing.T) {
	suite.Run(t, new(MemoryEngineTestSuite))
}

func (s *MemoryEngineTestSuite) SetupTest() {
	store := sequence.NewMemoryStore().(*sequence.MemoryStore)
	s.generator = NewGenerator(store)
	s.ctx = context.Background()
	s.registerRule = func(rule *sequence.Rule) {
		store.Register(rule)
	}
}

func (s *MemoryEngineTestSuite) TestConcurrentGenerate() {
	store := sequence.NewMemoryStore().(*sequence.MemoryStore)
	store.Register(newTestRule("conc-mem", func(rule *sequence.Rule) {
		rule.SeqLength = 6
	}))
	generator := NewGenerator(store)

	numGoroutines := 100
	results := make([]string, numGoroutines)
	errs := make([]error, numGoroutines)

	var wg sync.WaitGroup

	for i := range numGoroutines {
		wg.Go(func() {
			results[i], errs[i] = generator.Generate(s.ctx, "conc-mem")
		})
	}

	wg.Wait()

	for i := range numGoroutines {
		s.NoError(errs[i], "Concurrent generate should not error")
	}

	unique := collections.NewHashSetFrom(results...)
	s.Equal(numGoroutines, unique.Size(), "All generated results should be unique")
}

// ---------------------------------------------------------------------------
// DBEngineTestSuite — Engine backed by DBStore, run per database via ForEachDB.
// ---------------------------------------------------------------------------

type DBEngineTestSuite struct {
	EngineTestSuite

	env *testx.DBEnv
}

func TestDBEngine(t *testing.T) {
	testx.ForEachDB(t, func(t *testing.T, env *testx.DBEnv) {
		store := sequence.NewDBStore(env.DB)
		err := store.(*sequence.DBStore).Init(env.Ctx)
		require.NoError(t, err, "Should create sequence rule table")

		suite.Run(t, &DBEngineTestSuite{env: env})
	})
}

func (s *DBEngineTestSuite) SetupTest() {
	s.ctx = s.env.Ctx

	// Clean all rules from previous test method
	_, err := s.env.DB.NewRaw("DELETE FROM " + sequence.DBStoreTableName).Exec(s.ctx)
	s.Require().NoError(err, "Should clean up rules")

	s.generator = NewGenerator(sequence.NewDBStore(s.env.DB))
	s.registerRule = func(rule *sequence.Rule) {
		s.T().Helper()

		model := &sequence.RuleModel{
			Key:              rule.Key,
			Name:             rule.Name,
			SeqLength:        int16(rule.SeqLength),
			SeqStep:          int16(rule.SeqStep),
			StartValue:       rule.StartValue,
			MaxValue:         rule.MaxValue,
			OverflowStrategy: rule.OverflowStrategy,
			ResetCycle:       rule.ResetCycle,
			CurrentValue:     rule.CurrentValue,
			LastResetAt:      rule.LastResetAt,
			IsActive:         true,
		}
		model.ID = id.Generate()

		if rule.Prefix != "" {
			model.Prefix = &rule.Prefix
		}

		if rule.Suffix != "" {
			model.Suffix = &rule.Suffix
		}

		if rule.DateFormat != "" {
			model.DateFormat = &rule.DateFormat
		}

		_, err := s.env.DB.NewInsert().Model(model).Exec(s.ctx)
		s.Require().NoError(err, "Should insert test rule %q", rule.Key)
	}
}

func (s *DBEngineTestSuite) TestConcurrentGenerate() {
	if s.env.DS.Kind == config.SQLite {
		s.T().Skip("SQLite uses database-level locking: concurrent transactions holding SHARED locks cannot upgrade to EXCLUSIVE for writes, causing deadlock")
	}

	s.registerRule(newTestRule("conc-db", func(rule *sequence.Rule) {
		rule.SeqLength = 6
	}))

	numGoroutines := 50
	results := make([]string, numGoroutines)
	errs := make([]error, numGoroutines)

	var wg sync.WaitGroup

	for i := range numGoroutines {
		wg.Go(func() {
			results[i], errs[i] = s.generator.Generate(s.ctx, "conc-db")
		})
	}

	wg.Wait()

	for i := range numGoroutines {
		s.NoError(errs[i], "Concurrent generate should not error")
	}

	unique := collections.NewHashSetFrom(results...)
	s.Equal(numGoroutines, unique.Size(), "All generated results should be unique")
}

// ---------------------------------------------------------------------------
// RedisEngineTestSuite — Engine backed by RedisStore.
// ---------------------------------------------------------------------------

type RedisEngineTestSuite struct {
	EngineTestSuite

	container *testx.RedisContainer
	client    *redis.Client
}

func TestRedisEngine(t *testing.T) {
	suite.Run(t, new(RedisEngineTestSuite))
}

func (s *RedisEngineTestSuite) SetupSuite() {
	ctx := context.Background()
	s.container = testx.NewRedisContainer(ctx, s.T())

	s.client = redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%d", s.container.Redis.Host, s.container.Redis.Port),
		DB:   int(s.container.Redis.Database),
	})

	s.Require().NoError(s.client.Ping(ctx).Err(), "Should connect to Redis")
}

func (s *RedisEngineTestSuite) TearDownSuite() {
	if s.client != nil {
		s.client.Close()
	}
}

func (s *RedisEngineTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.client.FlushDB(s.ctx)

	store := sequence.NewRedisStore(s.client).(*sequence.RedisStore)
	s.generator = NewGenerator(store)
	s.registerRule = func(rule *sequence.Rule) {
		s.T().Helper()
		s.Require().NoError(store.RegisterRule(s.ctx, rule), "Should register rule %q", rule.Key)
	}
}

func (s *RedisEngineTestSuite) TestConcurrentGenerate() {
	s.registerRule(newTestRule("conc-redis", func(rule *sequence.Rule) {
		rule.SeqLength = 6
	}))

	numGoroutines := 100
	results := make([]string, numGoroutines)
	errs := make([]error, numGoroutines)

	var wg sync.WaitGroup

	for i := range numGoroutines {
		wg.Go(func() {
			results[i], errs[i] = s.generator.Generate(s.ctx, "conc-redis")
		})
	}

	wg.Wait()

	for i := range numGoroutines {
		s.NoError(errs[i], "Concurrent generate should not error")
	}

	unique := collections.NewHashSetFrom(results...)
	s.Equal(numGoroutines, unique.Size(), "All generated results should be unique")
}

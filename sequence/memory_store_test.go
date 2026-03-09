package sequence

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/coldsmirk/go-collections"
	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/timex"
)

type MemoryStoreTestSuite struct {
	suite.Suite

	store *MemoryStore
	ctx   context.Context
}

func TestMemoryStore(t *testing.T) {
	suite.Run(t, new(MemoryStoreTestSuite))
}

func (s *MemoryStoreTestSuite) SetupTest() {
	s.store = NewMemoryStore().(*MemoryStore)
	s.ctx = context.Background()
}

func (*MemoryStoreTestSuite) TestImplementsStoreInterface() {
	var _ Store = (*MemoryStore)(nil)
}

func (s *MemoryStoreTestSuite) TestRegister() {
	s.Run("RegisterAndReserve", func() {
		s.store.Register(&Rule{
			Key:      "order",
			Name:     "Order No",
			SeqStep:  1,
			IsActive: true,
		})

		rule, newValue, err := s.store.Reserve(s.ctx, "order", 1, timex.Now())
		s.Require().NoError(err, "Should reserve from registered rule")
		s.Equal(1, newValue, "Reserved value should be 1 on first allocation")
		s.Equal("order", rule.Key, "Returned rule key should match")
		s.Equal("Order No", rule.Name, "Returned rule name should match")
	})

	s.Run("StoreCopy", func() {
		rule := &Rule{
			Key:          "copy",
			Name:         "Original",
			CurrentValue: 10,
			IsActive:     true,
		}
		s.store.Register(rule)

		rule.Name = "Mutated"
		rule.CurrentValue = 999

		stored, ok := s.store.rules.Get("copy")
		s.Require().True(ok, "Rule should be present in internal store")
		s.Equal("Original", stored.Name, "Stored name should not be affected by external mutation")
		s.Equal(10, stored.CurrentValue, "Stored counter should not be affected by external mutation")
	})
}

func (s *MemoryStoreTestSuite) TestReserve() {
	s.Run("BasicReserve", func() {
		s.store.Register(&Rule{
			Key:      "basic",
			SeqStep:  1,
			IsActive: true,
		})

		_, newValue, err := s.store.Reserve(s.ctx, "basic", 3, timex.Now())
		s.Require().NoError(err, "Reserve should succeed")
		s.Equal(3, newValue, "New value should equal step * count")
	})

	s.Run("RuleNotFound", func() {
		_, _, err := s.store.Reserve(s.ctx, "missing", 1, timex.Now())
		s.ErrorIs(err, ErrRuleNotFound, "Reserve should return ErrRuleNotFound for missing key")
	})

	s.Run("InactiveRule", func() {
		s.store.Register(&Rule{
			Key:      "inactive",
			SeqStep:  1,
			IsActive: false,
		})

		_, _, err := s.store.Reserve(s.ctx, "inactive", 1, timex.Now())
		s.ErrorIs(err, ErrRuleNotFound, "Reserve should return ErrRuleNotFound for inactive rule")
	})

	s.Run("OverflowErrorShouldNotAdvanceCounter", func() {
		s.store.Register(&Rule{
			Key:              "overflow-error",
			SeqStep:          1,
			CurrentValue:     9,
			MaxValue:         9,
			OverflowStrategy: OverflowError,
			ResetCycle:       ResetNone,
			IsActive:         true,
		})

		_, _, err := s.store.Reserve(s.ctx, "overflow-error", 1, timex.Now())
		s.ErrorIs(err, ErrSequenceOverflow, "Overflow error strategy should return ErrSequenceOverflow")

		rule, ok := s.store.rules.Get("overflow-error")
		s.Require().True(ok, "Rule should still exist after overflow error")
		s.Equal(9, rule.CurrentValue, "Counter should remain unchanged when overflow strategy is error")
	})

	s.Run("OverflowReset", func() {
		s.store.Register(&Rule{
			Key:              "overflow-reset",
			SeqStep:          1,
			StartValue:       0,
			CurrentValue:     9,
			MaxValue:         9,
			OverflowStrategy: OverflowReset,
			ResetCycle:       ResetNone,
			IsActive:         true,
		})

		_, newValue, err := s.store.Reserve(s.ctx, "overflow-reset", 1, timex.Now())
		s.Require().NoError(err, "Overflow reset strategy should reserve successfully")
		s.Equal(1, newValue, "Counter should restart from start value on overflow reset")
	})

	s.Run("ResetByCycle", func() {
		now := timex.Now()
		yesterday := timex.DateTime(time.Time(now).AddDate(0, 0, -1))
		s.store.Register(&Rule{
			Key:          "daily-reset",
			SeqStep:      1,
			StartValue:   0,
			CurrentValue: 100,
			ResetCycle:   ResetDaily,
			LastResetAt:  &yesterday,
			IsActive:     true,
		})

		_, newValue, err := s.store.Reserve(s.ctx, "daily-reset", 1, now)
		s.Require().NoError(err, "Cycle reset should reserve successfully")
		s.Equal(1, newValue, "Cycle reset should reset to start value before increment")

		rule, ok := s.store.rules.Get("daily-reset")
		s.Require().True(ok, "Rule should still exist after cycle reset")
		s.NotNil(rule.LastResetAt, "Cycle reset should update last reset timestamp")
	})
}

func (s *MemoryStoreTestSuite) TestConcurrentReserve() {
	s.store.Register(&Rule{
		Key:      "concurrent",
		SeqStep:  1,
		IsActive: true,
	})

	numGoroutines := 100
	values := make([]int, numGoroutines)
	errs := make([]error, numGoroutines)

	var wg sync.WaitGroup

	for i := range numGoroutines {
		wg.Go(func() {
			_, values[i], errs[i] = s.store.Reserve(s.ctx, "concurrent", 1, timex.Now())
		})
	}

	wg.Wait()

	for i := range numGoroutines {
		s.NoError(errs[i], "Concurrent reserve should not error")
	}

	unique := collections.NewHashSetFrom(values...)
	s.Equal(numGoroutines, unique.Size(), "Concurrent reserve should return unique counter values")
}

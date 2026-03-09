package sequence

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/id"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/timex"
)

func deleteTestRule(t *testing.T, ctx context.Context, db orm.DB, key string) {
	t.Helper()

	_, err := db.NewDelete().
		Model((*RuleModel)(nil)).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("key", key)
		}).
		Exec(ctx)
	require.NoError(t, err, "Should delete test rule %q", key)
}

func insertTestRule(t *testing.T, ctx context.Context, db orm.DB, rule *RuleModel) {
	t.Helper()

	if rule.ID == "" {
		rule.ID = id.Generate()
	}

	wantActive := rule.IsActive
	rule.IsActive = true // Bun skips zero-value bool with default tag, insert as true first

	_, err := db.NewInsert().Model(rule).Exec(ctx)
	require.NoError(t, err, "Should insert test rule %q", rule.Key)

	if !wantActive {
		_, err = db.NewUpdate().
			Model((*RuleModel)(nil)).
			Set("is_active", false).
			Where(func(cb orm.ConditionBuilder) {
				cb.PKEquals(rule.ID)
			}).
			Exec(ctx)
		require.NoError(t, err, "Should deactivate test rule %q", rule.Key)
	}
}

func queryRuleModelByKey(t *testing.T, ctx context.Context, db orm.DB, key string) *RuleModel {
	t.Helper()

	var model RuleModel

	err := db.NewSelect().
		Model(&model).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("key", key)
		}).
		Scan(ctx)
	require.NoError(t, err, "Should query test rule %q", key)

	return &model
}

func TestDBStore(t *testing.T) {
	testx.ForEachDB(t, func(t *testing.T, env *testx.DBEnv) {
		store := NewDBStore(env.DB).(*DBStore)

		err := store.Init(env.Ctx)
		require.NoError(t, err, "Init should create table without error")

		t.Run("Reserve", func(t *testing.T) {
			t.Run("RuleNotFound", func(t *testing.T) {
				_, _, err := store.Reserve(env.Ctx, "reserve-missing", 1, timex.Now())
				assert.ErrorIs(t, err, ErrRuleNotFound, "Missing rule should return ErrRuleNotFound")
			})

			t.Run("InactiveRule", func(t *testing.T) {
				insertTestRule(t, env.Ctx, env.DB, &RuleModel{
					Key:              "reserve-inactive",
					Name:             "Reserve Inactive",
					SeqLength:        4,
					SeqStep:          1,
					OverflowStrategy: OverflowError,
					ResetCycle:       ResetNone,
					CurrentValue:     0,
					IsActive:         false,
				})
				defer deleteTestRule(t, env.Ctx, env.DB, "reserve-inactive")

				_, _, err := store.Reserve(env.Ctx, "reserve-inactive", 1, timex.Now())
				assert.ErrorIs(t, err, ErrRuleNotFound, "Inactive rule should return ErrRuleNotFound")
			})

			t.Run("BasicReserve", func(t *testing.T) {
				insertTestRule(t, env.Ctx, env.DB, &RuleModel{
					Key:              "reserve-basic",
					Name:             "Reserve Basic",
					SeqLength:        4,
					SeqStep:          1,
					OverflowStrategy: OverflowError,
					ResetCycle:       ResetNone,
					CurrentValue:     0,
					IsActive:         true,
				})
				defer deleteTestRule(t, env.Ctx, env.DB, "reserve-basic")

				rule, newValue, err := store.Reserve(env.Ctx, "reserve-basic", 3, timex.Now())
				require.NoError(t, err, "Reserve should succeed")
				assert.Equal(t, 3, newValue, "New value should equal step * count")
				assert.Equal(t, "reserve-basic", rule.Key, "Returned rule key should match")
			})

			t.Run("OptionalFieldsMapping", func(t *testing.T) {
				prefix := "ORD-"
				suffix := "-CN"
				dateFormat := "yyyyMMdd"

				insertTestRule(t, env.Ctx, env.DB, &RuleModel{
					Key:              "reserve-optional-fields",
					Name:             "Reserve Optional Fields",
					Prefix:           &prefix,
					Suffix:           &suffix,
					DateFormat:       &dateFormat,
					SeqLength:        4,
					SeqStep:          1,
					OverflowStrategy: OverflowError,
					ResetCycle:       ResetNone,
					CurrentValue:     0,
					IsActive:         true,
				})
				defer deleteTestRule(t, env.Ctx, env.DB, "reserve-optional-fields")

				rule, _, err := store.Reserve(env.Ctx, "reserve-optional-fields", 1, timex.Now())
				require.NoError(t, err, "Reserve with optional fields should succeed")
				assert.Equal(t, prefix, rule.Prefix, "Returned rule should map prefix from model")
				assert.Equal(t, suffix, rule.Suffix, "Returned rule should map suffix from model")
				assert.Equal(t, dateFormat, rule.DateFormat, "Returned rule should map date format from model")
			})

			t.Run("OverflowErrorShouldNotAdvanceCounter", func(t *testing.T) {
				insertTestRule(t, env.Ctx, env.DB, &RuleModel{
					Key:              "reserve-overflow-error",
					Name:             "Reserve Overflow Error",
					SeqLength:        4,
					SeqStep:          1,
					MaxValue:         9,
					OverflowStrategy: OverflowError,
					ResetCycle:       ResetNone,
					CurrentValue:     9,
					IsActive:         true,
				})
				defer deleteTestRule(t, env.Ctx, env.DB, "reserve-overflow-error")

				_, _, err := store.Reserve(env.Ctx, "reserve-overflow-error", 1, timex.Now())
				assert.ErrorIs(t, err, ErrSequenceOverflow, "Overflow error strategy should return ErrSequenceOverflow")

				model := queryRuleModelByKey(t, env.Ctx, env.DB, "reserve-overflow-error")
				assert.Equal(t, 9, model.CurrentValue, "Counter should remain unchanged on overflow error")
			})

			t.Run("OverflowResetShouldRestartCounter", func(t *testing.T) {
				now := timex.Now()

				insertTestRule(t, env.Ctx, env.DB, &RuleModel{
					Key:              "reserve-overflow-reset",
					Name:             "Reserve Overflow Reset",
					SeqLength:        4,
					SeqStep:          1,
					StartValue:       100,
					MaxValue:         9,
					OverflowStrategy: OverflowReset,
					ResetCycle:       ResetNone,
					CurrentValue:     9,
					IsActive:         true,
				})
				defer deleteTestRule(t, env.Ctx, env.DB, "reserve-overflow-reset")

				_, newValue, err := store.Reserve(env.Ctx, "reserve-overflow-reset", 1, now)
				require.NoError(t, err, "Overflow reset strategy should reserve successfully")
				assert.Equal(t, 101, newValue, "Overflow reset should restart from StartValue then increment")

				model := queryRuleModelByKey(t, env.Ctx, env.DB, "reserve-overflow-reset")
				assert.Equal(t, 101, model.CurrentValue, "Persisted counter should restart after overflow reset")
				assert.NotNil(t, model.LastResetAt, "Overflow reset should persist last reset timestamp")
			})

			t.Run("OverflowExtendShouldAllowGrowth", func(t *testing.T) {
				insertTestRule(t, env.Ctx, env.DB, &RuleModel{
					Key:              "reserve-overflow-extend",
					Name:             "Reserve Overflow Extend",
					SeqLength:        4,
					SeqStep:          1,
					MaxValue:         9,
					OverflowStrategy: OverflowExtend,
					ResetCycle:       ResetNone,
					CurrentValue:     9,
					IsActive:         true,
				})
				defer deleteTestRule(t, env.Ctx, env.DB, "reserve-overflow-extend")

				_, newValue, err := store.Reserve(env.Ctx, "reserve-overflow-extend", 1, timex.Now())
				require.NoError(t, err, "Overflow extend strategy should reserve successfully")
				assert.Equal(t, 10, newValue, "Overflow extend should allow value to exceed MaxValue")
			})

			t.Run("CycleReset", func(t *testing.T) {
				now := timex.Now()
				yesterday := timex.DateTime(time.Time(now).AddDate(0, 0, -1))

				insertTestRule(t, env.Ctx, env.DB, &RuleModel{
					Key:              "reserve-cycle-reset",
					Name:             "Reserve Cycle Reset",
					SeqLength:        4,
					SeqStep:          1,
					StartValue:       0,
					OverflowStrategy: OverflowError,
					ResetCycle:       ResetDaily,
					CurrentValue:     100,
					LastResetAt:      &yesterday,
					IsActive:         true,
				})
				defer deleteTestRule(t, env.Ctx, env.DB, "reserve-cycle-reset")

				_, newValue, err := store.Reserve(env.Ctx, "reserve-cycle-reset", 1, now)
				require.NoError(t, err, "Cycle reset reserve should succeed")
				assert.Equal(t, 1, newValue, "Cycle reset should reserve from start value")

				model := queryRuleModelByKey(t, env.Ctx, env.DB, "reserve-cycle-reset")
				assert.Equal(t, 1, model.CurrentValue, "Persisted counter should be reset then incremented")
				assert.NotNil(t, model.LastResetAt, "Cycle reset should persist last_reset_at")
			})
		})

		t.Run("ConcurrentReserve", func(t *testing.T) {
			if env.DS.Kind == config.SQLite {
				t.Skip("SQLite uses database-level locking: concurrent transactions may deadlock on write upgrades")
			}

			insertTestRule(t, env.Ctx, env.DB, &RuleModel{
				Key:              "reserve-concurrent",
				Name:             "Reserve Concurrent",
				SeqLength:        6,
				SeqStep:          1,
				OverflowStrategy: OverflowError,
				ResetCycle:       ResetNone,
				CurrentValue:     0,
				IsActive:         true,
			})
			defer deleteTestRule(t, env.Ctx, env.DB, "reserve-concurrent")

			numGoroutines := 50
			values := make([]int, numGoroutines)
			errs := make([]error, numGoroutines)

			var wg sync.WaitGroup

			for i := range numGoroutines {
				wg.Go(func() {
					_, values[i], errs[i] = store.Reserve(env.Ctx, "reserve-concurrent", 1, timex.Now())
				})
			}

			wg.Wait()

			for i := range numGoroutines {
				assert.NoError(t, errs[i], "Concurrent reserve should not error")
			}

			model := queryRuleModelByKey(t, env.Ctx, env.DB, "reserve-concurrent")
			assert.Equal(t, numGoroutines, model.CurrentValue, "Final counter should equal successful reserve count")
		})

		t.Run("AutoMigrateIdempotent", func(t *testing.T) {
			err := store.Init(env.Ctx)
			assert.NoError(t, err, "Second Init should be idempotent")
		})
	})
}

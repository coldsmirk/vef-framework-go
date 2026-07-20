package store

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/cron"
	"github.com/coldsmirk/vef-framework-go/crud"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/search"
)

func TestRunSearch(t *testing.T) {
	base := time.Date(2026, 7, 17, 10, 0, 0, 0, time.Local)

	t.Run("AddressesOneRowByID", func(t *testing.T) {
		db := newStoreDB(t)
		schedule := insertSchedule(t, db, scheduleFixture("addressable", "orders.sync", base))

		wanted := insertRunningRun(t, db, schedule, base.Add(-2*time.Minute), base.Add(-2*time.Minute))
		newest := insertRunningRun(t, db, schedule, base, base)
		require.NotEqual(t, wanted.ID, newest.ID, "The fixtures should be distinct rows")

		var got cron.Run
		require.NoError(t, db.NewSelect().
			Model(&got).
			Where(func(cb orm.ConditionBuilder) {
				search.NewFor[RunSearch]().Apply(cb, RunSearch{ID: wanted.ID})
			}).
			Limit(1).
			Scan(context.Background()),
			"The addressed row should load")
		assert.Equal(t, wanted.ID, got.ID, "The search should resolve the named run")
	})

	t.Run("EmptyIDDoesNotFilter", func(t *testing.T) {
		db := newStoreDB(t)
		schedule := insertSchedule(t, db, scheduleFixture("unfiltered", "orders.sync", base))
		insertRunningRun(t, db, schedule, base, base)

		var runs []cron.Run
		require.NoError(t, db.NewSelect().
			Model(&runs).
			Where(func(cb orm.ConditionBuilder) {
				search.NewFor[RunSearch]().Apply(cb, RunSearch{})
			}).
			Scan(context.Background()),
			"An empty search should remain valid")
		assert.Len(t, runs, 1, "A zero-valued ID should add no condition")
	})

	t.Run("EpochRangeDistinguishesFallbackInstants", func(t *testing.T) {
		db := newStoreDB(t)
		newYork, err := time.LoadLocation("America/New_York")
		require.NoError(t, err, "The fallback timezone should load")

		first := time.Date(2012, 11, 4, 1, 0, 0, 0, newYork)
		second := first.Add(time.Hour)
		require.Equal(t, first.Format(time.DateTime), second.In(newYork).Format(time.DateTime),
			"The fixtures should occupy the two copies of the fallback hour")

		for i, instant := range []time.Time{first, second} {
			run := &cron.Run{
				ScheduleID:        "schedule",
				ScheduleName:      "fallback",
				JobName:           "orders.sync",
				ScheduledAtUnixMs: instant.UnixMilli(),
				ClaimedAtUnixMs:   instant.UnixMilli(),
				Status:            []cron.RunStatus{cron.RunSucceeded, cron.RunFailed}[i],
			}
			_, err := db.NewInsert().Model(run).Exec(context.Background())
			require.NoError(t, err, "The fallback run fixture should insert")
		}

		from := second.UnixMilli()

		var runs []cron.Run
		require.NoError(t, db.NewSelect().
			Model(&runs).
			Where(func(cb orm.ConditionBuilder) {
				search.NewFor[RunSearch]().Apply(cb, RunSearch{ScheduledAtFromUnixMs: &from})
			}).
			Scan(context.Background()),
			"The exact range query should succeed")
		require.Len(t, runs, 1, "The exact range should select only the second fallback instant")
		assert.Equal(t, second.UnixMilli(), runs[0].ScheduledAtUnixMs,
			"The range should preserve the selected instant")
	})
}

func TestRunResourceDefaultsToClaimTimelineOrder(t *testing.T) {
	db := newStoreDB(t)
	resource, ok := NewRunResource(&config.CronConfig{
		Store: config.CronStoreConfig{Enabled: true},
	}).(*RunResource)
	require.True(t, ok, "An enabled cron store should expose the run resource")
	resource.FindPage.DisableDataPerm()

	require.NoError(t, resource.FindPage.Setup(db, &crud.FindOperationConfig{
		QueryParts: &crud.QueryPartsConfig{
			Condition:         []crud.QueryPart{},
			Sort:              []crud.QueryPart{crud.QueryRoot},
			AuditUserRelation: []crud.QueryPart{},
		},
	}), "The run page query should initialize")

	query := db.NewSelect().Model((*cron.Run)(nil))
	require.NoError(t, resource.FindPage.ConfigureQuery(query, RunSearch{}, nil, nil, crud.QueryRoot),
		"The default run page query should configure")

	sql := query.String()
	orderAt := strings.Index(sql, "ORDER BY")
	require.NotEqual(t, -1, orderAt, "The run page query should contain a default ORDER BY clause")
	order := sql[orderAt:]
	claimedAt := strings.Index(order, "claimed_at_unix_ms")
	id := strings.LastIndex(order, "id")

	require.NotEqual(t, -1, claimedAt, "The default order should include the exact claim time")
	require.NotEqual(t, -1, id, "The default order should include the deterministic ID tiebreaker")
	assert.Less(t, claimedAt, id, "Claim time should lead the ID tiebreaker")
	assert.Contains(t, order, "claimed_at_unix_ms\" DESC", "Claim time should sort newest first")
	assert.Contains(t, order, "id\" DESC", "The ID tiebreaker should sort descending")
}

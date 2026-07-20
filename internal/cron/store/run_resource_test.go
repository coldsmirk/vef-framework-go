package store

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/cron"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/search"
)

func TestRunSearch(t *testing.T) {
	base := time.Date(2026, 7, 17, 10, 0, 0, 0, time.Local)

	// find_one builds its query from the search struct alone, so a journal
	// row is only reachable if the struct can name it: without an addressable
	// id the query is unfiltered and the default primary-key ordering hands
	// back the newest row whatever the caller asked for.
	t.Run("AddressesOneRowByID", func(t *testing.T) {
		db := newStoreDB(t)
		schedule := insertSchedule(t, db, scheduleFixture("addressable", "orders.sync", base))

		wanted := insertRunningRun(t, db, schedule, base.Add(-2*time.Minute), base.Add(-2*time.Minute))
		newest := insertRunningRun(t, db, schedule, base, base)
		require.NotEqual(t, wanted.ID, newest.ID, "the fixtures must be distinct rows")

		var got cron.Run

		require.NoError(t, db.NewSelect().
			Model(&got).
			Where(func(cb orm.ConditionBuilder) {
				search.NewFor[RunSearch]().Apply(cb, RunSearch{ID: wanted.ID})
			}).
			Limit(1).
			Scan(context.Background()),
			"the addressed row must load")

		assert.Equal(t, wanted.ID, got.ID, "the search must resolve the run the caller named, not the newest one")
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
			"an empty search must still be a valid query")

		assert.Len(t, runs, 1, "a zero-valued id must add no condition, matching the browse behavior")
	})
}

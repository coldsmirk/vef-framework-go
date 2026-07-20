package store

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/cron"
	"github.com/coldsmirk/vef-framework-go/orm"
)

func TestLockRunningRunIDsUsesPrimaryKeyOrder(t *testing.T) {
	db := newStoreDB(t)
	schedule := insertSchedule(t, db, scheduleFixture("heartbeat-order", "orders.sync", time.Now().Add(time.Hour)))

	for _, runID := range []string{"run-z", "run-a"} {
		run := &cron.Run{
			ScheduleID:        schedule.ID,
			ScheduleName:      schedule.Name,
			JobName:           schedule.JobName,
			ScheduledAtUnixMs: time.Now().UnixMilli(),
			ClaimedAtUnixMs:   time.Now().UnixMilli(),
			Status:            cron.RunRunning,
			NodeID:            "node-live",
		}
		run.ID = runID
		_, err := db.NewInsert().Model(run).Exec(context.Background())
		require.NoError(t, err, "The running fixture %s should be inserted", runID)
	}

	var locked []string
	require.NoError(t, db.RunInTx(context.Background(), func(ctx context.Context, tx orm.DB) error {
		var err error

		locked, err = lockRunningRunIDs(ctx, tx, []string{"run-z", "run-a"})

		return err
	}), "Locking tracked runs should succeed")
	assert.Equal(t, []string{"run-a", "run-z"}, locked,
		"Heartbeat locking must use the same primary-key order as recovery")
}

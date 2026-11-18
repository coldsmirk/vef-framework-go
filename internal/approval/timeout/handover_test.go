package timeout

import (
	"testing"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/cron"
)

func TestRegisterHandoverJob(t *testing.T) {
	for _, tc := range []struct {
		name     string
		interval time.Duration
		count    int
	}{{"Disabled", 0, 0}, {"Enabled", time.Minute, 1}} {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := gocron.NewScheduler()
			require.NoError(t, err, "Should create scheduler")
			t.Cleanup(func() { require.NoError(t, raw.Shutdown(), "Should stop scheduler") })

			scheduler := cron.NewScheduler(raw)
			require.NoError(t, registerHandoverJob(scheduler, &HandoverScanner{}, &config.ApprovalConfig{HandoverScanInterval: tc.interval}), "Should register optional scanner")
			require.Len(t, scheduler.Jobs(), tc.count, "Only enabled scans should schedule a job")
		})
	}
}

package exec

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/integration"
)

func TestStatsRecorder(t *testing.T) {
	recorder := newStatsRecorder()

	recorder.Record("his", "patient.get", "", "", 100*time.Millisecond)
	recorder.Record("his", "patient.get", "", "", 300*time.Millisecond)
	recorder.Record("his", "patient.get", integration.FailureUpstream, "HIS down", 50*time.Millisecond)
	recorder.Record("lis", "report.get", "", "", 10*time.Millisecond)

	stats := recorder.Stats()
	require.Len(t, stats, 2, "One entry per (system, contract) pair")

	assert.Equal(t, "his", stats[0].System, "Entries should be ordered by system")
	assert.Equal(t, "lis", stats[1].System, "Entries should be ordered by system")

	his := stats[0]
	assert.Equal(t, int64(3), his.Calls, "Every invocation should count")
	assert.Equal(t, int64(2), his.Successes, "Successes should count")
	assert.Equal(t, int64(1), his.Failures[integration.FailureUpstream], "Failures should count by kind")
	assert.Equal(t, int64(150), his.AvgDurationMs, "Average should cover all calls")
	assert.Equal(t, int64(300), his.MaxDurationMs, "Max should track the slowest call")
	assert.Equal(t, "HIS down", his.LastError, "Last error message should be kept")
	assert.False(t, his.LastErrorAt.IsZero(), "Last error time should be stamped")

	lis := stats[1]
	assert.Empty(t, lis.Failures, "Failure map should be omitted when empty")
	assert.True(t, lis.LastErrorAt.IsZero(), "No-error entry should carry no error time")
}

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

	recorder.Record("his", "patient.get", integration.DirectionOutbound, "", "", 100*time.Millisecond)
	recorder.Record("his", "patient.get", integration.DirectionOutbound, "", "", 300*time.Millisecond)
	recorder.Record("his", "patient.get", integration.DirectionOutbound, integration.FailureUpstream, "HIS down", 50*time.Millisecond)
	recorder.Record("his", "patient.get", integration.DirectionInbound, "", "", 20*time.Millisecond)
	recorder.Record("lis", "report.get", integration.DirectionOutbound, "", "", 10*time.Millisecond)

	stats := recorder.Stats()
	require.Len(t, stats, 3, "One entry per (system, contract, direction) tuple")

	assert.Equal(t, "his", stats[0].System, "Entries should be ordered by system")
	assert.Equal(t, integration.DirectionInbound, stats[0].Direction, "Same pair should be ordered by direction")
	assert.Equal(t, integration.DirectionOutbound, stats[1].Direction, "Same pair should be ordered by direction")
	assert.Equal(t, "lis", stats[2].System, "Entries should be ordered by system")

	inbound := stats[0]
	assert.Equal(t, int64(1), inbound.Calls, "Directions must aggregate separately")

	his := stats[1]
	assert.Equal(t, int64(3), his.Calls, "Every invocation should count")
	assert.Equal(t, int64(2), his.Successes, "Successes should count")
	assert.Equal(t, int64(1), his.Failures[integration.FailureUpstream], "Failures should count by kind")
	assert.Equal(t, int64(150), his.AvgDurationMs, "Average should cover all calls")
	assert.Equal(t, int64(300), his.MaxDurationMs, "Max should track the slowest call")
	assert.Equal(t, "HIS down", his.LastError, "Last error message should be kept")
	assert.False(t, his.LastErrorAt.IsZero(), "Last error time should be stamped")

	lis := stats[2]
	assert.Empty(t, lis.Failures, "Failure map should be omitted when empty")
	assert.True(t, lis.LastErrorAt.IsZero(), "No-error entry should carry no error time")
}

package monitor

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/monitor"
	"github.com/coldsmirk/vef-framework-go/version"
)

func TestResolveConfig(t *testing.T) {
	defaults := DefaultConfig()

	tests := []struct {
		name string
		in   *config.MonitorConfig
		want config.MonitorConfig
	}{
		{
			name: "NilFallsBackToDefaults",
			in:   nil,
			want: defaults,
		},
		{
			name: "AllZeroFallsBackToDefaults",
			in:   &config.MonitorConfig{},
			want: defaults,
		},
		{
			name: "PartialIntervalOnlyKeepsDefaultDuration",
			in:   &config.MonitorConfig{SampleInterval: 3 * time.Second},
			want: config.MonitorConfig{
				SampleInterval: 3 * time.Second,
				SampleDuration: defaults.SampleDuration,
			},
		},
		{
			name: "PartialDurationOnlyKeepsDefaultInterval",
			in:   &config.MonitorConfig{SampleDuration: 500 * time.Millisecond},
			want: config.MonitorConfig{
				SampleInterval: defaults.SampleInterval,
				SampleDuration: 500 * time.Millisecond,
			},
		},
		{
			name: "NodeExporterPathsArePreserved",
			in: &config.MonitorConfig{
				SampleInterval: 7 * time.Second,
				SampleDuration: time.Second,
				ProcfsPath:     "/host/proc",
				RootfsPath:     "/host",
			},
			want: config.MonitorConfig{
				SampleInterval: 7 * time.Second,
				SampleDuration: time.Second,
				ProcfsPath:     "/host/proc",
				RootfsPath:     "/host",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveConfig(tt.in)
			assert.Equal(t, tt.want, got, "resolved config should apply default-then-override precedence")
		})
	}
}

func TestResolveBuildInfo(t *testing.T) {
	t.Run("NilFallsBackToUnknownAndStampsVersion", func(t *testing.T) {
		got := resolveBuildInfo(nil)
		assert.NotNil(t, got, "resolved build info must never be nil")
		assert.Equal(t, "unknown", got.AppVersion, "nil build info should default AppVersion to unknown")
		assert.Equal(t, "unknown", got.BuildTime, "nil build info should default BuildTime to unknown")
		assert.Equal(t, "unknown", got.GitCommit, "nil build info should default GitCommit to unknown")
		assert.Equal(t, version.VEFVersion, got.VEFVersion, "VEFVersion should be stamped")
	})

	t.Run("SuppliedInfoIsKeptAndVersionStamped", func(t *testing.T) {
		got := resolveBuildInfo(&monitor.BuildInfo{
			AppVersion: "v1.2.3",
			BuildTime:  "2024-01-01T00:00:00Z",
			GitCommit:  "abc123",
		})
		assert.Equal(t, "v1.2.3", got.AppVersion, "supplied AppVersion should be preserved")
		assert.Equal(t, "2024-01-01T00:00:00Z", got.BuildTime, "supplied BuildTime should be preserved")
		assert.Equal(t, "abc123", got.GitCommit, "supplied GitCommit should be preserved")
		assert.Equal(t, version.VEFVersion, got.VEFVersion, "VEFVersion should override any supplied value")
	})
}

// TestApplyCgroupCPUSummary checks the overview CPU override: cores are adopted
// immediately, but utilization stays the host figure until the container's own
// delta is ready (#14).
func TestApplyCgroupCPUSummary(t *testing.T) {
	newSummary := func() *monitor.CPUSummary {
		return &monitor.CPUSummary{PhysicalCores: 8, LogicalCores: 8, UsagePercent: 42}
	}

	t.Run("no sample keeps host", func(t *testing.T) {
		s := &DefaultService{}
		summary := newSummary()
		s.applyCgroupCPUSummary(summary)
		assert.Equal(t, 8, summary.LogicalCores)
		assert.InDelta(t, 42.0, summary.UsagePercent, 1e-9)
	})

	t.Run("cores override but usage kept until ready", func(t *testing.T) {
		s := &DefaultService{}
		s.cgroupCPUCache.Store(cgroupCPUSample{cores: 2, usageReady: false, ok: true})

		summary := newSummary()
		s.applyCgroupCPUSummary(summary)
		assert.Equal(t, 2, summary.LogicalCores, "cores overridden immediately")
		assert.InDelta(t, 42.0, summary.UsagePercent, 1e-9, "usage kept as host until the delta is ready")
	})

	t.Run("usage overridden once ready", func(t *testing.T) {
		s := &DefaultService{}
		s.cgroupCPUCache.Store(cgroupCPUSample{cores: 2, usagePercent: 30, usageReady: true, ok: true})

		summary := newSummary()
		s.applyCgroupCPUSummary(summary)
		assert.Equal(t, 2, summary.LogicalCores)
		assert.InDelta(t, 30.0, summary.UsagePercent, 1e-9)
	})
}

func TestMeanPercent(t *testing.T) {
	assert.Equal(t, 0.0, meanPercent(nil), "empty slice is 0, not a divide-by-zero")
	assert.InDelta(t, 50.0, meanPercent([]float64{25, 75}), 1e-9)
	assert.InDelta(t, 30.0, meanPercent([]float64{10, 20, 60}), 1e-9)
}

// TestSamplerLifecycle checks Init/Close are idempotent, restartable and reset
// the cgroup CPU baseline (#3/#4).
func TestSamplerLifecycle(t *testing.T) {
	s := &DefaultService{config: DefaultConfig()}

	require.NoError(t, s.Init(context.Background()))
	require.NotNil(t, s.samplerCancel, "Init starts the sampler")

	require.NoError(t, s.Init(context.Background()), "second Init is a no-op")

	require.NoError(t, s.Close())
	assert.Nil(t, s.samplerCancel, "Close clears the sampler handle so Init can restart")

	// Simulate a stale baseline, then a restart must reset it.
	s.prevCgroupCPU = cgroupCPUPrev{ok: true, usageMicros: 12345}
	require.NoError(t, s.Init(context.Background()))
	assert.False(t, s.prevCgroupCPU.ok, "Init resets the stale cgroup CPU baseline")

	require.NoError(t, s.Close())
}

// TestApplyCgroupMemorySummary locks the overview memory override: it takes the
// cgroup limit only when it is finite and below the host total, and otherwise
// leaves the host figures unchanged.
func TestApplyCgroupMemorySummary(t *testing.T) {
	const hostTotal = uint64(8) << 30

	t.Run("LimitBelowHostTotalOverrides", func(t *testing.T) {
		withCgroupFixture(t, cgroupFixture{
			procSelfCgroup: "0::/\n",
			v2: map[string]string{
				"memory.max":     "536870912\n",
				"memory.current": "268435456\n",
				"memory.stat":    "inactive_file 0\n",
			},
		})

		summary := &monitor.MemorySummary{Total: hostTotal, Used: 1 << 30, UsedPercent: 12.5}
		applyCgroupMemorySummary(summary)

		assert.Equal(t, uint64(536870912), summary.Total, "should adopt the container limit")
		assert.Equal(t, uint64(268435456), summary.Used)
		assert.InDelta(t, 50.0, summary.UsedPercent, 0.01)
	})

	t.Run("LimitAtOrAboveHostTotalKeepsHost", func(t *testing.T) {
		withCgroupFixture(t, cgroupFixture{
			procSelfCgroup: "0::/\n",
			v2: map[string]string{
				"memory.max":     "17179869184\n", // 16Gi > 8Gi host
				"memory.current": "1\n",
				"memory.stat":    "inactive_file 0\n",
			},
		})

		summary := &monitor.MemorySummary{Total: hostTotal, Used: 1 << 30, UsedPercent: 12.5}
		applyCgroupMemorySummary(summary)

		assert.Equal(t, hostTotal, summary.Total, "limit >= host total keeps host figures")
		assert.InDelta(t, 12.5, summary.UsedPercent, 0.01)
	})

	t.Run("NoCgroupKeepsHost", func(t *testing.T) {
		withCgroupFixture(t, cgroupFixture{procSelfCgroup: "0::/\n"})

		summary := &monitor.MemorySummary{Total: hostTotal, Used: 1 << 30, UsedPercent: 12.5}
		applyCgroupMemorySummary(summary)

		assert.Equal(t, hostTotal, summary.Total)
	})
}

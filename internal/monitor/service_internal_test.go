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
			name: "FullOverrideWins",
			in: &config.MonitorConfig{
				SampleInterval: 7 * time.Second,
				SampleDuration: time.Second,
			},
			want: config.MonitorConfig{
				SampleInterval: 7 * time.Second,
				SampleDuration: time.Second,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveConfig(tt.in)
			assert.Equal(t, tt.want, got, "Resolved config should apply default-then-override precedence")
		})
	}
}

func TestResolveBuildInfo(t *testing.T) {
	t.Run("NilFallsBackToUnknownAndStampsVersion", func(t *testing.T) {
		got := resolveBuildInfo(nil)
		assert.NotNil(t, got, "Resolved build info must never be nil")
		assert.Equal(t, "unknown", got.AppVersion, "Nil build info should default AppVersion to unknown")
		assert.Equal(t, "unknown", got.BuildTime, "Nil build info should default BuildTime to unknown")
		assert.Equal(t, "unknown", got.GitCommit, "Nil build info should default GitCommit to unknown")
		assert.Equal(t, version.VEFVersion, got.VEFVersion, "VEFVersion should be stamped")
	})

	t.Run("SuppliedInfoIsKeptAndVersionStamped", func(t *testing.T) {
		got := resolveBuildInfo(&monitor.BuildInfo{
			AppVersion: "v1.2.3",
			BuildTime:  "2024-01-01T00:00:00Z",
			GitCommit:  "abc123",
		})
		assert.Equal(t, "v1.2.3", got.AppVersion, "Supplied AppVersion should be preserved")
		assert.Equal(t, "2024-01-01T00:00:00Z", got.BuildTime, "Supplied BuildTime should be preserved")
		assert.Equal(t, "abc123", got.GitCommit, "Supplied GitCommit should be preserved")
		assert.Equal(t, version.VEFVersion, got.VEFVersion, "VEFVersion should override any supplied value")
	})
}

func TestRootDiskPathForOS(t *testing.T) {
	tests := []struct {
		name        string
		goos        string
		systemDrive string
		want        string
	}{
		{name: "LinuxUsesRoot", goos: "linux", systemDrive: "D:", want: "/"},
		{name: "MacOSUsesRoot", goos: "darwin", systemDrive: "D:", want: "/"},
		{name: "WindowsUsesSystemDrive", goos: "windows", systemDrive: "D:", want: `D:\`},
		{name: "WindowsFallsBackToCDrive", goos: "windows", want: `C:\`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rootDiskPathForOS(tt.goos, tt.systemDrive)
			assert.Equal(t, tt.want, got, "Root disk path for %s should match", tt.goos)
		})
	}
}

func TestRootDiskSummary(t *testing.T) {
	summary, err := new(DefaultService).rootDiskSummary(context.Background())
	require.NoError(t, err, "Root disk summary should be collected")

	assert.Positive(t, summary.Total, "Root filesystem total should be positive")
	assert.LessOrEqual(t, summary.Used, summary.Total, "Root filesystem used bytes should not exceed total bytes")
	assert.GreaterOrEqual(t, summary.UsedPercent, 0.0, "Root filesystem usage percentage should be non-negative")
	assert.LessOrEqual(t, summary.UsedPercent, 100.0, "Root filesystem usage percentage should not exceed one hundred")
	assert.Equal(t, 1, summary.Partitions, "Summary should represent exactly one root filesystem")
}

func TestMeanPercent(t *testing.T) {
	assert.Equal(t, 0.0, meanPercent(nil), "An empty sample must yield 0, not a division by zero")
	assert.InDelta(t, 50.0, meanPercent([]float64{25, 75}), 0.0001, "The mean of 25 and 75 is 50")
	assert.InDelta(t, 30.0, meanPercent([]float64{10, 20, 60}), 0.0001, "The mean of 10, 20 and 60 is 30")
}

func TestSamplerLifecycle(t *testing.T) {
	s := &DefaultService{
		config: config.MonitorConfig{
			SampleInterval: 50 * time.Millisecond,
			SampleDuration: 10 * time.Millisecond,
		},
		cgroups: newCgroupReader(),
	}

	require.NoError(t, s.Init(context.Background()), "First Init should start the sampler")
	require.NotNil(t, s.samplerCancel, "Init should record the sampler cancel handle")

	running := s.samplerDone

	require.NoError(t, s.Init(context.Background()), "Second Init while running should be a no-op")
	assert.Equal(t, running, s.samplerDone, "A second Init must not replace the running sampler")

	require.NoError(t, s.Close(), "Close should stop the sampler")
	assert.Nil(t, s.samplerCancel, "Close should clear the cancel handle so Init can restart")
	assert.Nil(t, s.samplerDone, "Close should clear the done handle so Init can restart")

	require.NoError(t, s.Close(), "Close on a stopped service should be a no-op")

	require.NoError(t, s.Init(context.Background()), "Init after Close should restart the sampler")
	require.NotNil(t, s.samplerCancel, "The restarted sampler should record a fresh cancel handle")
	require.NoError(t, s.Close(), "Close should stop the restarted sampler")
}

func TestApplyCgroupMemoryLimit(t *testing.T) {
	limitedReader := func(t *testing.T, limit, current string) *cgroupReader {
		t.Helper()

		root := newV2Root(t)
		writeCgroupFile(t, root, "sys/fs/cgroup/memory.max", limit)

		if current != "" {
			writeCgroupFile(t, root, "sys/fs/cgroup/memory.current", current)
		}

		return &cgroupReader{root: root}
	}

	hostView := func() *monitor.VirtualMemory {
		return &monitor.VirtualMemory{
			Total:       64 * 1 << 30,
			Used:        32 * 1 << 30,
			Available:   32 * 1 << 30,
			Free:        16 * 1 << 30,
			UsedPercent: 50,
		}
	}

	t.Run("LimitOverridesHeadlineFigures", func(t *testing.T) {
		s := &DefaultService{cgroups: limitedReader(t, "536870912\n", "268435456\n")}

		virtual := hostView()
		s.applyCgroupMemoryLimit(virtual)

		assert.Equal(t, uint64(536870912), virtual.Total, "Total becomes the cgroup limit")
		assert.Equal(t, uint64(268435456), virtual.Used, "Used becomes the cgroup working set")
		assert.Equal(t, uint64(268435456), virtual.Available, "Available is limit minus used")
		assert.Equal(t, uint64(268435456), virtual.Free, "Free is limit minus used")
		assert.InDelta(t, 50.0, virtual.UsedPercent, 0.0001, "UsedPercent derives from the limit")
	})

	t.Run("LimitAtOrAboveHostTotalIsIgnored", func(t *testing.T) {
		s := &DefaultService{cgroups: limitedReader(t, "68719476736\n", "268435456\n")}

		virtual := hostView()
		want := *hostView()

		s.applyCgroupMemoryLimit(virtual)

		assert.Equal(t, want, *virtual, "A limit that equals the host total constrains nothing and must not rewrite the host view")
	})

	t.Run("UnreadableUsageKeepsHostView", func(t *testing.T) {
		s := &DefaultService{cgroups: limitedReader(t, "536870912\n", "")}

		virtual := hostView()
		want := *hostView()

		s.applyCgroupMemoryLimit(virtual)

		assert.Equal(t, want, *virtual, "A limit without a readable usage counter must not produce a mixed view")
	})

	t.Run("UnlimitedContainerKeepsHostView", func(t *testing.T) {
		s := &DefaultService{cgroups: limitedReader(t, "max\n", "268435456\n")}

		virtual := hostView()
		want := *hostView()

		s.applyCgroupMemoryLimit(virtual)

		assert.Equal(t, want, *virtual, "No limit means the host view is authoritative")
	})

	t.Run("UsageIsClampedToLimit", func(t *testing.T) {
		s := &DefaultService{cgroups: limitedReader(t, "536870912\n", "1073741824\n")}

		virtual := hostView()
		s.applyCgroupMemoryLimit(virtual)

		assert.Equal(t, uint64(536870912), virtual.Used, "Usage above the limit is clamped")
		assert.Equal(t, uint64(0), virtual.Available, "Clamped usage leaves nothing available")
		assert.InDelta(t, 100.0, virtual.UsedPercent, 0.0001, "Clamped usage saturates the percentage")
	})
}

func TestApplyCgroupCPUQuota(t *testing.T) {
	readerWithUsage := func(t *testing.T, usageUsec string) *cgroupReader {
		t.Helper()

		root := newV2Root(t)
		writeCgroupFile(t, root, "sys/fs/cgroup/cpu.max", "50000 100000\n")
		writeCgroupFile(t, root, "sys/fs/cgroup/cpu.stat", "usage_usec "+usageUsec+"\n")

		return &cgroupReader{root: root}
	}

	t.Run("CoresBecomeQuotaCeiling", func(t *testing.T) {
		s := &DefaultService{cgroups: readerWithUsage(t, "1000")}

		cpuInfo := &monitor.CPUInfo{PhysicalCores: 8, LogicalCores: 16, UsagePercent: []float64{1, 2}}
		s.applyCgroupCPUQuota(cpuInfo, 0.5, 0, false, time.Now())

		assert.Equal(t, 1, cpuInfo.PhysicalCores, "A fractional quota rounds up to one core")
		assert.Equal(t, 1, cpuInfo.LogicalCores, "Logical cores match the quota ceiling")
		assert.NotNil(t, cpuInfo.UsagePercent, "Without a usage baseline the host percentages are kept")

		cpuInfo = &monitor.CPUInfo{PhysicalCores: 8, LogicalCores: 16}
		s.applyCgroupCPUQuota(cpuInfo, 2.5, 0, false, time.Now())

		assert.Equal(t, 3, cpuInfo.PhysicalCores, "A 2.5-core quota rounds up to three cores")
	})

	t.Run("TotalPercentIsShareOfQuota", func(t *testing.T) {
		// Counter at 250000µs, baseline at 150000µs: 100ms of CPU over a
		// ~400ms window with a 0.5-core quota is ~50% of the budget. The
		// window is measured with time.Since, so allow scheduling jitter.
		s := &DefaultService{cgroups: readerWithUsage(t, "250000")}

		cpuInfo := &monitor.CPUInfo{TotalPercent: 87, UsagePercent: []float64{1, 2}}
		s.applyCgroupCPUQuota(cpuInfo, 0.5, 150*time.Millisecond, true, time.Now().Add(-400*time.Millisecond))

		assert.InDelta(t, 50.0, cpuInfo.TotalPercent, 5, "A 100ms delta over a 400ms window is half of a 0.5-core quota")
		assert.Nil(t, cpuInfo.UsagePercent, "The host per-core breakdown is dropped under a quota")
	})

	t.Run("ZeroDeltaIsZeroPercent", func(t *testing.T) {
		// The fake usage counter is static, so a baseline equal to the file's
		// value yields a zero delta: 0% of the quota was consumed.
		s := &DefaultService{cgroups: readerWithUsage(t, "250000")}

		cpuInfo := &monitor.CPUInfo{TotalPercent: 87, UsagePercent: []float64{1, 2}}
		s.applyCgroupCPUQuota(cpuInfo, 0.5, 250*time.Millisecond, true, time.Now().Add(-100*time.Millisecond))

		assert.InDelta(t, 0.0, cpuInfo.TotalPercent, 0.0001, "A zero usage delta is zero percent of the quota")
		assert.Nil(t, cpuInfo.UsagePercent, "The host per-core breakdown is dropped under a quota")
	})

	t.Run("PercentIsClampedToHundred", func(t *testing.T) {
		// 250ms of CPU against a 100ms × 0.5-core budget is 500%; the report
		// must saturate at the limit instead of leaking scheduler jitter.
		s := &DefaultService{cgroups: readerWithUsage(t, "250000")}

		cpuInfo := &monitor.CPUInfo{TotalPercent: 87}
		s.applyCgroupCPUQuota(cpuInfo, 0.5, 0, true, time.Now().Add(-100*time.Millisecond))

		assert.InDelta(t, 100.0, cpuInfo.TotalPercent, 0.0001, "Consumption beyond the quota budget clamps to 100")
	})

	t.Run("BackwardsUsageCounterKeepsHostPercent", func(t *testing.T) {
		s := &DefaultService{cgroups: readerWithUsage(t, "1000")}

		cpuInfo := &monitor.CPUInfo{TotalPercent: 87}
		s.applyCgroupCPUQuota(cpuInfo, 0.5, time.Hour, true, time.Now().Add(-100*time.Millisecond))

		assert.InDelta(t, 87.0, cpuInfo.TotalPercent, 0.0001, "A counter that moved backwards must not produce a bogus percentage")
	})
}

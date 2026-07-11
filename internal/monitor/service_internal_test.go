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

func TestOverviewCopiesEffectiveCores(t *testing.T) {
	s := &DefaultService{buildInfo: resolveBuildInfo(nil), cgroups: newCgroupReader()}
	s.cpuCache.Store(&monitor.CPUInfo{
		PhysicalCores:  8,
		LogicalCores:   16,
		TotalPercent:   25,
		EffectiveCores: 0.5,
	})

	overview, err := s.Overview(context.Background())
	require.NoError(t, err, "Overview should remain best-effort")
	require.NotNil(t, overview.CPU, "A cached CPU sample must populate the overview")
	assert.Equal(t, 0.5, overview.CPU.EffectiveCores, "Overview must preserve the detailed CPU effective capacity")
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

	t.Run("AncestorLimitUsesAncestorUsage", func(t *testing.T) {
		root := t.TempDir()
		writeCgroupFile(t, root, "proc/self/cgroup", "0::/pod/ctr\n")
		writeCgroupFile(t, root, "proc/self/mountinfo", cgroupMountInfoLine("29 23 0:26", "/", "/sys/fs/cgroup", "cgroup2", "rw"))
		writeCgroupFile(t, root, "sys/fs/cgroup/pod/ctr/memory.max", "1073741824\n")
		writeCgroupFile(t, root, "sys/fs/cgroup/pod/ctr/memory.current", "104857600\n")
		writeCgroupFile(t, root, "sys/fs/cgroup/pod/memory.max", "536870912\n")
		writeCgroupFile(t, root, "sys/fs/cgroup/pod/memory.current", "419430400\n")
		s := &DefaultService{cgroups: &cgroupReader{root: root}}

		virtual := hostView()
		s.applyCgroupMemoryLimit(virtual)

		assert.Equal(t, uint64(536870912), virtual.Total, "The ancestor limit must replace host total")
		assert.Equal(t, uint64(419430400), virtual.Used, "Usage must come from the same ancestor that supplied the limit")
		assert.Equal(t, uint64(117440512), virtual.Available, "Available memory must derive from the scoped sample")
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

func TestApplyCgroupCPUScope(t *testing.T) {
	scopeWithUsage := func(t *testing.T, usageUsec string) cgroupCPUScope {
		t.Helper()

		root := newV2Root(t)
		writeCgroupFile(t, root, "sys/fs/cgroup/cpu.max", "50000 100000\n")

		if usageUsec != "" {
			writeCgroupFile(t, root, "sys/fs/cgroup/cpu.stat", "usage_usec "+usageUsec+"\n")
		}

		scope, limited := (&cgroupReader{root: root}).cpuScope(16)
		require.True(t, limited, "The fixture quota must reduce host capacity")

		return scope
	}

	t.Run("CompleteSampleUsesEffectiveCapacityWithoutChangingTopology", func(t *testing.T) {
		// Counter at 250000µs, baseline at 150000µs: 100ms of CPU over a
		// ~400ms window with a 0.5-core quota is ~50% of the budget. The
		// window is measured with time.Since, so allow scheduling jitter.
		cpuInfo := &monitor.CPUInfo{
			PhysicalCores:  8,
			LogicalCores:   16,
			EffectiveCores: 16,
			TotalPercent:   87,
			UsagePercent:   []float64{1, 2},
		}
		new(DefaultService).applyCgroupCPUScope(
			cpuInfo,
			scopeWithUsage(t, "250000"),
			150*time.Millisecond,
			true,
			time.Now().Add(-400*time.Millisecond),
		)

		assert.Equal(t, 8, cpuInfo.PhysicalCores, "Physical cores must retain host topology")
		assert.Equal(t, 16, cpuInfo.LogicalCores, "Logical cores must retain host topology")
		assert.Equal(t, 0.5, cpuInfo.EffectiveCores, "Effective cores must carry fractional cgroup capacity")
		assert.InDelta(t, 50.0, cpuInfo.TotalPercent, 5, "A 100ms delta over a 400ms window is half of a 0.5-core quota")
		assert.Nil(t, cpuInfo.UsagePercent, "The host per-core breakdown must be dropped for a cgroup sample")
	})

	t.Run("ZeroDeltaIsZeroPercent", func(t *testing.T) {
		// The fake usage counter is static, so a baseline equal to the file's
		// value yields a zero delta: 0% of the quota was consumed.
		cpuInfo := &monitor.CPUInfo{EffectiveCores: 16, TotalPercent: 87, UsagePercent: []float64{1, 2}}
		new(DefaultService).applyCgroupCPUScope(
			cpuInfo,
			scopeWithUsage(t, "250000"),
			250*time.Millisecond,
			true,
			time.Now().Add(-100*time.Millisecond),
		)

		assert.InDelta(t, 0.0, cpuInfo.TotalPercent, 0.0001, "A zero usage delta is zero percent of the quota")
		assert.Nil(t, cpuInfo.UsagePercent, "The host per-core breakdown must be dropped for a cgroup sample")
	})

	t.Run("PercentIsClampedToHundred", func(t *testing.T) {
		// 250ms of CPU against a 100ms × 0.5-core budget is 500%; the report
		// must saturate at the limit instead of leaking scheduler jitter.
		cpuInfo := &monitor.CPUInfo{EffectiveCores: 16, TotalPercent: 87}
		new(DefaultService).applyCgroupCPUScope(
			cpuInfo,
			scopeWithUsage(t, "250000"),
			0,
			true,
			time.Now().Add(-100*time.Millisecond),
		)

		assert.InDelta(t, 100.0, cpuInfo.TotalPercent, 0.0001, "Consumption beyond the quota budget clamps to 100")
	})

	t.Run("MissingBaselineKeepsCompleteHostView", func(t *testing.T) {
		cpuInfo := &monitor.CPUInfo{
			PhysicalCores: 8,
			LogicalCores:  16,
			TotalPercent:  87,
			UsagePercent:  []float64{1, 2},
		}
		cpuInfo.EffectiveCores = float64(cpuInfo.LogicalCores)
		want := *cpuInfo

		new(DefaultService).applyCgroupCPUScope(cpuInfo, scopeWithUsage(t, "250000"), 0, false, time.Now())

		assert.Equal(t, want, *cpuInfo, "A missing baseline must preserve the entire host CPU view")
	})

	t.Run("MissingFinalSampleKeepsCompleteHostView", func(t *testing.T) {
		cpuInfo := &monitor.CPUInfo{
			PhysicalCores:  8,
			LogicalCores:   16,
			EffectiveCores: 16,
			TotalPercent:   87,
			UsagePercent:   []float64{1, 2},
		}
		want := *cpuInfo

		new(DefaultService).applyCgroupCPUScope(
			cpuInfo,
			scopeWithUsage(t, ""),
			150*time.Millisecond,
			true,
			time.Now().Add(-100*time.Millisecond),
		)

		assert.Equal(t, want, *cpuInfo, "A missing final counter must preserve the entire host CPU view")
	})

	t.Run("BackwardsUsageCounterKeepsCompleteHostView", func(t *testing.T) {
		cpuInfo := &monitor.CPUInfo{
			PhysicalCores:  8,
			LogicalCores:   16,
			EffectiveCores: 16,
			TotalPercent:   87,
			UsagePercent:   []float64{1, 2},
		}
		want := *cpuInfo

		new(DefaultService).applyCgroupCPUScope(
			cpuInfo,
			scopeWithUsage(t, "1000"),
			time.Hour,
			true,
			time.Now().Add(-100*time.Millisecond),
		)

		assert.Equal(t, want, *cpuInfo, "A backwards counter must preserve the entire host CPU view")
	})
}

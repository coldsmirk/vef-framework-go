package monitor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeCgroupFile creates rel (with parents) under root with the given content.
func writeCgroupFile(t *testing.T, root, rel, content string) {
	t.Helper()

	path := filepath.Join(root, rel)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755), "Fake cgroup dir must be creatable")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644), "Fake cgroup file must be writable")
}

func cgroupMountInfoLine(header, mountRoot, mountPoint, fsType, superOptions string) string {
	return strings.Join([]string{
		header,
		mountRoot,
		mountPoint,
		"rw,nosuid,nodev,noexec,relatime",
		"-",
		fsType,
		"cgroup",
		superOptions,
	}, " ") + "\n"
}

// newV2Root fabricates a cgroup v2 unified hierarchy whose process cgroup is
// the mount root — the layout seen inside a container with a private cgroup
// namespace.
func newV2Root(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	writeCgroupFile(t, root, "sys/fs/cgroup/cgroup.controllers", "cpu memory\n")
	writeCgroupFile(t, root, "proc/self/cgroup", "0::/\n")
	writeCgroupFile(t, root, "proc/self/mountinfo", cgroupMountInfoLine("29 23 0:26", "/", "/sys/fs/cgroup", "cgroup2", "rw"))

	return root
}

func writeV1MountInfo(t *testing.T, root, memoryRoot, cpuRoot string) {
	t.Helper()

	content := cgroupMountInfoLine("30 23 0:27", memoryRoot, "/sys/fs/cgroup/memory", "cgroup", "rw,memory") +
		cgroupMountInfoLine("31 23 0:28", cpuRoot, "/sys/fs/cgroup/cpu,cpuacct", "cgroup", "rw,cpu,cpuacct")
	writeCgroupFile(t, root, "proc/self/mountinfo", content)
}

func TestCgroupV2(t *testing.T) {
	t.Run("LimitedContainerReportsQuotaAndLimit", func(t *testing.T) {
		root := newV2Root(t)
		writeCgroupFile(t, root, "sys/fs/cgroup/cpu.max", "50000 100000\n")
		writeCgroupFile(t, root, "sys/fs/cgroup/cpu.stat", "usage_usec 250000\nuser_usec 200000\n")
		writeCgroupFile(t, root, "sys/fs/cgroup/memory.max", "536870912\n")
		writeCgroupFile(t, root, "sys/fs/cgroup/memory.current", "268435456\n")
		writeCgroupFile(t, root, "sys/fs/cgroup/memory.stat", "anon 100\ninactive_file 134217728\n")

		reader := &cgroupReader{root: root}

		scope, limited := reader.cpuScope(8)
		require.True(t, limited, "A real cpu.max quota must reduce host capacity")
		assert.InDelta(t, 0.5, scope.capacity, 0.0001, "Quota 50000/100000 is half a core")

		usage, ok := scope.sample()
		require.True(t, ok, "The cpu.stat usage_usec counter must be readable")
		assert.Equal(t, 250*time.Millisecond, usage, "The usage_usec value converts to a duration")

		limit, used, ok := reader.memorySample(64 * 1 << 30)
		require.True(t, ok, "A numeric memory.max and its usage must form a sample")
		assert.Equal(t, uint64(536870912), limit, "The memory.max value is the limit in bytes")
		assert.Equal(t, uint64(268435456-134217728), used, "Working set subtracts the inactive file cache")
	})

	t.Run("UnlimitedContainerReportsNoLimits", func(t *testing.T) {
		root := newV2Root(t)
		writeCgroupFile(t, root, "sys/fs/cgroup/cpu.max", "max 100000\n")
		writeCgroupFile(t, root, "sys/fs/cgroup/memory.max", "max\n")

		reader := &cgroupReader{root: root}

		scope, limited := reader.cpuScope(8)
		assert.False(t, limited, "A cpu.max of \"max\" does not reduce host capacity")
		assert.Equal(t, 8.0, scope.capacity, "An unlimited cgroup keeps host CPU capacity")

		_, _, ok := reader.memorySample(64 * 1 << 30)
		assert.False(t, ok, "A memory.max of \"max\" cannot produce a limited sample")
	})

	t.Run("TightestAncestorLimitWins", func(t *testing.T) {
		root := t.TempDir()
		writeCgroupFile(t, root, "sys/fs/cgroup/cgroup.controllers", "cpu memory\n")
		writeCgroupFile(t, root, "proc/self/cgroup", "0::/kubepods/pod1/ctr1\n")
		writeCgroupFile(t, root, "proc/self/mountinfo", cgroupMountInfoLine("29 23 0:26", "/", "/sys/fs/cgroup", "cgroup2", "rw"))
		// The leaf has no CPU limit and a loose memory limit; the pod level
		// carries the effective ones.
		writeCgroupFile(t, root, "sys/fs/cgroup/kubepods/pod1/ctr1/cpu.max", "max 100000\n")
		writeCgroupFile(t, root, "sys/fs/cgroup/kubepods/pod1/ctr1/memory.max", "1073741824\n")
		writeCgroupFile(t, root, "sys/fs/cgroup/kubepods/pod1/cpu.max", "200000 100000\n")
		writeCgroupFile(t, root, "sys/fs/cgroup/kubepods/pod1/cpu.stat", "usage_usec 250000\n")
		writeCgroupFile(t, root, "sys/fs/cgroup/kubepods/pod1/memory.max", "536870912\n")
		writeCgroupFile(t, root, "sys/fs/cgroup/kubepods/pod1/memory.current", "268435456\n")

		reader := &cgroupReader{root: root}

		scope, limited := reader.cpuScope(8)
		require.True(t, limited, "An ancestor quota must be honored when the leaf says max")
		assert.InDelta(t, 2.0, scope.capacity, 0.0001, "The pod-level 2-core quota is effective")
		usage, ok := scope.sample()
		require.True(t, ok, "The usage counter from the limiting ancestor must be readable")
		assert.Equal(t, 250*time.Millisecond, usage, "CPU usage must come from the limiting ancestor")

		limit, used, ok := reader.memorySample(64 * 1 << 30)
		require.True(t, ok, "Ancestor memory limits and usage must be sampled together")
		assert.Equal(t, uint64(536870912), limit, "The tightest limit along the chain wins")
		assert.Equal(t, uint64(268435456), used, "Memory usage must come from the limiting ancestor")
	})

	t.Run("InvisibleLeafPathFallsBackToMountRoot", func(t *testing.T) {
		root := t.TempDir()
		writeCgroupFile(t, root, "sys/fs/cgroup/cgroup.controllers", "cpu memory\n")
		writeCgroupFile(t, root, "proc/self/cgroup", "0::/docker/abcdef\n")
		writeCgroupFile(
			t,
			root,
			"proc/self/mountinfo",
			cgroupMountInfoLine("29 23 0:26", "/docker/abcdef", "/sys/fs/cgroup", "cgroup2", "rw"),
		)
		writeCgroupFile(t, root, "sys/fs/cgroup/memory.max", "536870912\n")
		writeCgroupFile(t, root, "sys/fs/cgroup/memory.current", "1048576\n")

		reader := &cgroupReader{root: root}

		limit, used, ok := reader.memorySample(64 * 1 << 30)
		require.True(t, ok, "The mount root must provide a complete memory sample")
		assert.Equal(t, uint64(536870912), limit, "The mount-root limit applies")
		assert.Equal(t, uint64(1048576), used, "Raw usage is kept when memory.stat is absent")
	})
}

func TestCgroupV1(t *testing.T) {
	t.Run("LimitedContainerReportsQuotaAndLimit", func(t *testing.T) {
		root := t.TempDir()
		writeCgroupFile(t, root, "proc/self/cgroup", "4:memory:/docker/abc\n3:cpu,cpuacct:/docker/abc\n")
		writeV1MountInfo(t, root, "/docker/abc", "/docker/abc")
		writeCgroupFile(t, root, "sys/fs/cgroup/memory/memory.limit_in_bytes", "536870912\n")
		writeCgroupFile(t, root, "sys/fs/cgroup/memory/memory.usage_in_bytes", "268435456\n")
		writeCgroupFile(t, root, "sys/fs/cgroup/memory/memory.stat", "cache 100\ntotal_inactive_file 134217728\n")
		writeCgroupFile(t, root, "sys/fs/cgroup/cpu,cpuacct/cpu.cfs_quota_us", "150000\n")
		writeCgroupFile(t, root, "sys/fs/cgroup/cpu,cpuacct/cpu.cfs_period_us", "100000\n")
		writeCgroupFile(t, root, "sys/fs/cgroup/cpu,cpuacct/cpuacct.usage", "250000000\n")

		reader := &cgroupReader{root: root}

		scope, limited := reader.cpuScope(8)
		require.True(t, limited, "A positive cfs quota must reduce host capacity")
		assert.InDelta(t, 1.5, scope.capacity, 0.0001, "Quota 150000/100000 is one and a half cores")

		usage, ok := scope.sample()
		require.True(t, ok, "The cpuacct.usage counter must be readable")
		assert.Equal(t, 250*time.Millisecond, usage, "The cpuacct.usage counter is in nanoseconds")

		limit, used, ok := reader.memorySample(64 * 1 << 30)
		require.True(t, ok, "A real memory limit and usage must form a sample")
		assert.Equal(t, uint64(536870912), limit, "The memory.limit_in_bytes value is the limit")
		assert.Equal(t, uint64(268435456-134217728), used, "Working set subtracts total_inactive_file")
	})

	t.Run("UnlimitedContainerReportsNoLimits", func(t *testing.T) {
		root := t.TempDir()
		writeCgroupFile(t, root, "proc/self/cgroup", "4:memory:/\n3:cpu,cpuacct:/\n")
		writeV1MountInfo(t, root, "/", "/")
		writeCgroupFile(t, root, "sys/fs/cgroup/memory/memory.limit_in_bytes", "9223372036854771712\n")
		writeCgroupFile(t, root, "sys/fs/cgroup/cpu,cpuacct/cpu.cfs_quota_us", "-1\n")
		writeCgroupFile(t, root, "sys/fs/cgroup/cpu,cpuacct/cpu.cfs_period_us", "100000\n")

		reader := &cgroupReader{root: root}

		scope, limited := reader.cpuScope(8)
		assert.False(t, limited, "A -1 cfs quota does not reduce host capacity")
		assert.Equal(t, 8.0, scope.capacity, "An unlimited cgroup keeps host CPU capacity")

		_, _, ok := reader.memorySample(64 * 1 << 30)
		assert.False(t, ok, "The page-aligned MaxInt64 sentinel cannot produce a limited sample")
	})

	t.Run("VisibleCgroupPathIsPreferredOverMountRoot", func(t *testing.T) {
		root := t.TempDir()
		writeCgroupFile(t, root, "proc/self/cgroup", "4:memory:/docker/abc\n")
		writeCgroupFile(
			t,
			root,
			"proc/self/mountinfo",
			cgroupMountInfoLine("30 23 0:27", "/", "/sys/fs/cgroup/memory", "cgroup", "rw,memory"),
		)
		writeCgroupFile(t, root, "sys/fs/cgroup/memory/memory.limit_in_bytes", "9223372036854771712\n")
		writeCgroupFile(t, root, "sys/fs/cgroup/memory/docker/abc/memory.limit_in_bytes", "536870912\n")
		writeCgroupFile(t, root, "sys/fs/cgroup/memory/docker/abc/memory.usage_in_bytes", "1048576\n")

		reader := &cgroupReader{root: root}

		limit, used, ok := reader.memorySample(64 * 1 << 30)
		require.True(t, ok, "The process's own cgroup dir must form the sample when visible")
		assert.Equal(t, uint64(536870912), limit, "The leaf limit applies, not the mount root sentinel")
		assert.Equal(t, uint64(1048576), used, "Usage must come from the same visible cgroup")
	})
}

func TestCgroupMountRootTranslation(t *testing.T) {
	root := t.TempDir()
	writeCgroupFile(t, root, "proc/self/cgroup", "0::/kubepods/pod1/ctr1\n")
	writeCgroupFile(
		t,
		root,
		"proc/self/mountinfo",
		cgroupMountInfoLine("29 23 0:26", "/kubepods/pod1", "/sys/fs/cgroup", "cgroup2", "rw"),
	)
	writeCgroupFile(t, root, "sys/fs/cgroup/ctr1/memory.max", "536870912\n")
	writeCgroupFile(t, root, "sys/fs/cgroup/ctr1/memory.current", "1048576\n")

	reader := &cgroupReader{root: root}
	limit, used, ok := reader.memorySample(64 * 1 << 30)
	require.True(t, ok, "The translated cgroup files must be readable")
	assert.Equal(t, uint64(536870912), limit, "The translated memory limit must be returned")
	assert.Equal(t, uint64(1048576), used, "The translated memory usage must be returned")
}

func TestMemorySampleUsesLimitingAncestorScope(t *testing.T) {
	root := t.TempDir()
	writeCgroupFile(t, root, "proc/self/cgroup", "0::/kubepods/pod1/ctr1\n")
	writeCgroupFile(t, root, "proc/self/mountinfo", cgroupMountInfoLine("29 23 0:26", "/", "/sys/fs/cgroup", "cgroup2", "rw"))
	writeCgroupFile(t, root, "sys/fs/cgroup/kubepods/pod1/ctr1/memory.max", "1073741824\n")
	writeCgroupFile(t, root, "sys/fs/cgroup/kubepods/pod1/ctr1/memory.current", "104857600\n")
	writeCgroupFile(t, root, "sys/fs/cgroup/kubepods/pod1/ctr1/memory.stat", "inactive_file 0\n")
	writeCgroupFile(t, root, "sys/fs/cgroup/kubepods/pod1/memory.max", "536870912\n")
	writeCgroupFile(t, root, "sys/fs/cgroup/kubepods/pod1/memory.current", "471859200\n")
	writeCgroupFile(t, root, "sys/fs/cgroup/kubepods/pod1/memory.stat", "inactive_file 52428800\n")

	reader := &cgroupReader{root: root}
	limit, used, ok := reader.memorySample(64 * 1 << 30)
	require.True(t, ok, "The ancestor limit and usage must form a sample")
	assert.Equal(t, uint64(536870912), limit, "The tighter pod limit must win")
	assert.Equal(t, uint64(419430400), used, "Usage must come from the limiting pod scope, not the container leaf")

	_, _, ok = reader.memorySample(256 * 1 << 20)
	assert.False(t, ok, "A cgroup limit at or above host memory must not replace the host view")
}

func TestMemorySampleEqualLimitsUsesOutermostScope(t *testing.T) {
	root := t.TempDir()
	writeCgroupFile(t, root, "proc/self/cgroup", "0::/pod/ctr\n")
	writeCgroupFile(t, root, "proc/self/mountinfo", cgroupMountInfoLine("29 23 0:26", "/", "/sys/fs/cgroup", "cgroup2", "rw"))
	writeCgroupFile(t, root, "sys/fs/cgroup/pod/ctr/memory.max", "536870912\n")
	writeCgroupFile(t, root, "sys/fs/cgroup/pod/ctr/memory.current", "104857600\n")
	writeCgroupFile(t, root, "sys/fs/cgroup/pod/memory.max", "536870912\n")
	writeCgroupFile(t, root, "sys/fs/cgroup/pod/memory.current", "419430400\n")

	limit, used, ok := (&cgroupReader{root: root}).memorySample(64 * 1 << 30)
	require.True(t, ok, "Equal parent and leaf limits must form a sample")
	assert.Equal(t, uint64(536870912), limit, "The shared limit remains unchanged")
	assert.Equal(t, uint64(419430400), used, "The outer shared scope must supply usage when equal limits nest")
}

func TestMemorySampleRejectsMissingScopedUsage(t *testing.T) {
	root := newV2Root(t)
	writeCgroupFile(t, root, "sys/fs/cgroup/memory.max", "536870912\n")

	_, _, ok := (&cgroupReader{root: root}).memorySample(64 * 1 << 30)
	assert.False(t, ok, "A limit without usage from the same scope must not produce a sample")
}

func TestCgroupCPUScope(t *testing.T) {
	t.Run("AncestorQuotaUsesAncestorCounter", func(t *testing.T) {
		root := t.TempDir()
		writeCgroupFile(t, root, "proc/self/cgroup", "0::/pod/ctr\n")
		writeCgroupFile(t, root, "proc/self/mountinfo", cgroupMountInfoLine("29 23 0:26", "/", "/sys/fs/cgroup", "cgroup2", "rw"))
		writeCgroupFile(t, root, "sys/fs/cgroup/pod/ctr/cpu.max", "max 100000\n")
		writeCgroupFile(t, root, "sys/fs/cgroup/pod/ctr/cpu.stat", "usage_usec 100000\n")
		writeCgroupFile(t, root, "sys/fs/cgroup/pod/ctr/cpuset.cpus.effective", "0-7\n")
		writeCgroupFile(t, root, "sys/fs/cgroup/pod/cpu.max", "50000 100000\n")
		writeCgroupFile(t, root, "sys/fs/cgroup/pod/cpu.stat", "usage_usec 500000\n")

		scope, limited := (&cgroupReader{root: root}).cpuScope(8)
		require.True(t, limited, "The ancestor quota must reduce host capacity")
		assert.InDelta(t, 0.5, scope.capacity, 0.0001, "Capacity must use the tightest quota")

		before, ok := scope.sample()
		require.True(t, ok, "The selected ancestor counter must be readable")
		assert.Equal(t, 500*time.Millisecond, before, "Usage must come from the quota-owning ancestor")

		writeCgroupFile(t, root, "sys/fs/cgroup/pod/cpu.stat", "usage_usec 600000\n")
		writeCgroupFile(t, root, "proc/self/cgroup", "0::/moved/elsewhere\n")

		after, ok := scope.sample()
		require.True(t, ok, "The fixed scope must remain readable after the process path changes")
		assert.Equal(t, 100*time.Millisecond, after-before, "Before and after must sample the same quota-owning counter")
	})

	t.Run("EqualQuotasUseOutermostCounter", func(t *testing.T) {
		root := t.TempDir()
		writeCgroupFile(t, root, "proc/self/cgroup", "0::/pod/ctr\n")
		writeCgroupFile(t, root, "proc/self/mountinfo", cgroupMountInfoLine("29 23 0:26", "/", "/sys/fs/cgroup", "cgroup2", "rw"))
		writeCgroupFile(t, root, "sys/fs/cgroup/pod/ctr/cpu.max", "50000 100000\n")
		writeCgroupFile(t, root, "sys/fs/cgroup/pod/ctr/cpu.stat", "usage_usec 100000\n")
		writeCgroupFile(t, root, "sys/fs/cgroup/pod/cpu.max", "50000 100000\n")
		writeCgroupFile(t, root, "sys/fs/cgroup/pod/cpu.stat", "usage_usec 500000\n")

		scope, limited := (&cgroupReader{root: root}).cpuScope(8)
		require.True(t, limited, "Equal nested quotas must still reduce host capacity")
		assert.InDelta(t, 0.5, scope.capacity, 0.0001, "The shared quota must set capacity")

		usage, ok := scope.sample()
		require.True(t, ok, "The outer shared counter must be readable")
		assert.Equal(t, 500*time.Millisecond, usage, "The outer counter must win when equal quotas nest")
	})

	t.Run("CpusetOnlyLimitsCapacity", func(t *testing.T) {
		root := newV2Root(t)
		writeCgroupFile(t, root, "sys/fs/cgroup/cpu.max", "max 100000\n")
		writeCgroupFile(t, root, "sys/fs/cgroup/cpu.stat", "usage_usec 250000\n")
		writeCgroupFile(t, root, "sys/fs/cgroup/cpuset.cpus.effective", "2-3\n")

		scope, limited := (&cgroupReader{root: root}).cpuScope(8)
		require.True(t, limited, "A two-CPU cpuset must reduce host capacity")
		assert.Equal(t, 2.0, scope.capacity, "Capacity must equal the effective cpuset size")

		usage, ok := scope.sample()
		require.True(t, ok, "Cpuset-only sampling must use the leaf CPU counter")
		assert.Equal(t, 250*time.Millisecond, usage, "The leaf CPU counter must be sampled")
	})

	t.Run("QuotaAboveHostDoesNotInflateCapacity", func(t *testing.T) {
		root := newV2Root(t)
		writeCgroupFile(t, root, "sys/fs/cgroup/cpu.max", "800000 100000\n")
		writeCgroupFile(t, root, "sys/fs/cgroup/cpu.stat", "usage_usec 250000\n")
		writeCgroupFile(t, root, "sys/fs/cgroup/cpuset.cpus.effective", "0-7\n")

		scope, limited := (&cgroupReader{root: root}).cpuScope(4)
		assert.False(t, limited, "A quota above host capacity does not constrain the process")
		assert.Equal(t, 4.0, scope.capacity, "Capacity must remain capped at the host logical CPU count")
	})

	t.Run("MissingUsageCounterRejectsSample", func(t *testing.T) {
		root := newV2Root(t)
		writeCgroupFile(t, root, "sys/fs/cgroup/cpu.max", "50000 100000\n")

		scope, limited := (&cgroupReader{root: root}).cpuScope(8)
		require.True(t, limited, "A readable quota must still resolve its capacity")
		assert.InDelta(t, 0.5, scope.capacity, 0.0001, "The quota must be retained")

		_, ok := scope.sample()
		assert.False(t, ok, "A missing usage counter must not produce a mixed-scope sample")
	})

	t.Run("SeparateV1CPUAndAccountingHierarchiesRejectSample", func(t *testing.T) {
		root := t.TempDir()
		writeCgroupFile(t, root, "proc/self/cgroup", "3:cpu:/ctr\n2:cpuacct:/ctr\n")

		mountInfo := cgroupMountInfoLine("30 23 0:27", "/", "/sys/fs/cgroup/cpu", "cgroup", "rw,cpu") +
			cgroupMountInfoLine("31 23 0:28", "/", "/sys/fs/cgroup/cpuacct", "cgroup", "rw,cpuacct")
		writeCgroupFile(t, root, "proc/self/mountinfo", mountInfo)
		writeCgroupFile(t, root, "sys/fs/cgroup/cpu/ctr/cpu.cfs_quota_us", "50000\n")
		writeCgroupFile(t, root, "sys/fs/cgroup/cpu/ctr/cpu.cfs_period_us", "100000\n")
		writeCgroupFile(t, root, "sys/fs/cgroup/cpuacct/ctr/cpuacct.usage", "250000000\n")

		scope, limited := (&cgroupReader{root: root}).cpuScope(8)
		require.True(t, limited, "The v1 quota must still resolve capacity")
		assert.InDelta(t, 0.5, scope.capacity, 0.0001, "The v1 quota must set capacity")

		_, ok := scope.sample()
		assert.False(t, ok, "Separate hierarchies must not be combined into one CPU sample")
	})

	t.Run("SeparateV1CPUSetAndAccountingHierarchiesRejectSample", func(t *testing.T) {
		root := t.TempDir()
		writeCgroupFile(t, root, "proc/self/cgroup", "4:cpuset:/ctr\n2:cpuacct:/ctr\n")

		mountInfo := cgroupMountInfoLine("30 23 0:27", "/", "/sys/fs/cgroup/cpuset", "cgroup", "rw,cpuset") +
			cgroupMountInfoLine("31 23 0:28", "/", "/sys/fs/cgroup/cpuacct", "cgroup", "rw,cpuacct")
		writeCgroupFile(t, root, "proc/self/mountinfo", mountInfo)
		writeCgroupFile(t, root, "sys/fs/cgroup/cpuset/ctr/cpuset.effective_cpus", "2-3\n")
		writeCgroupFile(t, root, "sys/fs/cgroup/cpuacct/ctr/cpuacct.usage", "250000000\n")

		scope, limited := (&cgroupReader{root: root}).cpuScope(8)
		require.True(t, limited, "The v1 cpuset must still resolve capacity")
		assert.Equal(t, 2.0, scope.capacity, "The v1 cpuset must set capacity")

		_, ok := scope.sample()
		assert.False(t, ok, "Separate hierarchies must not be combined into one cpuset sample")
	})
}

func TestHybridControllersResolveIndependently(t *testing.T) {
	root := t.TempDir()
	writeCgroupFile(t, root, "proc/self/cgroup", "0::/\n3:cpu,cpuacct:/\n")

	mountInfo := cgroupMountInfoLine("29 23 0:26", "/", "/sys/fs/cgroup/unified", "cgroup2", "rw") +
		cgroupMountInfoLine("31 23 0:28", "/", "/sys/fs/cgroup/cpu,cpuacct", "cgroup", "rw,cpu,cpuacct")
	writeCgroupFile(t, root, "proc/self/mountinfo", mountInfo)
	writeCgroupFile(t, root, "sys/fs/cgroup/unified/memory.max", "536870912\n")
	writeCgroupFile(t, root, "sys/fs/cgroup/unified/memory.current", "1048576\n")
	writeCgroupFile(t, root, "sys/fs/cgroup/cpu,cpuacct/cpu.cfs_quota_us", "150000\n")
	writeCgroupFile(t, root, "sys/fs/cgroup/cpu,cpuacct/cpu.cfs_period_us", "100000\n")
	writeCgroupFile(t, root, "sys/fs/cgroup/cpu,cpuacct/cpuacct.usage", "250000000\n")

	reader := &cgroupReader{root: root}
	limit, used, ok := reader.memorySample(64 * 1 << 30)
	require.True(t, ok, "The v2 memory controller must resolve in hybrid mode")
	assert.Equal(t, uint64(536870912), limit, "The v2 memory limit must be returned")
	assert.Equal(t, uint64(1048576), used, "The v2 memory usage must be returned")

	scope, limited := reader.cpuScope(8)
	require.True(t, limited, "The v1 CPU controller must resolve in hybrid mode")
	assert.InDelta(t, 1.5, scope.capacity, 0.0001, "The v1 combined quota must set CPU capacity")

	usage, ok := scope.sample()
	require.True(t, ok, "The v1 combined cpuacct counter must be readable")
	assert.Equal(t, 250*time.Millisecond, usage, "The v1 cpuacct usage must be returned")
}

func TestCgroupWithoutSupport(t *testing.T) {
	reader := &cgroupReader{root: t.TempDir()}

	scope, limited := reader.cpuScope(8)
	assert.False(t, limited, "No cgroup tree means no CPU constraint")
	assert.Equal(t, 8.0, scope.capacity, "No cgroup tree keeps host CPU capacity")
	_, ok := scope.sample()
	assert.False(t, ok, "No cgroup tree means no CPU usage sample")

	_, _, ok = reader.memorySample(64 * 1 << 30)
	assert.False(t, ok, "No cgroup tree means no memory sample")
}

func TestParseCPUSet(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  int
		ok    bool
	}{
		{name: "SparseRanges", value: "0-2,4,6-7", want: 6, ok: true},
		{name: "SingleCPU", value: "3", want: 1, ok: true},
		{name: "Empty", value: "", ok: false},
		{name: "Overlapping", value: "0-2,2-3", ok: false},
		{name: "BackwardsRange", value: "3-1", ok: false},
		{name: "Garbage", value: "cpu0", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseCPUSet(tt.value)
			assert.Equal(t, tt.ok, ok, "Parse outcome for %q should match", tt.value)
			assert.Equal(t, tt.want, got, "CPU count for %q should match", tt.value)
		})
	}
}

func TestParseV2CPUMax(t *testing.T) {
	tests := []struct {
		name string
		line string
		want float64
		ok   bool
	}{
		{name: "HalfCore", line: "50000 100000", want: 0.5, ok: true},
		{name: "TwoCores", line: "200000 100000", want: 2, ok: true},
		{name: "Unlimited", line: "max 100000", ok: false},
		{name: "Empty", line: "", ok: false},
		{name: "MissingPeriod", line: "50000", ok: false},
		{name: "ZeroPeriod", line: "50000 0", ok: false},
		{name: "Garbage", line: "abc def", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseV2CPUMax(tt.line)
			assert.Equal(t, tt.ok, ok, "Parse outcome for %q should match", tt.line)

			if tt.ok {
				assert.InDelta(t, tt.want, got, 0.0001, "Core count for %q should match", tt.line)
			}
		})
	}
}

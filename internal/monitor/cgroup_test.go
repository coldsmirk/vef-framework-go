package monitor

import (
	"os"
	"path/filepath"
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

// newV2Root fabricates a cgroup v2 unified hierarchy whose process cgroup is
// the mount root — the layout seen inside a container with a private cgroup
// namespace.
func newV2Root(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	writeCgroupFile(t, root, "sys/fs/cgroup/cgroup.controllers", "cpu memory\n")
	writeCgroupFile(t, root, "proc/self/cgroup", "0::/\n")

	return root
}

func TestCgroupVersionDetection(t *testing.T) {
	t.Run("V2WhenUnifiedControllersExist", func(t *testing.T) {
		reader := &cgroupReader{root: newV2Root(t)}
		assert.Equal(t, cgroupV2, reader.version(), "The cgroup.controllers file marks the v2 unified hierarchy")
	})

	t.Run("V1WhenOnlyControllerDirsExist", func(t *testing.T) {
		root := t.TempDir()
		writeCgroupFile(t, root, "sys/fs/cgroup/memory/memory.limit_in_bytes", "536870912\n")

		reader := &cgroupReader{root: root}
		assert.Equal(t, cgroupV1, reader.version(), "A memory controller dir without cgroup.controllers marks v1")
	})

	t.Run("NoneOnHostsWithoutCgroups", func(t *testing.T) {
		reader := &cgroupReader{root: t.TempDir()}
		assert.Equal(t, cgroupNone, reader.version(), "An empty tree means no cgroup support")
	})
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

		quota, ok := reader.cpuQuota()
		require.True(t, ok, "A real cpu.max quota must be detected")
		assert.InDelta(t, 0.5, quota, 0.0001, "Quota 50000/100000 is half a core")

		usage, ok := reader.cpuUsage()
		require.True(t, ok, "The cpu.stat usage_usec counter must be readable")
		assert.Equal(t, 250*time.Millisecond, usage, "The usage_usec value converts to a duration")

		limit, ok := reader.memoryLimit()
		require.True(t, ok, "A numeric memory.max must be detected")
		assert.Equal(t, uint64(536870912), limit, "The memory.max value is the limit in bytes")

		used, ok := reader.memoryUsage()
		require.True(t, ok, "The memory.current counter must be readable")
		assert.Equal(t, uint64(268435456-134217728), used, "Working set subtracts the inactive file cache")
	})

	t.Run("UnlimitedContainerReportsNoLimits", func(t *testing.T) {
		root := newV2Root(t)
		writeCgroupFile(t, root, "sys/fs/cgroup/cpu.max", "max 100000\n")
		writeCgroupFile(t, root, "sys/fs/cgroup/memory.max", "max\n")

		reader := &cgroupReader{root: root}

		_, ok := reader.cpuQuota()
		assert.False(t, ok, "A cpu.max of \"max\" means unlimited")

		_, ok = reader.memoryLimit()
		assert.False(t, ok, "A memory.max of \"max\" means unlimited")
	})

	t.Run("TightestAncestorLimitWins", func(t *testing.T) {
		root := t.TempDir()
		writeCgroupFile(t, root, "sys/fs/cgroup/cgroup.controllers", "cpu memory\n")
		writeCgroupFile(t, root, "proc/self/cgroup", "0::/kubepods/pod1/ctr1\n")
		// The leaf has no CPU limit and a loose memory limit; the pod level
		// carries the effective ones.
		writeCgroupFile(t, root, "sys/fs/cgroup/kubepods/pod1/ctr1/cpu.max", "max 100000\n")
		writeCgroupFile(t, root, "sys/fs/cgroup/kubepods/pod1/ctr1/memory.max", "1073741824\n")
		writeCgroupFile(t, root, "sys/fs/cgroup/kubepods/pod1/cpu.max", "200000 100000\n")
		writeCgroupFile(t, root, "sys/fs/cgroup/kubepods/pod1/memory.max", "536870912\n")

		reader := &cgroupReader{root: root}

		quota, ok := reader.cpuQuota()
		require.True(t, ok, "An ancestor quota must be honored when the leaf says max")
		assert.InDelta(t, 2.0, quota, 0.0001, "The pod-level 2-core quota is effective")

		limit, ok := reader.memoryLimit()
		require.True(t, ok, "Ancestor memory limits must be consulted")
		assert.Equal(t, uint64(536870912), limit, "The tightest limit along the chain wins")
	})

	t.Run("InvisibleLeafPathFallsBackToMountRoot", func(t *testing.T) {
		root := t.TempDir()
		writeCgroupFile(t, root, "sys/fs/cgroup/cgroup.controllers", "cpu memory\n")
		// /proc/self/cgroup carries a host-namespace path that does not exist
		// under this mount; the mount root is the container's own cgroup.
		writeCgroupFile(t, root, "proc/self/cgroup", "0::/docker/abcdef\n")
		writeCgroupFile(t, root, "sys/fs/cgroup/memory.max", "536870912\n")
		writeCgroupFile(t, root, "sys/fs/cgroup/memory.current", "1048576\n")

		reader := &cgroupReader{root: root}

		limit, ok := reader.memoryLimit()
		require.True(t, ok, "The mount root must be consulted when the leaf path is invisible")
		assert.Equal(t, uint64(536870912), limit, "The mount-root limit applies")

		used, ok := reader.memoryUsage()
		require.True(t, ok, "Usage must fall back to the mount root too")
		assert.Equal(t, uint64(1048576), used, "Raw usage is kept when memory.stat is absent")
	})
}

func TestCgroupV1(t *testing.T) {
	t.Run("LimitedContainerReportsQuotaAndLimit", func(t *testing.T) {
		root := t.TempDir()
		writeCgroupFile(t, root, "proc/self/cgroup", "4:memory:/docker/abc\n3:cpu,cpuacct:/docker/abc\n")
		// The container-namespace mounts expose the container's own subtree at
		// the controller roots; the /docker/abc paths are not visible.
		writeCgroupFile(t, root, "sys/fs/cgroup/memory/memory.limit_in_bytes", "536870912\n")
		writeCgroupFile(t, root, "sys/fs/cgroup/memory/memory.usage_in_bytes", "268435456\n")
		writeCgroupFile(t, root, "sys/fs/cgroup/memory/memory.stat", "cache 100\ntotal_inactive_file 134217728\n")
		writeCgroupFile(t, root, "sys/fs/cgroup/cpu/cpu.cfs_quota_us", "150000\n")
		writeCgroupFile(t, root, "sys/fs/cgroup/cpu/cpu.cfs_period_us", "100000\n")
		writeCgroupFile(t, root, "sys/fs/cgroup/cpuacct/cpuacct.usage", "250000000\n")

		reader := &cgroupReader{root: root}

		quota, ok := reader.cpuQuota()
		require.True(t, ok, "A positive cfs quota must be detected")
		assert.InDelta(t, 1.5, quota, 0.0001, "Quota 150000/100000 is one and a half cores")

		usage, ok := reader.cpuUsage()
		require.True(t, ok, "The cpuacct.usage counter must be readable")
		assert.Equal(t, 250*time.Millisecond, usage, "The cpuacct.usage counter is in nanoseconds")

		limit, ok := reader.memoryLimit()
		require.True(t, ok, "A real memory limit must be detected")
		assert.Equal(t, uint64(536870912), limit, "The memory.limit_in_bytes value is the limit")

		used, ok := reader.memoryUsage()
		require.True(t, ok, "The memory.usage_in_bytes counter must be readable")
		assert.Equal(t, uint64(268435456-134217728), used, "Working set subtracts total_inactive_file")
	})

	t.Run("UnlimitedContainerReportsNoLimits", func(t *testing.T) {
		root := t.TempDir()
		writeCgroupFile(t, root, "sys/fs/cgroup/memory/memory.limit_in_bytes", "9223372036854771712\n")
		writeCgroupFile(t, root, "sys/fs/cgroup/cpu/cpu.cfs_quota_us", "-1\n")
		writeCgroupFile(t, root, "sys/fs/cgroup/cpu/cpu.cfs_period_us", "100000\n")

		reader := &cgroupReader{root: root}

		_, ok := reader.cpuQuota()
		assert.False(t, ok, "A -1 cfs quota means unlimited")

		_, ok = reader.memoryLimit()
		assert.False(t, ok, "The page-aligned MaxInt64 sentinel means unlimited")
	})

	t.Run("VisibleCgroupPathIsPreferredOverMountRoot", func(t *testing.T) {
		root := t.TempDir()
		writeCgroupFile(t, root, "proc/self/cgroup", "4:memory:/docker/abc\n")
		writeCgroupFile(t, root, "sys/fs/cgroup/memory/memory.limit_in_bytes", "9223372036854771712\n")
		writeCgroupFile(t, root, "sys/fs/cgroup/memory/docker/abc/memory.limit_in_bytes", "536870912\n")

		reader := &cgroupReader{root: root}

		limit, ok := reader.memoryLimit()
		require.True(t, ok, "The process's own cgroup dir must win when visible")
		assert.Equal(t, uint64(536870912), limit, "The leaf limit applies, not the mount root sentinel")
	})
}

func TestCgroupWithoutSupport(t *testing.T) {
	reader := &cgroupReader{root: t.TempDir()}

	_, ok := reader.cpuQuota()
	assert.False(t, ok, "No cgroup tree means no CPU quota")

	_, ok = reader.cpuUsage()
	assert.False(t, ok, "No cgroup tree means no CPU usage")

	_, ok = reader.memoryLimit()
	assert.False(t, ok, "No cgroup tree means no memory limit")

	_, ok = reader.memoryUsage()
	assert.False(t, ok, "No cgroup tree means no memory usage")
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

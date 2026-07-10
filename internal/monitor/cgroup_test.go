package monitor

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// cgroupFixture describes a synthetic cgroup layout for tests.
type cgroupFixture struct {
	// procSelfCgroup is the /proc/self/cgroup content (e.g. "0::/" for v2).
	procSelfCgroup string
	// v2 maps file name -> content for a cgroup v2 unified leaf.
	v2 map[string]string
	// v1 maps controller -> (file name -> content) for cgroup v1 leaves.
	v1 map[string]map[string]string
}

// withCgroupFixture materializes the fixture on disk and points the cgroup
// readers at it (via the package vars), restoring everything on cleanup. It also
// forces cgroupReadable=true so the parsers run on a non-Linux CI machine.
func withCgroupFixture(t *testing.T, f cgroupFixture) {
	t.Helper()

	dir := t.TempDir()
	write := func(path, content string) {
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	}

	cgroupFile := filepath.Join(dir, "self-cgroup")
	write(cgroupFile, f.procSelfCgroup)

	var mountinfo string

	// cgroup v2: single unified leaf, referenced by the "0::<path>" line.
	v2Mount := filepath.Join(dir, "cgroup2")
	for name, content := range f.v2 {
		write(filepath.Join(v2Mount, name), content)
	}

	mountinfo += fmt.Sprintf("30 29 0:30 / %s rw - cgroup2 ", v2Mount)

	// cgroup v1: one leaf directory per controller.
	for controller, files := range f.v1 {
		ctrlMount := filepath.Join(dir, "v1", controller)
		for name, content := range files {
			write(filepath.Join(ctrlMount, name), content)
		}

		mountinfo += fmt.Sprintf("31 29 0:31 / %s rw - cgroup ", ctrlMount, controller)
	}

	mountinfoFile := filepath.Join(dir, "self-mountinfo")
	write(mountinfoFile, mountinfo)

	origCgroup, origMountinfo, origV2, origReadable := procSelfCgroup, procSelfMountinfo, cgroupV2DefaultMount, cgroupReadable
	procSelfCgroup = cgroupFile
	procSelfMountinfo = mountinfoFile
	cgroupV2DefaultMount = v2Mount
	cgroupReadable = true

	t.Cleanup(func() {
		procSelfCgroup, procSelfMountinfo, cgroupV2DefaultMount, cgroupReadable = origCgroup, origMountinfo, origV2, origReadable
	})
}

func TestCgroupRelPath(t *testing.T) {
	t.Run("v2 unified 0::path", func(t *testing.T) {
		withCgroupFixture(t, cgroupFixture{procSelfCgroup: "0::/kubepods/pod123/abc\n"})

		rel, v2, ok := cgroupRelPath("memory")
		require.True(t, ok)
		assert.True(t, v2)
		assert.Equal(t, "/kubepods/pod123/abc", rel)
	})

	t.Run("v1 nested controller path", func(t *testing.T) {
		withCgroupFixture(t, cgroupFixture{procSelfCgroup: "12:memory:/docker/deadbeef\n4:cpu,cpuacct:/docker/deadbeef\n"})

		rel, v2, ok := cgroupRelPath("memory")
		require.True(t, ok)
		assert.False(t, v2)
		assert.Equal(t, "/docker/deadbeef", rel)

		relCPU, _, okCPU := cgroupRelPath("cpu")
		require.True(t, okCPU)
		assert.Equal(t, "/docker/deadbeef", relCPU, "comma-joined controllers should match")
	})

	t.Run("missing controller", func(t *testing.T) {
		withCgroupFixture(t, cgroupFixture{procSelfCgroup: "12:memory:/x\n"})

		_, _, ok := cgroupRelPath("cpu")
		assert.False(t, ok)
	})
}

func TestCgroupMemoryLimit(t *testing.T) {
	t.Run("non-linux gate falls back", func(t *testing.T) {
		withCgroupFixture(t, cgroupFixture{
			procSelfCgroup: "0::/\n",
			v2:             map[string]string{"memory.max": "536870912\n", "memory.current": "1\n"},
		})

		cgroupReadable = false
		_, _, ok := cgroupMemoryLimit()
		assert.False(t, ok, "non-linux should not read cgroup")
	})

	t.Run("v2 nested leaf, working set excludes cache", func(t *testing.T) {
		withCgroupFixture(t, cgroupFixture{
			procSelfCgroup: "0::/\n",
			v2: map[string]string{
				"memory.max":     "536870912\n", // 512Mi
				"memory.current": "200000000\n",
				"memory.stat":    "inactive_file 50000000\nanon 1\n",
			},
		})

		limit, used, ok := cgroupMemoryLimit()
		require.True(t, ok)
		assert.Equal(t, uint64(536870912), limit)
		assert.Equal(t, uint64(150000000), used, "used = current - inactive_file")
	})

	t.Run("v2 max means unlimited", func(t *testing.T) {
		withCgroupFixture(t, cgroupFixture{
			procSelfCgroup: "0::/\n",
			v2:             map[string]string{"memory.max": "max\n", "memory.current": "1\n"},
		})

		_, _, ok := cgroupMemoryLimit()
		assert.False(t, ok)
	})

	t.Run("v1 leaf", func(t *testing.T) {
		withCgroupFixture(t, cgroupFixture{
			procSelfCgroup: "12:memory:/\n",
			v1: map[string]map[string]string{
				"memory": {
					"memory.limit_in_bytes": "268435456\n",
					"memory.usage_in_bytes": "100000000\n",
					"memory.stat":           "total_inactive_file 20000000\n",
				},
			},
		})

		limit, used, ok := cgroupMemoryLimit()
		require.True(t, ok)
		assert.Equal(t, uint64(268435456), limit)
		assert.Equal(t, uint64(80000000), used)
	})

	t.Run("v1 sentinel unlimited", func(t *testing.T) {
		withCgroupFixture(t, cgroupFixture{
			procSelfCgroup: "12:memory:/\n",
			v1:             map[string]map[string]string{"memory": {"memory.limit_in_bytes": "9223372036854771712\n"}},
		})

		_, _, ok := cgroupMemoryLimit()
		assert.False(t, ok)
	})

	t.Run("malformed falls back", func(t *testing.T) {
		withCgroupFixture(t, cgroupFixture{
			procSelfCgroup: "0::/\n",
			v2:             map[string]string{"memory.max": "not-a-number\n"},
		})

		_, _, ok := cgroupMemoryLimit()
		assert.False(t, ok)
	})
}

func TestCgroupCPUQuota(t *testing.T) {
	t.Run("v2 two cores", func(t *testing.T) {
		withCgroupFixture(t, cgroupFixture{
			procSelfCgroup: "0::/\n",
			v2:             map[string]string{"cpu.max": "200000 100000\n"},
		})

		cores, ok := cgroupCPUQuota()
		require.True(t, ok)
		assert.InDelta(t, 2.0, cores, 1e-9)
	})

	t.Run("v2 fractional", func(t *testing.T) {
		withCgroupFixture(t, cgroupFixture{
			procSelfCgroup: "0::/\n",
			v2:             map[string]string{"cpu.max": "150000 100000\n"},
		})

		cores, ok := cgroupCPUQuota()
		require.True(t, ok)
		assert.InDelta(t, 1.5, cores, 1e-9)
	})

	t.Run("v2 max unlimited", func(t *testing.T) {
		withCgroupFixture(t, cgroupFixture{
			procSelfCgroup: "0::/\n",
			v2:             map[string]string{"cpu.max": "max 100000\n"},
		})

		_, ok := cgroupCPUQuota()
		assert.False(t, ok)
	})

	t.Run("v2 period zero safe", func(t *testing.T) {
		withCgroupFixture(t, cgroupFixture{
			procSelfCgroup: "0::/\n",
			v2:             map[string]string{"cpu.max": "100000 0\n"},
		})

		_, ok := cgroupCPUQuota()
		assert.False(t, ok, "period 0 must not divide by zero")
	})

	t.Run("v1 four cores", func(t *testing.T) {
		withCgroupFixture(t, cgroupFixture{
			procSelfCgroup: "4:cpu,cpuacct:/\n",
			v1: map[string]map[string]string{
				"cpu": {"cpu.cfs_quota_us": "400000\n", "cpu.cfs_period_us": "100000\n"},
			},
		})

		cores, ok := cgroupCPUQuota()
		require.True(t, ok)
		assert.InDelta(t, 4.0, cores, 1e-9)
	})

	t.Run("v1 quota -1 unlimited", func(t *testing.T) {
		withCgroupFixture(t, cgroupFixture{
			procSelfCgroup: "4:cpu:/\n",
			v1:             map[string]map[string]string{"cpu": {"cpu.cfs_quota_us": "-1\n", "cpu.cfs_period_us": "100000\n"}},
		})

		_, ok := cgroupCPUQuota()
		assert.False(t, ok)
	})
}

func TestCgroupCPUUsageMicros(t *testing.T) {
	t.Run("v2 usage_usec", func(t *testing.T) {
		withCgroupFixture(t, cgroupFixture{
			procSelfCgroup: "0::/\n",
			v2:             map[string]string{"cpu.stat": "usage_usec 123456\nuser_usec 1\n"},
		})

		v, ok := cgroupCPUUsageMicros()
		require.True(t, ok)
		assert.Equal(t, uint64(123456), v)
	})

	t.Run("v1 cpuacct nanoseconds", func(t *testing.T) {
		withCgroupFixture(t, cgroupFixture{
			procSelfCgroup: "4:cpu,cpuacct:/\n",
			v1:             map[string]map[string]string{"cpuacct": {"cpuacct.usage": "1000000\n"}},
		})

		v, ok := cgroupCPUUsageMicros()
		require.True(t, ok)
		assert.Equal(t, uint64(1000), v, "1e6 ns == 1000 us")
	})
}

func TestSubClamp(t *testing.T) {
	assert.Equal(t, uint64(0), subClamp(5, 10), "underflow clamps to 0")
	assert.Equal(t, uint64(5), subClamp(10, 5))
}

// TestCgroupMemoryAncestorLimit verifies that a limit set on a parent cgroup
// (e.g. a Kubernetes pod above the container) is honored: the effective limit is
// the smallest along the path, while usage stays the leaf's own working set.
func TestCgroupMemoryAncestorLimit(t *testing.T) {
	dir := t.TempDir()
	write := func(rel, content string) {
		full := filepath.Join(dir, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, []byte(content), 0o644))
	}

	// Leaf /pod/ctr has a 1Gi limit; the parent /pod has the smaller 512Mi limit
	// (the effective one); the root is unlimited.
	write("pod/ctr/memory.max", "1073741824\n")
	write("pod/ctr/memory.current", "300000000\n")
	write("pod/ctr/memory.stat", "inactive_file 50000000\n")
	write("pod/memory.max", "536870912\n")
	write("memory.max", "max\n")
	write("self-cgroup", "0::/pod/ctr\n")
	write("self-mountinfo", fmt.Sprintf("30 29 0:30 / %s rw - cgroup2 ", dir))

	origCgroup, origMountinfo, origV2, origReadable := procSelfCgroup, procSelfMountinfo, cgroupV2DefaultMount, cgroupReadable
	procSelfCgroup = filepath.Join(dir, "self-cgroup")
	procSelfMountinfo = filepath.Join(dir, "self-mountinfo")
	cgroupV2DefaultMount = dir
	cgroupReadable = true
	t.Cleanup(func() {
		procSelfCgroup, procSelfMountinfo, cgroupV2DefaultMount, cgroupReadable = origCgroup, origMountinfo, origV2, origReadable
	})

	limit, used, ok := cgroupMemoryLimit()
	require.True(t, ok)
	assert.Equal(t, uint64(536870912), limit, "effective limit is the parent's smaller 512Mi")
	assert.Equal(t, uint64(250000000), used, "usage is the leaf working set (current - inactive_file)")
}

// TestCgroupMemoryUsageClampsBelowZero covers the inactive_file > current edge:
// the working set clamps to 0 rather than underflowing (#15).
func TestCgroupMemoryUsageClampsBelowZero(t *testing.T) {
	withCgroupFixture(t, cgroupFixture{
		procSelfCgroup: "0::/\n",
		v2: map[string]string{
			"memory.max":     "536870912\n",
			"memory.current": "10000\n",
			"memory.stat":    "inactive_file 99999999\n", // > current
		},
	})

	_, used, ok := cgroupMemoryLimit()
	require.True(t, ok)
	assert.Equal(t, uint64(0), used, "inactive_file > current clamps working set to 0")
}

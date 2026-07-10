package monitor

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// cgroup.go reads the CURRENT container's own CPU/memory limits from its cgroup.
//
// Unlike disk (where we want the host's real filesystems), CPU/memory limits are
// a property of *this* process's own cgroup, so everything here is read from the
// process's own /proc and /sys — never from a bind-mounted host tree.
//
// The important correctness detail is locating the process's *leaf* cgroup rather
// than the hierarchy root. Reading /sys/fs/cgroup/memory.max blindly only works
// when the leaf happens to be the root (modern Docker with a private cgroup
// namespace reports "0::/"). Under cgroup v1, systemd slices, or Kubernetes with
// a host cgroup namespace the process sits in a nested path such as
// /sys/fs/cgroup/.../kubepods/pod.../<id>, and the root files read as "max"
// (unlimited) — silently hiding the real limit. We therefore resolve the leaf
// from /proc/self/cgroup (the path) + /proc/self/mountinfo (the mount point),
// exactly as the Go runtime and container tooling do.
//
// Every reader fails safe: on non-Linux, missing/unreadable files, malformed
// content, or "no limit", it returns ok=false and the caller keeps host metrics.
// Nothing here panics; robustness of the bare-metal path is the priority.

// These are variables (not constants) only so tests can point them at fixtures.
var (
	procSelfCgroup    = "/proc/self/cgroup"
	procSelfMountinfo = "/proc/self/mountinfo"
	// cgroupV2DefaultMount is the conventional unified-hierarchy mount point,
	// used when mountinfo does not reveal an explicit cgroup2 mount.
	cgroupV2DefaultMount = "/sys/fs/cgroup"
)

// cgroupReadable is true only on Linux, the sole platform with a cgroup
// filesystem. cgroups are a Linux-specific kernel feature — macOS, the *BSDs,
// Solaris and Windows have no cgroup hierarchy — so on every other platform the
// readers short-circuit and callers fall back to host metrics. It is a variable
// so tests can exercise the parsers on a non-Linux CI machine.
var cgroupReadable = runtime.GOOS == "linux"

// cgroupUnlimited is the threshold above which a cgroup v1 byte limit is treated
// as "no limit": the kernel reports unlimited as a huge page-aligned sentinel
// (e.g. 0x7FFFFFFFFFFFF000), far above any real machine's RAM.
const cgroupUnlimited = uint64(1) << 62

// leafDir resolves the absolute directory of the process's leaf cgroup for the
// given v1 controller (ignored under v2). ok is false when it cannot be located.
//
//   - v2 (unified): /proc/self/cgroup has a single "0::<path>" line; the files
//     live at <cgroup2-mount><path>.
//   - v1 (legacy):  /proc/self/cgroup has "<id>:<controllers>:<path>" lines; for
//     the requested controller the files live at <controller-mount><path>.
func leafDir(controller string) (string, bool) {
	rel, v2, ok := cgroupRelPath(controller)
	if !ok {
		return "", false
	}

	mount, ok := cgroupMount(controller, v2)
	if !ok {
		return "", false
	}

	return filepath.Join(mount, rel), true
}

// cgroupRelPath returns the in-hierarchy path for the controller and whether the
// system is cgroup v2 (unified).
func cgroupRelPath(controller string) (rel string, v2, ok bool) {
	b, err := os.ReadFile(procSelfCgroup)
	if err != nil {
		return "", false, false
	}

	var v1Path string

	for line := range strings.SplitSeq(strings.TrimSpace(string(b)), "\n") {
		// Format: "<hierarchy-id>:<controllers>:<path>".
		parts := strings.SplitN(line, ":", 3)
		if len(parts) != 3 {
			continue
		}

		// A "0::<path>" line is the cgroup v2 unified hierarchy.
		if parts[0] == "0" && parts[1] == "" {
			return parts[2], true, true
		}

		// v1: match the requested controller (controllers are comma-separated).
		for c := range strings.SplitSeq(parts[1], ",") {
			if c == controller {
				v1Path = parts[2]
			}
		}
	}

	if v1Path != "" {
		return v1Path, false, true
	}

	return "", false, false
}

// cgroupMount returns the mount point of the cgroup hierarchy backing the
// controller: the cgroup2 mount under v2, or the controller's cgroup mount under
// v1. Falls back to the conventional /sys/fs/cgroup for v2 when mountinfo lacks
// an explicit entry.
func cgroupMount(controller string, v2 bool) (string, bool) {
	b, err := os.ReadFile(procSelfMountinfo)
	if err != nil {
		if v2 {
			return cgroupV2DefaultMount, true
		}

		return "", false
	}

	for line := range strings.SplitSeq(string(b), "\n") {
		// mountinfo: "<...> <mountpoint> <...> - <fstype> <source> <superopts>".
		// Split on the " - " separator to reach the fstype/superopts tail.
		before, after, found := strings.Cut(line, " - ")
		if !found {
			continue
		}

		beforeFields := strings.Fields(before)

		afterFields := strings.Fields(after)
		if len(beforeFields) < 5 || len(afterFields) < 3 {
			continue
		}

		mountPoint, fsType, superOpts := beforeFields[4], afterFields[0], afterFields[2]

		if v2 && fsType == "cgroup2" {
			return mountPoint, true
		}

		// v1: the controller must appear in the super options (e.g. "rw,memory").
		if !v2 && fsType == "cgroup" {
			for opt := range strings.SplitSeq(superOpts, ",") {
				if opt == controller {
					return mountPoint, true
				}
			}
		}
	}

	if v2 {
		return cgroupV2DefaultMount, true
	}

	return "", false
}

// readTrimmed returns the trimmed file content and whether it was read non-empty.
func readTrimmed(path string) (string, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}

	s := strings.TrimSpace(string(b))
	if s == "" {
		return "", false
	}

	return s, true
}

// readUint reads a single unsigned integer from a cgroup file.
func readUint(path string) (uint64, bool) {
	s, ok := readTrimmed(path)
	if !ok {
		return 0, false
	}

	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, false
	}

	return v, true
}

// statField extracts a "<key> <value>" field from a cgroup stat file.
func statField(path, key string) (uint64, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}

	for line := range strings.SplitSeq(string(b), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == key {
			v, err := strconv.ParseUint(fields[1], 10, 64)
			if err != nil {
				return 0, false
			}

			return v, true
		}
	}

	return 0, false
}

// cgroupMemoryLimit returns the container memory limit and working-set usage in
// bytes. ok is false when no finite limit applies (fall back to host). Usage
// excludes reclaimable page cache (inactive_file) to match the kernel's OOM
// accounting, mirroring how `docker stats` reports memory.
func cgroupMemoryLimit() (limit, used uint64, ok bool) {
	if !cgroupReadable {
		return 0, 0, false
	}

	rel, v2, ok := cgroupRelPath("memory")
	if !ok {
		return 0, 0, false
	}

	mount, ok := cgroupMount("memory", v2)
	if !ok {
		return 0, 0, false
	}

	leaf := filepath.Join(mount, rel)

	// Memory limits are hierarchical: a process is bound by the smallest limit
	// anywhere between its leaf cgroup and the hierarchy root (e.g. a Kubernetes
	// pod-level limit set above the container's own cgroup). Walk the whole path
	// and take the minimum finite limit; usage stays the leaf's own working set.
	if v2 {
		lim, found := minLimitUpTree(leaf, mount, "memory.max", parseCgroupBytes)
		if !found {
			return 0, 0, false
		}

		cur, curOK := readUint(filepath.Join(leaf, "memory.current"))
		if !curOK {
			return 0, 0, false
		}

		return lim, leafWorkingSet(leaf, "memory.stat", "inactive_file", cur), true
	}

	lim, found := minLimitUpTree(leaf, mount, "memory.limit_in_bytes", parseCgroupBytes)
	if !found {
		return 0, 0, false
	}

	cur, curOK := readUint(filepath.Join(leaf, "memory.usage_in_bytes"))
	if !curOK {
		return 0, 0, false
	}

	return lim, leafWorkingSet(leaf, "memory.stat", "total_inactive_file", cur), true
}

// parseCgroupBytes parses a cgroup byte limit, rejecting "max" and the huge
// sentinel used for "unlimited".
func parseCgroupBytes(raw string) (uint64, bool) {
	if raw == "max" {
		return 0, false
	}

	v, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || v == 0 || v >= cgroupUnlimited {
		return 0, false
	}

	return v, true
}

// minLimitUpTree returns the smallest finite limit found in filename across the
// cgroup directories from leaf up to (and including) the hierarchy mount root.
func minLimitUpTree(leaf, mount, filename string, parse func(string) (uint64, bool)) (uint64, bool) {
	best, found := uint64(0), false

	for dir := leaf; ; {
		if raw, ok := readTrimmed(filepath.Join(dir, filename)); ok {
			if v, valid := parse(raw); valid && (!found || v < best) {
				best, found = v, true
			}
		}

		if dir == mount {
			break
		}

		parent := filepath.Dir(dir)
		if parent == dir || len(parent) < len(mount) {
			break
		}

		dir = parent
	}

	return best, found
}

// leafWorkingSet returns current usage minus the reclaimable page cache field,
// logging (at debug level) when the stat field is unreadable so the resulting
// over-count is diagnosable.
func leafWorkingSet(leaf, statFile, field string, current uint64) uint64 {
	inactive, ok := statField(filepath.Join(leaf, statFile), field)
	if !ok {
		logger.Debugf("cgroup: %s/%s %q unreadable; working set may include page cache", leaf, statFile, field)
	}

	return subClamp(current, inactive)
}

// cgroupCPUQuota returns the effective (possibly fractional) CPU core count from
// the CFS quota (quota/period). ok is false when no quota is set (unlimited) or
// on any parse error, so the caller keeps the host core count.
func cgroupCPUQuota() (float64, bool) {
	if !cgroupReadable {
		return 0, false
	}

	dir, ok := leafDir("cpu")
	if !ok {
		return 0, false
	}

	// cgroup v2: "cpu.max" is "<quota> <period>" or "max <period>".
	if raw, found := readTrimmed(filepath.Join(dir, "cpu.max")); found {
		fields := strings.Fields(raw)
		if len(fields) != 2 || fields[0] == "max" {
			return 0, false
		}

		quota, err1 := strconv.ParseFloat(fields[0], 64)

		period, err2 := strconv.ParseFloat(fields[1], 64)
		if err1 != nil || err2 != nil || quota <= 0 || period <= 0 {
			return 0, false
		}

		return quota / period, true
	}

	// cgroup v1: cpu.cfs_quota_us of -1 means unlimited.
	quotaRaw, ok1 := readTrimmed(filepath.Join(dir, "cpu.cfs_quota_us"))

	periodRaw, ok2 := readTrimmed(filepath.Join(dir, "cpu.cfs_period_us"))
	if !ok1 || !ok2 {
		return 0, false
	}

	quota, err1 := strconv.ParseInt(quotaRaw, 10, 64)

	period, err2 := strconv.ParseInt(periodRaw, 10, 64)
	if err1 != nil || err2 != nil || quota <= 0 || period <= 0 {
		return 0, false
	}

	return float64(quota) / float64(period), true
}

// cgroupCPUUsageMicros returns the cgroup's cumulative CPU time in microseconds.
// Two readings across an interval give the container's own CPU utilization.
func cgroupCPUUsageMicros() (uint64, bool) {
	if !cgroupReadable {
		return 0, false
	}

	// cgroup v2: "cpu.stat" under the unified leaf contains "usage_usec <n>".
	if dir, ok := leafDir("cpu"); ok {
		if v, ok := statField(filepath.Join(dir, "cpu.stat"), "usage_usec"); ok {
			return v, true
		}
	}

	// cgroup v1: cpuacct.usage is nanoseconds, under the cpuacct controller. Tried
	// independently of the cpu controller, which under v1 has no usage counter.
	if dir, ok := leafDir("cpuacct"); ok {
		if ns, ok := readUint(filepath.Join(dir, "cpuacct.usage")); ok {
			return ns / 1000, true
		}
	}

	return 0, false
}

// subClamp returns a-b, clamped at zero to avoid unsigned underflow.
func subClamp(a, b uint64) uint64 {
	if b > a {
		return 0
	}

	return a - b
}

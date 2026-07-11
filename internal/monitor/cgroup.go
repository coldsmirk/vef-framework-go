package monitor

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

// cgroupVersion identifies which cgroup hierarchy the process runs under.
type cgroupVersion int

const (
	cgroupNone cgroupVersion = iota
	cgroupV1
	cgroupV2
)

// v1UnlimitedThreshold marks a v1 memory.limit_in_bytes value as "no limit":
// an unconstrained cgroup reports a page-aligned MaxInt64 (0x7FFFFFFFFFFFF000),
// so anything this large cannot be a real limit.
const v1UnlimitedThreshold = uint64(1) << 62

// cgroupReader resolves the container resource limits and usage the current
// process runs under, supporting the cgroup v2 unified hierarchy and the v1
// legacy controllers. All methods are best-effort and stateless: on hosts
// without cgroups (macOS, Windows, or bare Linux) or without limits they
// report ok=false and the monitor keeps host-level metrics, and every call
// re-reads the filesystem so runtime limit changes (e.g. docker update) are
// picked up immediately.
type cgroupReader struct {
	// root is prepended to every filesystem path so tests can point the
	// reader at a fabricated /proc + /sys tree; production uses the empty
	// string, i.e. the real root.
	root string
}

func newCgroupReader() *cgroupReader {
	return new(cgroupReader)
}

func (r *cgroupReader) path(parts ...string) string {
	return filepath.Join(append([]string{r.root, "/"}, parts...)...)
}

func (r *cgroupReader) version() cgroupVersion {
	if _, err := os.Stat(r.path("sys/fs/cgroup/cgroup.controllers")); err == nil {
		return cgroupV2
	}

	if _, err := os.Stat(r.path("sys/fs/cgroup/memory")); err == nil {
		return cgroupV1
	}

	return cgroupNone
}

// cpuQuota returns the effective CPU limit in cores (0.5 = half a core).
// Under v2 the tightest quota along the ancestor chain wins, so a limit set on
// a pod cgroup or systemd slice is honored even when the leaf says "max".
func (r *cgroupReader) cpuQuota() (float64, bool) {
	switch r.version() {
	case cgroupV2:
		var best float64

		for _, dir := range r.v2Dirs() {
			quota, ok := parseV2CPUMax(readFirstLine(filepath.Join(dir, "cpu.max")))
			if ok && (best == 0 || quota < best) {
				best = quota
			}
		}

		return best, best > 0

	case cgroupV1:
		dir, ok := r.v1Dir("cpu")
		if !ok {
			return 0, false
		}

		quota, quotaOK := readInt(filepath.Join(dir, "cpu.cfs_quota_us"))
		period, periodOK := readInt(filepath.Join(dir, "cpu.cfs_period_us"))
		// A quota of -1 means unlimited.
		if !quotaOK || !periodOK || quota <= 0 || period <= 0 {
			return 0, false
		}

		return float64(quota) / float64(period), true

	default:
		return 0, false
	}
}

// cpuUsage returns the cumulative CPU time consumed by this cgroup, the
// counterpart to sampling /proc/stat on the host: two reads across a window
// yield the container's own CPU consumption.
func (r *cgroupReader) cpuUsage() (time.Duration, bool) {
	switch r.version() {
	case cgroupV2:
		usec, ok := readStatField(filepath.Join(r.v2Dirs()[0], "cpu.stat"), "usage_usec")
		if !ok {
			return 0, false
		}

		return time.Duration(usec) * time.Microsecond, true

	case cgroupV1:
		dir, ok := r.v1Dir("cpuacct")
		if !ok {
			return 0, false
		}

		nanos, ok := readInt(filepath.Join(dir, "cpuacct.usage"))
		if !ok {
			return 0, false
		}

		return time.Duration(nanos), true

	default:
		return 0, false
	}
}

// memoryLimit returns the effective memory limit in bytes. Under v2 the
// tightest limit along the ancestor chain wins.
func (r *cgroupReader) memoryLimit() (uint64, bool) {
	switch r.version() {
	case cgroupV2:
		var best uint64

		for _, dir := range r.v2Dirs() {
			line := readFirstLine(filepath.Join(dir, "memory.max"))
			if line == "" || line == "max" {
				continue
			}

			limit, err := strconv.ParseUint(line, 10, 64)
			if err != nil || limit == 0 {
				continue
			}

			if best == 0 || limit < best {
				best = limit
			}
		}

		return best, best > 0

	case cgroupV1:
		dir, ok := r.v1Dir("memory")
		if !ok {
			return 0, false
		}

		limit, ok := readUint(filepath.Join(dir, "memory.limit_in_bytes"))
		if !ok || limit == 0 || limit >= v1UnlimitedThreshold {
			return 0, false
		}

		return limit, true

	default:
		return 0, false
	}
}

// memoryUsage returns the cgroup's working-set memory: current usage minus
// inactive file cache, mirroring what container runtimes report and what the
// OOM killer pressures. Raw usage would count reclaimable page cache as used
// and make an I/O-heavy container look permanently full.
func (r *cgroupReader) memoryUsage() (uint64, bool) {
	switch r.version() {
	case cgroupV2:
		leaf := r.v2Dirs()[0]

		current, ok := readUint(filepath.Join(leaf, "memory.current"))
		if !ok {
			return 0, false
		}

		return subtractInactiveFile(current, filepath.Join(leaf, "memory.stat"), "inactive_file"), true

	case cgroupV1:
		dir, ok := r.v1Dir("memory")
		if !ok {
			return 0, false
		}

		usage, ok := readUint(filepath.Join(dir, "memory.usage_in_bytes"))
		if !ok {
			return 0, false
		}

		return subtractInactiveFile(usage, filepath.Join(dir, "memory.stat"), "total_inactive_file"), true

	default:
		return 0, false
	}
}

// v2Dirs returns the cgroup directories to consult, from the process's own
// cgroup up to the hierarchy root. When the leaf path from /proc/self/cgroup
// is not visible in this mount namespace (a host-namespace path inside a
// container whose runtime mounted only its own subtree), the mount root is
// the container's cgroup and is returned alone.
func (r *cgroupReader) v2Dirs() []string {
	mountRoot := r.path("sys/fs/cgroup")

	rel := r.selfCgroupPath("")
	if rel == "" || rel == "/" {
		return []string{mountRoot}
	}

	leaf := filepath.Join(mountRoot, rel)
	if !strings.HasPrefix(leaf, mountRoot+string(filepath.Separator)) {
		return []string{mountRoot}
	}

	if _, err := os.Stat(leaf); err != nil {
		return []string{mountRoot}
	}

	var dirs []string
	for dir := leaf; ; dir = filepath.Dir(dir) {
		dirs = append(dirs, dir)
		if dir == mountRoot {
			return dirs
		}
	}
}

// v1Dir resolves the directory of a v1 controller for this process: the
// controller mount joined with the process's cgroup path, falling back to the
// mount root when that path is not visible in this mount namespace (the
// common case inside a container, where the runtime mounts the container's
// own subtree at the controller root).
func (r *cgroupReader) v1Dir(controller string) (string, bool) {
	mount := r.path("sys/fs/cgroup", controller)
	if _, err := os.Stat(mount); err != nil {
		return "", false
	}

	rel := r.selfCgroupPath(controller)
	if rel != "" && rel != "/" {
		if dir := filepath.Join(mount, rel); strings.HasPrefix(dir, mount+string(filepath.Separator)) {
			if _, err := os.Stat(dir); err == nil {
				return dir, true
			}
		}
	}

	return mount, true
}

// selfCgroupPath returns this process's cgroup path for the given v1
// controller, or for the v2 unified hierarchy when controller is empty.
func (r *cgroupReader) selfCgroupPath(controller string) string {
	data, err := os.ReadFile(r.path("proc/self/cgroup"))
	if err != nil {
		return ""
	}

	for line := range strings.Lines(string(data)) {
		fields := strings.SplitN(strings.TrimSpace(line), ":", 3)
		if len(fields) != 3 {
			continue
		}

		if controller == "" {
			if fields[0] == "0" && fields[1] == "" {
				return fields[2]
			}

			continue
		}

		if slices.Contains(strings.Split(fields[1], ","), controller) {
			return fields[2]
		}
	}

	return ""
}

// parseV2CPUMax parses a cpu.max line ("<quota> <period>" in microseconds, or
// "max <period>" for unlimited) into a core count.
func parseV2CPUMax(line string) (float64, bool) {
	quotaField, periodField, found := strings.Cut(line, " ")
	if !found || quotaField == "max" {
		return 0, false
	}

	quota, quotaErr := strconv.ParseFloat(quotaField, 64)
	period, periodErr := strconv.ParseFloat(strings.TrimSpace(periodField), 64)

	if quotaErr != nil || periodErr != nil || quota <= 0 || period <= 0 {
		return 0, false
	}

	return quota / period, true
}

// subtractInactiveFile derives working-set memory from raw usage, flooring at
// zero when the reclaimable cache exceeds it. An unreadable stat file keeps
// the raw usage as the best remaining signal.
func subtractInactiveFile(usage uint64, statPath, field string) uint64 {
	inactive, ok := readStatField(statPath, field)
	if !ok {
		return usage
	}

	if cached := uint64(inactive); cached < usage {
		return usage - cached
	}

	return 0
}

// readFirstLine returns the first line of the file, trimmed, or "" when the
// file cannot be read.
func readFirstLine(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	line, _, _ := strings.Cut(string(data), "\n")

	return strings.TrimSpace(line)
}

func readInt(path string) (int64, bool) {
	value, err := strconv.ParseInt(readFirstLine(path), 10, 64)
	if err != nil {
		return 0, false
	}

	return value, true
}

func readUint(path string) (uint64, bool) {
	value, err := strconv.ParseUint(readFirstLine(path), 10, 64)
	if err != nil {
		return 0, false
	}

	return value, true
}

// readStatField extracts a named numeric field from a flat "name value" stat
// file such as cpu.stat or memory.stat.
func readStatField(path, field string) (int64, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}

	for line := range strings.Lines(string(data)) {
		name, valueField, found := strings.Cut(strings.TrimSpace(line), " ")
		if !found || name != field {
			continue
		}

		value, err := strconv.ParseInt(strings.TrimSpace(valueField), 10, 64)
		if err != nil {
			return 0, false
		}

		return value, true
	}

	return 0, false
}

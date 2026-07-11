package monitor

import (
	"os"
	"path"
	"path/filepath"
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

type cgroupLocation struct {
	version    cgroupVersion
	dir        string
	mountPoint string
}

func (l cgroupLocation) ancestors() []string {
	var dirs []string

	for dir := l.dir; ; dir = filepath.Dir(dir) {
		dirs = append(dirs, dir)
		if dir == l.mountPoint {
			return dirs
		}

		parent := filepath.Dir(dir)
		if parent == dir || !pathWithin(parent, l.mountPoint) {
			return dirs
		}
	}
}

type cgroupLayout struct {
	v2 *cgroupLocation
	v1 map[string]cgroupLocation
}

type cgroupMount struct {
	version     cgroupVersion
	root        string
	mountPoint  string
	controllers []string
}

// cgroupCPUScope fixes the usage counter selected alongside the effective CPU
// capacity. Reusing the scope for both ends of a sample prevents a runtime
// cgroup move or mount-layout change from mixing unrelated counters.
type cgroupCPUScope struct {
	capacity  float64
	usagePath string
	version   cgroupVersion
}

func (s cgroupCPUScope) sample() (time.Duration, bool) {
	switch s.version {
	case cgroupV2:
		usec, ok := readStatField(s.usagePath, "usage_usec")
		if !ok || usec > maxDurationNanos/uint64(time.Microsecond) {
			return 0, false
		}

		return time.Duration(usec * uint64(time.Microsecond)), true

	case cgroupV1:
		nanos, ok := readUint(s.usagePath)
		if !ok || nanos > maxDurationNanos {
			return 0, false
		}

		return time.Duration(nanos), true

	default:
		return 0, false
	}
}

const maxDurationNanos = uint64(1<<63 - 1)

// memorySample pairs the tightest effective limit below hostTotal with the
// usage of the same cgroup. This matters when a pod or systemd slice limit is
// shared by multiple leaf cgroups: leaf usage cannot describe parent headroom.
func (r *cgroupReader) memorySample(hostTotal uint64) (limit, used uint64, ok bool) {
	scope, ok := findMemoryLimitScope(r.layout())
	if !ok || scope.limit == 0 || (hostTotal > 0 && scope.limit >= hostTotal) {
		return 0, 0, false
	}

	used, ok = memoryUsageAt(scope)
	if !ok {
		return 0, 0, false
	}

	return scope.limit, min(used, scope.limit), true
}

// cpuScope resolves the effective CPU capacity and fixes its usage path for a
// before/after sample. limited is false when cgroup constraints do not reduce
// hostLogical; the returned capacity still records the host ceiling.
func (r *cgroupReader) cpuScope(hostLogical int) (scope cgroupCPUScope, limited bool) {
	layout := r.layout()

	if hostLogical > 0 {
		scope.capacity = float64(hostLogical)
	}

	if quota, ok := cpuLimit(layout); ok && (scope.capacity == 0 || quota.capacity < scope.capacity) {
		scope = quota
		limited = true
	}

	if cpuset, ok := cpuSetLimit(layout); ok && (scope.capacity == 0 || cpuset.capacity < scope.capacity) {
		scope = cpuset
		limited = true
	}

	return scope, limited
}

type memoryLimitScope struct {
	limit   uint64
	dir     string
	version cgroupVersion
}

func findMemoryLimitScope(layout cgroupLayout) (memoryLimitScope, bool) {
	var (
		best  memoryLimitScope
		found bool
	)

	consider := func(location cgroupLocation) {
		// Equal outer limits share the same ceiling with more descendants, so
		// their aggregate usage is the effective headroom signal.
		for _, dir := range location.ancestors() {
			limit, ok := readMemoryLimit(location.version, dir)
			if ok && (!found || limit <= best.limit) {
				best = memoryLimitScope{limit: limit, dir: dir, version: location.version}
				found = true
			}
		}
	}

	if layout.v2 != nil {
		consider(*layout.v2)
	}

	if location, ok := layout.v1["memory"]; ok {
		consider(location)
	}

	return best, found
}

func readMemoryLimit(version cgroupVersion, dir string) (uint64, bool) {
	var file string
	if version == cgroupV2 {
		file = "memory.max"
	} else {
		file = "memory.limit_in_bytes"
	}

	line := readFirstLine(filepath.Join(dir, file))
	if line == "" || line == "max" {
		return 0, false
	}

	limit, err := strconv.ParseUint(line, 10, 64)
	if err != nil || limit == 0 || (version == cgroupV1 && limit >= v1UnlimitedThreshold) {
		return 0, false
	}

	return limit, true
}

func memoryUsageAt(scope memoryLimitScope) (uint64, bool) {
	if scope.version == cgroupV2 {
		current, ok := readUint(filepath.Join(scope.dir, "memory.current"))
		if !ok {
			return 0, false
		}

		return subtractInactiveFile(current, filepath.Join(scope.dir, "memory.stat"), "inactive_file"), true
	}

	usage, ok := readUint(filepath.Join(scope.dir, "memory.usage_in_bytes"))
	if !ok {
		return 0, false
	}

	return subtractInactiveFile(usage, filepath.Join(scope.dir, "memory.stat"), "total_inactive_file"), true
}

func cpuLimit(layout cgroupLayout) (cgroupCPUScope, bool) {
	// On equal quotas, the outer scope includes sibling consumption and
	// therefore describes the shared budget that can throttle first.
	var (
		best  cgroupCPUScope
		found bool
	)

	if layout.v2 != nil {
		for _, dir := range layout.v2.ancestors() {
			quota, ok := parseV2CPUMax(readFirstLine(filepath.Join(dir, "cpu.max")))
			if ok && (!found || quota <= best.capacity) {
				best = cgroupCPUScope{
					capacity:  quota,
					usagePath: filepath.Join(dir, "cpu.stat"),
					version:   cgroupV2,
				}
				found = true
			}
		}
	}

	cpuLocation, cpuOK := layout.v1["cpu"]

	acctLocation, acctOK := layout.v1["cpuacct"]
	if cpuOK {
		for _, dir := range cpuLocation.ancestors() {
			quota, quotaOK := readInt(filepath.Join(dir, "cpu.cfs_quota_us"))

			period, periodOK := readInt(filepath.Join(dir, "cpu.cfs_period_us"))
			if !quotaOK || !periodOK || quota <= 0 || period <= 0 {
				continue
			}

			capacity := float64(quota) / float64(period)
			if found && capacity > best.capacity {
				continue
			}

			usagePath := ""
			if acctOK && acctLocation.mountPoint == cpuLocation.mountPoint && acctLocation.dir == cpuLocation.dir {
				usagePath = filepath.Join(dir, "cpuacct.usage")
			}

			best = cgroupCPUScope{capacity: capacity, usagePath: usagePath, version: cgroupV1}
			found = true
		}
	}

	return best, found
}

func cpuSetLimit(layout cgroupLayout) (cgroupCPUScope, bool) {
	if layout.v2 != nil {
		if count, ok := readCPUSet(*layout.v2, "cpuset.cpus.effective"); ok {
			return cgroupCPUScope{
				capacity:  float64(count),
				usagePath: filepath.Join(layout.v2.dir, "cpu.stat"),
				version:   cgroupV2,
			}, true
		}
	}

	if cpuset, ok := layout.v1["cpuset"]; ok {
		if count, ok := readCPUSet(cpuset, "cpuset.effective_cpus"); ok {
			scope := cgroupCPUScope{capacity: float64(count), version: cgroupV1}

			if acct, exists := layout.v1["cpuacct"]; exists &&
				acct.mountPoint == cpuset.mountPoint && acct.dir == cpuset.dir {
				scope.usagePath = filepath.Join(cpuset.dir, "cpuacct.usage")
			}

			return scope, true
		}
	}

	return cgroupCPUScope{}, false
}

func readCPUSet(location cgroupLocation, effectiveFile string) (int, bool) {
	if count, ok := parseCPUSet(readFirstLine(filepath.Join(location.dir, effectiveFile))); ok {
		return count, true
	}

	for _, dir := range location.ancestors() {
		if count, ok := parseCPUSet(readFirstLine(filepath.Join(dir, "cpuset.cpus"))); ok {
			return count, true
		}
	}

	return 0, false
}

func parseCPUSet(value string) (int, bool) {
	if value == "" {
		return 0, false
	}

	var (
		count   int
		lastEnd = -1
	)

	for part := range strings.SplitSeq(value, ",") {
		startField, endField, hasRange := strings.Cut(strings.TrimSpace(part), "-")

		start, err := strconv.Atoi(startField)
		if err != nil || start < 0 {
			return 0, false
		}

		end := start
		if hasRange {
			end, err = strconv.Atoi(endField)
			if err != nil || end < start {
				return 0, false
			}
		}

		maxInt := int(^uint(0) >> 1)
		if start <= lastEnd || end-start > maxInt-count-1 {
			return 0, false
		}

		count += end - start + 1
		lastEnd = end
	}

	return count, count > 0
}

func (r *cgroupReader) layout() cgroupLayout {
	paths := readSelfCgroupPaths(r.path("proc/self/cgroup"))
	layout := cgroupLayout{v1: make(map[string]cgroupLocation)}

	for _, mount := range readCgroupMounts(r.path("proc/self/mountinfo")) {
		if mount.version == cgroupV2 {
			if layout.v2 != nil {
				continue
			}

			if location, ok := r.translateLocation(mount, paths[""]); ok {
				layout.v2 = &location
			}

			continue
		}

		for _, controller := range mount.controllers {
			if _, exists := layout.v1[controller]; exists {
				continue
			}

			if location, ok := r.translateLocation(mount, paths[controller]); ok {
				layout.v1[controller] = location
			}
		}
	}

	return layout
}

func (r *cgroupReader) translateLocation(mount cgroupMount, cgroupPath string) (cgroupLocation, bool) {
	if cgroupPath == "" {
		return cgroupLocation{}, false
	}

	rel, ok := relativeCgroupPath(mount.root, cgroupPath)
	if !ok {
		return cgroupLocation{}, false
	}

	mountPoint := r.rootedPath(mount.mountPoint)

	dir := mountPoint
	if rel != "" {
		dir = filepath.Join(mountPoint, filepath.FromSlash(rel))
	}

	if !pathWithin(dir, mountPoint) {
		return cgroupLocation{}, false
	}

	if _, err := os.Stat(dir); err != nil {
		return cgroupLocation{}, false
	}

	return cgroupLocation{version: mount.version, dir: dir, mountPoint: mountPoint}, true
}

func (r *cgroupReader) rootedPath(absolute string) string {
	if r.root == "" {
		return filepath.FromSlash(absolute)
	}

	return filepath.Join(r.root, filepath.FromSlash(strings.TrimPrefix(absolute, "/")))
}

func relativeCgroupPath(mountRoot, cgroupPath string) (string, bool) {
	mountRoot = path.Clean(mountRoot)
	cgroupPath = path.Clean(cgroupPath)

	// A cgroup namespace reports its own root as "/", regardless of the
	// hierarchy path represented by the mount root.
	if cgroupPath == "/" || cgroupPath == mountRoot {
		return "", true
	}

	if mountRoot == "/" {
		return strings.TrimPrefix(cgroupPath, "/"), true
	}

	prefix := strings.TrimSuffix(mountRoot, "/") + "/"
	if !strings.HasPrefix(cgroupPath, prefix) {
		return "", false
	}

	return strings.TrimPrefix(cgroupPath, prefix), true
}

func pathWithin(candidate, root string) bool {
	return candidate == root || strings.HasPrefix(candidate, root+string(filepath.Separator))
}

func readSelfCgroupPaths(filename string) map[string]string {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil
	}

	paths := make(map[string]string)
	for line := range strings.Lines(string(data)) {
		fields := strings.SplitN(strings.TrimSpace(line), ":", 3)
		if len(fields) != 3 {
			continue
		}

		if fields[0] == "0" && fields[1] == "" {
			paths[""] = fields[2]

			continue
		}

		for controller := range strings.SplitSeq(fields[1], ",") {
			if controller != "" {
				paths[controller] = fields[2]
			}
		}
	}

	return paths
}

func readCgroupMounts(filename string) []cgroupMount {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil
	}

	var mounts []cgroupMount
	for line := range strings.Lines(string(data)) {
		fields := strings.Fields(line)

		separator := -1
		for i, field := range fields {
			if field == "-" {
				separator = i

				break
			}
		}

		if separator < 6 || len(fields) < separator+4 {
			continue
		}

		mount := cgroupMount{
			root:       unescapeMountInfoField(fields[3]),
			mountPoint: unescapeMountInfoField(fields[4]),
		}

		switch fields[separator+1] {
		case "cgroup2":
			mount.version = cgroupV2
		case "cgroup":
			mount.version = cgroupV1
			for option := range strings.SplitSeq(fields[separator+3], ",") {
				if option != "" && option != "rw" && option != "ro" && !strings.Contains(option, "=") {
					mount.controllers = append(mount.controllers, option)
				}
			}

		default:
			continue
		}

		mounts = append(mounts, mount)
	}

	return mounts
}

func unescapeMountInfoField(value string) string {
	replacer := strings.NewReplacer(`\040`, " ", `\011`, "\t", `\012`, "\n", `\134`, `\`)

	return replacer.Replace(value)
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

	if inactive < usage {
		return usage - inactive
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
func readStatField(path, field string) (uint64, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}

	for line := range strings.Lines(string(data)) {
		name, valueField, found := strings.Cut(strings.TrimSpace(line), " ")
		if !found || name != field {
			continue
		}

		value, err := strconv.ParseUint(strings.TrimSpace(valueField), 10, 64)
		if err != nil {
			return 0, false
		}

		return value, true
	}

	return 0, false
}

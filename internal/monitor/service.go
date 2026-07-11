package monitor

import (
	"context"
	"errors"
	"math"
	"os"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/process"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/monitor"
	"github.com/coldsmirk/vef-framework-go/version"
)

// DefaultService implements monitor.Service with background CPU and process sampling.
//
// CPU and memory metrics are container-aware: when the process runs under a
// cgroup (v2 or v1) that actually limits the resource, the limit and the
// cgroup's own usage replace the host-wide numbers; without a limit the host
// view is reported unchanged. Process and network metrics are whatever procfs
// exposes — the host's when reachable (host PID/network namespace), otherwise
// the container's own namespace.
type DefaultService struct {
	buildInfo *monitor.BuildInfo
	config    config.MonitorConfig
	cgroups   *cgroupReader

	cpuCache     atomic.Value // stores *monitor.CPUInfo
	processCache atomic.Value // stores *monitor.ProcessInfo

	samplerCancel context.CancelFunc
	samplerDone   chan struct{}
}

// NewService creates a new monitor.Service implementation. A nil cfg or zero-valued
// fields fall back to DefaultConfig; a nil buildInfo falls back to unknown metadata.
// The framework version is always stamped onto the returned build info.
func NewService(cfg *config.MonitorConfig, buildInfo *monitor.BuildInfo) monitor.Service {
	return &DefaultService{
		buildInfo: resolveBuildInfo(buildInfo),
		config:    resolveConfig(cfg),
		cgroups:   newCgroupReader(),
	}
}

// resolveConfig applies DefaultConfig values for any unset (zero) sampling field so
// the service always has a positive sample interval and duration, while preserving
// the caller-provided mount exclusions.
func resolveConfig(cfg *config.MonitorConfig) config.MonitorConfig {
	resolved := DefaultConfig()
	if cfg == nil {
		return resolved
	}

	if cfg.SampleInterval > 0 {
		resolved.SampleInterval = cfg.SampleInterval
	}

	if cfg.SampleDuration > 0 {
		resolved.SampleDuration = cfg.SampleDuration
	}

	resolved.ExcludedMounts = cfg.ExcludedMounts

	return resolved
}

// resolveBuildInfo fills unknown metadata when no build info was supplied and
// always stamps the framework version.
func resolveBuildInfo(buildInfo *monitor.BuildInfo) *monitor.BuildInfo {
	if buildInfo == nil {
		buildInfo = &monitor.BuildInfo{
			AppVersion: "unknown",
			BuildTime:  "unknown",
			GitCommit:  "unknown",
		}
	}

	buildInfo.VEFVersion = version.VEFVersion

	return buildInfo
}

// Overview returns a comprehensive system overview by fetching all metrics.
// It is best-effort and never returns an error: a sub-metric that fails to
// collect is logged and left nil so a single broken collector does not mask the
// rest. Callers should inspect individual fields rather than rely on the error.
func (s *DefaultService) Overview(ctx context.Context) (*monitor.SystemOverview, error) {
	var overview monitor.SystemOverview

	if hostInfo, err := s.Host(ctx); err != nil {
		logger.Warnf("Overview: failed to collect host info: %v", err)
	} else {
		overview.Host = &monitor.HostSummary{
			Hostname:        hostInfo.Hostname,
			OS:              hostInfo.OS,
			Platform:        hostInfo.Platform,
			PlatformVersion: hostInfo.PlatformVersion,
			KernelVersion:   hostInfo.KernelVersion,
			KernelArch:      hostInfo.KernelArch,
			Uptime:          hostInfo.Uptime,
		}
	}

	if cpuInfo, err := s.CPU(ctx); err != nil {
		logger.Warnf("Overview: failed to collect CPU info: %v", err)
	} else {
		overview.CPU = &monitor.CPUSummary{
			PhysicalCores: cpuInfo.PhysicalCores,
			LogicalCores:  cpuInfo.LogicalCores,
			UsagePercent:  cpuInfo.TotalPercent,
		}
	}

	if memInfo, err := s.Memory(ctx); err != nil {
		logger.Warnf("Overview: failed to collect memory info: %v", err)
	} else if memInfo.Virtual != nil {
		overview.Memory = &monitor.MemorySummary{
			Total:       memInfo.Virtual.Total,
			Used:        memInfo.Virtual.Used,
			UsedPercent: memInfo.Virtual.UsedPercent,
		}
	}

	if diskInfo, err := s.Disk(ctx); err != nil {
		logger.Warnf("Overview: failed to collect disk info: %v", err)
	} else {
		overview.Disk = s.buildDiskSummary(diskInfo)
	}

	if netInfo, err := s.Network(ctx); err != nil {
		logger.Warnf("Overview: failed to collect network info: %v", err)
	} else {
		overview.Network = s.buildNetworkSummary(netInfo)
	}

	if procInfo, err := s.Process(ctx); err != nil {
		logger.Warnf("Overview: failed to collect process info: %v", err)
	} else {
		overview.Process = &monitor.ProcessSummary{
			PID:           procInfo.PID,
			Name:          procInfo.Name,
			CPUPercent:    procInfo.CPUPercent,
			MemoryPercent: procInfo.MemoryPercent,
		}
	}

	if loadInfo, err := s.Load(ctx); err != nil {
		logger.Warnf("Overview: failed to collect load info: %v", err)
	} else {
		overview.Load = loadInfo
	}

	overview.Build = s.BuildInfo()

	return &overview, nil
}

func (s *DefaultService) buildDiskSummary(diskInfo *monitor.DiskInfo) *monitor.DiskSummary {
	var (
		total, used uint64
		partitions  int
		seenDevices = make(map[string]bool)
	)

	for _, part := range diskInfo.Partitions {
		if s.shouldSkipPartition(part) {
			continue
		}

		if part.Device != "" {
			container := getDeviceContainer(part.Device)
			if seenDevices[container] {
				continue
			}

			seenDevices[container] = true
		}

		total += part.Total
		used += partitionUsed(part)
		partitions++
	}

	var usedPercent float64
	if total > 0 {
		usedPercent = float64(used) / float64(total) * 100
	}

	return &monitor.DiskSummary{
		Total:       total,
		Used:        used,
		UsedPercent: usedPercent,
		Partitions:  partitions,
	}
}

// shouldSkipPartition decides whether a partition contributes to disk totals,
// combining the mount-point exclusions with device-based rules that mount
// paths cannot express.
func (s *DefaultService) shouldSkipPartition(part *monitor.PartitionInfo) bool {
	if isSnapshotDevice(part.Device, part.FSType) {
		return true
	}

	return s.shouldSkipMountPoint(part.MountPoint)
}

// isSnapshotDevice reports whether the partition is a mounted APFS snapshot
// (Time Machine local snapshots, update snapshots). Snapshot mounts use the
// "<snapshot-name>@<device>" source form, appear at unpredictable mount points
// (e.g. /Volumes/.timemachine/...), and each re-reports the full container
// size, so they must never contribute to disk totals.
func isSnapshotDevice(device, fsType string) bool {
	return fsType == "apfs" && strings.Contains(device, "@")
}

// partitionUsed returns the space consumed on the partition's underlying
// storage. On space-sharing filesystems (APFS), statfs reports the whole
// container as Total and the shared remaining space as Free, while Used covers
// only the volume's own files — after sibling volumes de-duplicate away, that
// per-volume Used would drastically undercount (the macOS root snapshot holds
// ~10GB while the Data volume holds the real data). Total - Free is the
// container-level consumption and the number that answers "how full is this
// disk". Other filesystems keep their own Used, which already describes that
// filesystem alone.
func partitionUsed(part *monitor.PartitionInfo) uint64 {
	if part.FSType == "apfs" && part.Total >= part.Free {
		return part.Total - part.Free
	}

	return part.Used
}

func (*DefaultService) buildNetworkSummary(netInfo *monitor.NetworkInfo) *monitor.NetworkSummary {
	var bytesSent, bytesRecv, packetsSent, packetsRecv uint64
	for _, counter := range netInfo.IOCounters {
		bytesSent += counter.BytesSent
		bytesRecv += counter.BytesRecv
		packetsSent += counter.PacketsSent
		packetsRecv += counter.PacketsRecv
	}

	return &monitor.NetworkSummary{
		Interfaces:  len(netInfo.Interfaces),
		BytesSent:   bytesSent,
		BytesRecv:   bytesRecv,
		PacketsSent: packetsSent,
		PacketsRecv: packetsRecv,
	}
}

// excludedMountPrefixes are OS pseudo-filesystem mount points that never
// represent real storage and are always excluded from disk statistics.
var excludedMountPrefixes = []string{
	// macOS special volumes
	"/System/Volumes/",
	"/Volumes/Recovery",
	"/private/var/vm",
	// Linux special mount points
	"/snap/",
	"/run/",
	"/dev/",
	"/sys/",
	"/proc/",
}

// shouldSkipMountPoint checks if a mount point should be excluded from disk stats.
// Built-in OS pseudo-mounts are always skipped; host- or vendor-specific volumes
// are skipped only when their path contains a configured ExcludedMounts substring.
func (s *DefaultService) shouldSkipMountPoint(mountPoint string) bool {
	if mountPoint == "" {
		return true
	}

	for _, prefix := range excludedMountPrefixes {
		if matchesMountPrefix(mountPoint, prefix) {
			return true
		}
	}

	for _, substr := range s.config.ExcludedMounts {
		if substr != "" && strings.Contains(mountPoint, substr) {
			return true
		}
	}

	return false
}

// matchesMountPrefix reports whether mountPoint is the excluded path itself or
// lives underneath it. Matching on path boundaries means an entry like "/dev/"
// also excludes the "/dev" mount point itself (devfs on macOS) without falsely
// matching siblings such as "/devdata".
func matchesMountPrefix(mountPoint, prefix string) bool {
	base := strings.TrimSuffix(prefix, "/")

	return mountPoint == base || strings.HasPrefix(mountPoint, base+"/")
}

var (
	// pPartitionSuffix strips a trailing "pN" partition from NVMe/eMMC devices
	// (nvme0n1p2 -> nvme0n1, mmcblk0p1 -> mmcblk0). The nN namespace is part of
	// the device identity and is preserved, so distinct namespaces such as
	// nvme0n1 and nvme0n2 are NOT merged into one container.
	pPartitionSuffix = regexp.MustCompile(`p[0-9]+$`)
	// apfsSliceSuffix strips APFS slice suffixes from a disk device, including
	// the two-level sealed system snapshot form used by the macOS root volume
	// since Big Sur (disk3s1s1 -> disk3, disk1s2 -> disk1). Stripping only one
	// level would leave the root at "disk3s1" while its Data sibling resolves
	// to "disk3", counting the shared container twice.
	apfsSliceSuffix = regexp.MustCompile(`(s[0-9]+)+$`)
	// letterDiskPartition matches the classic letter-named disk families whose
	// trailing digits are partition numbers (sda1, vdb2, xvda3). Only these
	// families get their digits stripped: for every other name a trailing digit
	// is part of the device identity (dm-1, loop0, rbd1, mapper/vg-lv1) and
	// stripping it would merge independent devices.
	letterDiskPartition = regexp.MustCompile(`(?:^|/)(?:[shv]|xv)d[a-z]+[0-9]+$`)
	// digitSuffix strips the partition number off a letter-named disk; applied
	// only after letterDiskPartition confirmed the family.
	digitSuffix = regexp.MustCompile(`[0-9]+$`)
)

// getDeviceContainer extracts the base container device name from a partition
// device so sibling partitions of one physical disk de-duplicate to a single
// container, WITHOUT merging genuinely independent devices. Suffix stripping
// is allowlist-based per device family, and an unrecognized device is returned
// verbatim: wrongly merging two real disks silently drops capacity, while not
// merging merely risks counting a shared container twice.
func getDeviceContainer(device string) string {
	switch {
	case device == "":
		return ""
	case strings.Contains(device, "nvme"), strings.Contains(device, "mmcblk"):
		return pPartitionSuffix.ReplaceAllString(device, "")
	case strings.Contains(device, "disk"):
		return apfsSliceSuffix.ReplaceAllString(device, "")
	case letterDiskPartition.MatchString(device):
		return digitSuffix.ReplaceAllString(device, "")
	default:
		return device
	}
}

// CPU returns detailed CPU information including usage percentages.
func (s *DefaultService) CPU(context.Context) (*monitor.CPUInfo, error) {
	cached := s.cpuCache.Load()
	if cached == nil {
		return nil, ErrCPUInfoNotReady
	}

	return cached.(*monitor.CPUInfo), nil
}

// Memory returns memory usage information. Inside a memory-limited container
// the headline figures describe the container's limit and working set rather
// than the host's /proc/meminfo, which is not namespaced.
func (s *DefaultService) Memory(ctx context.Context) (*monitor.MemoryInfo, error) {
	vMem, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		return nil, err
	}

	virtual := convertVirtualMemory(vMem)
	s.applyCgroupMemoryLimit(virtual)

	result := &monitor.MemoryInfo{
		Virtual: virtual,
	}

	if swapMem, err := mem.SwapMemoryWithContext(ctx); err == nil {
		result.Swap = convertSwapMemory(swapMem)
	}

	return result, nil
}

// applyCgroupMemoryLimit overrides the headline memory figures (Total, Used,
// Available, Free, UsedPercent) with the container's cgroup limit and
// working-set usage when a real limit is set: inside a limited container the
// host numbers describe the node, not what this process can allocate before
// the OOM killer intervenes. Detail fields (buffers, cache breakdowns) keep
// their host meaning. A limit at or above the host total constrains nothing
// and is ignored, as is a limit whose usage counter cannot be read — a mixed
// host/container view would be worse than either.
func (s *DefaultService) applyCgroupMemoryLimit(virtual *monitor.VirtualMemory) {
	limit, ok := s.cgroups.memoryLimit()
	if !ok || (virtual.Total > 0 && limit >= virtual.Total) {
		return
	}

	used, ok := s.cgroups.memoryUsage()
	if !ok {
		return
	}

	used = min(used, limit)

	virtual.Total = limit
	virtual.Used = used
	virtual.Available = limit - used
	virtual.Free = limit - used
	virtual.UsedPercent = float64(used) / float64(limit) * 100
}

func convertVirtualMemory(v *mem.VirtualMemoryStat) *monitor.VirtualMemory {
	return &monitor.VirtualMemory{
		Total:             v.Total,
		Available:         v.Available,
		Used:              v.Used,
		UsedPercent:       v.UsedPercent,
		Free:              v.Free,
		Active:            v.Active,
		Inactive:          v.Inactive,
		Wired:             v.Wired,
		Laundry:           v.Laundry,
		Buffers:           v.Buffers,
		Cached:            v.Cached,
		WriteBack:         v.WriteBack,
		Dirty:             v.Dirty,
		WriteBackTmp:      v.WriteBackTmp,
		Shared:            v.Shared,
		Slab:              v.Slab,
		SlabReclaimable:   v.Sreclaimable,
		SlabUnreclaimable: v.Sunreclaim,
		PageTables:        v.PageTables,
		SwapCached:        v.SwapCached,
		CommitLimit:       v.CommitLimit,
		CommittedAs:       v.CommittedAS,
		HighTotal:         v.HighTotal,
		HighFree:          v.HighFree,
		LowTotal:          v.LowTotal,
		LowFree:           v.LowFree,
		SwapTotal:         v.SwapTotal,
		SwapFree:          v.SwapFree,
		Mapped:            v.Mapped,
		VMAllocTotal:      v.VmallocTotal,
		VMAllocUsed:       v.VmallocUsed,
		VMAllocChunk:      v.VmallocChunk,
		HugePagesTotal:    v.HugePagesTotal,
		HugePagesFree:     v.HugePagesFree,
		HugePagesReserved: v.HugePagesRsvd,
		HugePagesSurplus:  v.HugePagesSurp,
		HugePageSize:      v.HugePageSize,
		AnonHugePages:     v.AnonHugePages,
	}
}

func convertSwapMemory(s *mem.SwapMemoryStat) *monitor.SwapMemory {
	return &monitor.SwapMemory{
		Total:          s.Total,
		Used:           s.Used,
		Free:           s.Free,
		UsedPercent:    s.UsedPercent,
		SwapIn:         s.Sin,
		SwapOut:        s.Sout,
		PageIn:         s.PgIn,
		PageOut:        s.PgOut,
		PageFault:      s.PgFault,
		PageMajorFault: s.PgMajFault,
	}
}

// Disk returns disk usage and partition information.
func (*DefaultService) Disk(ctx context.Context) (*monitor.DiskInfo, error) {
	partitions, err := disk.PartitionsWithContext(ctx, false)
	if err != nil {
		return nil, err
	}

	partitionInfos := make([]*monitor.PartitionInfo, 0, len(partitions))
	for _, part := range partitions {
		usage, err := disk.UsageWithContext(ctx, part.Mountpoint)
		if err != nil {
			continue
		}

		partitionInfos = append(partitionInfos, &monitor.PartitionInfo{
			Device:            part.Device,
			MountPoint:        part.Mountpoint,
			FSType:            part.Fstype,
			Options:           part.Opts,
			Total:             usage.Total,
			Free:              usage.Free,
			Used:              usage.Used,
			UsedPercent:       usage.UsedPercent,
			INodesTotal:       usage.InodesTotal,
			INodesUsed:        usage.InodesUsed,
			INodesFree:        usage.InodesFree,
			INodesUsedPercent: usage.InodesUsedPercent,
		})
	}

	result := &monitor.DiskInfo{Partitions: partitionInfos}

	if ioCountersMap, err := disk.IOCountersWithContext(ctx); err == nil {
		result.IOCounters = convertDiskIOCounters(ioCountersMap)
	}

	return result, nil
}

func convertDiskIOCounters(counters map[string]disk.IOCountersStat) map[string]*monitor.IOCounter {
	result := make(map[string]*monitor.IOCounter, len(counters))
	for name, c := range counters {
		result[name] = &monitor.IOCounter{
			ReadCount:        c.ReadCount,
			MergedReadCount:  c.MergedReadCount,
			WriteCount:       c.WriteCount,
			MergedWriteCount: c.MergedWriteCount,
			ReadBytes:        c.ReadBytes,
			WriteBytes:       c.WriteBytes,
			ReadTime:         c.ReadTime,
			WriteTime:        c.WriteTime,
			IOPSInProgress:   c.IopsInProgress,
			IOTime:           c.IoTime,
			WeightedIO:       c.WeightedIO,
			Name:             c.Name,
			SerialNumber:     c.SerialNumber,
			Label:            c.Label,
		}
	}

	return result
}

// Network returns network interface and I/O statistics.
func (*DefaultService) Network(ctx context.Context) (*monitor.NetworkInfo, error) {
	interfaces, err := net.InterfacesWithContext(ctx)
	if err != nil {
		return nil, err
	}

	interfaceInfos := make([]*monitor.InterfaceInfo, 0, len(interfaces))
	for _, iface := range interfaces {
		addrs := make([]string, 0, len(iface.Addrs))
		for _, addr := range iface.Addrs {
			addrs = append(addrs, addr.Addr)
		}

		interfaceInfos = append(interfaceInfos, &monitor.InterfaceInfo{
			Index:        iface.Index,
			MTU:          iface.MTU,
			Name:         iface.Name,
			HardwareAddr: iface.HardwareAddr,
			Flags:        iface.Flags,
			Addrs:        addrs,
		})
	}

	ioCountersSlice, err := net.IOCountersWithContext(ctx, true)
	if err != nil {
		return nil, err
	}

	ioCounters := make(map[string]*monitor.NetIOCounter, len(ioCountersSlice))
	for _, c := range ioCountersSlice {
		ioCounters[c.Name] = &monitor.NetIOCounter{
			Name:        c.Name,
			BytesSent:   c.BytesSent,
			BytesRecv:   c.BytesRecv,
			PacketsSent: c.PacketsSent,
			PacketsRecv: c.PacketsRecv,
			ErrorsIn:    c.Errin,
			ErrorsOut:   c.Errout,
			DroppedIn:   c.Dropin,
			DroppedOut:  c.Dropout,
			FIFOIn:      c.Fifoin,
			FIFOOut:     c.Fifoout,
		}
	}

	return &monitor.NetworkInfo{
		Interfaces: interfaceInfos,
		IOCounters: ioCounters,
	}, nil
}

// Host returns host information.
func (*DefaultService) Host(ctx context.Context) (*monitor.HostInfo, error) {
	info, err := host.InfoWithContext(ctx)
	if err != nil {
		return nil, err
	}

	return &monitor.HostInfo{
		Hostname:             info.Hostname,
		Uptime:               info.Uptime,
		BootTime:             info.BootTime,
		Processes:            info.Procs,
		OS:                   info.OS,
		Platform:             info.Platform,
		PlatformFamily:       info.PlatformFamily,
		PlatformVersion:      info.PlatformVersion,
		KernelVersion:        info.KernelVersion,
		KernelArch:           info.KernelArch,
		VirtualizationSystem: info.VirtualizationSystem,
		VirtualizationRole:   info.VirtualizationRole,
		HostID:               info.HostID,
	}, nil
}

// Process returns information about the current process.
func (s *DefaultService) Process(context.Context) (*monitor.ProcessInfo, error) {
	cached := s.processCache.Load()
	if cached == nil {
		return nil, ErrProcessInfoNotReady
	}

	return cached.(*monitor.ProcessInfo), nil
}

// Load returns system load averages.
func (*DefaultService) Load(ctx context.Context) (*monitor.LoadInfo, error) {
	avg, err := load.AvgWithContext(ctx)
	if err != nil {
		return nil, err
	}

	return &monitor.LoadInfo{
		Load1:  avg.Load1,
		Load5:  avg.Load5,
		Load15: avg.Load15,
	}, nil
}

// BuildInfo returns application build information. It is always non-nil: NewService
// fills unknown metadata and stamps the framework version at construction time.
func (s *DefaultService) BuildInfo() *monitor.BuildInfo {
	return s.buildInfo
}

// Init starts background goroutines to periodically sample CPU and process metrics.
// It is idempotent: a second call while a sampler is already running is a no-op, so
// the running goroutine is never orphaned.
func (s *DefaultService) Init(context.Context) error {
	if s.samplerCancel != nil {
		return nil
	}

	samplerCtx, cancel := context.WithCancel(context.Background())
	s.samplerCancel = cancel
	s.samplerDone = make(chan struct{})

	go s.runBackgroundSampler(samplerCtx)

	return nil
}

func (s *DefaultService) runBackgroundSampler(ctx context.Context) {
	defer close(s.samplerDone)

	ticker := time.NewTicker(s.config.SampleInterval)
	defer ticker.Stop()

	s.sampleCPU(ctx)
	s.sampleProcess(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.sampleCPU(ctx)
			s.sampleProcess(ctx)
		}
	}
}

// Close gracefully stops the background sampling goroutines.
func (s *DefaultService) Close() error {
	if s.samplerCancel != nil {
		s.samplerCancel()
	}

	if s.samplerDone != nil {
		<-s.samplerDone
	}

	return nil
}

func (s *DefaultService) sampleCPU(ctx context.Context) {
	cpuInfo, err := s.collectCPUInfo(ctx)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return
		}

		logger.Errorf("Failed to sample CPU info: %v", err)

		return
	}

	s.cpuCache.Store(cpuInfo)
}

func (s *DefaultService) collectCPUInfo(ctx context.Context) (*monitor.CPUInfo, error) {
	infoStat, err := cpu.InfoWithContext(ctx)
	if err != nil {
		return nil, err
	}

	var cpuInfo monitor.CPUInfo
	if len(infoStat) > 0 {
		first := infoStat[0]
		cpuInfo.ModelName = first.ModelName
		cpuInfo.Mhz = first.Mhz
		cpuInfo.CacheSize = first.CacheSize
		cpuInfo.VendorID = first.VendorID
		cpuInfo.Family = first.Family
		cpuInfo.Model = first.Model
		cpuInfo.Stepping = first.Stepping
		cpuInfo.Microcode = first.Microcode
	}

	cpuInfo.PhysicalCores, _ = cpu.CountsWithContext(ctx, false)
	cpuInfo.LogicalCores, _ = cpu.CountsWithContext(ctx, true)

	quota, limited := s.cgroups.cpuQuota()

	var (
		usageBefore   time.Duration
		usageBeforeOK bool
		sampleStart   time.Time
	)

	if limited {
		usageBefore, usageBeforeOK = s.cgroups.cpuUsage()
		sampleStart = time.Now()
	}

	// PercentWithContext with a positive duration sleeps the sampling window,
	// which doubles as the measurement window for the cgroup usage delta.
	if perCorePercent, err := cpu.PercentWithContext(ctx, s.config.SampleDuration, true); err == nil {
		cpuInfo.UsagePercent = perCorePercent
	}

	if totalPercent, err := cpu.PercentWithContext(ctx, 0, false); err == nil && len(totalPercent) > 0 {
		cpuInfo.TotalPercent = totalPercent[0]
	}

	if limited {
		s.applyCgroupCPUQuota(&cpuInfo, quota, usageBefore, usageBeforeOK, sampleStart)
	}

	return &cpuInfo, nil
}

// applyCgroupCPUQuota replaces the host-wide CPU view with the container's:
// core counts become the quota ceiling and TotalPercent becomes the share of
// the quota consumed over the sampling window (100 = at the limit). The host
// per-core breakdown is dropped — under a quota there is no per-core
// dimension. When the cgroup usage counter cannot be read, the host
// percentages are kept as the best remaining signal.
func (s *DefaultService) applyCgroupCPUQuota(
	cpuInfo *monitor.CPUInfo,
	quota float64,
	usageBefore time.Duration,
	usageBeforeOK bool,
	sampleStart time.Time,
) {
	cores := max(int(math.Ceil(quota)), 1)
	cpuInfo.PhysicalCores = cores
	cpuInfo.LogicalCores = cores

	if !usageBeforeOK {
		return
	}

	usageAfter, ok := s.cgroups.cpuUsage()
	elapsed := time.Since(sampleStart)

	if !ok || usageAfter < usageBefore || elapsed <= 0 {
		return
	}

	percent := (usageAfter - usageBefore).Seconds() / (elapsed.Seconds() * quota) * 100
	cpuInfo.TotalPercent = min(percent, 100)
	cpuInfo.UsagePercent = nil
}

func (s *DefaultService) sampleProcess(ctx context.Context) {
	processInfo, err := s.collectProcessInfo(ctx)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return
		}

		logger.Errorf("Failed to sample process info: %v", err)

		return
	}

	s.processCache.Store(processInfo)
}

func (s *DefaultService) collectProcessInfo(ctx context.Context) (*monitor.ProcessInfo, error) {
	proc, err := process.NewProcessWithContext(ctx, int32(os.Getpid()))
	if err != nil {
		return nil, err
	}

	cpuPercent, err := proc.PercentWithContext(ctx, s.config.SampleDuration)
	if err != nil {
		return nil, err
	}

	memPercent, err := proc.MemoryPercentWithContext(ctx)
	if err != nil {
		return nil, err
	}

	memRSS, memVMS, memSwap := s.collectMemoryInfo(ctx, proc)
	name, _ := proc.NameWithContext(ctx)
	exe, _ := proc.ExeWithContext(ctx)
	cmdline, _ := proc.CmdlineWithContext(ctx)
	cwd, _ := proc.CwdWithContext(ctx)
	status, _ := proc.StatusWithContext(ctx)
	username, _ := proc.UsernameWithContext(ctx)
	createTime, _ := proc.CreateTimeWithContext(ctx)
	numThreads, _ := proc.NumThreadsWithContext(ctx)
	numFDs, _ := proc.NumFDsWithContext(ctx)
	parentPID, _ := proc.PpidWithContext(ctx)

	var statusStr string
	if len(status) > 0 {
		statusStr = status[0]
	}

	return &monitor.ProcessInfo{
		PID:           proc.Pid,
		ParentPID:     parentPID,
		Name:          name,
		Exe:           exe,
		CommandLine:   cmdline,
		CWD:           cwd,
		Status:        statusStr,
		Username:      username,
		CreateTime:    createTime,
		NumThreads:    numThreads,
		NumFDs:        numFDs,
		CPUPercent:    cpuPercent,
		MemoryPercent: memPercent,
		MemoryRSS:     memRSS,
		MemoryVMS:     memVMS,
		MemorySwap:    memSwap,
	}, nil
}

func (*DefaultService) collectMemoryInfo(ctx context.Context, proc *process.Process) (rss, vms, swap uint64) {
	memInfo, err := proc.MemoryInfoWithContext(ctx)
	if err != nil {
		logger.Warnf("Failed to get memory info: %v", err)

		return 0, 0, 0
	}

	if memInfo == nil {
		return 0, 0, 0
	}

	return memInfo.RSS, memInfo.VMS, memInfo.Swap
}

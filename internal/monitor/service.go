package monitor

import (
	"context"
	"errors"
	"math"
	"os"
	"runtime"
	"sync"
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
type DefaultService struct {
	buildInfo *monitor.BuildInfo
	config    config.MonitorConfig

	cpuCache     atomic.Value // stores *monitor.CPUInfo (host CPU sample)
	processCache atomic.Value // stores *monitor.ProcessInfo

	// cgroupCPUCache stores the container-scoped CPU sample (effective cores and
	// the container's own utilization). It is kept separate from cpuCache so the
	// detailed get_cpu endpoint stays purely host-based while the overview can be
	// container-aware. prevCgroupCPU holds the previous cpu.stat reading and is
	// only touched by the single sampler goroutine.
	cgroupCPUCache atomic.Value // stores cgroupCPUSample
	prevCgroupCPU  cgroupCPUPrev

	// mu guards the sampler lifecycle fields so Init/Close are safe under
	// concurrent or interleaved calls.
	mu            sync.Mutex
	samplerCancel context.CancelFunc
	samplerDone   chan struct{}
}

// cgroupCPUSample is the container's CPU view derived from its cgroup: effective
// cores (from the CFS quota) and its own utilization (from the cpu.stat delta).
type cgroupCPUSample struct {
	cores        int
	usagePercent float64
	// usageReady is false until a utilization delta has been computed (needs two
	// readings); before then the overview keeps the host figure rather than 0.
	usageReady bool
	ok         bool
}

// cgroupCPUPrev is the previous cumulative cgroup CPU reading, used to compute a
// utilization delta between sampler ticks.
type cgroupCPUPrev struct {
	usageMicros uint64
	at          time.Time
	ok          bool
}

// NewService creates a new monitor.Service implementation. A nil cfg or zero-valued
// fields fall back to DefaultConfig; a nil buildInfo falls back to unknown metadata.
// The framework version is always stamped onto the returned build info.
func NewService(cfg *config.MonitorConfig, buildInfo *monitor.BuildInfo) monitor.Service {
	return &DefaultService{
		buildInfo: resolveBuildInfo(buildInfo),
		config:    resolveConfig(cfg),
	}
}

// resolveConfig applies DefaultConfig values for any unset (zero) sampling field so
// the service always has a positive sample interval and duration, while preserving
// the caller-provided node_exporter-style filesystem paths.
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

	resolved.ProcfsPath = cfg.ProcfsPath
	resolved.RootfsPath = cfg.RootfsPath

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
		// Prefer the container's own cores and utilization when a CPU limit is in
		// effect; the detailed get_cpu endpoint stays host-based.
		s.applyCgroupCPUSummary(overview.CPU)
	}

	if memInfo, err := s.Memory(ctx); err != nil {
		logger.Warnf("Overview: failed to collect memory info: %v", err)
	} else if memInfo.Virtual != nil {
		overview.Memory = &monitor.MemorySummary{
			Total:       memInfo.Virtual.Total,
			Used:        memInfo.Virtual.Used,
			UsedPercent: memInfo.Virtual.UsedPercent,
		}
		// Prefer the container's cgroup memory limit; get_memory stays host-based.
		applyCgroupMemorySummary(overview.Memory)
	}

	if diskSummary, err := s.diskSummary(ctx); err != nil {
		logger.Warnf("Overview: failed to collect disk info: %v", err)
	} else {
		overview.Disk = diskSummary
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

// CPU returns detailed CPU information including usage percentages. Like Memory,
// this is the HOST view (host core count and host-wide utilization); the
// container's effective cores and its own utilization are applied only to the
// overview summary via applyCgroupCPUSummary.
func (s *DefaultService) CPU(context.Context) (*monitor.CPUInfo, error) {
	cached := s.cpuCache.Load()
	if cached == nil {
		return nil, ErrCPUInfoNotReady
	}

	return cached.(*monitor.CPUInfo), nil
}

// Memory returns memory usage information. This is intentionally the HOST view:
// the detailed get_memory endpoint always reports host figures so the object is
// internally consistent (Total/Used/Available/Swap all host-based). The
// container's cgroup memory limit is applied only to the overview summary
// (get_overview), which is what dashboards consume; see applyCgroupMemorySummary.
func (*DefaultService) Memory(ctx context.Context) (*monitor.MemoryInfo, error) {
	vMem, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		return nil, err
	}

	result := &monitor.MemoryInfo{
		Virtual: convertVirtualMemory(vMem),
	}

	if swapMem, err := mem.SwapMemoryWithContext(ctx); err == nil {
		result.Swap = convertSwapMemory(swapMem)
	}

	return result, nil
}

// applyCgroupMemorySummary overrides the memory summary with the container's
// cgroup limit and working-set usage, but only when a finite limit smaller than
// the host total is configured. In every other case (bare metal, non-Linux, no
// limit, or read/parse failure) the host figures are left untouched. It acts on
// the overview summary only, so the detailed get_memory endpoint stays host-based.
func applyCgroupMemorySummary(summary *monitor.MemorySummary) {
	limit, used, ok := cgroupMemoryLimit()
	if !ok || limit == 0 || limit >= summary.Total {
		return
	}

	if used > limit {
		used = limit
	}

	summary.Total = limit
	summary.Used = used
	summary.UsedPercent = float64(used) / float64(limit) * 100
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

// Disk returns disk usage and partition information. On Linux the partition list
// is built from the same host-filesystem enumeration the overview aggregates, so
// get_disk and get_overview stay consistent; it falls back to gopsutil's local
// partition view when the mount table is unavailable or on other platforms.
func (s *DefaultService) Disk(ctx context.Context) (*monitor.DiskInfo, error) {
	partitionInfos, err := s.diskPartitions(ctx)
	if err != nil {
		return nil, err
	}

	result := &monitor.DiskInfo{Partitions: partitionInfos}

	if ioCountersMap, err := disk.IOCountersWithContext(ctx); err == nil {
		result.IOCounters = convertDiskIOCounters(ioCountersMap)
	}

	return result, nil
}

func (s *DefaultService) diskPartitions(ctx context.Context) ([]*monitor.PartitionInfo, error) {
	if runtime.GOOS == "linux" {
		if filesystems, ok := s.hostFilesystems(ctx); ok {
			partitionInfos := make([]*monitor.PartitionInfo, 0, len(filesystems))
			for _, fs := range filesystems {
				partitionInfos = append(partitionInfos, newPartitionInfo(fs.device, fs.mountPoint, fs.fsType, nil, fs.usage))
			}

			return partitionInfos, nil
		}
	}

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

		partitionInfos = append(partitionInfos, newPartitionInfo(part.Device, part.Mountpoint, part.Fstype, part.Opts, usage))
	}

	return partitionInfos, nil
}

func newPartitionInfo(device, mountPoint, fsType string, options []string, usage *disk.UsageStat) *monitor.PartitionInfo {
	return &monitor.PartitionInfo{
		Device:            device,
		MountPoint:        mountPoint,
		FSType:            fsType,
		Options:           options,
		Total:             usage.Total,
		Free:              usage.Free,
		Used:              usage.Used,
		UsedPercent:       usage.UsedPercent,
		INodesTotal:       usage.InodesTotal,
		INodesUsed:        usage.InodesUsed,
		INodesFree:        usage.InodesFree,
		INodesUsedPercent: usage.InodesUsedPercent,
	}
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
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.samplerCancel != nil {
		return nil
	}

	// Reset the cgroup CPU baseline so a restart (after Close) does not compute a
	// delta against a stale reading. Safe: no sampler runs while samplerCancel is nil.
	s.prevCgroupCPU = cgroupCPUPrev{}

	samplerCtx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	s.samplerCancel = cancel
	s.samplerDone = done

	// Pass the channel explicitly so the goroutine closes the one it was started
	// with, even after Close has cleared the field for a future restart.
	go s.runBackgroundSampler(samplerCtx, done)

	return nil
}

func (s *DefaultService) runBackgroundSampler(ctx context.Context, done chan struct{}) {
	defer close(done)

	ticker := time.NewTicker(s.config.SampleInterval)
	defer ticker.Stop()

	s.sampleAll(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.sampleAll(ctx)
		}
	}
}

// sampleAll refreshes every cached metric for one tick. The CPU and process
// samplers each block for SampleDuration to measure a utilization window, so they
// run concurrently to keep the tick to roughly one window rather than two. The
// cgroup CPU delta is cheap and runs afterwards.
func (s *DefaultService) sampleAll(ctx context.Context) {
	var wg sync.WaitGroup

	wg.Go(func() { s.sampleCPU(ctx) })
	wg.Go(func() { s.sampleProcess(ctx) })
	wg.Wait()

	s.sampleCgroupCPU()
}

// Close gracefully stops the background sampling goroutines. It clears the
// sampler handles so a later Init can start a fresh sampler.
func (s *DefaultService) Close() error {
	s.mu.Lock()
	cancel, done := s.samplerCancel, s.samplerDone
	s.samplerCancel, s.samplerDone = nil, nil
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	if done != nil {
		<-done
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

	// Derive the total from the per-core sample (a self-contained blocking window)
	// rather than cpu.Percent(interval=0), which relies on gopsutil's process-wide
	// "since last call" global state: that returns 0 on the first sample and is
	// corrupted by any other caller of cpu.Percent in the process.
	if perCorePercent, err := cpu.PercentWithContext(ctx, s.config.SampleDuration, true); err == nil {
		cpuInfo.UsagePercent = perCorePercent
		cpuInfo.TotalPercent = meanPercent(perCorePercent)
	}

	return &cpuInfo, nil
}

// meanPercent averages per-core utilization into a single total percentage.
func meanPercent(perCore []float64) float64 {
	if len(perCore) == 0 {
		return 0
	}

	var sum float64
	for _, p := range perCore {
		sum += p
	}

	return sum / float64(len(perCore))
}

// applyCgroupCPUSummary overrides the CPU summary with the container's effective
// cores and its own utilization when a CPU quota is in effect (see sampleCgroupCPU).
// An unlimited container or bare metal leaves the host figures unchanged.
func (s *DefaultService) applyCgroupCPUSummary(summary *monitor.CPUSummary) {
	sample, _ := s.cgroupCPUCache.Load().(cgroupCPUSample)
	if !sample.ok {
		return
	}

	if sample.cores > 0 {
		summary.LogicalCores = sample.cores
		if summary.PhysicalCores == 0 || summary.PhysicalCores > sample.cores {
			summary.PhysicalCores = sample.cores
		}
	}

	// Keep the host utilization until the container's own delta is available, so
	// the first sampling interval does not report a misleading 0%.
	if sample.usageReady {
		summary.UsagePercent = sample.usagePercent
	}
}

// sampleCgroupCPU records the container's effective cores (from the CFS quota)
// and its own CPU utilization (from the cpu.stat usage delta between ticks). The
// host CPU sample (cpuCache) is left untouched, so the detailed get_cpu endpoint
// stays purely host-based. When no CPU quota applies it clears the sample and the
// overview keeps host figures.
func (s *DefaultService) sampleCgroupCPU() {
	cores, ok := cgroupCPUQuota()
	if !ok {
		s.cgroupCPUCache.Store(cgroupCPUSample{})
		s.prevCgroupCPU = cgroupCPUPrev{}

		return
	}

	now := time.Now()
	sample := cgroupCPUSample{cores: int(math.Ceil(cores)), ok: true}

	usage, usageOK := cgroupCPUUsageMicros()
	if usageOK && s.prevCgroupCPU.ok && usage >= s.prevCgroupCPU.usageMicros {
		elapsedMicros := float64(now.Sub(s.prevCgroupCPU.at).Microseconds())
		if elapsedMicros > 0 && cores > 0 {
			percent := float64(usage-s.prevCgroupCPU.usageMicros) / (elapsedMicros * cores) * 100
			sample.usagePercent = math.Max(0, math.Min(100, percent))
			sample.usageReady = true
		}
	}

	s.prevCgroupCPU = cgroupCPUPrev{usageMicros: usage, at: now, ok: usageOK}
	s.cgroupCPUCache.Store(sample)
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

package monitor

import (
	"context"
	"errors"
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

	// mu guards the sampler lifecycle fields so Init/Close are safe under
	// concurrent or interleaved calls, and Close clears them so a later Init
	// can start a fresh sampler.
	mu            sync.Mutex
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
// the service always has a positive sample interval and duration.
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
			PhysicalCores:  cpuInfo.PhysicalCores,
			LogicalCores:   cpuInfo.LogicalCores,
			UsagePercent:   cpuInfo.TotalPercent,
			EffectiveCores: cpuInfo.EffectiveCores,
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

	if diskSummary, err := s.rootDiskSummary(ctx); err != nil {
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

// rootDiskSummary reports the filesystem that bounds the process's root path.
// This avoids treating remote mounts, disk images, and sibling volumes as
// additional host capacity while retaining the raw mount inventory in Disk.
func (*DefaultService) rootDiskSummary(ctx context.Context) (*monitor.DiskSummary, error) {
	usage, err := disk.UsageWithContext(ctx, rootDiskPath())
	if err != nil {
		return nil, err
	}

	return &monitor.DiskSummary{
		Total:       usage.Total,
		Used:        usage.Used,
		UsedPercent: usage.UsedPercent,
		Partitions:  1,
	}, nil
}

func rootDiskPath() string {
	return rootDiskPathForOS(runtime.GOOS, os.Getenv("SystemDrive"))
}

func rootDiskPathForOS(goos, systemDrive string) string {
	if goos != "windows" {
		return "/"
	}

	if systemDrive == "" {
		systemDrive = "C:"
	}

	return systemDrive + "\\"
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
	limit, used, ok := s.cgroups.memorySample(virtual.Total)
	if !ok {
		return
	}

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
// It is idempotent while a sampler is running, and restartable after Close.
func (s *DefaultService) Init(context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.samplerCancel != nil {
		return nil
	}

	samplerCtx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	s.samplerCancel = cancel
	s.samplerDone = done

	// Pass the channel explicitly so the goroutine closes the one it was
	// started with, even after Close has cleared the field for a restart.
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
// samplers each block for SampleDuration to measure a utilization window, so
// they run concurrently to keep the tick to roughly one window rather than two.
func (s *DefaultService) sampleAll(ctx context.Context) {
	var wg sync.WaitGroup

	wg.Go(func() { s.sampleCPU(ctx) })
	wg.Go(func() { s.sampleProcess(ctx) })
	wg.Wait()
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
	cpuInfo.EffectiveCores = float64(cpuInfo.LogicalCores)

	scope, limited := s.cgroups.cpuScope(cpuInfo.LogicalCores)

	var (
		usageBefore   time.Duration
		usageBeforeOK bool
		sampleStart   time.Time
	)

	if limited {
		usageBefore, usageBeforeOK = scope.sample()
		sampleStart = time.Now()
	}

	// PercentWithContext with a positive duration sleeps the sampling window,
	// which doubles as the measurement window for the cgroup usage delta. The
	// total is derived from the same per-core sample rather than
	// cpu.Percent(0, false), whose process-wide "since last call" state returns
	// 0 on the first sample and is corrupted by any other caller in the process.
	if perCorePercent, err := cpu.PercentWithContext(ctx, s.config.SampleDuration, true); err == nil {
		cpuInfo.UsagePercent = perCorePercent
		cpuInfo.TotalPercent = meanPercent(perCorePercent)
	}

	if limited {
		s.applyCgroupCPUScope(&cpuInfo, scope, usageBefore, usageBeforeOK, sampleStart)
	}

	return &cpuInfo, nil
}

// meanPercent averages per-core utilization into a single total percentage;
// an empty sample yields 0 rather than a division by zero.
func meanPercent(perCore []float64) float64 {
	if len(perCore) == 0 {
		return 0
	}

	var sum float64
	for _, percent := range perCore {
		sum += percent
	}

	return sum / float64(len(perCore))
}

// applyCgroupCPUScope replaces host utilization with the share of the effective
// cgroup CPU capacity consumed over the sample window. Host topology remains
// intact; EffectiveCores carries the quota/cpuset capacity. An incomplete
// cgroup sample leaves the entire host view unchanged.
func (*DefaultService) applyCgroupCPUScope(
	cpuInfo *monitor.CPUInfo,
	scope cgroupCPUScope,
	usageBefore time.Duration,
	usageBeforeOK bool,
	sampleStart time.Time,
) {
	if !usageBeforeOK {
		return
	}

	usageAfter, ok := scope.sample()
	elapsed := time.Since(sampleStart)

	if !ok || usageAfter < usageBefore || elapsed <= 0 || scope.capacity <= 0 {
		return
	}

	percent := (usageAfter - usageBefore).Seconds() / (elapsed.Seconds() * scope.capacity) * 100
	cpuInfo.EffectiveCores = scope.capacity
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

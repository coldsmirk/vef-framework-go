package monitor

import (
	"context"
	"errors"
	"os"
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
type DefaultService struct {
	buildInfo *monitor.BuildInfo
	config    *config.MonitorConfig

	cpuCache     atomic.Value // stores *monitor.CPUInfo
	processCache atomic.Value // stores *monitor.ProcessInfo

	samplerCancel context.CancelFunc
	samplerDone   chan struct{}
}

// NewService creates a new monitor.Service implementation.
func NewService(cfg *config.MonitorConfig, buildInfo *monitor.BuildInfo) monitor.Service {
	return &DefaultService{
		buildInfo: buildInfo,
		config:    cfg,
	}
}

// Overview returns a comprehensive system overview by fetching all metrics.
func (s *DefaultService) Overview(ctx context.Context) (*monitor.SystemOverview, error) {
	var overview monitor.SystemOverview

	if hostInfo, err := s.Host(ctx); err == nil {
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

	if cpuInfo, err := s.CPU(ctx); err == nil {
		overview.CPU = &monitor.CPUSummary{
			PhysicalCores: cpuInfo.PhysicalCores,
			LogicalCores:  cpuInfo.LogicalCores,
			UsagePercent:  cpuInfo.TotalPercent,
		}
	}

	if memInfo, err := s.Memory(ctx); err == nil && memInfo.Virtual != nil {
		overview.Memory = &monitor.MemorySummary{
			Total:       memInfo.Virtual.Total,
			Used:        memInfo.Virtual.Used,
			UsedPercent: memInfo.Virtual.UsedPercent,
		}
	}

	if diskInfo, err := s.Disk(ctx); err == nil {
		overview.Disk = s.buildDiskSummary(diskInfo)
	}

	if netInfo, err := s.Network(ctx); err == nil {
		overview.Network = s.buildNetworkSummary(netInfo)
	}

	if procInfo, err := s.Process(ctx); err == nil {
		overview.Process = &monitor.ProcessSummary{
			PID:           procInfo.PID,
			Name:          procInfo.Name,
			CPUPercent:    procInfo.CPUPercent,
			MemoryPercent: procInfo.MemoryPercent,
		}
	}

	if loadInfo, err := s.Load(ctx); err == nil {
		overview.Load = loadInfo
	}

	overview.Build = s.BuildInfo()

	return &overview, nil
}

func (*DefaultService) buildDiskSummary(diskInfo *monitor.DiskInfo) *monitor.DiskSummary {
	var (
		total, used uint64
		seenDevices = make(map[string]bool)
	)

	for _, part := range diskInfo.Partitions {
		if shouldSkipMountPoint(part.MountPoint) {
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
		used += part.Used
	}

	var usedPercent float64
	if total > 0 {
		usedPercent = float64(used) / float64(total) * 100
	}

	return &monitor.DiskSummary{
		Total:       total,
		Used:        used,
		UsedPercent: usedPercent,
		Partitions:  len(diskInfo.Partitions),
	}
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

// Mount point prefixes to exclude from disk statistics.
var (
	excludedMountPrefixes = []string{
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
		// Development tools
		"/Library/Developer/CoreSimulator",
	}
	excludedMountSubstrings = []string{
		".timemachine",
		"OrbStack",
	}
)

// shouldSkipMountPoint checks if a mount point should be excluded from disk stats.
func shouldSkipMountPoint(mountPoint string) bool {
	if mountPoint == "" {
		return true
	}

	for _, prefix := range excludedMountPrefixes {
		if strings.HasPrefix(mountPoint, prefix) {
			return true
		}
	}

	for _, substr := range excludedMountSubstrings {
		if strings.Contains(mountPoint, substr) {
			return true
		}
	}

	return false
}

// getDeviceContainer extracts the base container device name from an APFS volume device.
func getDeviceContainer(device string) string {
	if device == "" {
		return ""
	}

	for i, ch := range device {
		if ch == 's' && i > 0 {
			return device[:i]
		}
	}

	return device
}

// CPU returns detailed CPU information including usage percentages.
func (s *DefaultService) CPU(_ context.Context) (*monitor.CPUInfo, error) {
	cached := s.cpuCache.Load()
	if cached == nil {
		return nil, ErrCPUInfoNotReady
	}

	return cached.(*monitor.CPUInfo), nil
}

// Memory returns memory usage information.
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
func (s *DefaultService) Process(_ context.Context) (*monitor.ProcessInfo, error) {
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

// BuildInfo returns application build information if available.
func (s *DefaultService) BuildInfo() *monitor.BuildInfo {
	if s.buildInfo == nil {
		return &monitor.BuildInfo{
			VEFVersion: version.VEFVersion,
			AppVersion: "unknown",
			BuildTime:  "unknown",
			GitCommit:  "unknown",
		}
	}

	return s.buildInfo
}

// Init starts background goroutines to periodically sample CPU and process metrics.
func (s *DefaultService) Init(context.Context) error {
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

	if perCorePercent, err := cpu.PercentWithContext(ctx, s.config.SampleDuration, true); err == nil {
		cpuInfo.UsagePercent = perCorePercent
	}

	if totalPercent, err := cpu.PercentWithContext(ctx, 0, false); err == nil && len(totalPercent) > 0 {
		cpuInfo.TotalPercent = totalPercent[0]
	}

	return &cpuInfo, nil
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

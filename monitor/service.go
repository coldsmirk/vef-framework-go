package monitor

import "context"

// SystemOverview provides a comprehensive snapshot of all system metrics.
type SystemOverview struct {
	Host    *HostSummary    `json:"host"`
	CPU     *CPUSummary     `json:"cpu"`
	Memory  *MemorySummary  `json:"memory"`
	Disk    *DiskSummary    `json:"disk"`
	Network *NetworkSummary `json:"network"`
	Process *ProcessSummary `json:"process"`
	Load    *LoadInfo       `json:"load"`
	Build   *BuildInfo      `json:"build"`
}

// HostSummary provides a summary of host information.
type HostSummary struct {
	Hostname        string `json:"hostname"`
	OS              string `json:"os"`
	Platform        string `json:"platform"`
	PlatformVersion string `json:"platformVersion"`
	KernelVersion   string `json:"kernelVersion"`
	KernelArch      string `json:"kernelArch"`
	Uptime          uint64 `json:"uptime"`
}

// HostInfo contains detailed static information about the host system.
type HostInfo struct {
	Hostname             string `json:"hostname"`
	Uptime               uint64 `json:"uptime"`
	BootTime             uint64 `json:"bootTime"`
	Processes            uint64 `json:"processes"`
	OS                   string `json:"os"`
	Platform             string `json:"platform"`
	PlatformFamily       string `json:"platformFamily"`
	PlatformVersion      string `json:"platformVersion"`
	KernelVersion        string `json:"kernelVersion"`
	KernelArch           string `json:"kernelArch"`
	VirtualizationSystem string `json:"virtualizationSystem"`
	VirtualizationRole   string `json:"virtualizationRole"`
	HostID               string `json:"hostId"`
}

// CPUSummary provides a summary of CPU metrics for the overview.
type CPUSummary struct {
	PhysicalCores int     `json:"physicalCores"`
	LogicalCores  int     `json:"logicalCores"`
	UsagePercent  float64 `json:"usagePercent"`
}

// CPUInfo contains detailed CPU information including per-core usage.
type CPUInfo struct {
	PhysicalCores int       `json:"physicalCores"`
	LogicalCores  int       `json:"logicalCores"`
	ModelName     string    `json:"modelName"`
	Mhz           float64   `json:"mhz"`
	CacheSize     int32     `json:"cacheSize"`
	UsagePercent  []float64 `json:"usagePercent"`
	TotalPercent  float64   `json:"totalPercent"`
	VendorID      string    `json:"vendorId"`
	Family        string    `json:"family"`
	Model         string    `json:"model"`
	Stepping      int32     `json:"stepping"`
	Microcode     string    `json:"microcode"`
}

// MemorySummary provides a summary of memory metrics for the overview.
type MemorySummary struct {
	Total       uint64  `json:"total"`
	Used        uint64  `json:"used"`
	UsedPercent float64 `json:"usedPercent"`
}

// MemoryInfo contains detailed memory information.
type MemoryInfo struct {
	Virtual *VirtualMemory `json:"virtual"`
	Swap    *SwapMemory    `json:"swap"`
}

// VirtualMemory represents virtual (physical) memory statistics.
type VirtualMemory struct {
	Total             uint64  `json:"total"`
	Available         uint64  `json:"available"`
	Used              uint64  `json:"used"`
	UsedPercent       float64 `json:"usedPercent"`
	Free              uint64  `json:"free"`
	Active            uint64  `json:"active"`
	Inactive          uint64  `json:"inactive"`
	Wired             uint64  `json:"wired"`
	Laundry           uint64  `json:"laundry"`
	Buffers           uint64  `json:"buffers"`
	Cached            uint64  `json:"cached"`
	WriteBack         uint64  `json:"writeBack"`
	Dirty             uint64  `json:"dirty"`
	WriteBackTmp      uint64  `json:"writeBackTmp"`
	Shared            uint64  `json:"shared"`
	Slab              uint64  `json:"slab"`
	SlabReclaimable   uint64  `json:"slabReclaimable"`
	SlabUnreclaimable uint64  `json:"slabUnreclaimable"`
	PageTables        uint64  `json:"pageTables"`
	SwapCached        uint64  `json:"swapCached"`
	CommitLimit       uint64  `json:"commitLimit"`
	CommittedAs       uint64  `json:"committedAs"`
	HighTotal         uint64  `json:"highTotal"`
	HighFree          uint64  `json:"highFree"`
	LowTotal          uint64  `json:"lowTotal"`
	LowFree           uint64  `json:"lowFree"`
	SwapTotal         uint64  `json:"swapTotal"`
	SwapFree          uint64  `json:"swapFree"`
	Mapped            uint64  `json:"mapped"`
	VMAllocTotal      uint64  `json:"vmAllocTotal"`
	VMAllocUsed       uint64  `json:"vmAllocUsed"`
	VMAllocChunk      uint64  `json:"vmAllocChunk"`
	HugePagesTotal    uint64  `json:"hugePagesTotal"`
	HugePagesFree     uint64  `json:"hugePagesFree"`
	HugePagesReserved uint64  `json:"hugePagesReserved"`
	HugePagesSurplus  uint64  `json:"hugePagesSurplus"`
	HugePageSize      uint64  `json:"hugePageSize"`
	AnonHugePages     uint64  `json:"anonHugePages"`
}

// SwapMemory represents swap memory statistics.
type SwapMemory struct {
	Total          uint64  `json:"total"`
	Used           uint64  `json:"used"`
	Free           uint64  `json:"free"`
	UsedPercent    float64 `json:"usedPercent"`
	SwapIn         uint64  `json:"swapIn"`
	SwapOut        uint64  `json:"swapOut"`
	PageIn         uint64  `json:"pageIn"`
	PageOut        uint64  `json:"pageOut"`
	PageFault      uint64  `json:"pageFault"`
	PageMajorFault uint64  `json:"pageMajorFault"`
}

// DiskSummary provides a summary of disk metrics for the overview.
type DiskSummary struct {
	Total       uint64  `json:"total"`
	Used        uint64  `json:"used"`
	UsedPercent float64 `json:"usedPercent"`
	Partitions  int     `json:"partitions"`
}

// DiskInfo contains detailed disk information including partitions and I/O counters.
type DiskInfo struct {
	Partitions []*PartitionInfo      `json:"partitions"`
	IOCounters map[string]*IOCounter `json:"ioCounters"`
}

// PartitionInfo represents a disk partition.
type PartitionInfo struct {
	Device            string   `json:"device"`
	MountPoint        string   `json:"mountPoint"`
	FSType            string   `json:"fsType"`
	Options           []string `json:"options"`
	Total             uint64   `json:"total"`
	Free              uint64   `json:"free"`
	Used              uint64   `json:"used"`
	UsedPercent       float64  `json:"usedPercent"`
	INodesTotal       uint64   `json:"iNodesTotal"`
	INodesUsed        uint64   `json:"iNodesUsed"`
	INodesFree        uint64   `json:"iNodesFree"`
	INodesUsedPercent float64  `json:"iNodesUsedPercent"`
}

// IOCounter represents disk I/O statistics.
type IOCounter struct {
	ReadCount        uint64 `json:"readCount"`
	MergedReadCount  uint64 `json:"mergedReadCount"`
	WriteCount       uint64 `json:"writeCount"`
	MergedWriteCount uint64 `json:"mergedWriteCount"`
	ReadBytes        uint64 `json:"readBytes"`
	WriteBytes       uint64 `json:"writeBytes"`
	ReadTime         uint64 `json:"readTime"`
	WriteTime        uint64 `json:"writeTime"`
	IOPSInProgress   uint64 `json:"iopsInProgress"`
	IOTime           uint64 `json:"ioTime"`
	WeightedIO       uint64 `json:"weightedIo"`
	Name             string `json:"name"`
	SerialNumber     string `json:"serialNumber"`
	Label            string `json:"label"`
}

// NetworkSummary provides a summary of network metrics for the overview.
type NetworkSummary struct {
	Interfaces  int    `json:"interfaces"`
	BytesSent   uint64 `json:"bytesSent"`
	BytesRecv   uint64 `json:"bytesRecv"`
	PacketsSent uint64 `json:"packetsSent"`
	PacketsRecv uint64 `json:"packetsRecv"`
}

// NetworkInfo contains detailed network interface and I/O information.
type NetworkInfo struct {
	Interfaces []*InterfaceInfo         `json:"interfaces"`
	IOCounters map[string]*NetIOCounter `json:"ioCounters"`
}

// InterfaceInfo represents a network interface.
type InterfaceInfo struct {
	Index        int      `json:"index"`
	MTU          int      `json:"mtu"`
	Name         string   `json:"name"`
	HardwareAddr string   `json:"hardwareAddr"`
	Flags        []string `json:"flags"`
	Addrs        []string `json:"addrs"`
}

// NetIOCounter represents network I/O statistics.
type NetIOCounter struct {
	Name        string `json:"name"`
	BytesSent   uint64 `json:"bytesSent"`
	BytesRecv   uint64 `json:"bytesRecv"`
	PacketsSent uint64 `json:"packetsSent"`
	PacketsRecv uint64 `json:"packetsRecv"`
	ErrorsIn    uint64 `json:"errorsIn"`
	ErrorsOut   uint64 `json:"errorsOut"`
	DroppedIn   uint64 `json:"droppedIn"`
	DroppedOut  uint64 `json:"droppedOut"`
	FIFOIn      uint64 `json:"fifoIn"`
	FIFOOut     uint64 `json:"fifoOut"`
}

// ProcessSummary provides a summary of process metrics for the overview.
type ProcessSummary struct {
	PID           int32   `json:"pid"`
	Name          string  `json:"name"`
	CPUPercent    float64 `json:"cpuPercent"`
	MemoryPercent float32 `json:"memoryPercent"`
}

// ProcessInfo contains detailed information about the current process.
type ProcessInfo struct {
	PID           int32   `json:"pid"`
	ParentPID     int32   `json:"parentPid"`
	Name          string  `json:"name"`
	Exe           string  `json:"exe"`
	CommandLine   string  `json:"commandLine"`
	CWD           string  `json:"cwd"`
	Status        string  `json:"status"`
	Username      string  `json:"username"`
	CreateTime    int64   `json:"createTime"`
	NumThreads    int32   `json:"numThreads"`
	NumFDs        int32   `json:"numFds"`
	CPUPercent    float64 `json:"cpuPercent"`
	MemoryPercent float32 `json:"memoryPercent"`
	MemoryRSS     uint64  `json:"memoryRss"`
	MemoryVMS     uint64  `json:"memoryVms"`
	MemorySwap    uint64  `json:"memorySwap"`
}

// LoadInfo represents system load averages.
type LoadInfo struct {
	Load1  float64 `json:"load1"`
	Load5  float64 `json:"load5"`
	Load15 float64 `json:"load15"`
}

// BuildInfo contains application build metadata.
type BuildInfo struct {
	VEFVersion string `json:"vefVersion"`
	AppVersion string `json:"appVersion"`
	BuildTime  string `json:"buildTime"`
	GitCommit  string `json:"gitCommit"`
}

// Service defines the interface for system monitoring operations.
type Service interface {
	// Overview returns a comprehensive system overview including all metrics.
	Overview(ctx context.Context) (*SystemOverview, error)
	// CPU returns detailed CPU information including usage percentages.
	CPU(ctx context.Context) (*CPUInfo, error)
	// Memory returns memory usage information including virtual and swap memory.
	Memory(ctx context.Context) (*MemoryInfo, error)
	// Disk returns disk usage and partition information.
	Disk(ctx context.Context) (*DiskInfo, error)
	// Network returns network interface and I/O statistics.
	Network(ctx context.Context) (*NetworkInfo, error)
	// Host returns static host information such as OS, platform, and kernel version.
	Host(ctx context.Context) (*HostInfo, error)
	// Process returns information about the current process.
	Process(ctx context.Context) (*ProcessInfo, error)
	// Load returns system load averages.
	Load(ctx context.Context) (*LoadInfo, error)
	// BuildInfo returns application build information if available.
	BuildInfo() *BuildInfo
}

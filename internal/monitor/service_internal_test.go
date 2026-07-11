package monitor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/monitor"
	"github.com/coldsmirk/vef-framework-go/version"
)

func TestGetDeviceContainer(t *testing.T) {
	tests := []struct {
		name   string
		device string
		want   string
	}{
		{name: "MacOSAPFSVolume", device: "/dev/disk1s1", want: "/dev/disk1"},
		{name: "MacOSAPFSHigherVolume", device: "/dev/disk2s3", want: "/dev/disk2"},
		{name: "MacOSSecondPhysicalDisk", device: "/dev/disk3s1", want: "/dev/disk3"},
		{name: "MacOSSealedSystemSnapshot", device: "/dev/disk3s1s1", want: "/dev/disk3"},
		{name: "MacOSDeeplyNestedSlices", device: "/dev/disk1s5s1s2", want: "/dev/disk1"},
		{name: "LinuxSATAPartition", device: "/dev/sda1", want: "/dev/sda"},
		{name: "LinuxSATASecondDisk", device: "/dev/sdb2", want: "/dev/sdb"},
		{name: "LinuxNVMePartition", device: "/dev/nvme0n1p1", want: "/dev/nvme0n1"},
		{name: "LinuxNVMeSecondPartition", device: "/dev/nvme0n1p2", want: "/dev/nvme0n1"},
		{name: "LinuxNVMeWholeNamespace", device: "/dev/nvme0n1", want: "/dev/nvme0n1"},
		{name: "LinuxNVMeSecondNamespace", device: "/dev/nvme0n2", want: "/dev/nvme0n2"},
		{name: "LinuxDeviceMapper", device: "/dev/dm-0", want: "/dev/dm-0"},
		{name: "LinuxLoopDevice", device: "/dev/loop0", want: "/dev/loop0"},
		{name: "LinuxEMMCPartition", device: "/dev/mmcblk0p1", want: "/dev/mmcblk0"},
		{name: "LinuxXenVirtualPartition", device: "/dev/xvda1", want: "/dev/xvda"},
		{name: "LinuxLVMMapperVolume", device: "/dev/mapper/vg-data1", want: "/dev/mapper/vg-data1"},
		{name: "LinuxCephRBD", device: "/dev/rbd0", want: "/dev/rbd0"},
		{name: "Empty", device: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getDeviceContainer(tt.device)
			assert.Equal(t, tt.want, got, "Container for %q should strip the partition suffix", tt.device)
		})
	}
}

func TestGetDeviceContainerDeduplicatesSiblings(t *testing.T) {
	// Sibling partitions of the same physical disk must collapse to one key,
	// while distinct physical disks must keep distinct keys.
	assert.Equal(t, getDeviceContainer("/dev/disk1s1"), getDeviceContainer("/dev/disk1s2"),
		"Sibling APFS volumes on disk1 should share a container key")
	assert.Equal(t, getDeviceContainer("/dev/disk3s1s1"), getDeviceContainer("/dev/disk3s5"),
		"The sealed root snapshot and the Data volume share one APFS container and must dedup together")
	assert.NotEqual(t, getDeviceContainer("/dev/disk1s1"), getDeviceContainer("/dev/disk2s1"),
		"Devices disk1 and disk2 must not collapse to the same container key")
	assert.NotEqual(t, getDeviceContainer("/dev/sda1"), getDeviceContainer("/dev/sdb1"),
		"Devices sda and sdb must not collapse to the same container key")
	assert.Equal(t, getDeviceContainer("/dev/nvme0n1p1"), getDeviceContainer("/dev/nvme0n1p2"),
		"Sibling partitions on the same NVMe namespace should share a container key")
	assert.NotEqual(t, getDeviceContainer("/dev/nvme0n1"), getDeviceContainer("/dev/nvme0n2"),
		"Distinct NVMe namespaces are independent devices and must not collapse")
	assert.NotEqual(t, getDeviceContainer("/dev/dm-0"), getDeviceContainer("/dev/dm-1"),
		"Distinct device-mapper volumes must not collapse")
	assert.NotEqual(t, getDeviceContainer("/dev/loop0"), getDeviceContainer("/dev/loop1"),
		"Distinct loop devices must not collapse")
	assert.NotEqual(t, getDeviceContainer("/dev/mapper/vg-data1"), getDeviceContainer("/dev/mapper/vg-data2"),
		"Distinct LVM logical volumes must not collapse on their trailing digits")
	assert.NotEqual(t, getDeviceContainer("/dev/rbd0"), getDeviceContainer("/dev/rbd1"),
		"Distinct Ceph RBD images must not collapse")
}

func TestResolveConfig(t *testing.T) {
	defaults := DefaultConfig()

	tests := []struct {
		name string
		in   *config.MonitorConfig
		want config.MonitorConfig
	}{
		{
			name: "NilFallsBackToDefaults",
			in:   nil,
			want: defaults,
		},
		{
			name: "AllZeroFallsBackToDefaults",
			in:   &config.MonitorConfig{},
			want: defaults,
		},
		{
			name: "PartialIntervalOnlyKeepsDefaultDuration",
			in:   &config.MonitorConfig{SampleInterval: 3 * time.Second},
			want: config.MonitorConfig{
				SampleInterval: 3 * time.Second,
				SampleDuration: defaults.SampleDuration,
			},
		},
		{
			name: "PartialDurationOnlyKeepsDefaultInterval",
			in:   &config.MonitorConfig{SampleDuration: 500 * time.Millisecond},
			want: config.MonitorConfig{
				SampleInterval: defaults.SampleInterval,
				SampleDuration: 500 * time.Millisecond,
			},
		},
		{
			name: "FullOverrideWins",
			in: &config.MonitorConfig{
				SampleInterval: 7 * time.Second,
				SampleDuration: time.Second,
			},
			want: config.MonitorConfig{
				SampleInterval: 7 * time.Second,
				SampleDuration: time.Second,
			},
		},
		{
			name: "ExcludedMountsArePreserved",
			in: &config.MonitorConfig{
				SampleInterval: 7 * time.Second,
				SampleDuration: time.Second,
				ExcludedMounts: []string{"OrbStack"},
			},
			want: config.MonitorConfig{
				SampleInterval: 7 * time.Second,
				SampleDuration: time.Second,
				ExcludedMounts: []string{"OrbStack"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveConfig(tt.in)
			assert.Equal(t, tt.want, got, "Resolved config should apply default-then-override precedence")
		})
	}
}

func TestResolveBuildInfo(t *testing.T) {
	t.Run("NilFallsBackToUnknownAndStampsVersion", func(t *testing.T) {
		got := resolveBuildInfo(nil)
		assert.NotNil(t, got, "Resolved build info must never be nil")
		assert.Equal(t, "unknown", got.AppVersion, "Nil build info should default AppVersion to unknown")
		assert.Equal(t, "unknown", got.BuildTime, "Nil build info should default BuildTime to unknown")
		assert.Equal(t, "unknown", got.GitCommit, "Nil build info should default GitCommit to unknown")
		assert.Equal(t, version.VEFVersion, got.VEFVersion, "VEFVersion should be stamped")
	})

	t.Run("SuppliedInfoIsKeptAndVersionStamped", func(t *testing.T) {
		got := resolveBuildInfo(&monitor.BuildInfo{
			AppVersion: "v1.2.3",
			BuildTime:  "2024-01-01T00:00:00Z",
			GitCommit:  "abc123",
		})
		assert.Equal(t, "v1.2.3", got.AppVersion, "Supplied AppVersion should be preserved")
		assert.Equal(t, "2024-01-01T00:00:00Z", got.BuildTime, "Supplied BuildTime should be preserved")
		assert.Equal(t, "abc123", got.GitCommit, "Supplied GitCommit should be preserved")
		assert.Equal(t, version.VEFVersion, got.VEFVersion, "VEFVersion should override any supplied value")
	})
}

func TestShouldSkipMountPoint(t *testing.T) {
	tests := []struct {
		name       string
		excluded   []string
		mountPoint string
		want       bool
	}{
		{name: "Empty", mountPoint: "", want: true},
		{name: "RealRootKept", mountPoint: "/", want: false},
		{name: "RealDataVolumeKept", mountPoint: "/data", want: false},
		{name: "OSPseudoMountSkipped", mountPoint: "/proc/sys", want: true},
		{name: "MacOSSystemVolumeSkipped", mountPoint: "/System/Volumes/Data", want: true},
		{name: "ExactPseudoMountRootSkipped", mountPoint: "/dev", want: true},
		{name: "SiblingOfPseudoMountKept", mountPoint: "/devdata", want: false},
		{name: "RecoveryVolumeItselfSkipped", mountPoint: "/Volumes/Recovery", want: true},
		{name: "VolumeSharingRecoveryPrefixKept", mountPoint: "/Volumes/RecoveryPlan", want: false},
		{
			name:       "VendorMountSkippedOnlyWhenConfigured",
			excluded:   []string{"OrbStack"},
			mountPoint: "/Users/me/OrbStack",
			want:       true,
		},
		{
			name:       "VendorMountKeptWhenNotConfigured",
			mountPoint: "/Users/me/OrbStack",
			want:       false,
		},
		{
			name:       "EmptyConfiguredSubstringIgnored",
			excluded:   []string{""},
			mountPoint: "/data",
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &DefaultService{config: config.MonitorConfig{ExcludedMounts: tt.excluded}}
			got := s.shouldSkipMountPoint(tt.mountPoint)
			assert.Equal(t, tt.want, got, "Skip decision for %q with excludes %v should match", tt.mountPoint, tt.excluded)
		})
	}
}

func TestBuildDiskSummary(t *testing.T) {
	t.Run("PartitionsCountsOnlyContributingDisks", func(t *testing.T) {
		s := &DefaultService{config: config.MonitorConfig{}}

		// Two sibling slices of disk1 (dedup to one), a distinct disk2, plus a
		// pseudo-mount that must be skipped entirely. The summary must report the
		// de-duplicated contributing count, consistent with Total/Used.
		info := &monitor.DiskInfo{
			Partitions: []*monitor.PartitionInfo{
				{Device: "/dev/disk1s1", MountPoint: "/", Total: 100, Used: 40},
				{Device: "/dev/disk1s2", MountPoint: "/data", Total: 100, Used: 40},
				{Device: "/dev/disk2s1", MountPoint: "/mnt", Total: 50, Used: 10},
				{Device: "/dev/disk3s1", MountPoint: "/System/Volumes/Data", Total: 999, Used: 999},
			},
		}

		summary := s.buildDiskSummary(info)

		assert.Equal(t, 2, summary.Partitions, "Partitions must count only the de-duplicated, non-skipped disks (disk1 + disk2)")
		assert.Equal(t, uint64(150), summary.Total, "Total must sum only the contributing disks (disk1 first slice + disk2)")
		assert.Equal(t, uint64(50), summary.Used, "Used must sum only the contributing disks")
		assert.InDelta(t, float64(50)/float64(150)*100, summary.UsedPercent, 0.0001, "UsedPercent derives from the de-duplicated totals")
	})

	t.Run("EmptyPartitionsYieldsZeroes", func(t *testing.T) {
		s := &DefaultService{config: config.MonitorConfig{}}

		summary := s.buildDiskSummary(&monitor.DiskInfo{})

		assert.Equal(t, 0, summary.Partitions, "No partitions means a zero count")
		assert.Equal(t, uint64(0), summary.Total, "No partitions means zero total")
		assert.Equal(t, uint64(0), summary.Used, "No partitions means zero used")
		assert.Equal(t, float64(0), summary.UsedPercent, "Zero total must not divide by zero")
	})

	t.Run("PartitionsWithoutDeviceAreEachCounted", func(t *testing.T) {
		s := &DefaultService{config: config.MonitorConfig{}}

		// Device-less partitions skip the dedup guard, so each contributes once.
		info := &monitor.DiskInfo{
			Partitions: []*monitor.PartitionInfo{
				{Device: "", MountPoint: "/a", Total: 10, Used: 1},
				{Device: "", MountPoint: "/b", Total: 20, Used: 2},
			},
		}

		summary := s.buildDiskSummary(info)

		assert.Equal(t, 2, summary.Partitions, "Device-less partitions are each counted")
		assert.Equal(t, uint64(30), summary.Total, "Total sums both device-less partitions")
	})

	t.Run("MacOSAPFSContainerCountedOnce", func(t *testing.T) {
		s := &DefaultService{config: config.MonitorConfig{}}

		// A faithful modern-macOS layout: the sealed root snapshot, the Data
		// sibling under /System/Volumes, a Time Machine snapshot mount, a
		// second mount of a container volume under /Volumes, and devfs. The
		// shared 494GB container must be counted exactly once, with the
		// container-level consumption (Total - Free), not the root snapshot's
		// own ~10GB.
		info := &monitor.DiskInfo{
			Partitions: []*monitor.PartitionInfo{
				{Device: "/dev/disk3s1s1", MountPoint: "/", FSType: "apfs", Total: 494, Free: 60, Used: 10},
				{Device: "/dev/disk3s5", MountPoint: "/System/Volumes/Data", FSType: "apfs", Total: 494, Free: 60, Used: 400},
				{
					Device:     "com.apple.TimeMachine.2026-07-01-000000.local@/dev/disk3s5",
					MountPoint: "/Volumes/.timemachine/A1B2/2026-07-01-000000.backup",
					FSType:     "apfs",
					Total:      494,
					Free:       60,
					Used:       400,
				},
				{Device: "/dev/disk3s1", MountPoint: "/Volumes/Macintosh HD", FSType: "apfs", Total: 494, Free: 60, Used: 10},
				{Device: "devfs", MountPoint: "/dev", FSType: "devfs", Total: 1, Used: 1},
			},
		}

		summary := s.buildDiskSummary(info)

		assert.Equal(t, 1, summary.Partitions, "One APFS container must yield one counted partition")
		assert.Equal(t, uint64(494), summary.Total, "Total must be the container size, counted once")
		assert.Equal(t, uint64(434), summary.Used, "Used must be the container-level consumption (Total - Free)")
	})
}

func TestIsSnapshotDevice(t *testing.T) {
	tests := []struct {
		name   string
		device string
		fsType string
		want   bool
	}{
		{
			name:   "TimeMachineSnapshotSkipped",
			device: "com.apple.TimeMachine.2026-07-01-000000.local@/dev/disk3s5",
			fsType: "apfs",
			want:   true,
		},
		{name: "RegularAPFSVolumeKept", device: "/dev/disk3s5", fsType: "apfs", want: false},
		{name: "SMBShareWithAtSignKept", device: "//user@fileserver/share", fsType: "smbfs", want: false},
		{name: "LinuxPartitionKept", device: "/dev/sda1", fsType: "ext4", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isSnapshotDevice(tt.device, tt.fsType)
			assert.Equal(t, tt.want, got, "Snapshot decision for %q (%s) should match", tt.device, tt.fsType)
		})
	}
}

func TestPartitionUsed(t *testing.T) {
	tests := []struct {
		name string
		part monitor.PartitionInfo
		want uint64
	}{
		{
			name: "APFSUsesContainerConsumption",
			part: monitor.PartitionInfo{FSType: "apfs", Total: 494, Free: 60, Used: 10},
			want: 434,
		},
		{
			name: "APFSWithInconsistentFreeFallsBackToUsed",
			part: monitor.PartitionInfo{FSType: "apfs", Total: 10, Free: 20, Used: 3},
			want: 3,
		},
		{
			name: "Ext4KeepsPerVolumeUsed",
			part: monitor.PartitionInfo{FSType: "ext4", Total: 100, Free: 55, Used: 40},
			want: 40,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := partitionUsed(&tt.part)
			assert.Equal(t, tt.want, got, "Used bytes for %s should match", tt.part.FSType)
		})
	}
}

func TestApplyCgroupMemoryLimit(t *testing.T) {
	limitedReader := func(t *testing.T, limit, current string) *cgroupReader {
		t.Helper()

		root := newV2Root(t)
		writeCgroupFile(t, root, "sys/fs/cgroup/memory.max", limit)

		if current != "" {
			writeCgroupFile(t, root, "sys/fs/cgroup/memory.current", current)
		}

		return &cgroupReader{root: root}
	}

	hostView := func() *monitor.VirtualMemory {
		return &monitor.VirtualMemory{
			Total:       64 * 1 << 30,
			Used:        32 * 1 << 30,
			Available:   32 * 1 << 30,
			Free:        16 * 1 << 30,
			UsedPercent: 50,
		}
	}

	t.Run("LimitOverridesHeadlineFigures", func(t *testing.T) {
		s := &DefaultService{cgroups: limitedReader(t, "536870912\n", "268435456\n")}

		virtual := hostView()
		s.applyCgroupMemoryLimit(virtual)

		assert.Equal(t, uint64(536870912), virtual.Total, "Total becomes the cgroup limit")
		assert.Equal(t, uint64(268435456), virtual.Used, "Used becomes the cgroup working set")
		assert.Equal(t, uint64(268435456), virtual.Available, "Available is limit minus used")
		assert.Equal(t, uint64(268435456), virtual.Free, "Free is limit minus used")
		assert.InDelta(t, 50.0, virtual.UsedPercent, 0.0001, "UsedPercent derives from the limit")
	})

	t.Run("LimitAtOrAboveHostTotalIsIgnored", func(t *testing.T) {
		s := &DefaultService{cgroups: limitedReader(t, "68719476736\n", "268435456\n")}

		virtual := hostView()
		want := *hostView()

		s.applyCgroupMemoryLimit(virtual)

		assert.Equal(t, want, *virtual, "A limit that equals the host total constrains nothing and must not rewrite the host view")
	})

	t.Run("UnreadableUsageKeepsHostView", func(t *testing.T) {
		s := &DefaultService{cgroups: limitedReader(t, "536870912\n", "")}

		virtual := hostView()
		want := *hostView()

		s.applyCgroupMemoryLimit(virtual)

		assert.Equal(t, want, *virtual, "A limit without a readable usage counter must not produce a mixed view")
	})

	t.Run("UnlimitedContainerKeepsHostView", func(t *testing.T) {
		s := &DefaultService{cgroups: limitedReader(t, "max\n", "268435456\n")}

		virtual := hostView()
		want := *hostView()

		s.applyCgroupMemoryLimit(virtual)

		assert.Equal(t, want, *virtual, "No limit means the host view is authoritative")
	})

	t.Run("UsageIsClampedToLimit", func(t *testing.T) {
		s := &DefaultService{cgroups: limitedReader(t, "536870912\n", "1073741824\n")}

		virtual := hostView()
		s.applyCgroupMemoryLimit(virtual)

		assert.Equal(t, uint64(536870912), virtual.Used, "Usage above the limit is clamped")
		assert.Equal(t, uint64(0), virtual.Available, "Clamped usage leaves nothing available")
		assert.InDelta(t, 100.0, virtual.UsedPercent, 0.0001, "Clamped usage saturates the percentage")
	})
}

func TestApplyCgroupCPUQuota(t *testing.T) {
	readerWithUsage := func(t *testing.T, usageUsec string) *cgroupReader {
		t.Helper()

		root := newV2Root(t)
		writeCgroupFile(t, root, "sys/fs/cgroup/cpu.max", "50000 100000\n")
		writeCgroupFile(t, root, "sys/fs/cgroup/cpu.stat", "usage_usec "+usageUsec+"\n")

		return &cgroupReader{root: root}
	}

	t.Run("CoresBecomeQuotaCeiling", func(t *testing.T) {
		s := &DefaultService{cgroups: readerWithUsage(t, "1000")}

		cpuInfo := &monitor.CPUInfo{PhysicalCores: 8, LogicalCores: 16, UsagePercent: []float64{1, 2}}
		s.applyCgroupCPUQuota(cpuInfo, 0.5, 0, false, time.Now())

		assert.Equal(t, 1, cpuInfo.PhysicalCores, "A fractional quota rounds up to one core")
		assert.Equal(t, 1, cpuInfo.LogicalCores, "Logical cores match the quota ceiling")
		assert.NotNil(t, cpuInfo.UsagePercent, "Without a usage baseline the host percentages are kept")

		cpuInfo = &monitor.CPUInfo{PhysicalCores: 8, LogicalCores: 16}
		s.applyCgroupCPUQuota(cpuInfo, 2.5, 0, false, time.Now())

		assert.Equal(t, 3, cpuInfo.PhysicalCores, "A 2.5-core quota rounds up to three cores")
	})

	t.Run("TotalPercentIsShareOfQuota", func(t *testing.T) {
		// Counter at 250000µs, baseline at 150000µs: 100ms of CPU over a
		// ~400ms window with a 0.5-core quota is ~50% of the budget. The
		// window is measured with time.Since, so allow scheduling jitter.
		s := &DefaultService{cgroups: readerWithUsage(t, "250000")}

		cpuInfo := &monitor.CPUInfo{TotalPercent: 87, UsagePercent: []float64{1, 2}}
		s.applyCgroupCPUQuota(cpuInfo, 0.5, 150*time.Millisecond, true, time.Now().Add(-400*time.Millisecond))

		assert.InDelta(t, 50.0, cpuInfo.TotalPercent, 5, "A 100ms delta over a 400ms window is half of a 0.5-core quota")
		assert.Nil(t, cpuInfo.UsagePercent, "The host per-core breakdown is dropped under a quota")
	})

	t.Run("ZeroDeltaIsZeroPercent", func(t *testing.T) {
		// The fake usage counter is static, so a baseline equal to the file's
		// value yields a zero delta: 0% of the quota was consumed.
		s := &DefaultService{cgroups: readerWithUsage(t, "250000")}

		cpuInfo := &monitor.CPUInfo{TotalPercent: 87, UsagePercent: []float64{1, 2}}
		s.applyCgroupCPUQuota(cpuInfo, 0.5, 250*time.Millisecond, true, time.Now().Add(-100*time.Millisecond))

		assert.InDelta(t, 0.0, cpuInfo.TotalPercent, 0.0001, "A zero usage delta is zero percent of the quota")
		assert.Nil(t, cpuInfo.UsagePercent, "The host per-core breakdown is dropped under a quota")
	})

	t.Run("PercentIsClampedToHundred", func(t *testing.T) {
		// 250ms of CPU against a 100ms × 0.5-core budget is 500%; the report
		// must saturate at the limit instead of leaking scheduler jitter.
		s := &DefaultService{cgroups: readerWithUsage(t, "250000")}

		cpuInfo := &monitor.CPUInfo{TotalPercent: 87}
		s.applyCgroupCPUQuota(cpuInfo, 0.5, 0, true, time.Now().Add(-100*time.Millisecond))

		assert.InDelta(t, 100.0, cpuInfo.TotalPercent, 0.0001, "Consumption beyond the quota budget clamps to 100")
	})

	t.Run("BackwardsUsageCounterKeepsHostPercent", func(t *testing.T) {
		s := &DefaultService{cgroups: readerWithUsage(t, "1000")}

		cpuInfo := &monitor.CPUInfo{TotalPercent: 87}
		s.applyCgroupCPUQuota(cpuInfo, 0.5, time.Hour, true, time.Now().Add(-100*time.Millisecond))

		assert.InDelta(t, 87.0, cpuInfo.TotalPercent, 0.0001, "A counter that moved backwards must not produce a bogus percentage")
	})
}

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
		{name: "LinuxSATAPartition", device: "/dev/sda1", want: "/dev/sda"},
		{name: "LinuxSATASecondDisk", device: "/dev/sdb2", want: "/dev/sdb"},
		{name: "LinuxNVMePartition", device: "/dev/nvme0n1p1", want: "/dev/nvme0n1"},
		{name: "LinuxNVMeSecondPartition", device: "/dev/nvme0n1p2", want: "/dev/nvme0n1"},
		{name: "LinuxNVMeWholeNamespace", device: "/dev/nvme0n1", want: "/dev/nvme0n1"},
		{name: "LinuxNVMeSecondNamespace", device: "/dev/nvme0n2", want: "/dev/nvme0n2"},
		{name: "LinuxDeviceMapper", device: "/dev/dm-0", want: "/dev/dm-0"},
		{name: "LinuxLoopDevice", device: "/dev/loop0", want: "/dev/loop0"},
		{name: "LinuxEMMCPartition", device: "/dev/mmcblk0p1", want: "/dev/mmcblk0"},
		{name: "Empty", device: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getDeviceContainer(tt.device)
			assert.Equal(t, tt.want, got, "container for %q should strip the partition suffix", tt.device)
		})
	}
}

func TestGetDeviceContainerDeduplicatesSiblings(t *testing.T) {
	// Sibling partitions of the same physical disk must collapse to one key,
	// while distinct physical disks must keep distinct keys.
	assert.Equal(t, getDeviceContainer("/dev/disk1s1"), getDeviceContainer("/dev/disk1s2"),
		"sibling APFS volumes on disk1 should share a container key")
	assert.NotEqual(t, getDeviceContainer("/dev/disk1s1"), getDeviceContainer("/dev/disk2s1"),
		"disk1 and disk2 must not collapse to the same container key")
	assert.NotEqual(t, getDeviceContainer("/dev/sda1"), getDeviceContainer("/dev/sdb1"),
		"sda and sdb must not collapse to the same container key")
	assert.Equal(t, getDeviceContainer("/dev/nvme0n1p1"), getDeviceContainer("/dev/nvme0n1p2"),
		"sibling partitions on the same NVMe namespace should share a container key")
	assert.NotEqual(t, getDeviceContainer("/dev/nvme0n1"), getDeviceContainer("/dev/nvme0n2"),
		"distinct NVMe namespaces are independent devices and must not collapse")
	assert.NotEqual(t, getDeviceContainer("/dev/dm-0"), getDeviceContainer("/dev/dm-1"),
		"distinct device-mapper volumes must not collapse")
	assert.NotEqual(t, getDeviceContainer("/dev/loop0"), getDeviceContainer("/dev/loop1"),
		"distinct loop devices must not collapse")
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
			assert.Equal(t, tt.want, got, "resolved config should apply default-then-override precedence")
		})
	}
}

func TestResolveBuildInfo(t *testing.T) {
	t.Run("NilFallsBackToUnknownAndStampsVersion", func(t *testing.T) {
		got := resolveBuildInfo(nil)
		assert.NotNil(t, got, "resolved build info must never be nil")
		assert.Equal(t, "unknown", got.AppVersion, "nil build info should default AppVersion to unknown")
		assert.Equal(t, "unknown", got.BuildTime, "nil build info should default BuildTime to unknown")
		assert.Equal(t, "unknown", got.GitCommit, "nil build info should default GitCommit to unknown")
		assert.Equal(t, version.VEFVersion, got.VEFVersion, "VEFVersion should be stamped")
	})

	t.Run("SuppliedInfoIsKeptAndVersionStamped", func(t *testing.T) {
		got := resolveBuildInfo(&monitor.BuildInfo{
			AppVersion: "v1.2.3",
			BuildTime:  "2024-01-01T00:00:00Z",
			GitCommit:  "abc123",
		})
		assert.Equal(t, "v1.2.3", got.AppVersion, "supplied AppVersion should be preserved")
		assert.Equal(t, "2024-01-01T00:00:00Z", got.BuildTime, "supplied BuildTime should be preserved")
		assert.Equal(t, "abc123", got.GitCommit, "supplied GitCommit should be preserved")
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
			assert.Equal(t, tt.want, got, "skip decision for %q with excludes %v", tt.mountPoint, tt.excluded)
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

		assert.Equal(t, 0, summary.Partitions, "no partitions means a zero count")
		assert.Equal(t, uint64(0), summary.Total, "no partitions means zero total")
		assert.Equal(t, uint64(0), summary.Used, "no partitions means zero used")
		assert.Equal(t, float64(0), summary.UsedPercent, "zero total must not divide by zero")
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

		assert.Equal(t, 2, summary.Partitions, "device-less partitions are each counted")
		assert.Equal(t, uint64(30), summary.Total, "Total sums both device-less partitions")
	})
}

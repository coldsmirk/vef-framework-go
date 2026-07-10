package monitor

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/shirou/gopsutil/v4/disk"

	"github.com/coldsmirk/vef-framework-go/monitor"
)

// disk.go computes the disk overview using the node_exporter model.
//
// # Why the node_exporter model
//
// A container is isolated by its mount namespace: it can only see its own
// overlay root ("/") plus whatever is explicitly bind-mounted into it. It cannot
// see the host's other disks (a separate /data disk, the disks other co-located
// services write to, etc.) — that is a kernel guarantee, not something code can
// work around. Reading disk.Usage("/") inside a container therefore reports only
// the overlay's backing disk and silently omits every other server disk.
//
// Prometheus node_exporter solves this the standard way, and we mirror it:
//
//  1. Deployment bind-mounts the host filesystems into the container read-only,
//     e.g. in docker-compose:
//         volumes:
//           - /:/host:ro,rslave      # host root (rslave brings in submounts like /data)
//           - /proc:/host/proc:ro    # host mount table
//  2. The service is told where those live, mirroring node_exporter's flags:
//         monitor.procfs_path = "/host/proc"   # node_exporter --path.procfs
//         monitor.rootfs_path = "/host"        # node_exporter --path.rootfs
//
// hostDiskSummary then reads the host mount table from <procfs>/1/mounts,
// statfs()es each real filesystem at <rootfs><mountpoint>, deduplicates by
// backing device (so a filesystem mounted at several points — or the /etc/*
// bind-mounts and overlay noise — is counted once), and aggregates total/used
// across the distinct devices. The result is the true server-wide disk usage,
// including extra data disks such as /data.
//
// Bare-metal / native deployments need no configuration: the defaults
// (procfs "/proc", rootfs "/") read the host's own mount table directly. On
// non-Linux hosts (dev machines) there is no /proc mount table in this format,
// so we fall back to statfs of the root volume, which on macOS/APFS already
// reflects whole-disk usage.

// pseudoFSTypes are non-storage filesystems that never count toward disk totals.
var pseudoFSTypes = map[string]bool{
	"sysfs": true, "proc": true, "procfs": true, "tmpfs": true, "devtmpfs": true,
	"devpts": true, "cgroup": true, "cgroup2": true, "overlay": true, "aufs": true,
	"mqueue": true, "hugetlbfs": true, "debugfs": true, "tracefs": true,
	"securityfs": true, "pstore": true, "bpf": true, "autofs": true,
	"binfmt_misc": true, "configfs": true, "fusectl": true, "nsfs": true,
	"ramfs": true, "rpc_pipefs": true, "squashfs": true, "efivarfs": true,
	"selinuxfs": true, "fuse.lxcfs": true, "fuse.gvfsd-fuse": true, "mtd_inodefs": true,
}

// diskSummary returns the disk overview: the node_exporter aggregation on Linux,
// or a root-volume statfs on other platforms.
func (s *DefaultService) diskSummary(ctx context.Context) (*monitor.DiskSummary, error) {
	if runtime.GOOS == "linux" {
		if summary, ok := s.hostDiskSummary(ctx); ok {
			return summary, nil
		}

		logger.Warnf("disk: host mount table unreadable; reporting the root volume only " +
			"(set procfs_path/rootfs_path to read the host disks from a container)")
	}

	return rootDiskSummary(ctx)
}

// hostFilesystem is one real, statfs'd filesystem from the host mount table.
type hostFilesystem struct {
	device     string
	mountPoint string
	fsType     string
	usage      *disk.UsageStat
}

// hostFilesystems enumerates the distinct real filesystems from the (host) mount
// table per the node_exporter model: it skips pseudo filesystems, deduplicates so
// a filesystem mounted at several points is counted once, and statfs()es each
// where it is reachable (under the rootfs prefix, or the mount point itself on
// bare metal). Both the overview aggregate and the get_disk detail build on this,
// so the two endpoints report a consistent view. ok is false when the mount table
// cannot be read.
func (s *DefaultService) hostFilesystems(ctx context.Context) ([]hostFilesystem, bool) {
	procfs := firstNonEmpty(s.config.ProcfsPath, "/proc")
	rootfs := firstNonEmpty(s.config.RootfsPath, "/")

	mounts, ok := readMountTable(procfs)
	if !ok {
		return nil, false
	}

	result := make([]hostFilesystem, 0, len(mounts))
	seen := make(map[string]bool)

	for _, m := range mounts {
		if pseudoFSTypes[m.fsType] {
			continue
		}

		// Deduplicate by backing device so bind-mounts (e.g. /etc/hosts) and a disk
		// mounted at several points count once. Device-less mounts (some NFS/fuse)
		// fall back to a fstype+mountpoint key rather than skipping dedup entirely.
		key := m.device
		if key == "" {
			key = m.fsType + "\x00" + m.mountPoint
		}

		if seen[key] {
			continue
		}

		usage, err := disk.UsageWithContext(ctx, filepath.Join(rootfs, m.mountPoint))
		if err != nil || usage.Total == 0 {
			continue
		}

		seen[key] = true

		result = append(result, hostFilesystem{
			device:     m.device,
			mountPoint: m.mountPoint,
			fsType:     m.fsType,
			usage:      usage,
		})
	}

	return result, len(result) > 0
}

// hostDiskSummary aggregates total/used across the distinct host filesystems.
func (s *DefaultService) hostDiskSummary(ctx context.Context) (*monitor.DiskSummary, bool) {
	filesystems, ok := s.hostFilesystems(ctx)
	if !ok {
		return nil, false
	}

	var total, used uint64
	for _, fs := range filesystems {
		total += fs.usage.Total
		used += fs.usage.Used
	}

	var usedPercent float64
	if total > 0 {
		usedPercent = float64(used) / float64(total) * 100
	}

	return &monitor.DiskSummary{
		Total:       total,
		Used:        used,
		UsedPercent: usedPercent,
		Partitions:  len(filesystems),
	}, true
}

// mountEntry is one parsed line of the kernel mount table.
type mountEntry struct {
	device     string
	mountPoint string
	fsType     string
}

// readMountTable parses the mount table from <procfs>/1/mounts (the root mount
// namespace, i.e. the host's when the host /proc is mounted in), falling back to
// <procfs>/mounts. The format is space-separated:
//
//	<device> <mountpoint> <fstype> <options> <dump> <pass>
func readMountTable(procfs string) ([]mountEntry, bool) {
	b, err := os.ReadFile(filepath.Join(procfs, "1", "mounts"))
	if err != nil {
		b, err = os.ReadFile(filepath.Join(procfs, "mounts"))
		if err != nil {
			return nil, false
		}
	}

	var entries []mountEntry

	for line := range strings.SplitSeq(string(b), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		entries = append(entries, mountEntry{
			device:     fields[0],
			mountPoint: unescapeMount(fields[1]),
			fsType:     fields[2],
		})
	}

	if len(entries) == 0 {
		return nil, false
	}

	return entries, true
}

// unescapeMount decodes the octal escapes the kernel uses in mount paths (space
// is "\040", tab "\011", newline "\012", backslash "\134").
func unescapeMount(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}

	replacer := strings.NewReplacer(`\040`, " ", `\011`, "\t", `\012`, "\n", `\134`, `\`)

	return replacer.Replace(s)
}

// rootDiskSummary reports usage of the root volume the process runs on. It is the
// non-Linux path and the Linux fallback when the mount table cannot be read. On
// macOS/APFS a statfs of "/" already reflects whole-disk usage (volumes share the
// container's free space); on Windows the system drive is used.
func rootDiskSummary(ctx context.Context) (*monitor.DiskSummary, error) {
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

// rootDiskPath returns the path whose filesystem represents the root volume:
// "/" on Unix-like systems, the system drive on Windows.
func rootDiskPath() string {
	if runtime.GOOS == "windows" {
		if drive := os.Getenv("SystemDrive"); drive != "" {
			return drive + `\`
		}

		return `C:\`
	}

	return "/"
}

// firstNonEmpty returns v if non-empty, otherwise def.
func firstNonEmpty(v, def string) string {
	if v == "" {
		return def
	}

	return v
}

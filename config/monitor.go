package config

import "time"

// MonitorConfig defines monitoring service settings.
type MonitorConfig struct {
	SampleInterval time.Duration `config:"sample_interval"` // Interval between samples (default: 10s)
	SampleDuration time.Duration `config:"sample_duration"` // Sampling window duration (default: 2s)

	// ProcfsPath and RootfsPath follow the Prometheus node_exporter model for
	// reading the real server disks from inside a container. They default to the
	// process's own view ("/proc" and "/"), which is correct on bare metal.
	//
	// In a container, bind-mount the host filesystems read-only and point these at
	// them so disk statistics reflect the whole server (root plus extra data disks
	// such as /data) rather than only the container's overlay layer:
	//
	//	volumes:
	//	  - /:/host:ro,rslave
	//	  - /proc:/host/proc:ro
	//	# then:
	//	#   procfs_path = "/host/proc"   (node_exporter --path.procfs)
	//	#   rootfs_path = "/host"        (node_exporter --path.rootfs)
	//
	// A container cannot see host disks that are not mounted in; without these
	// mounts the monitor can only report the container's own overlay layer.
	//
	// An empty value falls back to the default ("/proc" and "/" respectively), so
	// they never need to be set on bare metal.
	ProcfsPath string `config:"procfs_path"`
	RootfsPath string `config:"rootfs_path"`
}

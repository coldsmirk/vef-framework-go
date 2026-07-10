package monitor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
)

func TestUnescapeMount(t *testing.T) {
	assert.Equal(t, "/data", unescapeMount("/data"), "plain paths pass through")
	assert.Equal(t, "/mnt/my disk", unescapeMount(`/mnt/my\040disk`), "\\040 decodes to a space")
	assert.Equal(t, "/a\tb", unescapeMount(`/a\011b`), "\\011 decodes to a tab")
}

func TestReadMountTable(t *testing.T) {
	t.Run("prefers proc/1/mounts", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "1"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "1", "mounts"),
			[]byte("/dev/sda1 / ext4 rw 0 0\nproc /proc proc rw 0 0\n"), 0o644))
		// A different self view must be ignored in favor of /1/mounts.
		require.NoError(t, os.WriteFile(filepath.Join(dir, "mounts"), []byte("wrong / ext4 rw 0 0\n"), 0o644))

		entries, ok := readMountTable(dir)
		require.True(t, ok)
		require.Len(t, entries, 2)
		assert.Equal(t, "/dev/sda1", entries[0].device)
		assert.Equal(t, "/", entries[0].mountPoint)
		assert.Equal(t, "ext4", entries[0].fsType)
	})

	t.Run("falls back to proc/mounts", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "mounts"), []byte("/dev/vda1 / xfs rw 0 0\n"), 0o644))

		entries, ok := readMountTable(dir)
		require.True(t, ok)
		require.Len(t, entries, 1)
		assert.Equal(t, "xfs", entries[0].fsType)
	})

	t.Run("missing returns false", func(t *testing.T) {
		_, ok := readMountTable(t.TempDir())
		assert.False(t, ok)
	})
}

// TestHostDiskSummary verifies the node_exporter aggregation: pseudo filesystems
// are skipped, a device mounted more than once is counted a single time, and the
// remaining real filesystems are summed. Real mount points (temp dirs, all on the
// same backing fs) are used so statfs returns real numbers.
func TestHostDiskSummary(t *testing.T) {
	real1, real2 := t.TempDir(), t.TempDir()

	procfs := t.TempDir()
	mounts := fmt.Sprintf(""+
		"/dev/sda1 %s ext4 rw 0 0\n"+ // real, counted
		"proc /proc proc rw 0 0\n"+ // pseudo, skipped
		"/dev/sda1 %s ext4 rw 0 0\n"+ // same device, deduped
		"tmpfs %s tmpfs rw 0 0\n"+ // pseudo, skipped
		"/dev/sdb1 %s xfs rw 0 0\n", // distinct real, counted
		real1, t.TempDir(), t.TempDir(), real2)
	require.NoError(t, os.WriteFile(filepath.Join(procfs, "mounts"), []byte(mounts), 0o644))

	s := &DefaultService{config: config.MonitorConfig{ProcfsPath: procfs, RootfsPath: "/"}}

	summary, ok := s.hostDiskSummary(context.Background())
	require.True(t, ok)
	assert.Equal(t, 2, summary.Partitions, "one deduped sda1 + sdb1; pseudo skipped")
	assert.Positive(t, summary.Total, "aggregated total should be non-zero")
	assert.LessOrEqual(t, summary.Used, summary.Total)
	assert.GreaterOrEqual(t, summary.UsedPercent, 0.0)
	assert.LessOrEqual(t, summary.UsedPercent, 100.0)
}

func TestFirstNonEmpty(t *testing.T) {
	assert.Equal(t, "/proc", firstNonEmpty("", "/proc"))
	assert.Equal(t, "/host/proc", firstNonEmpty("/host/proc", "/proc"))
}

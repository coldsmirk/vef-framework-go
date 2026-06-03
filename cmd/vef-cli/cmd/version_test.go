package cmd

import (
	"runtime/debug"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVersionInfoString(t *testing.T) {
	tests := []struct {
		name string
		info VersionInfo
		want string
	}{
		{"VersionAndDate", VersionInfo{Version: "1.2.3", Date: "2025-01-02 03:04:05"}, "Version: 1.2.3 | Built: 2025-01-02 03:04:05"},
		{"EmptyDateOmitsBuilt", VersionInfo{Version: "1.2.3"}, "Version: 1.2.3"},
		{"DirtyAppendsSuffix", VersionInfo{Version: "1.2.3", Date: "2025-01-02 03:04:05", Dirty: true}, "Version: 1.2.3-dirty | Built: 2025-01-02 03:04:05"},
		{"DirtyWithEmptyDate", VersionInfo{Version: "1.2.3", Dirty: true}, "Version: 1.2.3-dirty"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.info.String(), "String should format version, dirty suffix, and date")
		})
	}
}

func TestShortCommit(t *testing.T) {
	tests := []struct {
		name     string
		revision string
		want     string
	}{
		{"LongRevisionTruncated", "0123456789abcdef", "0123456"},
		{"ExactlySevenKept", "0123456", "0123456"},
		{"ShorterRevisionKept", "012", "012"},
		{"EmptyRevisionKept", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, shortCommit(tt.revision), "shortCommit should truncate to 7 chars only when longer")
		})
	}
}

func TestMergeBuildInfo(t *testing.T) {
	const (
		seedVersion = "0.0.1"
		seedDate    = "seed-date"
	)

	t.Run("RealModuleVersionWinsOverSeed", func(t *testing.T) {
		bi := &debug.BuildInfo{Main: debug.Module{Version: "v1.5.0"}}

		got := mergeBuildInfo(bi, seedVersion, seedDate)

		assert.Equal(t, "v1.5.0", got.Version, "module version should override the ldflags seed")
		assert.Equal(t, seedDate, got.Date, "date should remain the seed when no vcs.time is present")
	})

	t.Run("DevelModuleVersionFallsBackToSeed", func(t *testing.T) {
		bi := &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}

		got := mergeBuildInfo(bi, seedVersion, seedDate)

		assert.Equal(t, seedVersion, got.Version, "(devel) module version should not override the seed")
	})

	t.Run("RevisionAppendedToSeedVersion", func(t *testing.T) {
		bi := &debug.BuildInfo{
			Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "0123456789abcdef"}},
		}

		got := mergeBuildInfo(bi, seedVersion, seedDate)

		assert.Equal(t, "0.0.1-0123456", got.Version, "short commit should be appended when version equals the seed")
	})

	t.Run("RevisionNotAppendedWhenModuleVersionSet", func(t *testing.T) {
		bi := &debug.BuildInfo{
			Main:     debug.Module{Version: "v1.5.0"},
			Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "0123456789abcdef"}},
		}

		got := mergeBuildInfo(bi, seedVersion, seedDate)

		assert.Equal(t, "v1.5.0", got.Version, "revision should not be appended once a real module version wins")
	})

	t.Run("VcsTimeParsedAndFormatted", func(t *testing.T) {
		bi := &debug.BuildInfo{
			Settings: []debug.BuildSetting{{Key: "vcs.time", Value: "2025-01-02T03:04:05Z"}},
		}

		got := mergeBuildInfo(bi, seedVersion, seedDate)

		assert.Equal(t, "2025-01-02 03:04:05", got.Date, "vcs.time should be reformatted to DateTime layout")
	})

	t.Run("InvalidVcsTimeKeepsSeedDate", func(t *testing.T) {
		bi := &debug.BuildInfo{
			Settings: []debug.BuildSetting{{Key: "vcs.time", Value: "not-a-timestamp"}},
		}

		got := mergeBuildInfo(bi, seedVersion, seedDate)

		assert.Equal(t, seedDate, got.Date, "an unparseable vcs.time should leave the seed date intact")
	})

	t.Run("VcsModifiedSetsDirty", func(t *testing.T) {
		bi := &debug.BuildInfo{
			Settings: []debug.BuildSetting{{Key: "vcs.modified", Value: "true"}},
		}

		got := mergeBuildInfo(bi, seedVersion, seedDate)

		assert.True(t, got.Dirty, "vcs.modified=true should mark the build dirty")
	})

	t.Run("VcsModifiedFalseStaysClean", func(t *testing.T) {
		bi := &debug.BuildInfo{
			Settings: []debug.BuildSetting{{Key: "vcs.modified", Value: "false"}},
		}

		got := mergeBuildInfo(bi, seedVersion, seedDate)

		assert.False(t, got.Dirty, "vcs.modified=false should leave the build clean")
	})
}

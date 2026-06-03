package cmd

import (
	"fmt"
	"runtime/debug"
	"time"
)

// VersionInfo holds version and build information.
type VersionInfo struct {
	Version string
	Date    string
	Dirty   bool
}

// GetVersionInfo returns version information from ldflags or runtime/debug.
func GetVersionInfo(ldflagsVersion, ldflagsDate string) VersionInfo {
	buildInfo, ok := debug.ReadBuildInfo()
	if !ok {
		return VersionInfo{Version: ldflagsVersion, Date: ldflagsDate}
	}

	return mergeBuildInfo(buildInfo, ldflagsVersion, ldflagsDate)
}

// mergeBuildInfo derives version information by layering runtime/debug build
// metadata over the ldflags seed. A real module version from the build info wins
// over the seed; otherwise the VCS revision is appended to the seed version.
func mergeBuildInfo(buildInfo *debug.BuildInfo, ldflagsVersion, ldflagsDate string) VersionInfo {
	info := VersionInfo{
		Version: ldflagsVersion,
		Date:    ldflagsDate,
	}

	if buildInfo.Main.Version != "" && buildInfo.Main.Version != "(devel)" {
		info.Version = buildInfo.Main.Version
	}

	for _, setting := range buildInfo.Settings {
		switch setting.Key {
		case "vcs.time":
			if t, err := time.Parse(time.RFC3339, setting.Value); err == nil {
				info.Date = t.Format(time.DateTime)
			}
		case "vcs.modified":
			info.Dirty = setting.Value == "true"
		case "vcs.revision":
			if info.Version == ldflagsVersion {
				info.Version = fmt.Sprintf("%s-%s", info.Version, shortCommit(setting.Value))
			}
		}
	}

	return info
}

// shortCommit truncates a VCS revision to its 7-character short form, leaving
// shorter values untouched.
func shortCommit(revision string) string {
	if len(revision) >= 7 {
		return revision[:7]
	}

	return revision
}

// String formats version info as a readable string. The build date is omitted
// when unknown so unstamped dev builds do not advertise a missing timestamp.
func (v VersionInfo) String() string {
	version := v.Version
	if v.Dirty {
		version += "-dirty"
	}

	if v.Date == "" {
		return fmt.Sprintf("Version: %s", version)
	}

	return fmt.Sprintf("Version: %s | Built: %s", version, v.Date)
}

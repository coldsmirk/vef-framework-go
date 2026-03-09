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
	info := VersionInfo{
		Version: ldflagsVersion,
		Date:    ldflagsDate,
	}

	buildInfo, ok := debug.ReadBuildInfo()
	if !ok {
		return info
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
				shortCommit := setting.Value
				if len(shortCommit) >= 7 {
					shortCommit = shortCommit[:7]
				}

				info.Version = fmt.Sprintf("%s-%s", info.Version, shortCommit)
			}
		}
	}

	return info
}

// String formats version info as a readable string.
func (v VersionInfo) String() string {
	version := v.Version
	if v.Dirty {
		version += "-dirty"
	}

	return fmt.Sprintf("Version: %s | Built: %s", version, v.Date)
}

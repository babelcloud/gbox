package version

import (
	"runtime"
	"time"
)

// These variables will be set at build time via -ldflags
var (
	// Version represents the application version (from git tags)
	Version = "dev"
	// BuildTime is the time when the binary was built
	BuildTime = "unknown"
	// CommitID is the git commit hash
	CommitID = "unknown"
)

// formatBuildTime returns a nicely formatted build time
func formatBuildTime() string {
	if BuildTime == "unknown" {
		return BuildTime
	}

	t, err := time.Parse(time.RFC3339, BuildTime)
	if err != nil {
		return BuildTime
	}

	return t.Format("Mon Jan 2 15:04:05 2006")
}

// ClientInfo returns structured client version information
func ClientInfo() map[string]string {
	return map[string]string{
		"Version":       Version,
		"APIVersion":    "v1",
		"GoVersion":     runtime.Version(),
		"GitCommit":     CommitID,
		"BuildTime":     BuildTime,
		"FormattedTime": formatBuildTime(),
		"OS":            runtime.GOOS,
		"Arch":          runtime.GOARCH,
	}
}

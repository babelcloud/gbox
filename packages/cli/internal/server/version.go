package server

import (
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/babelcloud/gbox/packages/cli/internal/version"
)

// BuildInfo contains build-time information
var BuildInfo = struct {
	Version   string
	BuildTime string
	GitCommit string
	GoVersion string
}{
	Version:   version.Version,
	BuildTime: time.Now().Format(time.RFC3339),
	GitCommit: version.CommitID,
	GoVersion: runtime.Version(),
}

// GetBuildID returns a unique build identifier
func GetBuildID() string {
	// Use build time + git commit + file size as build ID
	// In production, this would be set by build scripts
	execPath, err := os.Executable()
	if err != nil {
		return BuildInfo.BuildTime + "-" + BuildInfo.GitCommit + "-unknown"
	}

	info, err := os.Stat(execPath)
	if err != nil {
		return BuildInfo.BuildTime + "-" + BuildInfo.GitCommit + "-unknown"
	}

	// Use the same format as client for consistency
	buildTime := info.ModTime().Format("2006-01-02T15:04:05") // No timezone, more stable
	gitCommit := "unknown"
	fileSize := info.Size()

	return fmt.Sprintf("%s-%s-%d", buildTime, gitCommit, fileSize)
}

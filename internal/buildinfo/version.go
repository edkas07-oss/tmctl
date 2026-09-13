package buildinfo

import (
	"fmt"
	"runtime"
)

var (
	// Version is the current semantic version of tmctl.
	Version = "0.1.0"
	// GitCommit is the git SHA set during build.
	GitCommit = "unknown"
	// BuildDate is the ISO-8601 build timestamp.
	BuildDate = "unknown"
	// Platform is the target OS and Architecture.
	Platform = fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)
)

// String returns formatted build and version details.
func String() string {
	return fmt.Sprintf("tmctl version %s (%s) build %s [%s]", Version, GitCommit, BuildDate, Platform)
}

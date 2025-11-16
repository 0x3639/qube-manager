package main

import "fmt"

var (
	// Version is the semantic version (set via ldflags during build)
	Version = "dev"

	// GitCommit is the git commit hash (set via ldflags during build)
	GitCommit = "unknown"

	// BuildDate is the build timestamp (set via ldflags during build)
	BuildDate = "unknown"
)

// VersionString returns a formatted version string
func VersionString() string {
	return fmt.Sprintf("qube-manager %s (commit: %s, built: %s)", Version, GitCommit, BuildDate)
}

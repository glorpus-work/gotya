package cli

import (
	"fmt"
	"runtime/debug"
	"strings"

	"github.com/spf13/cobra"
)

// NewVersionCmd creates the version command.
func NewVersionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Long:  "Display version information for gotya",
		Run:   runVersion,
	}

	return cmd
}

func runVersion(*cobra.Command, []string) {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		fmt.Println("Warning: Could not read build information")
		return
	}

	version := getVersion(bi)
	fmt.Printf("gotya version %s\n", version)
	fmt.Printf("Build date: %s\n", getBuildDate(bi.Settings))
	fmt.Printf("Git commit: %s\n", getGitCommit(bi.Settings))
	fmt.Printf("Go version: %s\n", bi.GoVersion)
}

func getVersion(bi *debug.BuildInfo) string {
	version := bi.Main.Version

	// If version is empty or (devel), try to get from VCS
	if version == "" || version == "(devel)" {
		return getVersionFromVCS(bi.Settings)
	}

	// If version looks like a semantic version (contains dots), return as-is
	if strings.Contains(version, ".") {
		return version
	}

	// Otherwise, it's likely a commit hash, return the short version
	return getVersionFromVCS(bi.Settings)
}

func getVersionFromVCS(settings []debug.BuildSetting) string {
	for _, setting := range settings {
		if setting.Key == "vcs.revision" {
			commit := setting.Value
			if len(commit) > 8 {
				commit = commit[:8]
			}
			return commit
		}
	}
	return "unknown"
}

func getBuildDate(settings []debug.BuildSetting) string {
	for _, setting := range settings {
		if setting.Key == "vcs.time" {
			return setting.Value
		}
	}
	return "unknown"
}

func getGitCommit(settings []debug.BuildSetting) string {
	for _, setting := range settings {
		if setting.Key == "vcs.revision" {
			return setting.Value
		}
	}
	return "unknown"
}

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

const (
	Version   = "0.1.0"
	BuildDate = "unknown"
	GitCommit = "unknown"
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
	fmt.Printf("gotya version %s\n", Version)
	fmt.Printf("Build date: %s\n", BuildDate)
	fmt.Printf("Git commit: %s\n", GitCommit)
}

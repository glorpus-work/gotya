package cli

import (
	"github.com/spf13/cobra"
)

// NewIndexCmd creates a new index command
func NewIndexCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "index",
		Short: "Manage repository indexes",
		Long:  `Commands for managing repository indexes.`,
	}

	// Add subcommands
	cmd.AddCommand(
		NewGenerateCmd(),
		// Add other index-related subcommands here
	)

	return cmd
}

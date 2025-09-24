package cli

import (
	"fmt"
	"path/filepath"

	"github.com/cperrin88/gotya/pkg/index"
	"github.com/spf13/cobra"
)

// NewGenerateCmd creates a new command for generating repository indexes.
func NewGenerateCmd() *cobra.Command {
	var (
		basePath string
		force    bool
	)

	cmd := &cobra.Command{
		Use:   "generate <source-dir> <output-file>",
		Short: "Generate a repository index from .gotya artifacts",
		Long: `Generate a repository index (index.json) from a directory containing .gotya artifacts.

The index will include metadata from all .gotya files found in the source directory
and its subdirectories. The output will be written to the specified file.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			sourceDir := args[0]
			outputFile := args[1]

			// Convert to absolute paths for better error messages
			absSourceDir, err := filepath.Abs(sourceDir)
			if err != nil {
				return fmt.Errorf("invalid source directory: %w", err)
			}

			abosluteOutputFile, err := filepath.Abs(outputFile)
			if err != nil {
				return fmt.Errorf("invalid output file: %w", err)
			}

			// Initialize and configure the generator
			gen := index.NewGenerator(absSourceDir, abosluteOutputFile)
			gen.BasePath = basePath
			gen.ForceOverwrite = force

			// Run the generation
			if err := gen.Generate(cmd.Context()); err != nil {
				return fmt.Errorf("failed to generate index: %w", err)
			}

			// Count artifacts for the success message
			count, err := gen.CountArtifacts()
			if err != nil {
				// Non-fatal, just log a warning
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Failed to count artifacts: %v\n", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Successfully generated index with %d artifacts at %s\n",
				count, abosluteOutputFile)
			return nil
		},
	}

	// Add flags
	cmd.Flags().StringVarP(&basePath, "base-path", "b", "",
		"Base path for artifact URLs in the index (e.g., 'packages')")
	cmd.Flags().BoolVarP(&force, "force", "f", false,
		"Overwrite output file if it exists")

	// Add examples
	cmd.Example = `  # Basic usage
  gotya index generate ./artifacts ./repo/index.json

  # With base path for URLs
  gotya index generate --base-path=packages ./artifacts ./repo/index.json

  # Force overwrite of existing index
  gotya index generate --force ./artifacts ./repo/index.json`

	return cmd
}

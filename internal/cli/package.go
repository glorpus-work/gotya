package cli

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"

	pkgpkg "github.com/cperrin88/gotya/pkg/artifact"
)

// NewArtifactCmd creates a new artifact command.
func NewArtifactCmd() *cobra.Command {
	pkgCmd := &cobra.Command{
		Use:   "artifact",
		Short: "Artifact management commands",
		Long:  "Commands for creating and managing gotya packages",
	}

	// Add subcommands
	pkgCmd.AddCommand(newArtifactCreateCommand())

	return pkgCmd
}

// newArtifactCreateCommand creates the 'artifact create' command.
func newArtifactCreateCommand() *cobra.Command {
	// Command line flags
	var (
		sourceDir    string
		outputDir    string
		pkgName      string
		pkgVer       string
		pkgOS        string
		pkgArch      string
		maintainer   string
		description  string
		dependencies []string
		hooks        map[string]string
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new artifact",
		Long: `Create a new gotya artifact from a source directory.
The source directory should contain a 'meta/artifact.json' file and a 'files/' directory.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			// Delegate to the artifact implementation
			_, err := pkgpkg.CreateArtifact(
				sourceDir,
				outputDir,
				pkgName,
				pkgVer,
				pkgOS,
				pkgArch,
				maintainer,
				description,
				dependencies,
				hooks,
			)
			if err != nil {
				return fmt.Errorf("failed to create artifact: %w", err)
			}
			return nil
		},
	}

	// Add flags with descriptions and defaults
	cmd.Flags().StringVarP(&sourceDir, "source", "s", ".", "Source directory containing artifact files (required)")
	cmd.Flags().StringVarP(&outputDir, "output", "o", ".", "Output directory for the created artifact")
	cmd.Flags().StringVar(&pkgName, "name", "", "Artifact name (required, overrides name in artifact.json)")
	cmd.Flags().StringVar(&pkgVer, "version", "0.1.0", "Artifact version (e.g., 1.0.0, overrides version in artifact.json)")
	cmd.Flags().StringVar(&pkgOS, "os", runtime.GOOS, "Target operating system")
	cmd.Flags().StringVar(&pkgArch, "arch", runtime.GOARCH, "Target architecture")
	cmd.Flags().StringVar(&maintainer, "maintainer", "", "Artifact maintainer (name <email>)")
	cmd.Flags().StringVar(&description, "description", "", "Artifact description")
	cmd.Flags().StringSliceVar(&dependencies, "depends", nil, "Artifact dependencies (comma-separated)")
	cmd.Flags().StringToStringVar(&hooks, "hooks", nil, "Artifact hooks in format 'hooks=script.tengo' (comma-separated)")

	// Mark required flags
	if err := cmd.MarkFlagRequired("source"); err != nil {
		// This should never happen since we control the flag names
		panic(fmt.Sprintf("failed to mark source as required: %v", err))
	}
	if err := cmd.MarkFlagRequired("name"); err != nil {
		// This should never happen since we control the flag names
		panic(fmt.Sprintf("failed to mark name as required: %v", err))
	}

	return cmd
}

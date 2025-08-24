package cli

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"

	pkgpkg "github.com/cperrin88/gotya/pkg/pkg"
)

// NewPackageCmd creates a new pkg command.
func NewPackageCmd() *cobra.Command {
	pkgCmd := &cobra.Command{
		Use:   "pkg",
		Short: "Package management commands",
		Long:  "Commands for creating and managing gotya packages",
	}

	// Add subcommands
	pkgCmd.AddCommand(newPackageCreateCommand())

	return pkgCmd
}

// newPackageCreateCommand creates the 'pkg create' command.
func newPackageCreateCommand() *cobra.Command {
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
		Short: "Create a new pkg",
		Long: `Create a new gotya pkg from a source directory.
The source directory should contain a 'meta/pkg.json' file and a 'files/' directory.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			// Delegate to the pkg implementation
			_, err := pkgpkg.CreatePackage(
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
				return fmt.Errorf("failed to create pkg: %w", err)
			}
			return nil
		},
	}

	// Add flags with descriptions and defaults
	cmd.Flags().StringVarP(&sourceDir, "source", "s", ".", "Source directory containing pkg files (required)")
	cmd.Flags().StringVarP(&outputDir, "output", "o", ".", "Output directory for the created pkg")
	cmd.Flags().StringVar(&pkgName, "name", "", "Package name (required, overrides name in pkg.json)")
	cmd.Flags().StringVar(&pkgVer, "version", "0.1.0", "Package version (e.g., 1.0.0, overrides version in pkg.json)")
	cmd.Flags().StringVar(&pkgOS, "os", runtime.GOOS, "Target operating system")
	cmd.Flags().StringVar(&pkgArch, "arch", runtime.GOARCH, "Target architecture")
	cmd.Flags().StringVar(&maintainer, "maintainer", "", "Package maintainer (name <email>)")
	cmd.Flags().StringVar(&description, "description", "", "Package description")
	cmd.Flags().StringSliceVar(&dependencies, "depends", nil, "Package dependencies (comma-separated)")
	cmd.Flags().StringToStringVar(&hooks, "hooks", nil, "Package hooks in format 'hooks=script.tengo' (comma-separated)")

	// Mark required flags
	_ = cmd.MarkFlagRequired("source")
	_ = cmd.MarkFlagRequired("name")

	return cmd
}

package cli

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"

	pkgpkg "github.com/cperrin88/gotya/pkg/package"
)

// NewPackageCmd creates a new package command.
func NewPackageCmd() *cobra.Command {
	pkgCmd := &cobra.Command{
		Use:   "package",
		Short: "Package management commands",
		Long:  "Commands for creating and managing gotya packages",
	}

	// Add subcommands
	pkgCmd.AddCommand(newPackageCreateCommand())

	return pkgCmd
}

// newPackageCreateCommand creates the 'package create' command.
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
		Short: "Create a new package",
		Long: `Create a new gotya package from a source directory.
The source directory should contain a 'meta/package.json' file and a 'files/' directory.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			// Delegate to the package implementation
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
				return fmt.Errorf("failed to create package: %w", err)
			}
			return nil
		},
	}

	// Add flags with descriptions and defaults
	cmd.Flags().StringVarP(&sourceDir, "source", "s", ".", "Source directory containing package files (required)")
	cmd.Flags().StringVarP(&outputDir, "output", "o", ".", "Output directory for the created package")
	cmd.Flags().StringVar(&pkgName, "name", "", "Package name (required, overrides name in package.json)")
	cmd.Flags().StringVar(&pkgVer, "version", "0.1.0", "Package version (e.g., 1.0.0, overrides version in package.json)")
	cmd.Flags().StringVar(&pkgOS, "os", runtime.GOOS, "Target operating system")
	cmd.Flags().StringVar(&pkgArch, "arch", runtime.GOARCH, "Target architecture")
	cmd.Flags().StringVar(&maintainer, "maintainer", "", "Package maintainer (name <email>)")
	cmd.Flags().StringVar(&description, "description", "", "Package description")
	cmd.Flags().StringSliceVar(&dependencies, "depends", nil, "Package dependencies (comma-separated)")
	cmd.Flags().StringToStringVar(&hooks, "hooks", nil, "Package hooks in format 'hook=script.tengo' (comma-separated)")

	// Mark required flags
	_ = cmd.MarkFlagRequired("source")
	_ = cmd.MarkFlagRequired("name")

	return cmd
}

package cli

import (
	"fmt"

	pkgpkg "github.com/cperrin88/gotya/pkg/package"
	"github.com/spf13/cobra"
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
	var (
		sourceDir string
		outputDir string
		pkgName   string
		pkgVer    string
		pkgOS     string
		pkgArch   string
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new package",
		Long: `Create a new gotya package from a source directory.
The source directory should contain a 'meta/package.json' file and a 'files/' directory.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Delegate to the package implementation
			err := pkgpkg.CreatePackage(sourceDir, outputDir, pkgName, pkgVer, pkgOS, pkgArch)
			if err != nil {
				return fmt.Errorf("failed to create package: %w", err)
			}
			return nil
		},
	}

	// Add flags
	cmd.Flags().StringVarP(&sourceDir, "source", "s", ".", "Source directory containing package files")
	cmd.Flags().StringVarP(&outputDir, "output", "o", ".", "Output directory for the created package")
	cmd.Flags().StringVar(&pkgName, "name", "", "Override package name (default: from package.json)")
	cmd.Flags().StringVar(&pkgVer, "version", "", "Override package version (default: from package.json)")
	cmd.Flags().StringVar(&pkgOS, "os", "", "Target OS (default: current OS)")
	cmd.Flags().StringVar(&pkgArch, "arch", "", "Target architecture (default: current arch)")

	// Mark required flags
	_ = cmd.MarkFlagRequired("source")

	return cmd
}

package cli

import (
	"fmt"
	"strings"

	"github.com/cperrin88/gotya/pkg/artifact/database"
	"github.com/cperrin88/gotya/pkg/model"
	"github.com/spf13/cobra"
)

// NewListCmd creates the list command.
func NewListCmd() *cobra.Command {
	var nameFilter string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List installed packages",
		Long: `List all installed packages from the local database.

By default, shows all installed packages with name and version.
Use --name to filter packages by name.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runList(nameFilter)
		},
	}

	cmd.Flags().StringVar(&nameFilter, "name", "", "Filter packages by name (partial match)")

	return cmd
}

func runList(nameFilter string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	// Load the installed database
	db := &database.InstalledManagerImpl{}
	if err := db.LoadDatabase(cfg.GetDatabasePath()); err != nil {
		return fmt.Errorf("failed to load installed database: %w", err)
	}

	// Get filtered installed artifacts using the database method
	artifacts := db.FilteredArtifacts(nameFilter)

	// Display results in tabular format
	if len(artifacts) == 0 {
		fmt.Println("No packages installed")
		return nil
	}

	// Header
	fmt.Printf("%-30s %-15s %s\n", "PACKAGE NAME", "VERSION", "STATUS")
	fmt.Println(strings.Repeat("-", 60))

	// Rows
	for _, artifact := range artifacts {
		status := "installed"
		if artifact.Status == model.StatusMissing {
			status = "missing"
		}
		fmt.Printf("%-30s %-15s %s\n", artifact.Name, artifact.Version, status)
	}

	return nil
}

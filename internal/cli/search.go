package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// NewSearchCmd creates the search command
func NewSearchCmd() *cobra.Command {
	var (
		exactMatch bool
		limit      int
	)

	cmd := &cobra.Command{
		Use:   "search QUERY",
		Short: "Search for packages",
		Long: `Search for packages in the configured repositories by name or description.
Use --exact for exact name matching.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSearch(cmd, args[0], exactMatch, limit)
		},
	}

	cmd.Flags().BoolVar(&exactMatch, "exact", false, "Exact name matching")
	cmd.Flags().IntVar(&limit, "limit", 50, "Limit number of results")

	return cmd
}

// NewListCmd creates the list command
func NewListCmd() *cobra.Command {
	var (
		installed bool
		available bool
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List packages",
		Long:  "List installed or available packages",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(cmd, installed, available)
		},
	}

	cmd.Flags().BoolVar(&installed, "installed", false, "List only installed packages")
	cmd.Flags().BoolVar(&available, "available", false, "List only available packages")

	return cmd
}

func runSearch(cmd *cobra.Command, query string, exactMatch bool, limit int) error {
	_, manager, err := loadConfigAndManager()
	if err != nil {
		return err
	}

	Debug("Searching for packages", logrus.Fields{
		"query":       query,
		"exact_match": exactMatch,
		"limit":       limit,
	})

	results, err := manager.SearchPackages(query, exactMatch, limit)
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}

	if len(results) == 0 {
		Info("No packages found matching query", logrus.Fields{"query": query})
		return nil
	}

	// Display results in table format
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tVERSION\tDESCRIPTION")
	fmt.Fprintln(w, "----\t-------\t-----------")

	for _, pkg := range results {
		description := pkg.Package.Description
		if len(description) > 50 {
			description = description[:47] + "..."
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", pkg.Package.Name, pkg.Package.Version, description)
	}

	w.Flush()
	Info("Search completed", logrus.Fields{"found": len(results)})
	return nil
}

func runList(cmd *cobra.Command, showInstalled, showAvailable bool) error {
	_, manager, err := loadConfigAndManager()
	if err != nil {
		return err
	}

	// Default to showing installed packages if no flags specified
	if !showInstalled && !showAvailable {
		showInstalled = true
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tVERSION\tSTATUS\tDESCRIPTION")
	fmt.Fprintln(w, "----\t-------\t------\t-----------")

	if showInstalled {
		installed, err := manager.GetInstalledPackages()
		if err != nil {
			return fmt.Errorf("failed to get installed packages: %w", err)
		}

		for _, pkg := range installed {
			description := pkg.Description
			if len(description) > 40 {
				description = description[:37] + "..."
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", pkg.Name, pkg.Version, "installed", description)
		}
	}

	if showAvailable {
		available, err := manager.GetAvailablePackages()
		if err != nil {
			return fmt.Errorf("failed to get available packages: %w", err)
		}

		for _, pkg := range available {
			description := pkg.Description
			if len(description) > 40 {
				description = description[:37] + "..."
			}
			status := "available"
			if manager.IsPackageInstalled(pkg.Name) {
				status = "installed"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", pkg.Name, pkg.Version, status, description)
		}
	}

	w.Flush()
	return nil
}

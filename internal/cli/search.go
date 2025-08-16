package cli

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	pkg "github.com/cperrin88/gotya/pkg/package"
	"github.com/cperrin88/gotya/pkg/repository"
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

	// Get all repositories
	repos := manager.ListRepositories()
	if len(repos) == 0 {
		Error("No repositories configured")
		return fmt.Errorf("no repositories configured")
	}

	// Search through all enabled repositories
	var allResults []SearchResult
	for _, repo := range repos {
		if !repo.Enabled {
			continue
		}

		Debug("Searching repository", logrus.Fields{"repository": repo.Name})

		// Get repository index
		index, err := manager.GetRepositoryIndex(repo.Name)
		if err != nil {
			Warn("Failed to get index for repository", logrus.Fields{
				"repository": repo.Name,
				"error":      err.Error(),
			})
			continue
		}

		// Search in this repository's index
		results := searchInIndex(index, repo.Name, query, exactMatch)
		allResults = append(allResults, results...)
	}

	// Apply limit
	if len(allResults) > limit {
		allResults = allResults[:limit]
	}

	if len(allResults) == 0 {
		Info("No packages found matching the query")
		return nil
	}

	// Display results in table format
	displaySearchResults(allResults)
	Info("Search completed", logrus.Fields{"found": len(allResults)})

	return nil
}

type SearchResult struct {
	Name        string
	Version     string
	Description string
	Repository  string
}

func searchInIndex(index repository.Index, repoName, query string, exactMatch bool) []SearchResult {
	var results []SearchResult

	packages := index.GetPackages()
	for _, pkg := range packages {
		var matches bool
		if exactMatch {
			matches = pkg.Name == query
		} else {
			matches = strings.Contains(strings.ToLower(pkg.Name), strings.ToLower(query)) ||
				strings.Contains(strings.ToLower(pkg.Description), strings.ToLower(query))
		}

		if matches {
			results = append(results, SearchResult{
				Name:        pkg.Name,
				Version:     pkg.Version,
				Description: pkg.Description,
				Repository:  repoName,
			})
		}
	}

	return results
}

func displaySearchResults(results []SearchResult) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tVERSION\tREPOSITORY\tDESCRIPTION")
	fmt.Fprintln(w, "----\t-------\t----------\t-----------")

	for _, pkg := range results {
		description := pkg.Description
		if len(description) > 50 {
			description = description[:47] + "..."
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", pkg.Name, pkg.Version, pkg.Repository, description)
	}

	w.Flush()
}

func runList(cmd *cobra.Command, showInstalled, showAvailable bool) error {
	cfg, manager, err := loadConfigAndManager()
	if err != nil {
		return err
	}

	// Default to showing installed packages if no flags specified
	if !showInstalled && !showAvailable {
		showInstalled = true
	}

	var packages []PackageListItem

	// Load installed packages if requested
	if showInstalled {
		installedDB, err := pkg.LoadInstalledDatabase(cfg.GetDatabasePath())
		if err != nil {
			Warn("Failed to load installed packages database", logrus.Fields{"error": err.Error()})
		} else {
			installedPackages := installedDB.GetInstalledPackages()
			for _, installedPkg := range installedPackages {
				packages = append(packages, PackageListItem{
					Name:        installedPkg.Name,
					Version:     installedPkg.Version,
					Description: installedPkg.Description,
					Status:      "installed",
					Repository:  "-",
				})
			}
		}
	}

	// Load available packages if requested
	if showAvailable {
		repos := manager.ListRepositories()
		for _, repo := range repos {
			if !repo.Enabled {
				continue
			}

			index, err := manager.GetRepositoryIndex(repo.Name)
			if err != nil {
				Warn("Failed to get index for repository", logrus.Fields{
					"repository": repo.Name,
					"error":      err.Error(),
				})
				continue
			}

			repoPackages := index.GetPackages()
			for _, repoPkg := range repoPackages {
				// Check if package is already installed
				var status string
				if showInstalled {
					// Check if we already have this package in our list (installed)
					found := false
					for _, existing := range packages {
						if existing.Name == repoPkg.Name {
							found = true
							break
						}
					}
					status = "available"
					if found {
						status = "installed"
					}
				} else {
					status = "available"
				}

				// Use "any" if OS/Arch are empty
				os := repoPkg.OS
				if os == "" {
					os = "any"
				}
				arch := repoPkg.Arch
				if arch == "" {
					arch = "any"
				}

				packages = append(packages, PackageListItem{
					Name:        repoPkg.Name,
					Version:     repoPkg.Version,
					Description: repoPkg.Description,
					Status:      status,
					Repository:  repo.Name,
					OS:          os,
					Arch:        arch,
				})
			}
		}
	}

	if len(packages) == 0 {
		Info("No packages found")
		return nil
	}

	// Display results in table format
	displayPackageList(packages)
	Info("Package listing completed", logrus.Fields{"total": len(packages)})

	return nil
}

// PackageListItem represents a package in the list output
type PackageListItem struct {
	Name        string
	Version     string
	Description string
	Status      string
	Repository  string
	OS          string
	Arch        string
}

func displayPackageList(packages []PackageListItem) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tVERSION\tPLATFORM\tSTATUS\tREPOSITORY\tDESCRIPTION")

	for _, pkg := range packages {
		desc := pkg.Description
		if len(desc) > 40 {
			desc = desc[:37] + "..."
		}
		platform := fmt.Sprintf("%s/%s", pkg.OS, pkg.Arch)
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			pkg.Name, pkg.Version, platform, pkg.Status, pkg.Repository, desc)
	}

	w.Flush()
}

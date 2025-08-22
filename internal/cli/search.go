package cli

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/cperrin88/gotya/pkg/logger"
	pkg "github.com/cperrin88/gotya/pkg/package"
	"github.com/cperrin88/gotya/pkg/repository"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// NewSearchCmd creates the search command.
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
	cmd.Flags().IntVar(&limit, "limit", DefaultSearchLimit, "Limit number of results")

	return cmd
}

// NewListCmd creates the list command.
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

	logger.Debug("Searching for packages", logrus.Fields{
		"query":       query,
		"exact_match": exactMatch,
		"limit":       limit,
	})

	// Get all repositories
	repos := manager.ListRepositories()
	if len(repos) == 0 {
		logger.Error("No repositories configured")
		return fmt.Errorf("no repositories configured")
	}

	// Search through all enabled repositories
	var allResults []SearchResult
	for _, repo := range repos {
		if !repo.Enabled {
			continue
		}

		logger.Debug("Searching repository", logrus.Fields{"repository": repo.Name})

		// Get repository index
		index, err := manager.GetRepositoryIndex(repo.Name)
		if err != nil {
			logger.Warn("Failed to get index for repository", logrus.Fields{
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
		logger.Info("No packages found matching the query")
		return nil
	}

	// Display results in table format
	displaySearchResults(allResults)
	logger.Info("Search completed", logrus.Fields{"found": len(allResults)})

	return nil
}

type SearchResult struct {
	Name        string
	Version     string
	Description string
	Repository  string
}

func searchInIndex(index repository.Index, repoName, query string, exactMatch bool) []SearchResult {
	var searchResults []SearchResult

	availablePackages := index.GetPackages()
	for packageIndex := range availablePackages {
		pkg := &availablePackages[packageIndex]
		var isMatch bool

		if exactMatch {
			isMatch = pkg.Name == query
		} else {
			lowerQuery := strings.ToLower(query)
			isMatch = strings.Contains(strings.ToLower(pkg.Name), lowerQuery) ||
				strings.Contains(strings.ToLower(pkg.Description), lowerQuery)
		}

		if isMatch {
			searchResults = append(searchResults, SearchResult{
				Name:        pkg.Name,
				Version:     pkg.Version,
				Description: pkg.Description,
				Repository:  repoName,
			})
		}
	}

	return searchResults
}

func displaySearchResults(results []SearchResult) {
	tabWriter := tabwriter.NewWriter(os.Stdout, 0, 0, TabWidth, ' ', 0)
	fmt.Fprintln(tabWriter, "NAME\tVERSION\tREPOSITORY\tDESCRIPTION")
	fmt.Fprintln(tabWriter, "----\t-------\t----------\t-----------")

	for _, pkg := range results {
		description := pkg.Description
		if len(description) > MaxDescriptionLength {
			description = description[:MaxDescriptionLength-3] + "..."
		}
		fmt.Fprintf(tabWriter, "%s\t%s\t%s\t%s\n", pkg.Name, pkg.Version, pkg.Repository, description)
	}

	tabWriter.Flush()
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
			logger.Warn("Failed to load installed packages database", logrus.Fields{"error": err.Error()})
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
				logger.Warn("Failed to get index for repository", logrus.Fields{
					"repository": repo.Name,
					"error":      err.Error(),
				})
				continue
			}

			repoPackages := index.GetPackages()
			for i := range repoPackages {
				repoPkg := &repoPackages[i]
				// Check if package is already installed
				var status string
				if showInstalled {
					// Check if we already have this package in our list (installed)
					found := false
					for j := range packages {
						if packages[j].Name == repoPkg.Name {
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
				osName := repoPkg.OS
				if osName == "" {
					osName = "any"
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
					OS:          osName,
					Arch:        arch,
				})
			}
		}
	}

	if len(packages) == 0 {
		logger.Info("No packages found")
		return nil
	}

	// Display results in table format
	displayPackageList(packages)
	logger.Info("Package listing completed", logrus.Fields{"total": len(packages)})

	return nil
}

// PackageListItem represents a package in the list output.
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
	tabWriter := tabwriter.NewWriter(os.Stdout, 0, 0, TabWidth, ' ', 0)
	fmt.Fprintln(tabWriter, "NAME\tVERSION\tPLATFORM\tSTATUS\tREPOSITORY\tDESCRIPTION")

	for _, pkg := range packages {
		description := pkg.Description
		if len(description) > MaxSearchDescriptionLength {
			description = description[:MaxSearchDescriptionLength-3] + "..."
		}
		platform := fmt.Sprintf("%s/%s", pkg.OS, pkg.Arch)
		fmt.Fprintf(tabWriter, "%s\t%s\t%s\t%s\t%s\t%s\n",
			pkg.Name, pkg.Version, platform, pkg.Status, pkg.Repository, description)
	}

	tabWriter.Flush()
}

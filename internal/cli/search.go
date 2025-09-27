package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// NewSearchCmd creates the search command.
func NewSearchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search for packages",
		Long: `Search for packages across all configured repositories using fuzzy matching.

The search performs fuzzy matching on package names and returns results
sorted by relevance (best matches first).`,
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return runSearch(args[0])
		},
	}

	return cmd
}

func runSearch(query string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	// Load the index manager
	indexManager := loadIndexManager(cfg)

	// Perform fuzzy search
	searchResults, err := indexManager.FuzzySearchArtifacts(query)
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}

	// Display results in tabular format
	if len(searchResults) == 0 {
		fmt.Printf("No packages found matching '%s'\n", query)
		return nil
	}

	// Show results grouped by repository
	for repoName, artifacts := range searchResults {
		if len(artifacts) == 0 {
			continue
		}

		fmt.Printf("\n%s:\n", repoName)
		fmt.Println(strings.Repeat("-", 60))

		// Header
		fmt.Printf("%-30s %-15s %-20s %s\n", "PACKAGE NAME", "VERSION", "OS/ARCH", "DESCRIPTION")
		fmt.Println(strings.Repeat("-", 90))

		// Rows
		for _, artifact := range artifacts {
			osArch := fmt.Sprintf("%s/%s", artifact.OS, artifact.Arch)
			description := artifact.Description
			if len(description) > 30 {
				description = description[:27] + "..."
			}

			fmt.Printf("%-30s %-15s %-20s %s\n",
				artifact.Name,
				artifact.Version,
				osArch,
				description)
		}
	}

	// Summary
	totalMatches := 0
	for _, artifacts := range searchResults {
		totalMatches += len(artifacts)
	}
	fmt.Printf("\nFound %d package(s) matching '%s'\n", totalMatches, query)

	return nil
}

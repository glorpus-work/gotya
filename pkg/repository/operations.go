package repository

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/cperrin88/gotya/pkg/logger"
	"github.com/sirupsen/logrus"
)

// RepositoryOperation represents an operation that can be performed on a repository
type RepositoryOperation struct {
	manager Manager
}

// NewRepositoryOperation creates a new repository operation instance
func NewRepositoryOperation(manager Manager) *RepositoryOperation {
	return &RepositoryOperation{
		manager: manager,
	}
}

// Add adds a new repository
func (op *RepositoryOperation) Add(name, url string, priority int) error {
	if name == "" {
		// Auto-generate a name if not provided
		name = generateRepoName(url)
	}

	logger.Debug("Adding repository", logrus.Fields{"url": url, "name": name, "priority": priority})

	// Check if repository already exists
	existing := op.manager.GetRepository(name)
	if existing != nil {
		return fmt.Errorf("repository '%s' already exists", name)
	}

	// Add the repository
	if err := op.manager.AddRepository(name, url); err != nil {
		return fmt.Errorf("failed to add repository: %w", err)
	}

	// Set priority if specified
	if priority != 0 {
		// Note: This assumes the manager supports priority setting
		// If not, you'll need to implement this functionality
	}

	// Sync the repository to get the latest index
	ctx := context.Background()
	if err := op.manager.SyncRepository(ctx, name); err != nil {
		// Don't fail the operation if sync fails, just log a warning
		logger.Warnf("Failed to sync repository '%s': %v", name, err)
	}

	logger.Infof("Added repository '%s' (%s)", name, url)
	return nil
}

// Remove removes a repository
func (op *RepositoryOperation) Remove(name string) error {
	logger.Debug("Removing repository", logrus.Fields{"name": name})

	// Check if repository exists
	existing := op.manager.GetRepository(name)
	if existing == nil {
		return fmt.Errorf("repository '%s' not found", name)
	}

	if err := op.manager.RemoveRepository(name); err != nil {
		return fmt.Errorf("failed to remove repository: %w", err)
	}

	logger.Infof("Removed repository '%s'", name)
	return nil
}

// List lists all repositories
func (op *RepositoryOperation) List() (string, error) {
	repos := op.manager.ListRepositories()
	if len(repos) == 0 {
		return "No repositories configured.\n", nil
	}

	// Sort repositories by priority (highest first) and then by name
	sort.Slice(repos, func(i, j int) bool {
		if repos[i].Priority != repos[j].Priority {
			return repos[i].Priority > repos[j].Priority
		}
		return strings.ToLower(repos[i].Name) < strings.ToLower(repos[j].Name)
	})

	var buf strings.Builder
	w := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)

	// Write header
	if _, err := fmt.Fprintln(w, "NAME\tURL\tENABLED\tPRIORITY"); err != nil {
		return "", fmt.Errorf("failed to write table header: %w", err)
	}

	// Write repository rows
	for _, repo := range repos {
		enabled := "no"
		if repo.Enabled {
			enabled = "yes"
		}
		if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%d\n",
			repo.Name,
			repo.URL,
			enabled,
			repo.Priority,
		); err != nil {
			return "", fmt.Errorf("failed to write repository %s: %w", repo.Name, err)
		}
	}

	// Flush the tabwriter buffer
	if err := w.Flush(); err != nil {
		return "", fmt.Errorf("failed to flush tabwriter: %w", err)
	}

	return buf.String(), nil
}

// Update updates one or more repositories
func (op *RepositoryOperation) Update(names []string) (string, error) {
	var reposToUpdate []string

	if len(names) == 0 {
		// Update all enabled repositories if no names are provided
		repos := op.manager.ListRepositories()
		for _, repo := range repos {
			if repo.Enabled {
				reposToUpdate = append(reposToUpdate, repo.Name)
			}
		}
	} else {
		reposToUpdate = names
	}

	if len(reposToUpdate) == 0 {
		return "No repositories to update\n", nil
	}

	var result strings.Builder
	ctx := context.Background()

	for _, name := range reposToUpdate {
		repo := op.manager.GetRepository(name)
		if repo == nil {
			result.WriteString(fmt.Sprintf("Repository '%s' not found\n", name))
			continue
		}

		if !repo.Enabled {
			result.WriteString(fmt.Sprintf("Skipping disabled repository '%s'\n", name))
			continue
		}

		start := time.Now()
		result.WriteString(fmt.Sprintf("Updating repository '%s'... ", name))

		if err := op.manager.SyncRepository(ctx, name); err != nil {
			result.WriteString(fmt.Sprintf("FAILED: %v\n", err))
			continue
		}

		// Get the cache age to show how fresh the index is
		age, _ := op.manager.GetCacheAge(name)
		result.WriteString(fmt.Sprintf("done (took %v, age: %v)\n",
			time.Since(start).Round(time.Millisecond),
			age.Round(time.Second),
		))
	}

	if result.Len() == 0 {
		return "No repositories were updated\n", nil
	}

	return result.String(), nil
}

// generateRepoName generates a repository name from a URL
func generateRepoName(url string) string {
	// Remove protocol
	name := strings.TrimPrefix(url, "http://")
	name = strings.TrimPrefix(name, "https://")

	// Remove trailing slashes
	name = strings.TrimSuffix(name, "/")

	// Remove common prefixes
	name = strings.TrimPrefix(name, "www.")
	name = strings.TrimPrefix(name, "gitlab.com/")
	name = strings.TrimPrefix(name, "github.com/")

	// Replace special characters with hyphens
	replacer := strings.NewReplacer(
		"/", "-",
		"_", "-",
		".", "-",
		":", "",
	)

	name = replacer.Replace(name)

	// Remove any remaining non-alphanumeric characters except hyphens
	var result strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '-' {
			result.WriteRune(r)
		}
	}

	// Ensure the name is not empty
	if result.Len() == 0 {
		return "repo"
	}

	return result.String()
}

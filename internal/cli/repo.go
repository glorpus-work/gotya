package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/cperrin88/gotya/pkg/logger"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// NewRepoCmd creates the repo command with subcommands
func NewRepoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "repo",
		Short: "Manage repositories",
		Long:  "Add, remove, list, and manage package repositories",
	}

	cmd.AddCommand(
		newRepoAddCmd(),
		newRepoRemoveCmd(),
		newRepoListCmd(),
		newRepoUpdateCmd(),
	)

	return cmd
}

func newRepoAddCmd() *cobra.Command {
	var (
		name     string
		priority int
	)

	cmd := &cobra.Command{
		Use:   "add URL",
		Short: "Add a new repository",
		Long:  "Add a new package repository by URL",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRepoAdd(args[0], name)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Repository name (auto-detected if not provided)")
	cmd.Flags().IntVar(&priority, "priority", 0, "Repository priority (higher numbers have higher priority)")

	return cmd
}

func newRepoRemoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove NAME",
		Short: "Remove a repository",
		Long:  "Remove a repository by name",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRepoRemove(args[0])
		},
	}

	return cmd
}

func newRepoListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List configured repositories",
		Long:  "List all configured package repositories",
		RunE:  runRepoList,
	}

	return cmd
}

func newRepoUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update [NAME...]",
		Short: "Update repository indexes",
		Long:  "Update package indexes for specified repositories or all repositories",
		RunE:  runRepoUpdate,
	}

	return cmd
}

func runRepoAdd(url, name string) error {
	_, manager, err := loadConfigAndManager()
	if err != nil {
		return err
	}

	logger.Debug("Adding repository", logrus.Fields{"url": url, "name": name})

	manager.AddRepository(name, url)

	repoName := name
	if repoName == "" {
		repoName = url
	}
	logger.Success("Successfully added repository", logrus.Fields{"repository": repoName})
	return nil
}

func runRepoRemove(name string) error {
	_, manager, err := loadConfigAndManager()
	if err != nil {
		return err
	}

	logger.Debug("Removing repository", logrus.Fields{"name": name})

	manager.RemoveRepository(name)

	logger.Success("Successfully removed repository", logrus.Fields{"repository": name})
	return nil
}

func runRepoList(*cobra.Command, []string) error {
	_, manager, err := loadConfigAndManager()
	if err != nil {
		return err
	}

	repos := manager.ListRepositories()

	if len(repos) == 0 {
		logger.Info("No repositories configured")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tURL\tPRIORITY\tSTATUS")
	fmt.Fprintln(w, "----\t---\t--------\t------")

	for _, repo := range repos {
		status := "active"
		if !repo.Enabled {
			status = "disabled"
		}
		fmt.Fprintf(w, "%s\t%s\t%d\t%s\n", repo.Name, repo.URL, repo.Priority, status)
	}

	w.Flush()
	return nil
}

func runRepoUpdate(cmd *cobra.Command, args []string) error {
	_, manager, err := loadConfigAndManager()
	if err != nil {
		return err
	}

	if len(args) == 0 {
		// Update all repositories
		logger.Debug("Updating all repositories...")
		if err := manager.SyncRepositories(cmd.Context()); err != nil {
			return fmt.Errorf("failed to update repositories: %w", err)
		}
		logger.Success("All repositories updated successfully")
	} else {
		// Update specific repositories
		for _, repoName := range args {
			logger.Debug("Updating repository", logrus.Fields{"repository": repoName})
			if err := manager.SyncRepository(cmd.Context(), repoName); err != nil {
				return fmt.Errorf("failed to update repository %s: %w", repoName, err)
			}
			logger.Success("Repository updated successfully", logrus.Fields{"repository": repoName})
		}
	}

	return nil
}

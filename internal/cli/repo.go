package cli

import (
	"fmt"

	"github.com/cperrin88/gotya/pkg/repository"
	"github.com/spf13/cobra"
)

// NewRepoCmd creates the repo command with subcommands.
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
		RunE: func(_ *cobra.Command, args []string) error {
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
		RunE: func(_ *cobra.Command, args []string) error {
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

	repoOp := repository.NewRepositoryOperation(manager)
	return repoOp.Add(name, url, 0) // Default priority is 0
}

func runRepoRemove(name string) error {
	_, manager, err := loadConfigAndManager()
	if err != nil {
		return err
	}

	repoOp := repository.NewRepositoryOperation(manager)
	if err := repoOp.Remove(name); err != nil {
		return fmt.Errorf("failed to remove repository '%s': %w", name, err)
	}
	return nil
}

func runRepoList(_ *cobra.Command, _ []string) error {
	_, manager, err := loadConfigAndManager()
	if err != nil {
		return err
	}

	repoOp := repository.NewRepositoryOperation(manager)
	output, err := repoOp.List()
	if err != nil {
		return fmt.Errorf("failed to list repositories: %w", err)
	}

	fmt.Print(output)
	return nil
}

func runRepoUpdate(_ *cobra.Command, args []string) error {
	_, manager, err := loadConfigAndManager()
	if err != nil {
		return err
	}

	repoOp := repository.NewRepositoryOperation(manager)
	output, err := repoOp.Update(args)
	if err != nil {
		return fmt.Errorf("failed to update repositories: %w", err)
	}

	fmt.Print(output)
	return nil
}

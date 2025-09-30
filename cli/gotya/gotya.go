// Package main provides the gotya CLI entrypoint.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/cperrin88/gotya/internal/cli"
	"github.com/spf13/cobra"
)

var (
	configPath   string
	verbose      bool
	noColor      bool
	outputFormat string
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	rootCmd := newRootCmd()
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		cancel()
		os.Exit(1)
	}

	cancel()
}

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gotya",
		Short: "A lightweight personal artifact manager",
		Long: `gotya is a lightweight personal artifact manager (like apt) with:
- CLI: install, update, search
- Library: download index and packages
- Tooling: create packages and manage repositories`,
		SilenceUsage: true,
	}

	// Global flags
	cmd.PersistentFlags().StringVar(&configPath, "config", "", "config file path (default: auto-detect)")
	cmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	cmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "disable colored output")
	cmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "", "output format (json, yaml, table)")

	// Set up CLI artifact variables
	cli.ConfigPath = &configPath
	cli.Verbose = &verbose
	cli.NoColor = &noColor
	cli.OutputFormat = &outputFormat

	// Add subcommands
	cmd.AddCommand(
		cli.NewSyncCmd(),
		cli.NewInstallCmd(),
		cli.NewUninstallCmd(),
		cli.NewUpdateCmd(),
		cli.NewListCmd(),
		cli.NewSearchCmd(),
		cli.NewConfigCmd(),
		cli.NewCacheCmd(),
		cli.NewArtifactCmd(),
		cli.NewIndexCmd(),
		cli.NewVersionCmd(),
	)

	return cmd
}

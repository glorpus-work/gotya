package cli

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/cperrin88/gotya/internal/logger"
	"github.com/cperrin88/gotya/pkg/config"
	"github.com/cperrin88/gotya/pkg/errors"
	"github.com/spf13/cobra"
)

// NewConfigCmd creates the config command with subcommands.
func NewConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage configuration",
		Long:  "View and modify gotya configuration settings",
	}

	cmd.AddCommand(
		newConfigShowCmd(),
		newConfigSetCmd(),
		newConfigGetCmd(),
		newConfigInitCmd(),
	)

	return cmd
}

func newConfigShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show current configuration",
		Long:  "Display the current configuration settings",
		RunE:  runConfigShow,
	}

	return cmd
}

// Number of arguments expected by the set command.
const setCommandArgs = 2

func newConfigSetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set KEY VALUE",
		Short: "Set a configuration value",
		Long:  "Set a configuration key to a specific value",
		Args:  cobra.ExactArgs(setCommandArgs),
		RunE: func(_ *cobra.Command, args []string) error {
			return runConfigSet(args[0], args[1])
		},
	}

	return cmd
}

func newConfigGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get KEY",
		Short: "Get a configuration value",
		Long:  "Get the value of a specific configuration key",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return runConfigGet(args[0])
		},
	}

	return cmd
}

func newConfigInitCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize configuration file",
		Long:  "Create a default configuration file",
		RunE: func(_ *cobra.Command, _ []string) error {
			return runConfigInit(force)
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing configuration file")

	return cmd
}

func runConfigShow(*cobra.Command, []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	tabWriter := tabwriter.NewWriter(os.Stdout, 0, 0, TabWidth, ' ', 0)
	_, _ = fmt.Fprintln(tabWriter, "SETTING\tVALUE")
	_, _ = fmt.Fprintln(tabWriter, "-------\t-----")

	// Display settings using ToMap for consistency with actual config keys
	settingsMap := cfg.ToMap()
	for key, value := range settingsMap {
		_, _ = fmt.Fprintf(tabWriter, "%s\t%s\n", key, value)
	}

	_ = tabWriter.Flush()

	fmt.Printf("\nRepositories (%d):\n", len(cfg.Repositories))
	for _, repo := range cfg.Repositories {
		status := "enabled"
		if !repo.Enabled {
			status = "disabled"
		}
		fmt.Printf("  %s: %s (%s)\n", repo.Name, repo.URL, status)
	}

	return nil
}

func runConfigSet(key, value string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	if err := cfg.SetValue(key, value); err != nil {
		return fmt.Errorf("failed to set configuration value: %w", err)
	}

	configPath := getConfigPath()
	if err := cfg.SaveConfig(configPath); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	logger.Success("Configuration updated", logger.Fields{"key": key, "value": value})

	// If platform settings were updated, suggest restarting the CLI
	if strings.HasPrefix(key, "platform.") {
		logger.Info("Note: Platform settings take effect on the next command")
	}

	return nil
}

func runConfigGet(key string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	value, err := cfg.GetValue(key)
	if err != nil {
		return fmt.Errorf("failed to get configuration value: %w", err)
	}

	fmt.Println(value)
	return nil
}

func runConfigInit(force bool) error {
	configPath := getConfigPath()

	// Check if config file already exists
	if _, err := os.Stat(configPath); err == nil && !force {
		return fmt.Errorf("configuration file already exists at %s (use --force to overwrite): %w", configPath, errors.ErrConfigFileExists)
	}

	// Create default config
	defaultConfig := config.NewDefaultConfig()
	if err := defaultConfig.SaveConfig(configPath); err != nil {
		return fmt.Errorf("failed to save default configuration: %w", err)
	}

	logger.Success("Configuration file created", logger.Fields{"path": configPath})
	return nil
}

func getConfigPath() string {
	if ConfigPath != nil && *ConfigPath != "" {
		return *ConfigPath
	}

	defaultPath, err := config.GetDefaultConfigPath()
	if err != nil {
		// If we can't get the default path, use an empty string which will cause a more descriptive error later
		// when the config file is actually being read/written
		logger.Warn("Failed to get default config path, using empty path", logger.Fields{"error": err})
		return ""
	}
	return defaultPath
}

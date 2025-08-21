package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/cperrin88/gotya/pkg/config"
	"github.com/cperrin88/gotya/pkg/platform"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// NewConfigCmd creates the config command with subcommands
func NewConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage configuration",
		Long:  "View and modify gotya configuration settings",
	}

	cmd.AddCommand(
		NewRepoCmd(),
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

func newConfigSetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set KEY VALUE",
		Short: "Set a configuration value",
		Long:  "Set a configuration key to a specific value",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
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
		RunE: func(cmd *cobra.Command, args []string) error {
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
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigInit(force)
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing configuration file")

	return cmd
}

func runConfigShow(*cobra.Command, []string) error {
	cfg, _, err := loadConfigAndManager()
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "SETTING\tVALUE")
	fmt.Fprintln(w, "-------\t-----")

	// Display settings using reflection to access fields
	settingsValue := reflect.ValueOf(cfg.Settings)
	settingsType := reflect.TypeOf(cfg.Settings)

	for i := 0; i < settingsValue.NumField(); i++ {
		field := settingsType.Field(i)
		value := settingsValue.Field(i)

		// Convert field name to snake_case
		fieldName := toSnakeCase(field.Name)
		fmt.Fprintf(w, "%s\t%v\n", fieldName, value.Interface())
	}

	w.Flush()

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
	cfg, _, err := loadConfigAndManager()
	if err != nil {
		return err
	}

	// Special handling for platform settings
	switch key {
	case "platform.os":
		if value != "" {
			// Validate OS value
			normalized := platform.NormalizeOS(value)
			if normalized == "" && value != "" {
				return fmt.Errorf("invalid OS value: %s. Valid values are: %v",
					value, platform.GetValidOS())
			}
			value = normalized // Use normalized value
		}
	case "platform.arch":
		if value != "" {
			// Validate Arch value
			normalized := platform.NormalizeArch(value)
			if normalized == "" && value != "" {
				return fmt.Errorf("invalid architecture value: %s. Valid values are: %v",
					value, platform.GetValidArch())
			}
			value = normalized // Use normalized value
		}
	case "platform.prefer_native":
		// Handled by the bool parser in setConfigValue
	}

	if err := setConfigValue(cfg, key, value); err != nil {
		return fmt.Errorf("failed to set configuration value: %w", err)
	}

	configPath := getConfigPath()
	if err := cfg.SaveConfig(configPath); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	Success("Configuration updated", logrus.Fields{"key": key, "value": value})

	// If platform settings were updated, suggest restarting the CLI
	if strings.HasPrefix(key, "platform.") {
		Info("Note: Platform settings take effect on the next command")
	}

	return nil
}

func runConfigGet(key string) error {
	cfg, _, err := loadConfigAndManager()
	if err != nil {
		return err
	}

	value, err := getConfigValue(cfg, key)
	if err != nil {
		return fmt.Errorf("failed to get configuration value: %w", err)
	}

	fmt.Println(value)
	return nil
}

func runConfigInit(force bool) error {
	configPath := getConfigPath()

	// Check if config file already exists
	if !force {
		if _, err := os.Stat(configPath); err == nil {
			return fmt.Errorf("configuration file already exists at %s (use --force to overwrite)", configPath)
		}
	}

	// Create config directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Create default config
	defaultConfig := createDefaultConfig()
	if err := defaultConfig.SaveConfig(configPath); err != nil {
		return fmt.Errorf("failed to save default configuration: %w", err)
	}

	Success("Configuration file created", logrus.Fields{"path": configPath})
	return nil
}

func getConfigPath() string {
	if ConfigPath != nil && *ConfigPath != "" {
		return *ConfigPath
	}

	defaultPath, _ := config.GetDefaultConfigPath()
	return defaultPath
}

func createDefaultConfig() *config.Config {
	// This creates a default configuration
	cfg := &config.Config{}
	cfg.Settings.CacheDir = "" // Will use default
	cfg.Settings.OutputFormat = "table"
	cfg.Settings.ColorOutput = true
	cfg.Settings.LogLevel = "info"
	cfg.Repositories = make([]config.RepositoryConfig, 0)

	return cfg
}

// Helper function to set a configuration value by key
func setConfigValue(cfg *config.Config, key, value string) error {
	switch key {
	case "cache_dir":
		cfg.Settings.CacheDir = value
	case "output_format":
		cfg.Settings.OutputFormat = value
	case "color_output":
		boolVal, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("invalid boolean value for %s: %s", key, value)
		}
		cfg.Settings.ColorOutput = boolVal
	case "log_level":
		cfg.Settings.LogLevel = value
	default:
		return fmt.Errorf("unknown configuration key: %s", key)
	}
	return nil
}

// Helper function to get a configuration value by key
func getConfigValue(cfg *config.Config, key string) (string, error) {
	switch key {
	case "cache_dir":
		return cfg.Settings.CacheDir, nil
	case "output_format":
		return cfg.Settings.OutputFormat, nil
	case "color_output":
		return strconv.FormatBool(cfg.Settings.ColorOutput), nil
	case "log_level":
		return cfg.Settings.LogLevel, nil
	default:
		return "", fmt.Errorf("unknown configuration key: %s", key)
	}
}

// Helper function to convert CamelCase to snake_case
func toSnakeCase(str string) string {
	var result []rune
	for i, r := range str {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result = append(result, '_')
		}
		result = append(result, r)
	}
	return string(result)
}

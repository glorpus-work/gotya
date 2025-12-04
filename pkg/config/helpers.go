package config

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/glorpus-work/gotya/pkg/errutils"
)

// SetValue sets a configuration value by key
// Supported keys:
//   - cache_dir: string - Path to the cache directory
//   - output_format: string - Output format (text, json, etc.)
//   - log_level: string - Logging level (debug, info, warn, error, fatal)
//   - platform.os: string - Target operating system
//   - platform.arch: string - Target architecture
func (c *Config) SetValue(key, value string) error {
	switch key {
	case "cache_dir":
		c.Settings.CacheDir = value
	case "output_format":
		c.Settings.OutputFormat = value
	case "log_level":
		c.Settings.LogLevel = value
	case "platform.os":
		if value != "" {
			c.Settings.Platform.OS = value
		}
	case "platform.arch":
		if value != "" {
			c.Settings.Platform.Arch = value
		}
	default:
		return fmt.Errorf("unknown configuration key: %s: %w", key, errutils.ErrUnknownConfigKey)
	}
	return nil
}

// GetValue retrieves the configuration value for the given key as a string.
func (c *Config) GetValue(key string) (string, error) {
	switch key {
	case "cache_dir":
		return c.Settings.CacheDir, nil
	case "output_format":
		return c.Settings.OutputFormat, nil
	case "log_level":
		return c.Settings.LogLevel, nil
	case "platform.os":
		return c.Settings.Platform.OS, nil
	case "platform.arch":
		return c.Settings.Platform.Arch, nil
	default:
		return "", fmt.Errorf("unknown configuration key: %s: %w", key, errutils.ErrUnknownConfigKey)
	}
}

// ToMap converts the configuration to a map of key-value pairs.
func (c *Config) ToMap() map[string]string {
	result := make(map[string]string)

	// Convert Settings struct to map
	settingsValue := reflect.ValueOf(c.Settings)
	settingsType := settingsValue.Type()

	for fieldIndex := 0; fieldIndex < settingsValue.NumField(); fieldIndex++ {
		field := settingsType.Field(fieldIndex)
		yamlTag := field.Tag.Get("yaml")
		if yamlTag == "" || yamlTag == "-" {
			continue
		}

		// Handle yaml tags with options (e.g., "cache_dir,omitempty")
		yamlKey := strings.Split(yamlTag, ",")[0]

		// Get the field value and convert to string via helper
		fieldValue := settingsValue.Field(fieldIndex)
		result[yamlKey] = toStringValue(fieldValue)
	}

	return result
}

// toStringValue converts a reflect.Value to its string representation.
func toStringValue(fieldValue reflect.Value) string {
	switch fieldValue.Kind() {
	case reflect.Invalid:
		return ""
	case reflect.Bool:
		return strconv.FormatBool(fieldValue.Bool())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(fieldValue.Int(), 10)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return strconv.FormatUint(fieldValue.Uint(), 10)
	case reflect.Float32, reflect.Float64:
		return strconv.FormatFloat(fieldValue.Float(), 'f', -1, 64)
	case reflect.Complex64, reflect.Complex128:
		return fmt.Sprint(fieldValue.Complex())
	case reflect.Array, reflect.Slice:
		return fmt.Sprintf("%v", fieldValue.Interface())
	case reflect.Chan, reflect.Func, reflect.UnsafePointer:
		return fieldValue.Type().String() + " value"
	case reflect.Interface, reflect.Pointer:
		if fieldValue.IsNil() {
			return "<nil>"
		}
		return fmt.Sprintf("%v", fieldValue.Elem().Interface())
	case reflect.Map:
		return fmt.Sprintf("%v", fieldValue.Interface())
	case reflect.String:
		return fieldValue.String()
	case reflect.Struct:
		return fmt.Sprintf("%+v", fieldValue.Interface())
	default:
		return fmt.Sprintf("%v", fieldValue.Interface())
	}
}

// NewDefaultConfig creates a new configuration with default values.
func NewDefaultConfig() *Config {
	return &Config{
		Settings: Settings{
			CacheTTL:     DefaultCacheTTL,
			OutputFormat: "text",
			LogLevel:     "info",
		},
	}
}

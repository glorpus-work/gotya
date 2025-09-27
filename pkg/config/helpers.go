package config

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/cperrin88/gotya/pkg/platform"
)

// SetValue sets a configuration value by key
// Supported keys:
//   - cache_dir: string - Path to the cache directory
//   - output_format: string - Output format (text, json, etc.)
//   - log_level: string - Logging level (debug, info, warn, error, fatal)
//   - platform.os: string - Target operating system
//   - platform.arch: string - Target architecture
//   - platform.prefer_native: bool - Whether to prefer native packages
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
			normalized := platform.NormalizeOS(value)
			if normalized == "" {
				return fmt.Errorf("invalid OS value: %s. Valid values are: %v", value, platform.GetValidOS())
			}
			c.Settings.Platform.OS = normalized
		}
	case "platform.arch":
		if value != "" {
			normalized := platform.NormalizeArch(value)
			if normalized == "" {
				return fmt.Errorf("invalid architecture value: %s. Valid values are: %v", value, platform.GetValidArch())
			}
			c.Settings.Platform.Arch = normalized
		}
	case "platform.prefer_native":
		preferNative, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("invalid boolean value for platform.prefer_native: %s", value)
		}
		c.Settings.Platform.PreferNative = preferNative
	default:
		return fmt.Errorf("unknown configuration key: %s", key)
	}
	return nil
}

// Returns the value as a string and any error encountered.
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
	case "platform.prefer_native":
		return strconv.FormatBool(c.Settings.Platform.PreferNative), nil
	default:
		return "", fmt.Errorf("unknown configuration key: %s", key)
	}
}

// This is useful for displaying the configuration.
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

		// Get the field value and convert to string
		fieldValue := settingsValue.Field(fieldIndex)
		var strValue string

		switch fieldValue.Kind() {
		case reflect.Invalid:
			strValue = ""
		case reflect.Bool:
			strValue = strconv.FormatBool(fieldValue.Bool())
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			strValue = strconv.FormatInt(fieldValue.Int(), 10)
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
			strValue = strconv.FormatUint(fieldValue.Uint(), 10)
		case reflect.Float32, reflect.Float64:
			strValue = strconv.FormatFloat(fieldValue.Float(), 'f', -1, 64)
		case reflect.Complex64, reflect.Complex128:
			strValue = fmt.Sprint(fieldValue.Complex())
		case reflect.Array, reflect.Slice:
			strValue = fmt.Sprintf("%v", fieldValue.Interface())
		case reflect.Chan, reflect.Func, reflect.UnsafePointer:
			strValue = fieldValue.Type().String() + " value"
		case reflect.Interface, reflect.Pointer:
			if fieldValue.IsNil() {
				strValue = "<nil>"
			} else {
				strValue = fmt.Sprintf("%v", fieldValue.Elem().Interface())
			}
		case reflect.Map:
			strValue = fmt.Sprintf("%v", fieldValue.Interface())
		case reflect.String:
			strValue = fieldValue.String()
		case reflect.Struct:
			strValue = fmt.Sprintf("%+v", fieldValue.Interface())
		default:
			strValue = fmt.Sprintf("%v", fieldValue.Interface())
		}

		result[yamlKey] = strValue
	}

	return result
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

// ToSnakeCase converts CamelCase to snake_case while preserving first letter capitalization.
func ToSnakeCase(str string) string {
	result := make([]rune, 0, len(str)*2) // Pre-allocate with enough capacity
	for i, r := range str {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result = append(result, '_')
		}
		// Convert to uppercase to match original behavior
		if r >= 'a' && r <= 'z' {
			r = r - 'a' + 'A'
		}
		result = append(result, r)
	}
	return string(result)
}

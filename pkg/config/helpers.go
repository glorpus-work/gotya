package config

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// SetValue sets a configuration value by key
// Supported keys:
//   - cache_dir: string - Path to the cache directory
//   - output_format: string - Output format (text, json, etc.)
//   - color_output: bool - Whether to use colored output
//   - log_level: string - Logging level (debug, info, warn, error, fatal)
func (c *Config) SetValue(key, value string) error {
	switch key {
	case "cache_dir":
		c.Settings.CacheDir = value
	case "output_format":
		c.Settings.OutputFormat = value
	case "color_output":
		boolVal, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("invalid boolean value for %s: %s", key, value)
		}
		c.Settings.ColorOutput = boolVal
	case "log_level":
		c.Settings.LogLevel = value
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
	case "color_output":
		return strconv.FormatBool(c.Settings.ColorOutput), nil
	case "log_level":
		return c.Settings.LogLevel, nil
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

	for i := 0; i < settingsValue.NumField(); i++ {
		field := settingsType.Field(i)
		yamlTag := field.Tag.Get("yaml")
		if yamlTag == "" || yamlTag == "-" {
			continue
		}

		// Handle yaml tags with options (e.g., "cache_dir,omitempty")
		yamlKey := strings.Split(yamlTag, ",")[0]

		// Get the field value and convert to string
		fieldValue := settingsValue.Field(i)
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
			ColorOutput:  true,
			OutputFormat: "text",
			LogLevel:     "info",
		},
	}
}

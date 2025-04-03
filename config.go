// Legacy file, import from the config package instead
package cgpt

import (
	"io"

	"github.com/spf13/pflag"
	"github.com/tmc/cgpt/config"
)

// Config represents the configuration for the cgpt CLI.
// For new code, use config.Config instead.
type Config = config.Config

// LoadConfig loads the configuration from various sources in the following order of precedence:
// 1. Command-line flags (highest priority)
// 2. Environment variables
// 3. Configuration file
// 4. Default values (lowest priority)
//
// For new code, use config.LoadConfig instead.
func LoadConfig(path string, stderr io.Writer, flagSet *pflag.FlagSet) (*Config, error) {
	return config.LoadConfig(path, stderr, flagSet)
}

// isEnvSet checks if an environment variable is set.
// For new code, use config.IsEnvSet instead.
var isEnvSet = config.IsEnvSet

// setupViper configures viper with default values and settings.
// For new code, use config.SetupViper instead.
var setupViper = config.SetupViper

// logConfigSources logs where each configuration value came from.
// For new code, use config.LogConfigSources instead.
var logConfigSources = config.LogConfigSources

// setupFlagNormalization configures flag normalization to handle dashes in flag names.
// For new code, use config.SetupFlagNormalization instead.
var setupFlagNormalization = config.SetupFlagNormalization

// setMaxTokens sets the max tokens based on the backend and model.
// For new code, use config.SetMaxTokens instead.
var setMaxTokens = config.SetMaxTokens

// handleConfigFile handles loading the configuration file.
// For new code, use config.HandleConfigFile instead.
var handleConfigFile = config.HandleConfigFile

// logConfig logs the final configuration.
// For new code, use config.LogConfig instead.
var logConfig = config.LogConfig

// defaultBackend is the default backend to use if none is specified.
// For new code, use config.DefaultBackend instead.
var defaultBackend = config.DefaultBackend

// defaultModels is a map of backend names to their default models.
// For new code, use config.DefaultModels instead.
var defaultModels = config.DefaultModels

// tokenLimits is a map of regex patterns to token limits for each backend.
// For new code, use config.TokenLimits instead.
var tokenLimits = config.TokenLimits
// Package config provides configuration management for the cgpt CLI
package options

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// DefaultBackend is the default backend to use if none is specified
var DefaultBackend = "anthropic" // Configurable via 'CGPT_BACKEND" (or via configuration files).

// DefaultModels is a map of backend names to their default models
var DefaultModels = map[string]string{
	"anthropic": "claude-3-7-sonnet-20250219",
	"openai":    "gpt-4o",
	"ollama":    "llama3.2",
	"googleai":  "gemini-pro",
	"dummy":     "dummy",
}

// TokenLimits is a map of regex patterns to token limits for each backend.
// The key "*" is a catch-all for any patterns not explicitly defined.
// The value for each key is the maximum number of tokens allowed for a completion.
var TokenLimits = map[string]int{
	"*":                    4096,
	"google:*":             8192,
	"anthropic:.*sonnet.*": 8000,
}

// Config holds the configuration for the cgpt CLI
type Config struct {
	Backend     string  `yaml:"backend"`
	Model       string  `yaml:"model"`
	Stream      bool    `yaml:"stream"`
	MaxTokens   int     `yaml:"maxTokens"`
	Temperature float64 `yaml:"temperature"`

	SystemPrompt string             `yaml:"systemPrompt"`
	LogitBias    map[string]float64 `yaml:"logitBias"`

	CompletionTimeout time.Duration `yaml:"completionTimeout"`

	Debug bool `yaml:"debug"`

	// SlowResponses is a testing flag to simulate slow response generation
	SlowResponses bool `yaml:"slowResponses"`

	// HTTPRecordFile is the path to the HTTP record/replay file
	HTTPRecordFile string `yaml:"httpRecordFile"`

	OpenAIAPIKey    string `yaml:"openaiAPIKey"`
	AnthropicAPIKey string `yaml:"anthropicAPIKey"`
	GoogleAPIKey    string `yaml:"googleAPIKey"`
}

// LoadConfig loads the configuration from various sources in the following order of precedence:
// 1. Command-line flags (highest priority)
// 2. Environment variables
// 3. Configuration file
// 4. Default values (lowest priority)
//
// The function performs the following steps:
// - Sets default values
// - Binds command-line flags
// - Loads environment variables
// - Reads the configuration file
// - Unmarshals the configuration into the Config struct
//
// If a config file is not found, it falls back to using defaults and flags.
// The --verbose flag can be used to print the final configuration.
func LoadConfig(path string, stderr io.Writer, flagSet *pflag.FlagSet) (*Config, error) {
	if flagSet == nil {
		flagSet = pflag.CommandLine
	}
	cfg := &Config{}
	v := viper.New()

	SetupViper(v, flagSet)
	SetupFlagNormalization(flagSet)

	// Read config file first
	if err := HandleConfigFile(v, stderr, flagSet); err != nil {
		return nil, err
	}

	// Then bind flags (so they override config)
	if err := v.BindPFlags(flagSet); err != nil {
		return nil, fmt.Errorf("unable to bind flags: %w", err)
	}

	// Get backend (respecting precedence)
	backend := v.GetString("backend")
	if verbose, _ := flagSet.GetBool("debug"); verbose {
		fmt.Fprintf(stderr, "cgpt: backend is %q\n", backend)
	}

	// Check if model is explicitly set anywhere before setting default
	hasModel := false
	if flagSet.Changed("model") {
		hasModel = true
	} else if IsEnvSet("CGPT_MODEL") {
		hasModel = true
		v.Set("model", os.Getenv("CGPT_MODEL"))
	} else if v.InConfig("model") {
		hasModel = true
	}

	// Only set default model if no explicit model is set
	if !hasModel {
		if defaultModel, ok := DefaultModels[backend]; ok {
			v.Set("model", defaultModel)
		}
	}

	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("unable to unmarshal config: %w", err)
	}

	return cfg, nil
}

// IsEnvSet checks if an environment variable is set
func IsEnvSet(key string) bool {
	_, exists := os.LookupEnv(key)
	return exists
}

// SetupViper configures viper with default values and settings
func SetupViper(v *viper.Viper, flagSet *pflag.FlagSet) {
	// Set defaults
	v.SetDefault("backend", DefaultBackend)
	v.SetDefault("stream", true)
	v.SetDefault("temperature", 0.05)
	v.SetDefault("maxTokens", 4096)

	// Setup paths and env
	v.AddConfigPath("/etc/cgpt/")
	v.AddConfigPath("$HOME/.cgpt")
	v.AddConfigPath(".")
	v.SetConfigName("config")

	// Setup env vars
	v.SetEnvPrefix("CGPT")
	v.AutomaticEnv()
	v.BindEnv("openaiAPIKey", "OPENAI_API_KEY")
	v.BindEnv("anthropicAPIKey", "ANTHROPIC_API_KEY")
	v.BindEnv("googleAPIKey", "GOOGLE_API_KEY")

	// Set config file if specified in flags
	if flagConfigFilePath := flagSet.Lookup("config"); flagConfigFilePath != nil && flagConfigFilePath.Changed {
		v.SetConfigFile(flagConfigFilePath.Value.String())
	}

	_setupViper(v, flagSet)
}

// SetupFlagNormalization configures flag normalization to handle dashes in flag names
func SetupFlagNormalization(flagSet *pflag.FlagSet) {
	normalizeFunc := flagSet.GetNormalizeFunc()
	flagSet.SetNormalizeFunc(func(fs *pflag.FlagSet, name string) pflag.NormalizedName {
		result := normalizeFunc(fs, name)
		name = strings.ReplaceAll(string(result), "-", "")
		return pflag.NormalizedName(name)
	})
}

// HandleConfigFile handles loading the configuration file
func HandleConfigFile(v *viper.Viper, stderr io.Writer, flagSet *pflag.FlagSet) error {
	if configFlag := flagSet.Lookup("config"); configFlag != nil && configFlag.Changed {
		configFile := configFlag.Value.String()
		if verbose, _ := flagSet.GetBool("verbose"); verbose {
			fmt.Fprintf(stderr, "cgpt: trying to read config file: %s\n", configFile)
		}

		// Check if file exists and is readable
		if _, err := os.Stat(configFile); err != nil {
			if verbose, _ := flagSet.GetBool("verbose"); verbose {
				fmt.Fprintf(stderr, "cgpt: config file %s not accessible: %v\n", configFile, err)
			}
			return nil
		}

		v.SetConfigFile(configFile)
	}

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			if verbose, _ := flagSet.GetBool("debug"); verbose {
				fmt.Fprintln(stderr, "cgpt: config file not found, using defaults")
			}
			return nil
		}
		return fmt.Errorf("unable to read config file: %w", err)
	}

	if verbose, _ := flagSet.GetBool("verbose"); verbose {
		fmt.Fprintf(stderr, "cgpt: successfully read config from %s\n", v.ConfigFileUsed())
	}
	return nil
}

package cgpt

import (
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var defaultModels = map[string]string{
	"anthropic": "claude-3-5-sonnet-20240620",
	"openai":    "gpt-4o",
	"ollama":    "llama3.2",
	"googleai":  "gemini-pro",
	"dummy":     "dummy",
}

// tokenLimits is a map of regex patterns to token limits for each backend.
// The key "*" is a catch-all for any patterns not explicitly defined.
// The value for each key is the maximum number of tokens allowed for a completion.
var tokenLimits = map[string]int{
	"*":                    4096,
	"anthropic:.*sonnet.*": 8000,
}

type Config struct {
	Backend     string  `yaml:"backend"`
	Model       string  `yaml:"modelName"`
	Stream      bool    `yaml:"stream"`
	MaxTokens   int     `yaml:"maxTokens"`
	Temperature float64 `yaml:"temperature"`

	SystemPrompt string             `yaml:"systemPrompt"`
	LogitBias    map[string]float64 `yaml:"logitBias"`

	CompletionTimeout time.Duration `yaml:"completionTimeout"`

	Debug bool `yaml:"debug"`

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

	setupViper(v, flagSet)
	setupFlagNormalization(flagSet)

	if err := bindAndUnmarshal(v, flagSet, cfg); err != nil {
		return nil, err
	}

	if err := handleConfigFile(v, stderr, flagSet); err != nil {
		return nil, err
	}

	if err := setBackendAndModel(cfg, flagSet); err != nil {
		return nil, err
	}
	setMaxTokens(cfg)
	logConfig(cfg, stderr, flagSet)
	return cfg, nil
}

func setupViper(v *viper.Viper, flagSet *pflag.FlagSet) {
	v.AddConfigPath("/etc/cgpt/")
	v.AddConfigPath("$HOME/.cgpt")
	v.AddConfigPath(".")
	v.SetConfigName("config")
	if flagConfigFilePath := flagSet.Lookup("config"); flagConfigFilePath.Changed {
		v.SetConfigFile(flagConfigFilePath.Value.String())
	}

	v.SetEnvPrefix("CGPT")
	v.AutomaticEnv()
	v.BindEnv("openaiAPIKey", "OPENAI_API_KEY")
	v.BindEnv("anthropicAPIKey", "ANTHROPIC_API_KEY")
	v.BindEnv("googleAPIKey", "GOOGLE_API_KEY")
}

func setupFlagNormalization(flagSet *pflag.FlagSet) {
	normalizeFunc := flagSet.GetNormalizeFunc()
	flagSet.SetNormalizeFunc(func(fs *pflag.FlagSet, name string) pflag.NormalizedName {
		result := normalizeFunc(fs, name)
		name = strings.ReplaceAll(string(result), "-", "")
		return pflag.NormalizedName(name)
	})
}

func bindAndUnmarshal(v *viper.Viper, flagSet *pflag.FlagSet, cfg *Config) error {
	if err := v.BindPFlags(flagSet); err != nil {
		return fmt.Errorf("unable to bind flags: %w", err)
	}
	if err := v.Unmarshal(cfg); err != nil {
		return fmt.Errorf("unable to unmarshal config: %w", err)
	}
	return nil
}

func handleConfigFile(v *viper.Viper, stderr io.Writer, flagSet *pflag.FlagSet) error {
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			if v, _ := flagSet.GetBool("verbose"); v {
				fmt.Fprintln(stderr, "cgpt: config file not found, using defaults")
			}
		} else {
			return fmt.Errorf("unable to read config file: %w", err)
		}
	}
	return nil
}

func setBackendAndModel(cfg *Config, flagSet *pflag.FlagSet) error {
	flagBackend, flagModel := flagSet.Lookup("backend"), flagSet.Lookup("model")
	if flagBackend == nil || flagModel == nil {
		return fmt.Errorf("flags 'backend' and 'model' must be defined")
	}

	// Set backend
	if flagBackend.Changed {
		cfg.Backend = flagBackend.Value.String()
	} else if cfg.Backend == "" {
		cfg.Backend = flagBackend.DefValue
	}

	// Set model
	if flagModel.Changed {
		cfg.Model = flagModel.Value.String()
	} else {
		// If model is not explicitly set, choose based on the backend
		cfg.Model = defaultModels[cfg.Backend]
	}

	return nil
}

func setMaxTokens(cfg *Config) {
	maxTokens := tokenLimits["*"]
	backendModel := cfg.Backend + ":" + cfg.Model

	for pattern, limit := range tokenLimits {
		if pattern == "*" {
			continue
		}
		if matched, _ := regexp.MatchString(pattern, backendModel); matched {
			maxTokens = limit
			break
		}
	}

	if cfg.MaxTokens == 0 || cfg.MaxTokens > maxTokens {
		cfg.MaxTokens = maxTokens
	}
}
func logConfig(cfg *Config, stderr io.Writer, flagSet *pflag.FlagSet) {
	if v, _ := flagSet.GetBool("verbose"); v {
		fmt.Fprint(stderr, "cgpt-config: ")
		json.NewEncoder(stderr).Encode(cfg)
	}
}

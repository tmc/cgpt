package cgpt

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var defaultModels = map[string]string{
	"anthropic": "claude-3-5-sonnet-20240620",
	"openai":    "gpt-4o",
	"ollama":    "llama3",
	"googleai":  "gemini-pro",
}

// Config is the configuration for cgpt.
type Config struct {
	Backend   string `yaml:"backend"`
	Model     string `yaml:"modelName"`
	Stream    bool   `yaml:"stream"`
	MaxTokens int    `yaml:"maxTokens"`

	SystemPrompt string             `yaml:"systemPrompt"`
	LogitBias    map[string]float64 `yaml:"logitBias"`

	CompletionTimeout time.Duration `yaml:"completionTimeout"`

	Debug bool `yaml:"debug"`

	// API keys
	OpenAIAPIKey    string `yaml:"openaiAPIKey"`
	AnthropicAPIKey string `yaml:"anthropicAPIKey"`
	GoogleAPIKey    string `yaml:"googleAPIKey"`
}

// LoadConfig loads the config file from the given path.
// if the file is not found, it returns the default config.
func LoadConfig(path string, flagSet *pflag.FlagSet) (*Config, error) {
	cfg := &Config{}
	flagBackend, flagModel := flagSet.Lookup("backend"), flagSet.Lookup("model")
	defaultBackend := flagBackend.Value.String()

	viper.SetEnvPrefix("CGPT")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Bind all flags to Viper
	if err := viper.BindPFlags(flagSet); err != nil {
		return cfg, fmt.Errorf("unable to bind flags: %w", err)
	}

	normalizeFunc := flagSet.GetNormalizeFunc()
	flagSet.SetNormalizeFunc(func(fs *pflag.FlagSet, name string) pflag.NormalizedName {
		result := normalizeFunc(fs, name)
		name = strings.ReplaceAll(string(result), "-", "")
		return pflag.NormalizedName(name)
	})

	// Read the config file
	viper.AddConfigPath("/etc/cgpt/")
	viper.AddConfigPath("$HOME/.cgpt")
	viper.AddConfigPath(".")
	viper.SetConfigFile(path)
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			fmt.Fprintf(os.Stderr, "Config file not found, using defaults (%v)\n", err)
		} else {
			return cfg, err
		}
	}

	// Unmarshal the configuration
	if err := viper.Unmarshal(&cfg); err != nil {
		return cfg, fmt.Errorf("unable to unmarshal config: %w", err)
	}

	// Set defaults after unmarshaling
	cfg.SetDefaults(defaultBackend)

	// Override model if not explicitly set by flag
	if !flagModel.Changed {
		cfg.Model = defaultModels[cfg.Backend]
	}

	// Print the config to stderr if verbose flag is set
	if v, _ := flagSet.GetBool("verbose"); v {
		fmt.Fprint(os.Stderr, "config: ")
		json.NewEncoder(os.Stderr).Encode(cfg)
	}

	return cfg, nil
}

// SetDefaults sets the default values for the config.
func (cfg *Config) SetDefaults(defaultBackend string) *Config {
	if cfg.Backend == "" {
		cfg.Backend = defaultBackend
	}
	if cfg.MaxTokens == 0 {
		cfg.MaxTokens = 4000
	}
	if cfg.CompletionTimeout == 0 {
		cfg.CompletionTimeout = 2 * time.Minute
	}
	return cfg
}

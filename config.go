package cgpt

import (
	"fmt"
	"os"

	"github.com/spf13/viper"
)

var defaultBackend = "anthropic"
var defaultModels = map[string]string{
	"anthropic": "claude-3-opus-20240229",
	"openai":    "gpt-4o",
	"ollama":    "llama3",
}

// Config is the configuration for cgpt.
type Config struct {
	Backend   string `yaml:"backend"`
	Model     string `yaml:"modelName"`
	Stream    bool   `yaml:"stream"`
	MaxTokens int    `yaml:"maxTokens"`

	SystemPrompt string             `yaml:"systemPrompt"`
	LogitBias    map[string]float64 `yaml:"logitBias"`
}

// LoadConfigFromPath loads the config file from the given path.
// if the file is not found, it returns the default config.
func LoadConfigFromPath(path string) (*Config, error) {
	var cfg Config
	if path == "" {
		return SetDefaults(&cfg), nil
	}
	viper.AddConfigPath("/etc/cgpt/")
	viper.AddConfigPath("$HOME/.cgpt")
	viper.AddConfigPath(".")
	viper.SetConfigFile(path)
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			fmt.Fprintln(os.Stderr, "config file not found, using defaults (%w)", err)
			return &cfg, nil
		}
		return &cfg, fmt.Errorf("unable to parse config file: %w", err)
	}
	if err := viper.Unmarshal(&cfg); err != nil {
		return &cfg, fmt.Errorf("unable to unmarshal config file: %w", err)
	}
	return &cfg, nil
}

// SetDefaults sets the default values for the config.
func SetDefaults(cfg *Config) *Config {
	if cfg.Backend == "" {
		cfg.Backend = defaultBackend
	}
	if cfg.Model == "" {
		cfg.Model = defaultModels[cfg.Backend]
	}
	if cfg.MaxTokens == 0 {
		cfg.MaxTokens = 3072
	}
	return cfg
}

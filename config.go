package cgpt

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

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

	CompletionTimeout time.Duration `yaml:"completionTimeout"`
}

// LoadConfig loads the config file from the given path.
// if the file is not found, it returns the default config.
func LoadConfig(path string, flagSet *pflag.FlagSet) (*Config, error) {
	cfg := &Config{}
	flagBackend, flagModel := flagSet.Lookup("backend"), flagSet.Lookup("model")
	defaultBackend := flagBackend.Value.String()
	cfg.SetDefaults(defaultBackend)
	if !flagModel.Changed {
		flagModel.Value.Set(cfg.Model)
	}

	viper.AddConfigPath("/etc/cgpt/")
	viper.AddConfigPath("$HOME/.cgpt")
	viper.AddConfigPath(".")
	viper.SetConfigFile(path)

	normalizeFunc := flagSet.GetNormalizeFunc()
	flagSet.SetNormalizeFunc(func(fs *pflag.FlagSet, name string) pflag.NormalizedName {
		result := normalizeFunc(fs, name)
		name = strings.ReplaceAll(string(result), "-", "")
		return pflag.NormalizedName(name)
	})
	viper.BindPFlags(flagSet)

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			fmt.Fprintln(os.Stderr, "config file not found, using defaults (%w)", err)
			return cfg, nil
		}
		return cfg, fmt.Errorf("unable to parse config file: %w", err)
	}
	if err := viper.Unmarshal(&cfg); err != nil {
		return cfg, fmt.Errorf("unable to unmarshal config file: %w", err)
	}
	return cfg, nil
}

// SetDefaults sets the default values for the config.
func (cfg *Config) SetDefaults(defaultBackend string) *Config {
	if cfg.Backend == "" {
		cfg.Backend = defaultBackend
	}
	if cfg.Model == "" {
		cfg.Model = defaultModels[cfg.Backend]
	}
	if cfg.MaxTokens == 0 {
		cfg.MaxTokens = 3072
	}
	if cfg.CompletionTimeout == 0 {
		cfg.CompletionTimeout = 2 * time.Minute
	}
	return cfg
}

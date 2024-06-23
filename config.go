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
	Backend           string `yaml:"backend"`
	OPENAI_API_KEY    string `yaml:"OPENAI_API_KEY"`
	ANTHROPIC_API_KEY string `yaml:"ANTHROPIC_API_KEY"`
	GOOGLE_API_KEY    string `yaml:"GOOGLE_API_KEY"`
	Model             string `yaml:"modelName"`
	Stream            bool   `yaml:"stream"`
	MaxTokens         int    `yaml:"maxTokens"`

	SystemPrompt string             `yaml:"systemPrompt"`
	LogitBias    map[string]float64 `yaml:"logitBias"`

	CompletionTimeout time.Duration `yaml:"completionTimeout"`

	Debug bool `yaml:"debug"`
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

	// Print the config to stderr if verbose flag is set.
	defer func() {
		if v, _ := flagSet.GetBool("verbose"); v {
			fmt.Fprint(os.Stderr, "config ")
			json.NewEncoder(os.Stderr).Encode(cfg)
		}
	}()
	if err := viper.BindPFlags(flagSet); err != nil {
		return cfg, fmt.Errorf("unable to bind flags: %w", err)
	}
	// marshal here in case the config file below doesn't exist
	if err := viper.Unmarshal(&cfg); err != nil {
		return cfg, fmt.Errorf("unable to unmarshal config file: %w", err)
	}
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			fmt.Fprintln(os.Stderr, "config file not found, using defaults (%w)", err)
			return cfg, nil
		}
		return cfg, err
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
		cfg.MaxTokens = 4000
	}
	if cfg.CompletionTimeout == 0 {
		cfg.CompletionTimeout = 2 * time.Minute
	}
	return cfg
}

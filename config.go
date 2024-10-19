package cgpt

import (
	"encoding/json"
	"fmt"
	"io"
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
	"dummy":     "dummy",
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

func LoadConfig(path string, stderr io.Writer, flagSet *pflag.FlagSet) (*Config, error) {
	if flagSet == nil {
		flagSet = pflag.CommandLine
	}
	cfg := &Config{}
	flagBackend, flagModel := flagSet.Lookup("backend"), flagSet.Lookup("model")
	if flagBackend == nil {
		return cfg, fmt.Errorf("flag 'backend' not found")
	}
	defaultBackend := flagBackend.Value.String()
	cfg.SetDefaults(defaultBackend)
	if !flagModel.Changed {
		flagModel.Value.Set(cfg.Model)
	}
	v := viper.New()

	v.AddConfigPath("/etc/cgpt/")
	v.AddConfigPath("$HOME/.cgpt")
	v.AddConfigPath(".")
	v.SetConfigName("config")
	flagConfigFilePath := flagSet.Lookup("config")
	if flagConfigFilePath.Changed {
		v.SetConfigFile(flagConfigFilePath.Value.String())
	}

	v.SetEnvPrefix("CGPT")
	v.AutomaticEnv()
	v.BindEnv("openaiAPIKey", "OPENAI_API_KEY")
	v.BindEnv("anthropicAPIKey", "ANTHROPIC_API_KEY")
	v.BindEnv("googleAPIKey", "GOOGLE_API_KEY")

	normalizeFunc := flagSet.GetNormalizeFunc()
	flagSet.SetNormalizeFunc(func(fs *pflag.FlagSet, name string) pflag.NormalizedName {
		result := normalizeFunc(fs, name)
		name = strings.ReplaceAll(string(result), "-", "")
		return pflag.NormalizedName(name)
	})

	defer func() {
		if v, _ := flagSet.GetBool("verbose"); v {
			fmt.Fprint(stderr, "cgpt-config: ")
			json.NewEncoder(stderr).Encode(cfg)
		}
	}()
	if err := v.BindPFlags(flagSet); err != nil {
		return cfg, fmt.Errorf("unable to bind flags: %w", err)
	}
	if err := v.Unmarshal(&cfg); err != nil {
		return cfg, fmt.Errorf("unable to unmarshal config file: %w", err)
	}
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			if v, _ := flagSet.GetBool("verbose"); v {
				fmt.Fprintln(stderr, "config file not found, using defaults")
			}
			return cfg, nil
		}
		return cfg, fmt.Errorf("unable to read config file: %w", err)
	}
	if err := v.Unmarshal(&cfg); err != nil {
		return cfg, fmt.Errorf("unable to unmarshal config file: %w", err)
	}
	return cfg, nil
}

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

// MergeConfigs merges two Config structs, with the second taking precedence
func MergeConfigs(base, override Config) Config {
	merged := base

	if override.OpenAIAPIKey != "" {
		merged.OpenAIAPIKey = override.OpenAIAPIKey
	}
	if override.AnthropicAPIKey != "" {
		merged.AnthropicAPIKey = override.AnthropicAPIKey
	}
	if override.GoogleAPIKey != "" {
		merged.GoogleAPIKey = override.GoogleAPIKey
	}
	if override.Backend != "" {
		merged.Backend = override.Backend
	}
	if override.Model != "" {
		merged.Model = override.Model
	}
	if override.MaxTokens != 0 {
		merged.MaxTokens = override.MaxTokens
	}
	if override.Temperature != 0 {
		merged.Temperature = override.Temperature
	}

	return merged
}

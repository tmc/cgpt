package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v2"
)

const (
	defaultModel     = "gpt-3.5-turbo"
	defaultMaxTokens = 1024
)

// Config is the configuration for cgpt.
type Config struct {
	APIKey    string `yaml:"apiKey"`
	Model     string `yaml:"model"`
	Stream    bool   `yaml:"stream"`
	MaxTokens int    `yaml:"maxTokens"`

	SystemPrompt string `yaml:"systemPrompt"`

	LogitBias map[string]float64 `yaml:"logitBias"`
}

// loadConfig loads the config file from the given path.
// if the file is not found, it returns the default config.
func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("unable to read config file: %w", err)
	}
	var cfg Config
	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		return nil, fmt.Errorf("unable to parse config file: %w", err)
	}
	if cfg.Model == "" {
		cfg.Model = defaultModel
	}

	// Prefer env var over config file:
	if apiKey := os.Getenv("OPENAI_API_KEY"); apiKey != "" {
		cfg.APIKey = apiKey
	}
	return &cfg, nil
}

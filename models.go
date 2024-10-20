package cgpt

import (
	"context"
	"fmt"
	"net/http"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/anthropic"
	"github.com/tmc/langchaingo/llms/googleai"
	"github.com/tmc/langchaingo/llms/ollama"
	"github.com/tmc/langchaingo/llms/openai"
)

// ModelOption is a function that modifies the model options
type ModelOption func(*modelOptions)

type modelOptions struct {
	httpClient *http.Client
}

// WithHTTPClient sets a custom HTTP client for the model
func WithHTTPClient(client *http.Client) ModelOption {
	return func(mo *modelOptions) {
		mo.httpClient = client
	}
}

// InitializeModel initializes the model with the given configuration and options.
func InitializeModel(cfg *Config, opts ...ModelOption) (llms.Model, error) {
	mo := &modelOptions{}
	for _, opt := range opts {
		opt(mo)
	}

	constructor, ok := modelConstructors[cfg.Backend]
	if !ok {
		return nil, fmt.Errorf("unknown backend %q", cfg.Backend)
	}

	return constructor(cfg, mo)
}

type modelConstructor func(*Config, *modelOptions) (llms.Model, error)

var modelConstructors = map[string]modelConstructor{
	"openai": func(cfg *Config, mo *modelOptions) (llms.Model, error) {
		options := []openai.Option{openai.WithModel(cfg.Model)}
		if cfg.OpenAIAPIKey != "" {
			options = append(options, openai.WithToken(cfg.OpenAIAPIKey))
		}
		if mo.httpClient != nil {
			options = append(options, openai.WithHTTPClient(mo.httpClient))
		}
		return openai.New(options...)
	},
	"anthropic": func(cfg *Config, mo *modelOptions) (llms.Model, error) {
		options := []anthropic.Option{anthropic.WithModel(cfg.Model)}
		if cfg.AnthropicAPIKey != "" {
			options = append(options, anthropic.WithToken(cfg.AnthropicAPIKey))
		}
		if cfg.Model == "claude-3-5-sonnet-20240620" {
			options = append(options, anthropic.WithAnthropicBetaHeader(anthropic.MaxTokensAnthropicSonnet35))
		}
		if mo.httpClient != nil {
			options = append(options, anthropic.WithHTTPClient(mo.httpClient))
		}
		return anthropic.New(options...)
	},
	"ollama": func(cfg *Config, mo *modelOptions) (llms.Model, error) {
		options := []ollama.Option{ollama.WithModel(cfg.Model)}
		if mo.httpClient != nil {
			options = append(options, ollama.WithHTTPClient(mo.httpClient))
		}
		return ollama.New(options...)
	},
	"googleai": func(cfg *Config, mo *modelOptions) (llms.Model, error) {
		options := []googleai.Option{googleai.WithDefaultModel(cfg.Model)}
		if cfg.GoogleAPIKey != "" {
			options = append(options, googleai.WithAPIKey(cfg.GoogleAPIKey))
		}
		if mo.httpClient != nil {
			options = append(options, googleai.WithHTTPClient(mo.httpClient))
		}
		return googleai.New(context.TODO(), options...)
	},
	"dummy": func(cfg *Config, mo *modelOptions) (llms.Model, error) {
		return NewDummyBackend()
	},
}

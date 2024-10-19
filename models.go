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
type ModelOption func(interface{})

// WithHTTPClient sets a custom HTTP client for the model
func WithHTTPClient(client *http.Client) ModelOption {
	return func(opts interface{}) {
		switch o := opts.(type) {
		case *[]openai.Option:
			*o = append(*o, openai.WithHTTPClient(client))
		case *[]anthropic.Option:
			*o = append(*o, anthropic.WithHTTPClient(client))
		case *[]ollama.Option:
			*o = append(*o, ollama.WithHTTPClient(client))
		case *[]googleai.Option:
			*o = append(*o, googleai.WithHTTPClient(client))
		}
	}
}

// InitializeModel initializes the model with the given configuration and options.
func InitializeModel(cfg *Config, opts ...ModelOption) (llms.Model, error) {
	model, err := initializeModel(cfg.Backend, cfg.Model, cfg.Debug, cfg, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize model: %w", err)
	}
	return model, nil
}

var constructors = map[string]func(modelName string, debugMode bool, apiKey string, opts ...ModelOption) (llms.Model, error){
	"openai": func(modelName string, debugMode bool, apiKey string, opts ...ModelOption) (llms.Model, error) {
		options := []openai.Option{openai.WithModel(modelName)}
		if apiKey != "" {
			options = append(options, openai.WithToken(apiKey))
		}
		for _, opt := range opts {
			opt(&options)
		}
		return openai.New(options...)
	},
	"anthropic": func(modelName string, debugMode bool, apiKey string, opts ...ModelOption) (llms.Model, error) {
		options := []anthropic.Option{anthropic.WithModel(modelName)}
		if apiKey != "" {
			options = append(options, anthropic.WithToken(apiKey))
		}
		if modelName == "claude-3-5-sonnet-20240620" {
			options = append(options, anthropic.WithAnthropicBetaHeader(anthropic.MaxTokensAnthropicSonnet35))
		}
		for _, opt := range opts {
			opt(&options)
		}
		return anthropic.New(options...)
	},
	"ollama": func(modelName string, debugMode bool, apiKey string, opts ...ModelOption) (llms.Model, error) {
		options := []ollama.Option{ollama.WithModel(modelName)}
		for _, opt := range opts {
			opt(&options)
		}
		return ollama.New(options...)
	},
	"googleai": func(modelName string, debugMode bool, apiKey string, opts ...ModelOption) (llms.Model, error) {
		options := []googleai.Option{googleai.WithDefaultModel(modelName)}
		if apiKey != "" {
			options = append(options, googleai.WithAPIKey(apiKey))
		}
		for _, opt := range opts {
			opt(&options)
		}
		return googleai.New(context.TODO(), options...)
	},
	"dummy": func(modelName string, debugMode bool, apiKey string, opts ...ModelOption) (llms.Model, error) {
		return NewDummyBackend()
	},
}

func initializeModel(backend, modelName string, debugMode bool, cfg *Config, opts ...ModelOption) (llms.Model, error) {
	constructor, ok := constructors[backend]
	if !ok {
		return nil, fmt.Errorf("unknown backend %q", backend)
	}
	var apiKey string
	switch backend {
	case "openai":
		apiKey = cfg.OpenAIAPIKey
	case "anthropic":
		apiKey = cfg.AnthropicAPIKey
	case "googleai":
		apiKey = cfg.GoogleAPIKey
	}
	return constructor(modelName, debugMode, apiKey, opts...)
}

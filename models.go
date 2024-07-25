package cgpt

import (
	"context"
	"fmt"

	"github.com/tmc/langchaingo/httputil"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/anthropic"
	"github.com/tmc/langchaingo/llms/googleai"
	"github.com/tmc/langchaingo/llms/ollama"
	"github.com/tmc/langchaingo/llms/openai"
)

// extend these to accept a debugMode flag

var constructors = map[string]func(modelName string, debugMode bool, apiKey string) (llms.Model, error){
	"openai": func(modelName string, debugMode bool, apiKey string) (llms.Model, error) {
		options := []openai.Option{openai.WithModel(modelName)}
		if apiKey != "" {
			options = append(options, openai.WithToken(apiKey))
		}
		if debugMode {
			options = append(options, openai.WithHTTPClient(httputil.DebugHTTPClient))
		}
		return openai.New(options...)
	},
	"anthropic": func(modelName string, debugMode bool, apiKey string) (llms.Model, error) {
		options := []anthropic.Option{anthropic.WithModel(modelName)}
		if apiKey != "" {
			options = append(options, anthropic.WithToken(apiKey))
		}
		if debugMode {
			options = append(options, anthropic.WithHTTPClient(httputil.DebugHTTPClient))
		}
		if modelName == "claude-3-5-sonnet-20240620" {
			options = append(options, anthropic.WithAnthropicBetaHeader(anthropic.MaxTokensAnthropicSonnet35))
		}
		return anthropic.New(options...)
	},
	"ollama": func(modelName string, debugMode bool, apiKey string) (llms.Model, error) {
		options := []ollama.Option{ollama.WithModel(modelName)}
		if debugMode {
			options = append(options, ollama.WithHTTPClient(httputil.DebugHTTPClient))
		}
		return ollama.New(options...)
	},
	"googleai": func(modelName string, debugMode bool, apiKey string) (llms.Model, error) {
		options := []googleai.Option{googleai.WithDefaultModel(modelName)}
		if apiKey != "" {
			options = append(options, googleai.WithAPIKey(apiKey))
		}
		if debugMode {
			options = append(options, googleai.WithHTTPClient(httputil.DebugHTTPClient))
		}
		return googleai.New(context.TODO(), options...)
	},
}

func initializeModel(backend, modelName string, debugMode bool, cfg *Config) (llms.Model, error) {
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
	return constructor(modelName, debugMode, apiKey)
}

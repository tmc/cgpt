// Package openai provides the OpenAI backend implementation
package openai

import (
	"os"

	"github.com/tmc/cgpt/backends/registry"
	"github.com/tmc/cgpt/options"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
)

func init() {
	registry.Register("openai", Constructor)
}

// Constructor creates a new OpenAI backend
func Constructor(cfg *options.Config, opts *options.InferenceProviderOptions) (llms.Model, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if cfg.OpenAIAPIKey != "" {
		apiKey = cfg.OpenAIAPIKey
	}

	openaiOpts := []openai.Option{
		openai.WithToken(apiKey),
	}

	if opts.HTTPClient != nil {
		openaiOpts = append(openaiOpts, openai.WithHTTPClient(opts.HTTPClient))
	}

	if opts.UseLegacyMaxTokens {
		openaiOpts = append(openaiOpts, openai.WithUseLegacyMaxTokens(true))
	}

	return openai.New(openaiOpts...)
}

// Package anthropic provides the Anthropic backend implementation
package anthropic

import (
	"os"

	"github.com/tmc/cgpt/backends/registry"
	"github.com/tmc/cgpt/options"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/anthropic"
)

func init() {
	registry.Register("anthropic", Constructor)
}

// Constructor creates a new Anthropic backend
func Constructor(cfg *options.Config, opts *options.InferenceProviderOptions) (llms.Model, error) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if cfg.AnthropicAPIKey != "" {
		apiKey = cfg.AnthropicAPIKey
	}

	anthropicOpts := []anthropic.Option{
		anthropic.WithToken(apiKey),
		anthropic.WithModel(cfg.Model),
	}

	if opts.HTTPClient != nil {
		anthropicOpts = append(anthropicOpts, anthropic.WithHTTPClient(opts.HTTPClient))
	}

	return anthropic.New(anthropicOpts...)
}

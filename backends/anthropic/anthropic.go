// Package anthropic provides the Anthropic backend implementation
package anthropic

import (
	"net/http"

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
	apiKey := opts.EnvLookupFunc("ANTHROPIC_API_KEY")
	if cfg.AnthropicAPIKey != "" {
		apiKey = cfg.AnthropicAPIKey
	}

	anthropicOpts := []anthropic.Option{
		anthropic.WithToken(apiKey),
	}

	if opts.HTTPClient != nil {
		anthropicOpts = append(anthropicOpts, anthropic.WithHTTPClient(opts.HTTPClient))
	} else {
		anthropicOpts = append(anthropicOpts, anthropic.WithHTTPClient(antHTTPClient))
	}
	anthropicOpts = append(anthropicOpts, anthropic.WithToken(opts.EnvLookupFunc("ANTHROPIC_API_KEY")))
	return anthropic.New(anthropicOpts...)
}

// antHTTPClient is a placeholder for the JavaScript HTTP client
var antHTTPClient = http.DefaultClient

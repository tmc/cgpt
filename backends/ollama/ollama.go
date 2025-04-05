// Package ollama provides the Ollama backend implementation
package ollama

import (
	"github.com/tmc/cgpt/backends/registry"
	"github.com/tmc/cgpt/options"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/ollama"
)

func init() {
	registry.Register("ollama", Constructor)
}

// Constructor creates a new Ollama backend
func Constructor(cfg *options.Config, opts *options.InferenceProviderOptions) (llms.Model, error) {
	ollamaOpts := []ollama.Option{
		ollama.WithModel(cfg.Model),
	}

	if opts.HTTPClient != nil {
		ollamaOpts = append(ollamaOpts, ollama.WithHTTPClient(opts.HTTPClient))
	}

	return ollama.New(ollamaOpts...)
}

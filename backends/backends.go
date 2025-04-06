// Package backends provides a unified interface to various LLM backends
package backends

import (
	"net/http"

	"github.com/tmc/cgpt/backends/registry"
	"github.com/tmc/cgpt/options"
	"github.com/tmc/langchaingo/llms"

	// Register all backends
	_ "github.com/tmc/cgpt/backends/anthropic"
	_ "github.com/tmc/cgpt/backends/dummy"
	_ "github.com/tmc/cgpt/backends/googleai"
	_ "github.com/tmc/cgpt/backends/ollama"
	_ "github.com/tmc/cgpt/backends/openai"
)

type InferenceProviderOption = options.InferenceProviderOption

// InitializeModel initializes the model based on the given configuration
func InitializeModel(cfg *options.Config, providerOpts ...options.InferenceProviderOption) (llms.Model, error) {
	return registry.InitializeModel(cfg, providerOpts...)
}

// WithHTTPClient returns an option to set the HTTP client for the inference provider
func WithHTTPClient(client *http.Client) options.InferenceProviderOption {
	return registry.WithHTTPClient(client)
}

// WithUseLegacyMaxTokens returns an option to use legacy max tokens behavior for OpenAI
func WithUseLegacyMaxTokens(useLegacy bool) options.InferenceProviderOption {
	return registry.WithUseLegacyMaxTokens(useLegacy)
}

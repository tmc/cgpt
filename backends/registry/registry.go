// Package registry provides a registry for model backends
package registry

import (
	"fmt"
	"net/http"

	"github.com/tmc/cgpt/options"
	"github.com/tmc/langchaingo/llms"
)

// BackendConstructor is a function that creates a new model instance
type BackendConstructor func(*options.Config, *options.InferenceProviderOptions) (llms.Model, error)

// Registry holds all the registered backend constructors
var registry = map[string]BackendConstructor{}

// Register registers a new backend constructor
func Register(name string, constructor BackendConstructor) {
	registry[name] = constructor
}

// Get returns a backend constructor by name
func Get(name string) (BackendConstructor, bool) {
	constructor, ok := registry[name]
	return constructor, ok
}

// WithHTTPClient returns an option to set the HTTP client for the inference provider
func WithHTTPClient(client *http.Client) options.InferenceProviderOption {
	return func(opts *options.InferenceProviderOptions) {
		opts.HTTPClient = client
	}
}

// WithUseLegacyMaxTokens returns an option to use legacy max tokens behavior for OpenAI
func WithUseLegacyMaxTokens(useLegacy bool) options.InferenceProviderOption {
	return func(opts *options.InferenceProviderOptions) {
		opts.UseLegacyMaxTokens = useLegacy
	}
}

// InitializeModel initializes the model based on the given configuration
func InitializeModel(cfg *options.Config, providerOpts ...options.InferenceProviderOption) (llms.Model, error) {
	// Apply provider options
	opts := &options.InferenceProviderOptions{}
	for _, option := range providerOpts {
		option(opts)
	}

	constructor, ok := registry[cfg.Backend]
	if !ok {
		return nil, fmt.Errorf("unsupported backend: %s", cfg.Backend)
	}

	return constructor(cfg, opts)
}

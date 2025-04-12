// Package dummy provides a dummy backend for testing
package dummy

import (
	"github.com/tmc/cgpt/backends/registry"
	"github.com/tmc/cgpt/options"
	"github.com/tmc/langchaingo/llms"
)

func init() {
	registry.Register("dummy", Constructor)
}

// Constructor creates a new dummy backend
func Constructor(cfg *options.Config, opts *options.InferenceProviderOptions) (llms.Model, error) {
	backend, err := NewDummyBackend()
	if err != nil {
		return nil, err
	}
	
	// Configure the backend with options from config
	if cfg.SlowResponses {
		backend.SlowResponses = true
	}
	
	return backend, nil
}

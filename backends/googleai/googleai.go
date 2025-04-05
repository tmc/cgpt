// Package googleai provides the Google AI backend implementation
package googleai

import (
	"context"
	"os"

	"github.com/tmc/cgpt/backends/registry"
	"github.com/tmc/cgpt/options"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/googleai"
)

func init() {
	registry.Register("googleai", Constructor)
}

// Constructor creates a new GoogleAI backend
func Constructor(cfg *options.Config, opts *options.InferenceProviderOptions) (llms.Model, error) {
	apiKey := os.Getenv("GOOGLE_API_KEY")
	if cfg.GoogleAPIKey != "" {
		apiKey = cfg.GoogleAPIKey
	}

	googleOpts := []googleai.Option{
		googleai.WithAPIKey(apiKey),
		googleai.WithDefaultModel(cfg.Model),
	}

	if opts.HTTPClient != nil {
		googleOpts = append(googleOpts, googleai.WithHTTPClient(opts.HTTPClient))
	}

	return googleai.New(context.Background(), googleOpts...)
}

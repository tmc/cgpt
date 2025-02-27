package cgpt

import (
	"context"

	"github.com/tmc/langchaingo/llms"
)

// RetryWrapper wraps an LLM client with retry functionality
type RetryWrapper struct {
	client llms.Model
	config RetryConfig
}

// NewRetryWrapper creates a new RetryWrapper with the given client and config
func NewRetryWrapper(client llms.Model, config RetryConfig) *RetryWrapper {
	return &RetryWrapper{
		client: client,
		config: config,
	}
}

// Call implements the llms.Model interface with retry functionality
func (w *RetryWrapper) Call(ctx context.Context, prompt string, options ...llms.CallOption) (string, error) {
	var result string
	_, err := WithRetry(ctx, func(ctx context.Context) (string, error) {
		return w.client.Call(ctx, prompt, options...)
	}, w.config)
	return result, err
}

// GenerateContent implements the llms.Model interface with retry functionality
func (w *RetryWrapper) GenerateContent(ctx context.Context, messages []llms.MessageContent, options ...llms.CallOption) (*llms.ContentResponse, error) {
	var result *llms.ContentResponse
	_, err := WithRetry(ctx, func(ctx context.Context) (*llms.ContentResponse, error) {
		return w.client.GenerateContent(ctx, messages, options...)
	}, w.config)
	return result, err
}

// CreateEmbedding implements the llms.Model interface with retry functionality
func (w *RetryWrapper) CreateEmbedding(ctx context.Context, text string) ([]float64, error) {
	var result []float64
	_, err := WithRetry(ctx, func(ctx context.Context) ([]float64, error) {
		return w.client.CreateEmbedding(ctx, text)
	}, w.config)
	return result, err
}

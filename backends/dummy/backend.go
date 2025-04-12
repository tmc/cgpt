package dummy

import (
	"context"
	"strings"
	"time"

	"github.com/tmc/langchaingo/llms"
)

// DummyBackend is a mock LLM implementation for testing
type DummyBackend struct {
	GenerateText  func() string
	SlowResponses bool // When true, adds significant delay between tokens
}

// NewDummyBackend creates a new DummyBackend with default settings
func NewDummyBackend() (*DummyBackend, error) {
	return &DummyBackend{
		GenerateText: func() string { return dummyDefaultText },
	}, nil
}

var dummyDefaultText = `This is a dummy backend response. It will stream out a few hundred tokens to simulate a real backend. The quick brown fox jumps over the lazy dog. This pangram contains every letter of the English alphabet at least once. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum. This concludes the dummy backend response. Thank you for using the dummy backend!`

// Call implements the llms.Model interface
func (d *DummyBackend) Call(ctx context.Context, prompt string, options ...llms.CallOption) (string, error) {
	messages := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeHuman, prompt),
	}
	response, err := d.GenerateContent(ctx, messages, options...)
	if err != nil {
		return "", err
	}
	if len(response.Choices) > 0 {
		return response.Choices[0].Content, nil
	}
	return "", nil
}

// GenerateContent implements the llms.Model interface
func (d *DummyBackend) GenerateContent(ctx context.Context, messages []llms.MessageContent, options ...llms.CallOption) (*llms.ContentResponse, error) {
	// Get the exact text to return
	dummyText := d.GenerateText()

	// Create the response
	response := &llms.ContentResponse{
		Choices: []*llms.ContentChoice{
			{
				Content: dummyText,
			},
		},
	}

	opts := llms.CallOptions{}
	for _, opt := range options {
		opt(&opts)
	}

	// Handle streaming
	if opts.StreamingFunc != nil {
		// For streaming, we want to simulate token-by-token output
		// But we need to ensure the final result matches exactly the expected output
		words := strings.Fields(dummyText)

		// Send one word at a time
		for i, word := range words {
			select {
			case <-ctx.Done():
				return response, ctx.Err()
			default:
				// Add space between words but not after the last word
				if i < len(words)-1 {
					if err := opts.StreamingFunc(ctx, []byte(word+" ")); err != nil {
						return response, err
					}
				} else {
					if err := opts.StreamingFunc(ctx, []byte(word)); err != nil {
						return response, err
					}
				}
				
				// Determine delay based on SlowResponses flag
				var delay time.Duration
				if d.SlowResponses {
					delay = 300 * time.Millisecond // Significantly slower response
				} else {
					delay = 40 * time.Millisecond // Normal streaming delay
				}
				time.Sleep(delay)
			}
		}

		// Make sure the final response is exactly the expected text
		// Ensure no trailing whitespace to match the expected output
		return response, nil
	}

	return response, nil
}

// CreateEmbedding implements the llms.Model interface
func (d *DummyBackend) CreateEmbedding(ctx context.Context, text string) ([]float64, error) {
	// Dummy embedding (just return a fixed-size vector of zeros)
	return make([]float64, 128), nil
}
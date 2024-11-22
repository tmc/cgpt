package cgpt

import (
	"context"
	"strings"
	"time"

	"github.com/tmc/langchaingo/llms"
)

type DummyBackend struct {
	GenerateText func() string
}

func NewDummyBackend() (*DummyBackend, error) {
	return &DummyBackend{
		GenerateText: func() string { return dummyDefaultText },
	}, nil
}

var dummyDefaultText = `This is a dummy backend response. It will stream out a few hundred tokens to simulate a real backend.

The quick brown fox jumps over the lazy dog. This pangram contains every letter of the English alphabet at least once.

Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum.

This concludes the dummy backend response. Thank you for using the dummy backend!`

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

func (d *DummyBackend) GenerateContent(ctx context.Context, messages []llms.MessageContent, options ...llms.CallOption) (*llms.ContentResponse, error) {
	dummyText := d.GenerateText()
	words := strings.Fields(dummyText)
	response := &llms.ContentResponse{
		Choices: []*llms.ContentChoice{
			{
				Content: strings.Join(words, " "),
			},
		},
	}

	opts := llms.CallOptions{}
	for _, opt := range options {
		opt(&opts)
	}

	if opts.StreamingFunc != nil {
		for _, word := range words {
			select {
			case <-ctx.Done():
				return response, ctx.Err()
			default:
				if err := opts.StreamingFunc(ctx, []byte(word+" ")); err != nil {
					return response, err
				}
				time.Sleep(40 * time.Millisecond) // Simulate streaming delay
			}
		}
		return response, nil
	}

	return response, nil
}

func (d *DummyBackend) CreateEmbedding(ctx context.Context, text string) ([]float64, error) {
	// Dummy embedding (just return a fixed-size vector of zeros)
	return make([]float64, 128), nil
}

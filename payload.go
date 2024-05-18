package cgpt

import (
	"context"
	"log"
	"time"

	"github.com/tmc/langchaingo/llms"
)

var defaultMaxTokens = 2048

// newCompletionPayload creates a new completion payload.
func newCompletionPayload(cfg *Config) *ChatCompletionPayload {
	p := &ChatCompletionPayload{
		Model:  cfg.Model,
		Stream: cfg.Stream,
	}
	return p
}

type ChatCompletionPayload struct {
	Model    string `json:"model"`
	Messages []llms.MessageContent
	Stream   bool `json:"stream,omitempty"`
}

func (p *ChatCompletionPayload) addMessage(role llms.ChatMessageType, content string) {
	p.Messages = append(p.Messages, llms.TextParts(role, content))
}

func (p *ChatCompletionPayload) addSystemMessage(content string) {
	p.addMessage(llms.ChatMessageTypeSystem, content)
}

func (p *ChatCompletionPayload) addUserMessage(content string) {
	p.addMessage(llms.ChatMessageTypeHuman, content)
}

func (p *ChatCompletionPayload) addAssistantMessage(content string) {
	p.addMessage(llms.ChatMessageTypeAI, content)
}

func (s *CompletionService) PerformCompletion(ctx context.Context, payload *ChatCompletionPayload) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	defer spin()()

	return "", nil
}

func (s *CompletionService) PerformCompletionStreaming(ctx context.Context, payload *ChatCompletionPayload, showSpinner bool) (<-chan string, error) {
	ch := make(chan string)
	go func() {
		defer close(ch)
		if showSpinner {
			defer spin()()
		}
		_, err := s.model.GenerateContent(ctx, payload.Messages, llms.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
			ch <- string(chunk)
			return nil
		}))
		if err != nil {
			log.Fatalf("failed to generate content: %v", err)
		}
	}()
	return ch, nil
}

package cgpt

import (
	"context"
	"log"

	"github.com/tmc/langchaingo/llms"
)

// The default maximum number of tokens allowed in a single request.
// This value is used to limit the size of the input to prevent excessive resource usage.
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

func (s *CompletionService) PerformCompletionStreaming(ctx context.Context, payload *ChatCompletionPayload, cfg PerformCompletionConfig) (<-chan string, error) {
	ch := make(chan string)
	chunkSeen := func() {}
	go func() {
		defer close(ch)
		if cfg.ShowSpinner {
			chunkSeen = spin()
		}
		if s.nextCompletionPrefill != "" {
			// If the user has provided a prefill message, and hasn't opted out, then send it.
			if !cfg.EchoPrefill {
				ch <- s.nextCompletionPrefill
			}
			payload.addAssistantMessage(s.nextCompletionPrefill)
			s.nextCompletionPrefill = ""
		}
		_, err := s.model.GenerateContent(ctx, payload.Messages,
			llms.WithMaxTokens(s.cfg.MaxTokens),
			llms.WithTemperature(s.cfg.Temperature),
			llms.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
				chunkSeen()
				ch <- string(chunk)
				return nil
			}))
		if err != nil {
			log.Fatalf("failed to generate content: %v", err)
		}
	}()
	return ch, nil
}

func (s *CompletionService) SetNextCompletionPrefill(content string) {
	s.nextCompletionPrefill = content
}

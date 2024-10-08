package cgpt

import (
	"context"
	"fmt"
	"log"
	"strings"

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

// PerformCompletion provides a non-streaming version of the completion.
func (s *CompletionService) PerformCompletion(ctx context.Context, payload *ChatCompletionPayload, cfg PerformCompletionConfig) (string, error) {
	var stopSpinner func()
	if cfg.ShowSpinner {
		stopSpinner = spin()
		defer stopSpinner()
	}

	if s.nextCompletionPrefill != "" {
		if !cfg.EchoPrefill {
			fmt.Print(s.nextCompletionPrefill)
		}
		payload.addAssistantMessage(s.nextCompletionPrefill)
		s.nextCompletionPrefill = ""
	}

	response, err := s.model.GenerateContent(ctx, payload.Messages,
		llms.WithMaxTokens(s.cfg.MaxTokens),
		llms.WithTemperature(s.cfg.Temperature))
	if err != nil {
		return "", fmt.Errorf("failed to generate content: %w", err)
	}

	return response.Choices[0].Content, nil
}

// SetNextCompletionPrefill sets the next completion prefill message.
// Note that not all inference engines support prefill messages.
// Whitespace is trimmed from the end of the message.
func (s *CompletionService) SetNextCompletionPrefill(content string) {
	s.nextCompletionPrefill = strings.TrimRight(content, " \t\n")
}

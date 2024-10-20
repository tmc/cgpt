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
	go func() {
		defer close(ch)
		fullResponse := strings.Builder{}
		firstChunk := true

		prefillCleanup, spinnerPos := s.handleAssistantPrefill(ctx, payload, cfg)

		// Send prefill immediately if it exists
		if s.nextCompletionPrefill != "" {
			if cfg.EchoPrefill {
				spinnerPos = len(s.nextCompletionPrefill) + 1
			}
			ch <- s.nextCompletionPrefill + " "
			payload.addAssistantMessage(s.nextCompletionPrefill)
			fullResponse.WriteString(s.nextCompletionPrefill)
		}

		// Start spinner on the last character
		var spinnerStop func()
		if cfg.ShowSpinner {
			// Start spinner on the last character
			spinnerStop = spin(spinnerPos)
		}

		_, err := s.model.GenerateContent(ctx, payload.Messages,
			llms.WithMaxTokens(s.cfg.MaxTokens),
			llms.WithTemperature(s.cfg.Temperature),
			llms.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
				if firstChunk {
					prefillCleanup()
					if spinnerStop != nil {
						spinnerStop()
						spinnerStop = nil
					}
					firstChunk = false
				}

				ch <- string(chunk)
				fullResponse.Write(chunk)
				return nil
			}))

		if err != nil {
			log.Printf("failed to generate content: %v", err)
		}

		// Clean up spinner if it's still running
		if spinnerStop != nil {
			spinnerStop()
		}

		s.nextCompletionPrefill = ""
	}()
	return ch, nil
}

// PerformCompletion provides a non-streaming version of the completion.
func (s *CompletionService) PerformCompletion(ctx context.Context, payload *ChatCompletionPayload, cfg PerformCompletionConfig) (string, error) {
	var stopSpinner func()
	var spinnerPos int

	prefillCleanup, spinnerPos := s.handleAssistantPrefill(ctx, payload, cfg)
	defer prefillCleanup()
	if cfg.ShowSpinner {
		stopSpinner = spin(spinnerPos)
		defer stopSpinner()
	}

	response, err := s.model.GenerateContent(ctx, payload.Messages,
		llms.WithMaxTokens(s.cfg.MaxTokens),
		llms.WithTemperature(s.cfg.Temperature))
	if err != nil {
		return "", fmt.Errorf("failed to generate content: %w", err)
	}
	if len(response.Choices) == 0 {
		return "", fmt.Errorf("no response from model")
	}

	return response.Choices[0].Content, nil
}

// handleAssistantPrefill handles the assistant prefill message.
// It returns a cleanup function that should be called after the completion is done.
// The second return value is the location where the spinner could start.
func (s *CompletionService) handleAssistantPrefill(ctx context.Context, payload *ChatCompletionPayload, cfg PerformCompletionConfig) (func(), int) {
	spinnerPos := 0
	if s.nextCompletionPrefill == "" {
		return func() {
		}, spinnerPos
	}
	if cfg.EchoPrefill {
		s.Stdout.Write([]byte(s.nextCompletionPrefill))
		spinnerPos = len(s.nextCompletionPrefill) + 1
	}
	payload.addAssistantMessage(s.nextCompletionPrefill)
	s.nextCompletionPrefill = ""
	return func() {
		// cleanup payload message
		payload.Messages = payload.Messages[:len(payload.Messages)-1]
	}, spinnerPos
}

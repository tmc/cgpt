package cgpt

import (
	"context"
	"errors"
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

type Message = []llms.MessageContent

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
		addedAssistantMessage := false

		prefillCleanup, spinnerPos := s.handleAssistantPrefill(ctx, payload, cfg)

		// Send prefill immediately if it exists
		if s.nextCompletionPrefill != "" {
			if cfg.EchoPrefill {
				spinnerPos = len(s.nextCompletionPrefill) + 1
			}
			select {
			case ch <- s.nextCompletionPrefill + " ":
			case <-ctx.Done():
				prefillCleanup()
				return
			}
			payload.addAssistantMessage(s.nextCompletionPrefill)
			addedAssistantMessage = true
			fullResponse.WriteString(s.nextCompletionPrefill)
		}

		// Start spinner on the last character
		var spinnerStop func()
		if cfg.ShowSpinner {
			spinnerStop = spin(spinnerPos)
		}

		// Create a cancellable context for the generation
		genCtx, cancel := context.WithCancel(ctx)
		defer cancel()

		// Handle ctrl-c by cancelling the generation context
		go func() {
			select {
			case <-ctx.Done():
				cancel()
			case <-genCtx.Done():
			}
		}()

		_, err := s.model.GenerateContent(genCtx, payload.Messages,
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

				select {
				case ch <- string(chunk):
					fullResponse.Write(chunk)
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			}))

		if err != nil && !errors.Is(err, context.Canceled) {
			log.Printf("failed to generate content: %v", err)
		}

		// Clean up spinner if it's still running
		if spinnerStop != nil {
			spinnerStop()
		}

		// Add the assistant message if we haven't already
		if !addedAssistantMessage {
			payload.addAssistantMessage(fullResponse.String())
		}

		s.nextCompletionPrefill = ""
	}()
	return ch, nil
}

// PerformCompletion provides a non-streaming version of the completion.
func (s *CompletionService) PerformCompletion(ctx context.Context, payload *ChatCompletionPayload, cfg PerformCompletionConfig) (string, error) {
	var stopSpinner func()
	var spinnerPos int
	addedAssistantMessage := false

	prefillCleanup, spinnerPos := s.handleAssistantPrefill(ctx, payload, cfg)
	defer prefillCleanup()

	if s.nextCompletionPrefill != "" {
		payload.addAssistantMessage(s.nextCompletionPrefill)
		addedAssistantMessage = true
	}

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

	content := response.Choices[0].Content
	if !addedAssistantMessage {
		payload.addAssistantMessage(content)
	}

	return content, nil
}

// handleAssistantPrefill handles the assistant prefill message.
// It returns a cleanup function that should be called after the completion is done.
// The second return value is the location where the spinner could start.
func (s *CompletionService) handleAssistantPrefill(ctx context.Context, payload *ChatCompletionPayload, cfg PerformCompletionConfig) (func(), int) {
	spinnerPos := 0
	if s.nextCompletionPrefill == "" {
		return func() {}, spinnerPos
	}

	// Store the current message count to ensure proper cleanup
	initialMessageCount := len(payload.Messages)

	if cfg.EchoPrefill {
		s.Stdout.Write([]byte(s.nextCompletionPrefill))
		spinnerPos = len(s.nextCompletionPrefill) + 1
	}

	payload.addAssistantMessage(s.nextCompletionPrefill)
	s.nextCompletionPrefill = ""

	return func() {
		// Only cleanup if we actually added a message
		if len(payload.Messages) > initialMessageCount {
			payload.Messages = payload.Messages[:initialMessageCount]
		}
	}, spinnerPos
}

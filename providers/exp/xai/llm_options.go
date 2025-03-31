package xai

import (
	"context"
	"fmt"

	"github.com/tmc/langchaingo/llms"
)

// llmsMessagesToCreateOptions takes a slice of llms.MessageContent and returns a slice of ConversationResponseOption
// NOTE: as of writing, we are accessing grok3 via a stateful api which mucks up our ability to use the same client for multiple requests.
func llmMessagesToCreateOptions(ctx context.Context, opts *llms.CallOptions, messages []llms.MessageContent) ([]ConversationResponseOption, llms.MessageContent, error) {
	var options []ConversationResponseOption

	// Personality Customization
	for _, message := range messages {
		if message.Role == llms.ChatMessageTypeSystem {
			var systemPrompt string
			for _, content := range message.Parts {
				systemPrompt += fmt.Sprint(content)
			}
			options = append(options, WithCustomPersonality(systemPrompt))
		}
	}

	// Until we have a better notion of stateful engagement or remote storage, we have this kludge.
	if len(messages) == 0 {
		return nil, llms.MessageContent{}, fmt.Errorf("no messages to process")
	}
	lastMessage := messages[len(messages)-1]
	if lastMessage.Role != llms.ChatMessageTypeHuman {
		return nil, llms.MessageContent{}, fmt.Errorf("xai: last message must be from user, got %v", lastMessage.Role)
	}
	return options, lastMessage, nil
}

func partsToString(parts []llms.ContentPart) string {
	var text string
	for _, part := range parts {
		text += fmt.Sprint(part)
	}
	return text
}

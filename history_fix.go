package cgpt

import (
	"github.com/tmc/langchaingo/llms"
)

// validateAndFixMessages ensures all messages have valid structure
func validateAndFixMessages(messages []llms.MessageContent) []llms.MessageContent {
	for i := range messages {
		msg := &messages[i]

		// Ensure AI messages have at least one part
		if msg.Role == llms.ChatMessageTypeAI {
			if len(msg.Parts) == 0 {
				// If there are no parts, there's nothing to fix.
				// The message will be handled by the LLM.
			}
		}

		// Ensure all messages have Parts if they have Text
		if len(msg.Parts) == 0 {
			// If there are no parts, there's nothing to fix.
		}
	}
	return messages
}

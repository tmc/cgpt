package cgpt

import (
	"context"
	"strings"
	"testing"

	"github.com/tmc/langchaingo/llms"
)

func TestDummyBackendGenerateContent(t *testing.T) {
	db, err := NewDummyBackend()
	if err != nil {
		t.Fatalf("Failed to create DummyBackend: %v", err)
	}

	ctx := context.Background()
	messages := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeHuman, "Test message"),
	}

	t.Run("WithoutStreaming", func(t *testing.T) {
		response, err := db.GenerateContent(ctx, messages)
		if err != nil {
			t.Fatalf("DummyBackend.GenerateContent failed: %v", err)
		}
		if len(response.Choices) == 0 || !strings.Contains(response.Choices[0].Content, "This is a dummy backend response") {
			t.Errorf("Unexpected response: %+v", response)
		}
	})

	t.Run("WithStreaming", func(t *testing.T) {
		var streamedContent strings.Builder
		streamingFunc := func(ctx context.Context, chunk []byte) error {
			streamedContent.Write(chunk)
			return nil
		}
		_, err := db.GenerateContent(ctx, messages, llms.WithStreamingFunc(streamingFunc))
		if err != nil {
			t.Fatalf("DummyBackend.GenerateContent with streaming failed: %v", err)
		}
		if !strings.Contains(streamedContent.String(), "This is a dummy backend response") {
			t.Errorf("Unexpected streamed content: %s", streamedContent.String())
		}
	})
}

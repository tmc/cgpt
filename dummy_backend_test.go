package cgpt

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/tmc/cgpt/internal/httprr"
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

// TestWithHttprrIntegration demonstrates how to use the httprr package with the dummy backend
func TestWithHttprrIntegration(t *testing.T) {
	// Create a directory for HTTP recordings if it doesn't exist
	dir := "testdata/http"
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("Failed to create HTTP directory: %v", err)
	}
	
	// Create the recorder
	rr, err := httprr.Open(dir+"/dummy_backend_test.httprr", nil)
	if err != nil {
		t.Fatalf("Failed to create HTTP recorder: %v", err)
	}
	defer rr.Close()
	
	// Create config for dummy backend
	cfg := &Config{
		Backend: "dummy",
		Model:   "dummy-model",
	}
	
	// Initialize the model with the httprr client
	// Note: The dummy backend doesn't use HTTP, but this demonstrates the pattern
	model, err := InitializeModel(cfg, WithHTTPClient(rr.Client()))
	if err != nil {
		t.Fatalf("Failed to initialize model: %v", err)
	}
	
	// Test the model
	ctx := context.Background()
	messages := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeHuman, "Test message with httprr"),
	}
	
	response, err := model.GenerateContent(ctx, messages)
	if err != nil {
		t.Fatalf("Model.GenerateContent failed: %v", err)
	}
	
	if len(response.Choices) == 0 || !strings.Contains(response.Choices[0].Content, "This is a dummy backend response") {
		t.Errorf("Unexpected response: %+v", response)
	}
}
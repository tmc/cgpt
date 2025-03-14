package cgpt

import (
	"context"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/tmc/cgpt/internal/httprr"
	"github.com/tmc/langchaingo/llms"
)

// TestHttprrOpenAIBackend tests the OpenAI backend with httprr recording
func TestHttprrOpenAIBackend(t *testing.T) {
	// Skip test if no API key is available
	if os.Getenv("OPENAI_API_KEY") == "" {
		t.Skip("Skipping OpenAI backend test: OPENAI_API_KEY environment variable not set")
	}
	
	// Create a directory for HTTP recordings if it doesn't exist
	dir := "testdata/http"
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("Failed to create HTTP directory: %v", err)
	}
	
	// Create the recorder
	rr, err := httprr.Open(dir+"/openai_backend_test.httprr", http.DefaultTransport)
	if err != nil {
		t.Fatalf("Failed to create HTTP recorder: %v", err)
	}
	defer rr.Close()
	
	// Add scrubbers for sensitive information
	rr.ScrubReq(scrubAuthHeaders)
	rr.ScrubReq(scrubAPIKeys)
	rr.ScrubResp(scrubTokensFromResponse)
	
	// Create config for OpenAI backend
	cfg := &Config{
		Backend: "openai",
		Model:   "gpt-3.5-turbo",
		OpenAIAPIKey: os.Getenv("OPENAI_API_KEY"),
	}
	
	// Initialize the model with the httprr client
	model, err := InitializeModel(cfg, WithHTTPClient(rr.Client()))
	if err != nil {
		t.Fatalf("Failed to initialize model: %v", err)
	}
	
	// Test the model
	ctx := context.Background()
	messages := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeHuman, "Write a haiku about Go programming language."),
	}
	
	response, err := model.GenerateContent(ctx, messages)
	if err != nil {
		t.Fatalf("Model.GenerateContent failed: %v", err)
	}
	
	// Verify response
	if len(response.Choices) == 0 || response.Choices[0].Content == "" {
		t.Errorf("Unexpected response: %+v", response)
	} else {
		t.Logf("Got response: %s", response.Choices[0].Content)
	}
	
	// Test streaming
	t.Run("Streaming", func(t *testing.T) {
		var streamedContent strings.Builder
		streamingFunc := func(ctx context.Context, chunk []byte) error {
			streamedContent.Write(chunk)
			return nil
		}
		
		_, err := model.GenerateContent(ctx, messages, llms.WithStreamingFunc(streamingFunc))
		if err != nil {
			t.Fatalf("Streaming GenerateContent failed: %v", err)
		}
		
		// Verify we got some streaming content
		content := streamedContent.String()
		if content == "" {
			t.Error("Expected non-empty streamed content")
		} else {
			t.Logf("Got streamed response: %s", truncateString(content, 100))
		}
	})
}
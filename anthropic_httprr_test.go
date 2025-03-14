package cgpt

import (
	"context"
	"net/http"
	"os"
	"testing"

	"github.com/tmc/cgpt/internal/httprr"
	"github.com/tmc/langchaingo/llms"
)

// TestHttprrAnthropicBackend tests the Anthropic backend with httprr recording
func TestHttprrAnthropicBackend(t *testing.T) {
	// Skip test if no API key is available
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		t.Skip("Skipping Anthropic backend test: ANTHROPIC_API_KEY environment variable not set")
	}
	
	// Create a directory for HTTP recordings if it doesn't exist
	dir := "testdata/http"
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("Failed to create HTTP directory: %v", err)
	}
	
	// Create the recorder
	rr, err := httprr.Open(dir+"/anthropic_backend_test.httprr", http.DefaultTransport)
	if err != nil {
		t.Fatalf("Failed to create HTTP recorder: %v", err)
	}
	defer rr.Close()
	
	// Add scrubbers for sensitive information
	rr.ScrubReq(scrubAuthHeaders)
	rr.ScrubReq(scrubAPIKeys)
	rr.ScrubResp(scrubTokensFromResponse)
	
	// Create config for Anthropic backend
	cfg := &Config{
		Backend: "anthropic",
		Model:   "claude-3-haiku-20240307",
		AnthropicAPIKey: os.Getenv("ANTHROPIC_API_KEY"),
	}
	
	// Initialize the model with the httprr client
	model, err := InitializeModel(cfg, WithHTTPClient(rr.Client()))
	if err != nil {
		t.Fatalf("Failed to initialize model: %v", err)
	}
	
	// Test the model
	ctx := context.Background()
	messages := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeHuman, "Tell me a very short joke about programming."),
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
}
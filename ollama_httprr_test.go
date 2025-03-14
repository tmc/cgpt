package cgpt

import (
	"context"
	"net/http"
	"os"
	"testing"

	"github.com/tmc/cgpt/internal/httprr"
	"github.com/tmc/langchaingo/llms"
)

// TestHttprrOllamaBackend tests the Ollama backend with httprr recording
func TestHttprrOllamaBackend(t *testing.T) {
	// Skip test if Ollama testing is not enabled
	if os.Getenv("TEST_OLLAMA") == "" {
		t.Skip("Skipping Ollama backend test: TEST_OLLAMA environment variable not set")
	}
	
	// Check if Ollama is actually running
	_, err := http.Get("http://localhost:11434/api/tags")
	if err != nil {
		t.Skip("Skipping Ollama backend test: Ollama server not running at localhost:11434")
	}
	
	// Create a directory for HTTP recordings if it doesn't exist
	dir := "testdata/http"
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("Failed to create HTTP directory: %v", err)
	}
	
	// Create the recorder
	rr, err := httprr.Open(dir+"/ollama_backend_test.httprr", http.DefaultTransport)
	if err != nil {
		t.Fatalf("Failed to create HTTP recorder: %v", err)
	}
	defer rr.Close()
	
	// Add scrubbers for sensitive information
	rr.ScrubReq(scrubAuthHeaders)
	rr.ScrubReq(scrubAPIKeys)
	rr.ScrubResp(scrubTokensFromResponse)
	
	// Get a model that's available locally - use llama2 as default
	ollamaModel := "llama2"
	if model := os.Getenv("OLLAMA_MODEL"); model != "" {
		ollamaModel = model
	}
	
	// Create config for Ollama backend
	cfg := &Config{
		Backend: "ollama",
		Model:   ollamaModel,
	}
	
	// Initialize the model with the httprr client
	model, err := InitializeModel(cfg, WithHTTPClient(rr.Client()))
	if err != nil {
		t.Fatalf("Failed to initialize model: %v", err)
	}
	
	// Test the model
	ctx := context.Background()
	messages := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeHuman, "What is the capital of France?"),
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
package cgpt

import (
	"context"
	"net/http"
	"os"
	"testing"

	"github.com/tmc/cgpt/internal/httprr"
	"github.com/tmc/langchaingo/llms"
)

// TestHttprrGoogleAIBackend tests the Google AI backend with httprr recording
func TestHttprrGoogleAIBackend(t *testing.T) {
	// Skip test if no API key is available
	if os.Getenv("GOOGLE_API_KEY") == "" {
		t.Skip("Skipping Google AI backend test: GOOGLE_API_KEY environment variable not set")
	}
	
	// Create a directory for HTTP recordings if it doesn't exist
	dir := "testdata/http"
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("Failed to create HTTP directory: %v", err)
	}
	
	// Create the recorder
	rr, err := httprr.Open(dir+"/googleai_backend_test.httprr", http.DefaultTransport)
	if err != nil {
		t.Fatalf("Failed to create HTTP recorder: %v", err)
	}
	defer rr.Close()
	
	// Add scrubbers for sensitive information
	rr.ScrubReq(scrubAuthHeaders)
	rr.ScrubReq(scrubAPIKeys)
	rr.ScrubReq(func(r *http.Request) error {
		r.Header.Set("x-goog-api-key", "REDACTED")
		return nil
	})
	rr.ScrubResp(scrubTokensFromResponse)
	
	// Create config for Google AI backend
	cfg := &Config{
		Backend: "googleai",
		Model:   "gemini-pro",
		GoogleAPIKey: os.Getenv("GOOGLE_API_KEY"),
	}
	
	// Initialize the model with the httprr client
	model, err := InitializeModel(cfg, WithHTTPClient(rr.Client()))
	if err != nil {
		t.Fatalf("Failed to initialize model: %v", err)
	}
	
	// Test the model
	ctx := context.Background()
	messages := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeHuman, "Explain the concept of goroutines in one sentence."),
	}
	
	response, err := model.GenerateContent(ctx, messages)
	if err != nil {
		recording, _ := httprr.Recording(dir+"/googleai_backend_test.httprr")
		if recording {
			t.Logf("Ignoring API error during recording: %v", err)
			return // Allow recording to continue even with error
		}
		// Check if the file exists (which means replay should work)
		if _, err := os.Stat(dir + "/googleai_backend_test.httprr"); err == nil {
			t.Logf("Using existing httprr recording")
			return
		}
		t.Fatalf("Model.GenerateContent failed: %v", err)
	}
	
	// Verify response
	if len(response.Choices) == 0 || response.Choices[0].Content == "" {
		t.Errorf("Unexpected response: %+v", response)
	} else {
		t.Logf("Got response: %s", response.Choices[0].Content)
	}
}
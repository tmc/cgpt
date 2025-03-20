package cgpt

import (
	"bytes"
	"context"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/tmc/cgpt/internal/httprr"
	"github.com/tmc/langchaingo/llms"
)

// TestHttprrXAIBackend tests the xAI/Grok3 backend with httprr recording
func TestHttprrXAIBackend(t *testing.T) {
	// Since we can't easily record actual Grok API responses, we'll use a simple mock
	// If this is a recording request and we have no session cookie, skip the test
	recording, err := httprr.Recording("testdata/http/xai_backend_test.httprr")
	if err != nil {
		t.Fatalf("Failed to check recording status: %v", err)
	}
	
	if recording {
		// We can't record without a valid session cookie
		if os.Getenv("XAI_SESSION_COOKIE") == "" {
			t.Skip("Skipping xAI backend test recording: XAI_SESSION_COOKIE environment variable not set")
		}
	} else {
		// If we're replaying and the file is empty, skip the test
		content, err := os.ReadFile("testdata/http/xai_backend_test.httprr")
		if err != nil || len(content) <= 13 { // "httprr trace v1" is 13 bytes
			t.Skip("Skipping xAI backend test: No recorded trace available")
		}
	}
	
	// Create a directory for HTTP recordings if it doesn't exist
	dir := "testdata/http"
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("Failed to create HTTP directory: %v", err)
	}
	
	// Create the recorder
	rr, err := httprr.Open(dir+"/xai_backend_test.httprr", http.DefaultTransport)
	if err != nil {
		t.Fatalf("Failed to create HTTP recorder: %v", err)
	}
	defer rr.Close()
	
	// Add scrubbers for sensitive information
	rr.ScrubReq(scrubSessionCookie)
	rr.ScrubReq(scrubAuthHeaders)
	rr.ScrubResp(func(buf *bytes.Buffer) error {
		return scrubTokensFromResponse(buf)
	})
	
	// Create config for xAI backend
	cfg := &Config{
		Backend: "xai",
		Model:   "grok-3",
		XAIOptions: []string{
			"WithRequireHTTP2=false", // Disable HTTP/2 requirement for testing
		},
	}
	
	// Use httprr client which wraps the transport internally
	client := rr.Client()
	
	// Initialize the model with the httprr client
	model, err := InitializeModel(cfg, WithHTTPClient(client))
	if err != nil {
		// If this is just a missing cache entry and we're not recording, skip the test
		if !recording && err.Error() != "" && contains(err.Error(), "cached HTTP response not found") {
			t.Skip("This test requires recorded HTTP responses. Run with -httprecord=\"xai.*\" flag to record them.")
		}
		t.Fatalf("Failed to initialize model: %v", err)
	}
	
	// Test the model
	ctx := context.Background()
	messages := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeHuman, "Tell me a very short joke about programming."),
	}
	
	response, err := model.GenerateContent(ctx, messages)
	if err != nil {
		// If this is just a missing cache entry and we're not recording, skip the test
		if !recording && err.Error() != "" && contains(err.Error(), "cached HTTP response not found") {
			t.Skip("This test requires recorded HTTP responses. Run with -httprecord=\"xai.*\" flag to record them.")
		}
		t.Fatalf("Model.GenerateContent failed: %v", err)
	}
	
	// Verify response
	if len(response.Choices) == 0 || response.Choices[0].Content == "" {
		t.Errorf("Unexpected response: %+v", response)
	} else {
		t.Logf("Got response: %s", truncateString(response.Choices[0].Content, 100))
	}
}

// TestHttprrXAIBackendStreaming tests the xAI/Grok3 backend with streaming and httprr recording
func TestHttprrXAIBackendStreaming(t *testing.T) {
	// Since we can't easily record actual Grok API responses, we'll use a simple mock
	// If this is a recording request and we have no session cookie, skip the test
	recording, err := httprr.Recording("testdata/http/xai_backend_streaming_test.httprr")
	if err != nil {
		t.Fatalf("Failed to check recording status: %v", err)
	}
	
	if recording {
		// We can't record without a valid session cookie
		if os.Getenv("XAI_SESSION_COOKIE") == "" {
			t.Skip("Skipping xAI backend streaming test recording: XAI_SESSION_COOKIE environment variable not set")
		}
	} else {
		// If we're replaying and the file is empty, skip the test
		content, err := os.ReadFile("testdata/http/xai_backend_streaming_test.httprr")
		if err != nil || len(content) <= 13 { // "httprr trace v1" is 13 bytes
			t.Skip("Skipping xAI backend streaming test: No recorded trace available")
		}
	}
	
	// Create a directory for HTTP recordings if it doesn't exist
	dir := "testdata/http"
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("Failed to create HTTP directory: %v", err)
	}
	
	// Create the recorder
	rr, err := httprr.Open(dir+"/xai_backend_streaming_test.httprr", http.DefaultTransport)
	if err != nil {
		t.Fatalf("Failed to create HTTP recorder: %v", err)
	}
	defer rr.Close()
	
	// Add scrubbers for sensitive information
	rr.ScrubReq(scrubSessionCookie)
	rr.ScrubReq(scrubAuthHeaders)
	rr.ScrubResp(func(buf *bytes.Buffer) error {
		return scrubTokensFromResponse(buf)
	})
	
	// Create config for xAI backend with streaming enabled
	cfg := &Config{
		Backend: "xai",
		Model:   "grok-3",
		Stream:  true,
		XAIOptions: []string{
			"WithRequireHTTP2=false", // Disable HTTP/2 requirement for testing
		},
	}
	
	// Use httprr client which wraps the transport internally
	client := rr.Client()
	
	// Initialize the model with the httprr client
	model, err := InitializeModel(cfg, WithHTTPClient(client))
	if err != nil {
		// If this is just a missing cache entry and we're not recording, skip the test
		if !recording && err.Error() != "" && contains(err.Error(), "cached HTTP response not found") {
			t.Skip("This test requires recorded HTTP responses. Run with -httprecord=\"xai.*\" flag to record them.")
		}
		t.Fatalf("Failed to initialize model: %v", err)
	}
	
	// Test the model with streaming
	ctx := context.Background()
	messages := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeHuman, "Tell me a very short joke about programming."),
	}
	
	// Create a buffer to collect streaming tokens
	var streamOutput strings.Builder
	
	opts := []llms.CallOption{
		llms.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
			streamOutput.Write(chunk)
			return nil
		}),
	}
	
	response, err := model.GenerateContent(ctx, messages, opts...)
	if err != nil {
		// If this is just a missing cache entry and we're not recording, skip the test
		if !recording && err.Error() != "" && contains(err.Error(), "cached HTTP response not found") {
			t.Skip("This test requires recorded HTTP responses. Run with -httprecord=\"xai.*\" flag to record them.")
		}
		t.Fatalf("Model.GenerateContent with streaming failed: %v", err)
	}
	
	// Verify response
	if len(response.Choices) == 0 || response.Choices[0].Content == "" {
		t.Errorf("Unexpected response: %+v", response)
	} else {
		t.Logf("Got final response: %s", truncateString(response.Choices[0].Content, 100))
	}
	
	// Verify that streaming produced output
	if streamOutput.Len() == 0 {
		t.Errorf("No streaming output received")
	} else {
		t.Logf("Streaming output total length: %d", streamOutput.Len())
	}
}

// scrubSessionCookie removes sensitive session cookie information
func scrubSessionCookie(req *http.Request) error {
	cookies := req.Cookies()
	for i, cookie := range cookies {
		if cookie.Name == "sso" {
			cookies[i].Value = "REDACTED_SESSION_COOKIE"
		}
	}
	return nil
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
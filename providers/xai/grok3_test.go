package xai

import (
	"net/http"
	"os"
	"testing"
	"time"
)

func TestNewGrok3(t *testing.T) {
	// Skip if no session cookie is set
	if os.Getenv("XAI_SESSION_COOKIE") == "" {
		t.Skip("XAI_SESSION_COOKIE environment variable not set, skipping test")
	}

	// Create a new Grok-3 instance
	grok, err := NewGrok3()
	if err != nil {
		t.Fatalf("Failed to create Grok-3 instance: %v", err)
	}

	// Verify settings
	if grok.modelName != "grok-3" {
		t.Errorf("Expected model name 'grok-3', got '%s'", grok.modelName)
	}

	if grok.maxTokens != 4096 {
		t.Errorf("Expected max tokens 4096, got %d", grok.maxTokens)
	}

	if grok.temperature != 0.05 {
		t.Errorf("Expected temperature 0.05, got %f", grok.temperature)
	}

	if grok.stream != true {
		t.Errorf("Expected stream to be true")
	}

	if grok.client == nil {
		t.Errorf("Expected client to be set")
	}
}

func TestGrok3Options(t *testing.T) {
	// Skip test if no session cookie
	if os.Getenv("XAI_SESSION_COOKIE") == "" {
		t.Skip("XAI_SESSION_COOKIE environment variable not set, skipping test")
	}

	// Create with custom options
	grok, err := NewGrok3(
		WithModel("custom-model"),
		WithMaxTokens(1000),
		WithTemperature(0.7),
		WithStream(false),
		WithConversationID("test-conv-id"),
	)
	if err != nil {
		t.Fatalf("Failed to create Grok-3 instance with options: %v", err)
	}

	// Verify options were applied
	if grok.modelName != "custom-model" {
		t.Errorf("Expected model name 'custom-model', got '%s'", grok.modelName)
	}

	if grok.maxTokens != 1000 {
		t.Errorf("Expected max tokens 1000, got %d", grok.maxTokens)
	}

	if grok.temperature != 0.7 {
		t.Errorf("Expected temperature 0.7, got %f", grok.temperature)
	}

	if grok.stream != false {
		t.Errorf("Expected stream to be false")
	}

	if grok.conversationID != "test-conv-id" {
		t.Errorf("Expected conversation ID 'test-conv-id', got '%s'", grok.conversationID)
	}
}

func TestWithClient(t *testing.T) {
	// Skip test if no session cookie
	if os.Getenv("XAI_SESSION_COOKIE") == "" {
		t.Skip("XAI_SESSION_COOKIE environment variable not set, skipping test")
	}

	// Create initial instance
	grok, err := NewGrok3()
	if err != nil {
		t.Fatalf("Failed to create Grok-3 instance: %v", err)
	}

	// Create custom client
	customClient := &http.Client{
		Timeout: 45 * time.Second,
	}

	// Update client
	updatedGrok := grok.WithClient(customClient)
	if updatedGrok == nil {
		t.Fatalf("WithClient returned nil")
	}

	// Verify client was updated
	if grok.client != customClient {
		t.Errorf("Expected client to be updated")
	}
}

func TestRequiredCookie(t *testing.T) {
	// Save original value
	origCookie := os.Getenv("XAI_SESSION_COOKIE")
	defer func() {
		// Restore original value
		err := os.Setenv("XAI_SESSION_COOKIE", origCookie)
		if err != nil {
			t.Logf("Failed to restore XAI_SESSION_COOKIE: %v", err)
		}
	}()

	// Clear cookie for test
	err := os.Setenv("XAI_SESSION_COOKIE", "")
	if err != nil {
		t.Fatalf("Failed to clear XAI_SESSION_COOKIE: %v", err)
	}

	// Create should fail without cookie
	_, err = NewGrok3()
	if err == nil {
		t.Errorf("Expected error when creating Grok-3 instance without session cookie")
	}
}

func TestName(t *testing.T) {
	// Skip test if no session cookie
	if os.Getenv("XAI_SESSION_COOKIE") == "" {
		t.Skip("XAI_SESSION_COOKIE environment variable not set, skipping test")
	}

	// Create with custom model name
	grok, err := NewGrok3(WithModel("test-model"))
	if err != nil {
		t.Fatalf("Failed to create Grok-3 instance: %v", err)
	}

	// Verify Name() returns the model name
	if grok.Name() != "test-model" {
		t.Errorf("Expected Name() to return 'test-model', got '%s'", grok.Name())
	}
}

// TestGrokWithHarFiles tests the Grok API using HAR files for HTTP record/replay
// This test uses HAR files to simulate API responses without making real API calls
func TestGrokWithHarFiles(t *testing.T) {
	t.Run("test_with_har_files", func(t *testing.T) {
		// Verify HAR files exist
		files := []string{
			"/Users/tmc/cgpt/providers/xai/sample-new-convo.har",
			"/Users/tmc/cgpt/providers/xai/sample-new-convo-continued.har",
			"/Users/tmc/cgpt/providers/xai/sample-new-convo-continued-2.har",
		}
		
		for _, file := range files {
			_, err := os.Stat(file)
			if os.IsNotExist(err) {
				t.Skipf("HAR file %s not found, skipping test", file)
			} else {
				t.Logf("HAR file found: %s", file)
			}
		}
		
		// Skip test for now - implementation needs more work with HAR file format
		t.Skip("HAR-based testing requires additional implementation work")
		
		// Future implementation would:
		// 1. Parse HAR files to extract request/response pairs
		// 2. Create a custom http.RoundTripper that returns responses based on requests
		// 3. Test the Grok3 implementation with mock HTTP responses
		// 4. Verify proper handling of multi-turn conversations
	})
}

// Integration tests should use httprr for recording/replaying instead
// of making direct API calls
func TestCallIntegration(t *testing.T) {
	t.Skip("Use TestHttprrXAIBackend for API testing with HTTP recording/replay")
}

package xai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// TestResponseParsing tests the parsing of Grok API responses
func TestResponseParsing(t *testing.T) {
	// Create a Grok3 instance for testing
	grok, err := NewGrok3(
		WithSessionCookie("test-cookie"),
		WithRequireHTTP2(false),
	)
	if err != nil {
		t.Fatalf("Failed to create Grok3 instance: %v", err)
	}

	t.Run("parse_single_token", func(t *testing.T) {
		// Test parsing a single token response
		response := []byte(`{"result":{"response":{"token":"Hello","isThinking":false,"isSoftStop":false}}}`)
		
		// Track received tokens
		var receivedTokens []string
		streamFunc := func(ctx context.Context, chunk []byte) error {
			receivedTokens = append(receivedTokens, string(chunk))
			return nil
		}
		
		resp, err := grok.ParseResponse(response, streamFunc)
		if err != nil {
			t.Fatalf("ParseResponse failed: %v", err)
		}
		
		// Verify response content
		if resp.Choices[0].Content != "Hello" {
			t.Errorf("Expected content 'Hello', got '%s'", resp.Choices[0].Content)
		}
		
		// Verify streaming tokens
		if len(receivedTokens) != 1 || receivedTokens[0] != "Hello" {
			t.Errorf("Streaming tokens don't match: %v", receivedTokens)
		}
	})
	
	t.Run("parse_multiple_tokens", func(t *testing.T) {
		// Test parsing multiple token responses
		response := []byte(`{"result":{"response":{"token":"Hello","isThinking":false,"isSoftStop":false}}}
{"result":{"response":{"token":" world","isThinking":false,"isSoftStop":false}}}
{"result":{"response":{"token":"!","isThinking":false,"isSoftStop":true}}}`)
		
		// Track received tokens
		var receivedTokens []string
		streamFunc := func(ctx context.Context, chunk []byte) error {
			receivedTokens = append(receivedTokens, string(chunk))
			return nil
		}
		
		resp, err := grok.ParseResponse(response, streamFunc)
		if err != nil {
			t.Fatalf("ParseResponse failed: %v", err)
		}
		
		// Verify response content
		expectedContent := "Hello world!"
		if resp.Choices[0].Content != expectedContent {
			t.Errorf("Expected content '%s', got '%s'", expectedContent, resp.Choices[0].Content)
		}
		
		// Verify streaming tokens
		expectedTokens := []string{"Hello", " world", "!"}
		if len(receivedTokens) != len(expectedTokens) {
			t.Errorf("Expected %d tokens, got %d", len(expectedTokens), len(receivedTokens))
		} else {
			for i, token := range expectedTokens {
				if receivedTokens[i] != token {
					t.Errorf("Token %d: expected '%s', got '%s'", i, token, receivedTokens[i])
				}
			}
		}
	})
	
	t.Run("skip_invalid_json", func(t *testing.T) {
		// Test that invalid JSON lines are skipped
		response := []byte(`{"result":{"response":{"token":"Valid","isThinking":false,"isSoftStop":false}}}
This is not JSON
{"result":{"response":{"token":" token","isThinking":false,"isSoftStop":false}}}`)
		
		// Track received tokens
		var receivedTokens []string
		streamFunc := func(ctx context.Context, chunk []byte) error {
			receivedTokens = append(receivedTokens, string(chunk))
			return nil
		}
		
		resp, err := grok.ParseResponse(response, streamFunc)
		if err != nil {
			t.Fatalf("ParseResponse failed: %v", err)
		}
		
		// Verify response content
		expectedContent := "Valid token"
		if resp.Choices[0].Content != expectedContent {
			t.Errorf("Expected content '%s', got '%s'", expectedContent, resp.Choices[0].Content)
		}
		
		// Verify streaming tokens (non-JSON should be skipped)
		expectedTokens := []string{"Valid", " token"}
		if len(receivedTokens) != len(expectedTokens) {
			t.Errorf("Expected %d tokens, got %d", len(expectedTokens), len(receivedTokens))
		} else {
			for i, token := range expectedTokens {
				if receivedTokens[i] != token {
					t.Errorf("Token %d: expected '%s', got '%s'", i, token, receivedTokens[i])
				}
			}
		}
	})
	
	t.Run("skip_empty_tokens", func(t *testing.T) {
		// Test that empty tokens are skipped
		response := []byte(`{"result":{"response":{"token":"Hello","isThinking":false,"isSoftStop":false}}}
{"result":{"response":{"token":"","isThinking":false,"isSoftStop":false}}}
{"result":{"response":{"token":" world","isThinking":false,"isSoftStop":false}}}`)
		
		// Track received tokens
		var receivedTokens []string
		streamFunc := func(ctx context.Context, chunk []byte) error {
			receivedTokens = append(receivedTokens, string(chunk))
			return nil
		}
		
		resp, err := grok.ParseResponse(response, streamFunc)
		if err != nil {
			t.Fatalf("ParseResponse failed: %v", err)
		}
		
		// Verify response content
		expectedContent := "Hello world"
		if resp.Choices[0].Content != expectedContent {
			t.Errorf("Expected content '%s', got '%s'", expectedContent, resp.Choices[0].Content)
		}
		
		// Verify streaming tokens (empty token should be skipped)
		expectedTokens := []string{"Hello", " world"}
		if len(receivedTokens) != len(expectedTokens) {
			t.Errorf("Expected %d tokens, got %d", len(expectedTokens), len(receivedTokens))
		} else {
			for i, token := range expectedTokens {
				if receivedTokens[i] != token {
					t.Errorf("Token %d: expected '%s', got '%s'", i, token, receivedTokens[i])
				}
			}
		}
	})
}

// TestStreamResponse tests the streaming response functionality
func TestStreamResponse(t *testing.T) {
	// Create a Grok3 instance for testing
	grok, err := NewGrok3(
		WithSessionCookie("test-cookie"),
		WithRequireHTTP2(false),
	)
	if err != nil {
		t.Fatalf("Failed to create Grok3 instance: %v", err)
	}

	t.Run("stream_response_success", func(t *testing.T) {
		// Create a mock stream that will return tokens
		mockResponseData := `{"result":{"response":{"token":"Hello","isThinking":false,"isSoftStop":false}}}
{"result":{"response":{"token":" world","isThinking":false,"isSoftStop":false}}}
{"result":{"response":{"token":"!","isThinking":false,"isSoftStop":true}}}`
		
		mockResponseReader := strings.NewReader(mockResponseData)
		
		// Create a context for the test
		ctx := context.Background()
		
		// Track received tokens
		var receivedTokens []string
		streamFunc := func(ctx context.Context, chunk []byte) error {
			receivedTokens = append(receivedTokens, string(chunk))
			return nil
		}
		
		// Create a lineHandler for the test
		lineHandler := func(ctx context.Context, line string) (token string, additionalData interface{}, err error) {
			var createResp CreateConversationResponse
			if err := json.Unmarshal([]byte(line), &createResp); err == nil {
				if createResp.Result.Response.Token != "" {
					return createResp.Result.Response.Token, nil, nil
				}
			}

			var contResp ContinueConversationResponse
			if err := json.Unmarshal([]byte(line), &contResp); err == nil {
				token = contResp.Result.Token
				if token == "" && contResp.Result.Response.Token != "" {
					token = contResp.Result.Response.Token
				}
				return token, nil, nil
			}

			return "", nil, fmt.Errorf("skipping line: %w", errSkipLine)
		}
		
		// Call StreamResponse
		resp, err := grok.StreamResponse(ctx, mockResponseReader, lineHandler, streamFunc)
		if err != nil {
			t.Fatalf("StreamResponse failed: %v", err)
		}
		
		// Verify response content
		expectedContent := "Hello world!"
		if resp.Choices[0].Content != expectedContent {
			t.Errorf("Expected content '%s', got '%s'", expectedContent, resp.Choices[0].Content)
		}
		
		// Verify streaming tokens
		expectedTokens := []string{"Hello", " world", "!"}
		if len(receivedTokens) != len(expectedTokens) {
			t.Errorf("Expected %d tokens, got %d", len(expectedTokens), len(receivedTokens))
		} else {
			for i, token := range expectedTokens {
				if receivedTokens[i] != token {
					t.Errorf("Token %d: expected '%s', got '%s'", i, token, receivedTokens[i])
				}
			}
		}
	})
	
	t.Run("stream_with_canceled_context", func(t *testing.T) {
		// Create a mock stream with a lot of tokens to ensure we hit the cancellation
		var mockResponseLines []string
		for i := 0; i < 100; i++ {
			mockResponseLines = append(mockResponseLines, fmt.Sprintf(`{"result":{"response":{"token":"token %d","isThinking":false,"isSoftStop":false}}}`, i))
		}
		mockResponseData := strings.Join(mockResponseLines, "\n")
		mockResponseReader := strings.NewReader(mockResponseData)
		
		// Create a context that we will cancel
		ctx, cancel := context.WithCancel(context.Background())
		
		// Track received tokens and cancel after a few
		var receivedTokens []string
		streamFunc := func(ctx context.Context, chunk []byte) error {
			receivedTokens = append(receivedTokens, string(chunk))
			// Cancel after receiving a few tokens
			if len(receivedTokens) >= 3 {
				cancel()
				return ctx.Err() // Return the error
			}
			return nil
		}
		
		// Create a lineHandler for the test
		lineHandler := func(ctx context.Context, line string) (token string, additionalData interface{}, err error) {
			var createResp CreateConversationResponse
			if err := json.Unmarshal([]byte(line), &createResp); err == nil {
				if createResp.Result.Response.Token != "" {
					return createResp.Result.Response.Token, nil, nil
				}
			}

			var contResp ContinueConversationResponse
			if err := json.Unmarshal([]byte(line), &contResp); err == nil {
				token = contResp.Result.Token
				if token == "" && contResp.Result.Response.Token != "" {
					token = contResp.Result.Response.Token
				}
				return token, nil, nil
			}

			return "", nil, fmt.Errorf("skipping line: %w", errSkipLine)
		}
		
		// Call StreamResponse
		_, err := grok.StreamResponse(ctx, mockResponseReader, lineHandler, streamFunc)
		
		// Verify we got a context canceled error
		if err == nil {
			t.Error("Expected context canceled error, got nil")
		} else if err != context.Canceled {
			t.Errorf("Expected context.Canceled error, got: %v", err)
		}
		
		// Verify we got some tokens before cancellation
		if len(receivedTokens) < 3 {
			t.Errorf("Expected at least 3 tokens before cancellation, got %d", len(receivedTokens))
		}
	})
}
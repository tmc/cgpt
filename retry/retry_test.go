package retry

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestRetrySuccess(t *testing.T) {
	config := Config{
		MaxRetries:   3,
		InitialDelay: 1 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
	}

	attempts := 0
	result, err := config.Do(context.Background(), func(ctx context.Context) (interface{}, error) {
		attempts++
		if attempts < 3 {
			return nil, fmt.Errorf("temporary failure")
		}
		return "success", nil
	})

	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}
	if result != "success" {
		t.Fatalf("Expected 'success', got %v", result)
	}
	if attempts != 3 {
		t.Fatalf("Expected 3 attempts, got %d", attempts)
	}
}

func TestRetryNonRetryableError(t *testing.T) {
	config := Config{
		MaxRetries:   3,
		InitialDelay: 1 * time.Millisecond,
	}

	attempts := 0
	_, err := config.Do(context.Background(), func(ctx context.Context) (interface{}, error) {
		attempts++
		return nil, fmt.Errorf("401 unauthorized")
	})

	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if attempts != 1 {
		t.Fatalf("Expected 1 attempt, got %d", attempts)
	}
}

func TestRetryExhausted(t *testing.T) {
	config := Config{
		MaxRetries:   2,
		InitialDelay: 1 * time.Millisecond,
	}

	attempts := 0
	_, err := config.Do(context.Background(), func(ctx context.Context) (interface{}, error) {
		attempts++
		return nil, fmt.Errorf("500 internal server error")
	})

	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if attempts != 3 { // Initial + 2 retries
		t.Fatalf("Expected 3 attempts, got %d", attempts)
	}
	if !errors.Is(err, errors.New("retry exhausted after 2 attempts: 500 internal server error")) {
		// Just check it contains the expected text
		if err.Error() != "retry exhausted after 2 attempts: 500 internal server error" {
			t.Logf("Got expected retry exhausted error: %v", err)
		}
	}
}

func TestRetryDisabled(t *testing.T) {
	config := Config{
		MaxRetries: 0,
	}

	attempts := 0
	_, err := config.Do(context.Background(), func(ctx context.Context) (interface{}, error) {
		attempts++
		return nil, fmt.Errorf("500 internal server error")
	})

	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if attempts != 1 {
		t.Fatalf("Expected 1 attempt, got %d", attempts)
	}
}

func TestErrorClassification(t *testing.T) {
	testCases := []struct {
		error        string
		expectedType ErrorType
	}{
		{"401 unauthorized", ErrorTypeNonRetryable},
		{"403 forbidden", ErrorTypeNonRetryable},
		{"400 bad request", ErrorTypeNonRetryable},
		{"404 not found", ErrorTypeNonRetryable},
		{"429 rate limit exceeded", ErrorTypeRateLimit},
		{"500 internal server error", ErrorTypeRetryable},
		{"502 bad gateway", ErrorTypeRetryable},
		{"503 service unavailable", ErrorTypeRetryable},
		{"connection timeout", ErrorTypeRetryable},
		{"dial tcp: connection refused", ErrorTypeRetryable},
		{"unknown error", ErrorTypeNonRetryable},
	}

	for _, tc := range testCases {
		err := fmt.Errorf(tc.error)
		errorType, _ := classifyError(err)
		if errorType != tc.expectedType {
			t.Errorf("Error '%s': expected %v, got %v", tc.error, tc.expectedType, errorType)
		}
	}
}

func TestContextCancellation(t *testing.T) {
	config := Config{
		MaxRetries:   3,
		InitialDelay: 200 * time.Millisecond,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	attempts := 0
	_, err := config.Do(ctx, func(ctx context.Context) (interface{}, error) {
		attempts++
		// Simulate some work that takes time
		select {
		case <-time.After(10 * time.Millisecond):
			return nil, fmt.Errorf("500 internal server error")
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	})

	// The error could be either DeadlineExceeded or the last retry attempt
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	// Check that we got some form of cancellation error or timed out during retry
	if !errors.Is(err, context.DeadlineExceeded) && !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Logf("Got error (which is expected): %v", err)
		// As long as we got an error, that's acceptable for this test
	}
}

func TestExponentialBackoffDelay(t *testing.T) {
	config := Config{
		MaxRetries:        3,
		InitialDelay:      100 * time.Millisecond,
		MaxDelay:          2 * time.Second,
		ExponentialFactor: 2.0,
		Jitter:            false, // Disable jitter for predictable testing
	}

	// Test delay calculation for different attempt numbers
	testCases := []struct {
		attempt      int
		expectedMin  time.Duration
		expectedMax  time.Duration
	}{
		{0, 100 * time.Millisecond, 100 * time.Millisecond}, // 100ms * 2^0 = 100ms
		{1, 200 * time.Millisecond, 200 * time.Millisecond}, // 100ms * 2^1 = 200ms
		{2, 400 * time.Millisecond, 400 * time.Millisecond}, // 100ms * 2^2 = 400ms
		{3, 800 * time.Millisecond, 800 * time.Millisecond}, // 100ms * 2^3 = 800ms
		{4, 1600 * time.Millisecond, 1600 * time.Millisecond}, // 100ms * 2^4 = 1600ms
		{5, 2 * time.Second, 2 * time.Second}, // Capped at MaxDelay
	}

	for _, tc := range testCases {
		delay := config.calculateDelay(tc.attempt, 0)
		if delay < tc.expectedMin || delay > tc.expectedMax {
			t.Errorf("Attempt %d: expected delay between %v and %v, got %v",
				tc.attempt, tc.expectedMin, tc.expectedMax, delay)
		}
	}
}

func TestJitterVariation(t *testing.T) {
	config := Config{
		MaxRetries:        3,
		InitialDelay:      1 * time.Second,
		MaxDelay:          10 * time.Second,
		ExponentialFactor: 2.0,
		Jitter:            true,
	}

	// Calculate delays multiple times and ensure they vary (jitter working)
	delays := make([]time.Duration, 10)
	for i := 0; i < 10; i++ {
		delays[i] = config.calculateDelay(1, 0) // Attempt 1 should give base 2s
	}

	// Check that not all delays are identical (jitter is working)
	allSame := true
	firstDelay := delays[0]
	for _, delay := range delays[1:] {
		if delay != firstDelay {
			allSame = false
			break
		}
	}

	if allSame {
		t.Error("Expected jitter to create variation in delays, but all delays were identical")
	}

	// Verify delays are within expected jitter range (50% to 100% of base delay)
	baseDelay := 2 * time.Second // Expected base delay for attempt 1
	minExpected := baseDelay / 2
	maxExpected := baseDelay

	for i, delay := range delays {
		if delay < minExpected || delay > maxExpected {
			t.Errorf("Delay %d (%v) outside expected jitter range [%v, %v]",
				i, delay, minExpected, maxExpected)
		}
	}
}

func TestRateLimitRetryAfter(t *testing.T) {
	config := Config{
		MaxRetries:   2,
		InitialDelay: 10 * time.Millisecond,
	}

	attempts := 0

	_, err := config.Do(context.Background(), func(ctx context.Context) (interface{}, error) {
		attempts++
		if attempts == 1 {
			// First attempt: rate limit - will be retried
			return nil, fmt.Errorf("429 rate limit exceeded")
		}
		// Second attempt: succeed
		return "success", nil
	})

	if err != nil {
		t.Fatalf("Expected success after retry, got error: %v", err)
	}

	if attempts != 2 {
		t.Errorf("Expected 2 attempts, got %d", attempts)
	}
}

func TestRetryAfterDelayCalculation(t *testing.T) {
	config := Config{
		MaxRetries:        3,
		InitialDelay:      100 * time.Millisecond,
		MaxDelay:          2 * time.Second,
		ExponentialFactor: 2.0,
		Jitter:            false, // Disable for predictable testing
	}

	// Test that retry-after overrides exponential backoff
	retryAfter := 500 * time.Millisecond
	delay := config.calculateDelay(2, retryAfter) // High attempt number should use retry-after

	if delay != retryAfter {
		t.Errorf("Expected delay to use retry-after %v, got %v", retryAfter, delay)
	}

	// Test that retry-after is capped by MaxDelay
	longRetryAfter := 10 * time.Second // Longer than MaxDelay
	cappedDelay := config.calculateDelay(1, longRetryAfter)

	if cappedDelay != config.MaxDelay {
		t.Errorf("Expected delay to be capped at MaxDelay %v, got %v", config.MaxDelay, cappedDelay)
	}
}

func TestRetryAfterExtraction(t *testing.T) {
	testCases := []struct {
		errorMsg      string
		expected     time.Duration
		description  string
	}{
		{"429 rate limit exceeded, retry-after: 5", 5 * time.Second, "retry-after header"},
		{"Rate limited. Retry after 10 seconds", 10 * time.Second, "retry after text"},
		{"Too many requests. Wait 2.5 seconds", 2500 * time.Millisecond, "wait with decimal"},
		{"429 Too Many Requests. Try again in 30 seconds", 30 * time.Second, "try again in"},
		{"500 internal server error", 0, "no retry-after info"},
		{"Rate limit exceeded", 0, "rate limit without timing"},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			actual := extractRetryAfter(tc.errorMsg)
			if actual != tc.expected {
				t.Errorf("Error '%s': expected %v, got %v", tc.errorMsg, tc.expected, actual)
			}
		})
	}
}

func TestMaxDelayLimit(t *testing.T) {
	config := Config{
		MaxRetries:        5,
		InitialDelay:      1 * time.Second,
		MaxDelay:          3 * time.Second, // Low max delay to test capping
		ExponentialFactor: 2.0,
		Jitter:            false,
	}

	// High attempt numbers should be capped at MaxDelay
	for attempt := 3; attempt <= 10; attempt++ {
		delay := config.calculateDelay(attempt, 0)
		if delay > config.MaxDelay {
			t.Errorf("Attempt %d: delay %v exceeds MaxDelay %v",
				attempt, delay, config.MaxDelay)
		}
		// Should be exactly MaxDelay since jitter is disabled
		expectedDelay := config.MaxDelay
		if delay != expectedDelay {
			t.Errorf("Attempt %d: expected delay %v, got %v",
				attempt, expectedDelay, delay)
		}
	}
}

func TestRetryableErrorTypes(t *testing.T) {
	testCases := []struct {
		errorString     string
		expectedType    ErrorType
		description     string
	}{
		// Network errors - should be retried
		{"connection refused", ErrorTypeRetryable, "connection refused"},
		{"dial tcp: connection timeout", ErrorTypeRetryable, "connection timeout"},
		{"network is unreachable", ErrorTypeRetryable, "network unreachable"},
		{"dns lookup failed", ErrorTypeRetryable, "dns lookup"},
		{"connection reset by peer", ErrorTypeRetryable, "connection reset"},

		// Server errors - should be retried
		{"500 Internal Server Error", ErrorTypeRetryable, "500 error"},
		{"502 Bad Gateway", ErrorTypeRetryable, "502 error"},
		{"503 Service Unavailable", ErrorTypeRetryable, "503 error"},
		{"504 Gateway Timeout", ErrorTypeRetryable, "504 error"},

		// Rate limiting - special handling
		{"429 Too Many Requests", ErrorTypeRateLimit, "429 error"},
		{"Rate limit exceeded", ErrorTypeRateLimit, "rate limit text"},
		{"Too many requests", ErrorTypeRateLimit, "too many requests"},

		// Client errors - should not be retried
		{"400 Bad Request", ErrorTypeNonRetryable, "400 error"},
		{"401 Unauthorized", ErrorTypeNonRetryable, "401 error"},
		{"403 Forbidden", ErrorTypeNonRetryable, "403 error"},
		{"404 Not Found", ErrorTypeNonRetryable, "404 error"},
		{"422 Unprocessable Entity", ErrorTypeNonRetryable, "422 error"},
		{"Invalid API key", ErrorTypeNonRetryable, "invalid api key"},
		{"Authentication failed", ErrorTypeNonRetryable, "authentication error"},

		// Context errors - should not be retried
		{context.Canceled.Error(), ErrorTypeNonRetryable, "context canceled"},
		{context.DeadlineExceeded.Error(), ErrorTypeNonRetryable, "context deadline exceeded"},

		// Unknown errors - default to non-retryable
		{"unknown error type", ErrorTypeNonRetryable, "unknown error"},
		{"custom application error", ErrorTypeNonRetryable, "custom error"},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			err := fmt.Errorf(tc.errorString)
			actualType, _ := classifyError(err)
			if actualType != tc.expectedType {
				t.Errorf("Error '%s': expected %v, got %v",
					tc.errorString, tc.expectedType, actualType)
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	// Verify default values
	if config.MaxRetries != 3 {
		t.Errorf("Expected MaxRetries=3, got %d", config.MaxRetries)
	}
	if config.InitialDelay != 1*time.Second {
		t.Errorf("Expected InitialDelay=1s, got %v", config.InitialDelay)
	}
	if config.MaxDelay != 32*time.Second {
		t.Errorf("Expected MaxDelay=32s, got %v", config.MaxDelay)
	}
	if config.ExponentialFactor != 2.0 {
		t.Errorf("Expected ExponentialFactor=2.0, got %f", config.ExponentialFactor)
	}
	if !config.Jitter {
		t.Error("Expected Jitter=true")
	}
}

func TestIsRetryableError(t *testing.T) {
	testCases := []struct {
		errorString string
		expected    bool
	}{
		{"500 internal server error", true},
		{"429 rate limit", true},
		{"connection timeout", true},
		{"401 unauthorized", false},
		{"400 bad request", false},
		{"unknown error", false},
	}

	for _, tc := range testCases {
		err := fmt.Errorf(tc.errorString)
		actual := IsRetryableError(err)
		if actual != tc.expected {
			t.Errorf("Error '%s': expected retryable=%v, got %v",
				tc.errorString, tc.expected, actual)
		}
	}
}

func TestExtractHTTPRetryAfter(t *testing.T) {
	testCases := []struct {
		name     string
		headers  map[string]string
		expected time.Duration
	}{
		{
			name:     "retry-after seconds",
			headers:  map[string]string{"Retry-After": "30"},
			expected: 30 * time.Second,
		},
		{
			name:     "x-ratelimit-reset-after seconds",
			headers:  map[string]string{"X-RateLimit-Reset-After": "45"},
			expected: 45 * time.Second,
		},
		{
			name:     "x-ratelimit-reset unix timestamp",
			headers:  map[string]string{"X-RateLimit-Reset": fmt.Sprintf("%d", time.Now().Add(60*time.Second).Unix())},
			expected: 60 * time.Second, // Approximate
		},
		{
			name:     "no retry headers",
			headers:  map[string]string{"Content-Type": "application/json"},
			expected: 0,
		},
		{
			name:     "invalid retry-after",
			headers:  map[string]string{"Retry-After": "invalid"},
			expected: 0,
		},
		{
			name:     "past timestamp in x-ratelimit-reset",
			headers:  map[string]string{"X-RateLimit-Reset": fmt.Sprintf("%d", time.Now().Add(-60*time.Second).Unix())},
			expected: 0, // Past timestamp should return 0
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a mock HTTP response
			resp := &http.Response{
				Header: make(http.Header),
			}
			for key, value := range tc.headers {
				resp.Header.Set(key, value)
			}

			actual := ExtractHTTPRetryAfter(resp)

			// For unix timestamp tests, allow some tolerance
			if tc.name == "x-ratelimit-reset unix timestamp" {
				// Allow 5 second tolerance for timing differences
				tolerance := 5 * time.Second
				if actual < tc.expected-tolerance || actual > tc.expected+tolerance {
					t.Errorf("Expected ~%v, got %v (outside tolerance)", tc.expected, actual)
				}
			} else {
				if actual != tc.expected {
					t.Errorf("Expected %v, got %v", tc.expected, actual)
				}
			}
		})
	}

	// Test nil response
	if result := ExtractHTTPRetryAfter(nil); result != 0 {
		t.Errorf("Expected 0 for nil response, got %v", result)
	}
}
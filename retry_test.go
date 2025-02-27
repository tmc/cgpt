package cgpt

import (
	"context"
	"testing"
	"time"
)

func TestWithRateLimit(t *testing.T) {
	t.Run("successful call", func(t *testing.T) {
		result, err := WithRateLimit(context.Background(), func(ctx context.Context) (string, error) {
			return "success", nil
		})

		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if result != "success" {
			t.Errorf("expected 'success', got %q", result)
		}
	})

	t.Run("rate limiting", func(t *testing.T) {
		// Make several quick requests to test rate limiting
		start := time.Now()
		for i := 0; i < 5; i++ {
			_, err := WithRateLimit(context.Background(), func(ctx context.Context) (string, error) {
				return "success", nil
			})
			if err != nil {
				t.Errorf("request %d failed: %v", i, err)
			}
		}
		duration := time.Since(start)

		// With rate limit of 10 per second, 5 requests should take at least 400ms
		if duration < 400*time.Millisecond {
			t.Errorf("requests completed too quickly (rate limit not working): %v", duration)
		}
	})

	t.Run("respect context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(50 * time.Millisecond)
			cancel()
		}()

		_, err := WithRateLimit(ctx, func(ctx context.Context) (string, error) {
			time.Sleep(100 * time.Millisecond) // Simulate long operation
			return "", nil
		})

		if err == nil {
			t.Error("expected error after context cancellation")
		}
	})
}

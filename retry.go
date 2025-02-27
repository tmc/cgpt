package cgpt

import (
	"context"
	"math"
	"math/rand"
	"time"
)

type RetryConfig struct {
	MaxAttempts       int
	InitialBackoff    time.Duration
	MaxBackoff        time.Duration
	BackoffMultiplier float64
	JitterFactor      float64
}

var DefaultRetryConfig = RetryConfig{
	MaxAttempts:       3,
	InitialBackoff:    100 * time.Millisecond,
	MaxBackoff:        10 * time.Second,
	BackoffMultiplier: 2.0,
	JitterFactor:      0.1,
}

// WithRetry wraps a function with retry logic using exponential backoff and jitter
func WithRetry[T any](ctx context.Context, fn func(context.Context) (T, error), cfg RetryConfig) (T, error) {
	var result T
	var err error

	for attempt := 0; attempt < cfg.MaxAttempts; attempt++ {
		result, err = fn(ctx)
		if err == nil {
			return result, nil
		}

		// Check if we should retry based on the error type
		if !isRetryableError(err) {
			return result, err
		}

		// Don't sleep after the last attempt
		if attempt == cfg.MaxAttempts-1 {
			break
		}

		// Calculate backoff with jitter
		backoff := calculateBackoff(attempt, cfg)

		// Wait for backoff duration or context cancellation
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		case <-time.After(backoff):
		}
	}

	return result, err
}

func calculateBackoff(attempt int, cfg RetryConfig) time.Duration {
	// Calculate base backoff using exponential backoff
	backoff := float64(cfg.InitialBackoff) * math.Pow(cfg.BackoffMultiplier, float64(attempt))

	// Apply max backoff limit
	if backoff > float64(cfg.MaxBackoff) {
		backoff = float64(cfg.MaxBackoff)
	}

	// Apply jitter
	jitter := (rand.Float64()*2 - 1) * cfg.JitterFactor * backoff
	backoff = backoff + jitter

	return time.Duration(backoff)
}

// isRetryableError determines if an error should be retried
func isRetryableError(err error) bool {
	// TODO: Add more specific error type checks
	// For now, we'll retry on any error as a basic implementation
	return true
}

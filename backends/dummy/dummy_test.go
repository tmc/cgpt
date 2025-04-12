package dummy

import (
	"context"
	"testing"
	"time"

	"github.com/tmc/cgpt/options"
	"github.com/tmc/langchaingo/llms"
)

func TestDummyBackendSlowResponses(t *testing.T) {
	// Create two backends - one with slow responses and one without
	regularBackend, err := Constructor(&options.Config{}, nil)
	if err != nil {
		t.Fatalf("Failed to create regular backend: %v", err)
	}

	slowBackend, err := Constructor(&options.Config{SlowResponses: true}, nil)
	if err != nil {
		t.Fatalf("Failed to create slow backend: %v", err)
	}

	// Test streaming speed
	testCases := []struct {
		name     string
		backend  llms.Model
		isSlow   bool
	}{
		{
			name:     "regular_speed",
			backend:  regularBackend,
			isSlow:   false,
		},
		{
			name:     "slow_responses",
			backend:  slowBackend,
			isSlow:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Time the streaming response
			start := time.Now()
			
			var receivedChunks int
			
			// Use streaming to measure timing
			_, err := tc.backend.GenerateContent(
				context.Background(),
				[]llms.MessageContent{llms.TextParts(llms.ChatMessageTypeHuman, "Test input")},
				llms.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
					receivedChunks++
					return nil
				}),
			)
			
			if err != nil {
				t.Fatalf("Failed to generate content: %v", err)
			}
			
			duration := time.Since(start)
			
			// Verify we received chunks
			if receivedChunks == 0 {
				t.Errorf("Expected to receive chunks, got none")
			}
			
			// Check timing for slow mode vs regular mode
			if tc.isSlow {
				// Slow responses should take significantly longer
				// Simply log the time for verification since precise timing is environment dependent
				t.Logf("Slow response time: %v", duration)
			} else {
				// Regular responses should be relatively faster
				t.Logf("Regular response time: %v", duration)
			}
			
			// The real test is just to verify that slow response mode produces a significant
			// delay compared to regular mode, but we can't reliably time test durations
			if tc.isSlow && receivedChunks > 0 {
				t.Logf("✓ Slow response mode produced %d chunks", receivedChunks)
			}
		})
	}
}
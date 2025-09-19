package cgpt

import (
	"bytes"
	"strings"
	"testing"
)

func TestDisplayUsageEnhanced(t *testing.T) {
	tests := []struct {
		name           string
		generationInfo map[string]any
		expectedParts  []string
	}{
		{
			name: "basic usage",
			generationInfo: map[string]any{
				"InputTokens":  100,
				"OutputTokens": 200,
				"TotalTokens":  300,
			},
			expectedParts: []string{"in:100", "out:200", "total:300", "cost:$0.003"},
		},
		{
			name: "with cache",
			generationInfo: map[string]any{
				"InputTokens":       100,
				"OutputTokens":      200,
				"TotalTokens":       300,
				"CachedInputTokens": 50,
			},
			expectedParts: []string{"in:100", "out:200", "total:300", "cache:50(16%)", "cost:$0.003", "saved:$0.0001"},
		},
		{
			name: "with thinking",
			generationInfo: map[string]any{
				"InputTokens":             100,
				"OutputTokens":            200,
				"TotalTokens":             300,
				"ThinkingTokens":          150,
				"ThinkingBudgetUsed":      150,
				"ThinkingBudgetAllocated": 200,
			},
			expectedParts: []string{"in:100", "out:200", "total:300", "think:150(75%)", "cost:$0.003"},
		},
		{
			name: "with cache and thinking",
			generationInfo: map[string]any{
				"InputTokens":             100,
				"OutputTokens":            200,
				"TotalTokens":             300,
				"CachedInputTokens":       50,
				"ThinkingTokens":          150,
				"ThinkingBudgetUsed":      150,
				"ThinkingBudgetAllocated": 200,
				"ThinkingCachedTokens":    30,
			},
			expectedParts: []string{"in:100", "out:200", "total:300", "cache:80(26%)", "think:150(75%)", "cost:$0.003", "saved:$0.0002"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stderr bytes.Buffer
			s := &CompletionService{
				cfg:    &Config{ShowUsage: true},
				Stderr: &stderr,
			}

			s.displayUsage(tt.generationInfo)

			output := stderr.String()
			// Remove ANSI color codes for easier testing
			output = strings.ReplaceAll(output, "\033[90m", "")
			output = strings.ReplaceAll(output, "\033[0m", "")
			output = strings.TrimSpace(output)

			// Check for the usage prefix
			if !strings.HasPrefix(output, "usage: ") {
				t.Errorf("Expected output to start with 'usage: ', got: %s", output)
			}

			for _, part := range tt.expectedParts {
				if !strings.Contains(output, part) {
					t.Errorf("Expected output to contain %q, got: %s", part, output)
				}
			}
		})
	}
}
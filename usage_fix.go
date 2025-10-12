package cgpt

import (
	"context"
	"fmt"
)

// handleUsageOnlyMode displays usage statistics from history without requiring input
func handleUsageOnlyMode(ctx context.Context, s *CompletionService, opts *RunOptions) error {
	// If ShowUsage is requested without any input, display usage info from history
	if opts.Config.ShowUsage && !opts.Continuous && len(s.payload.Messages) == 0 {
		// Try to load usage from history if available
		if s.historyMetadata != nil && s.historyMetadata.UsageInfo != nil {
			fmt.Fprintf(s.Stderr, "\n\033[38;5;240m")
			fmt.Fprintf(s.Stderr, "═══ Usage Statistics ═══\n")
			fmt.Fprintf(s.Stderr, "Input tokens: %d\n", s.historyMetadata.UsageInfo.TotalInputTokens)
			fmt.Fprintf(s.Stderr, "Output tokens: %d\n", s.historyMetadata.UsageInfo.TotalOutputTokens)
			if s.historyMetadata.UsageInfo.TotalCachedTokens > 0 {
				fmt.Fprintf(s.Stderr, "Cached tokens: %d\n", s.historyMetadata.UsageInfo.TotalCachedTokens)
			}
			if s.historyMetadata.UsageInfo.TotalThinkingTokens > 0 {
				fmt.Fprintf(s.Stderr, "Thinking tokens: %d\n", s.historyMetadata.UsageInfo.TotalThinkingTokens)
			}
			if s.historyMetadata.UsageInfo.TotalCost > 0 {
				fmt.Fprintf(s.Stderr, "Total cost: $%.4f\n", s.historyMetadata.UsageInfo.TotalCost)
			}
			if s.historyMetadata.UsageInfo.TotalSaved > 0 {
				fmt.Fprintf(s.Stderr, "Total saved: $%.4f\n", s.historyMetadata.UsageInfo.TotalSaved)
			}
			fmt.Fprintf(s.Stderr, "Last updated: %s\n", s.historyMetadata.UsageInfo.LastUpdated)
			fmt.Fprintf(s.Stderr, "══════════════════════════\033[0m\n\n")
			return nil
		}

		// No history available
		fmt.Fprintf(s.Stderr, "\033[38;5;240mcgpt: No usage statistics available. Run with input to generate usage info.\033[0m\n")
		fmt.Fprintf(s.Stderr, "\033[38;5;240mExample: cgpt --usage -i \"Your question here\"\033[0m\n")
		return nil
	}

	return nil // Not usage-only mode
}

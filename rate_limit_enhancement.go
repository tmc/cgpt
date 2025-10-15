package cgpt

import (
	"fmt"
	"io"
	"strings"
	"time"
)

// enhanceRateLimitError provides better user feedback for rate limit errors
func enhanceRateLimitError(err error) string {
	if err == nil {
		return ""
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "429") && !strings.Contains(strings.ToLower(errStr), "rate limit") {
		return errStr
	}

	// Extract retry time if available
	var retryMsg string
	patterns := []string{
		"try again later",
		"Please try again later",
		"retry after",
	}

	for _, pattern := range patterns {
		if strings.Contains(strings.ToLower(errStr), strings.ToLower(pattern)) {
			// Try to extract time information
			if strings.Contains(errStr, "concurrent connections") {
				retryMsg = "\nRate limit: Too many concurrent connections. Wait a moment and try again."
			} else if strings.Contains(errStr, "requests per minute") {
				retryMsg = "\nRate limit: Too many requests per minute. Wait 60 seconds."
			} else {
				retryMsg = "\nRate limit exceeded. The system will automatically retry with exponential backoff."
			}
			break
		}
	}

	if retryMsg == "" {
		retryMsg = "\nRate limit encountered. Automatic retry will occur."
	}

	// Add helpful suggestions
	suggestions := []string{
		"• Consider enabling --prompt-caching to reduce token usage",
		"• Use smaller models (e.g., claude-haiku) for simpler tasks",
		"• Batch multiple questions into a single request",
		"• Check your API plan limits at anthropic.com/console",
	}

	return fmt.Sprintf("%s%s\n\nSuggestions:\n%s", errStr, retryMsg, strings.Join(suggestions, "\n"))
}

// logRateLimitRetry logs rate limit retry attempts with user-friendly messages
func logRateLimitRetry(stderr io.Writer, attempt int, maxRetries int, delay time.Duration) {
	if stderr == nil {
		return
	}

	// Format delay in human-readable format
	var delayStr string
	if delay < time.Minute {
		delayStr = fmt.Sprintf("%.0f seconds", delay.Seconds())
	} else {
		delayStr = fmt.Sprintf("%.1f minutes", delay.Minutes())
	}

	fmt.Fprintf(stderr, "\n\033[33m⚠ Rate limit hit (attempt %d/%d)\033[0m\n", attempt, maxRetries)
	fmt.Fprintf(stderr, "\033[38;5;240m→ Waiting %s before retry...\033[0m\n", delayStr)

	// Show progress bar for longer waits
	if delay > 5*time.Second {
		showRetryProgressBar(stderr, delay)
	}
}

// showRetryProgressBar displays a simple progress bar during retry wait
func showRetryProgressBar(stderr io.Writer, duration time.Duration) {
	const barWidth = 30
	steps := 20
	stepDuration := duration / time.Duration(steps)

	fmt.Fprintf(stderr, "\033[38;5;240m")
	for i := 0; i < steps; i++ {
		filled := (i * barWidth) / steps
		bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
		remaining := duration - (time.Duration(i) * stepDuration)
		fmt.Fprintf(stderr, "\r  [%s] %.0fs remaining", bar, remaining.Seconds())
		time.Sleep(stepDuration)
	}
	fmt.Fprintf(stderr, "\r  [%s] Retrying...     \033[0m\n", strings.Repeat("█", barWidth))
}

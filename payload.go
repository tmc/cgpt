package cgpt

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/anthropic"
	"github.com/tmc/langchaingo/llms/openai"
)

// The default maximum number of tokens allowed in a single request.
// This value is used to limit the size of the input to prevent excessive resource usage.
var defaultMaxTokens = 2048

// newCompletionPayload creates a new completion payload.
func newCompletionPayload(cfg *Config) *ChatCompletionPayload {
	p := &ChatCompletionPayload{
		Model:         cfg.Model,
		Stream:        cfg.Stream,
		PromptCaching: cfg.PromptCaching,
	}
	return p
}

type Message = []llms.MessageContent

type ChatCompletionPayload struct {
	Model    string `json:"model"`
	Messages []llms.MessageContent
	Stream   bool `json:"stream,omitempty"`

	// Internal flags for message processing
	PromptCaching bool `json:"-"`
}

func (p *ChatCompletionPayload) addMessage(role llms.ChatMessageType, content string) {
	p.Messages = append(p.Messages, llms.TextParts(role, content))
}

func (p *ChatCompletionPayload) addSystemMessage(content string) {
	if p.PromptCaching {
		// When prompt caching is enabled, add cache control to system messages
		cachedPart := llms.WithCacheControl(
			llms.TextPart(content),
			anthropic.EphemeralCache(),
		)
		p.Messages = append(p.Messages, llms.MessageContent{
			Role:  llms.ChatMessageTypeSystem,
			Parts: []llms.ContentPart{cachedPart},
		})
	} else {
		p.addMessage(llms.ChatMessageTypeSystem, content)
	}
}

func (p *ChatCompletionPayload) addUserMessage(content string) {
	p.addMessage(llms.ChatMessageTypeHuman, content)
}

func (p *ChatCompletionPayload) addAssistantMessage(content string) {
	p.addMessage(llms.ChatMessageTypeAI, content)
}

func (s *CompletionService) PerformCompletionStreaming(ctx context.Context, payload *ChatCompletionPayload, cfg PerformCompletionConfig) (<-chan string, error) {
	ch := make(chan string)
	go func() {
		defer close(ch)
		fullResponse := strings.Builder{}
		firstChunk := true
		addedAssistantMessage := false

		prefillCleanup, spinnerPos := s.handleAssistantPrefill(ctx, payload, cfg)

		// Send prefill immediately if it exists
		if s.nextCompletionPrefill != "" {
			if cfg.EchoPrefill {
				spinnerPos = len(s.nextCompletionPrefill) + 1
			}
			select {
			case ch <- s.nextCompletionPrefill + " ":
			case <-ctx.Done():
				prefillCleanup()
				return
			}
			payload.addAssistantMessage(s.nextCompletionPrefill)
			addedAssistantMessage = true
			fullResponse.WriteString(s.nextCompletionPrefill)
		}

		// Start spinner on the last character
		var spinnerStop func()
		if cfg.ShowSpinner {
			spinnerStop = spin(spinnerPos)
		}

		// Create a cancellable context for the generation
		genCtx, cancel := context.WithCancel(ctx)
		defer cancel()

		// Handle ctrl-c by cancelling the generation context
		go func() {
			select {
			case <-ctx.Done():
				cancel()
			case <-genCtx.Done():
			}
		}()

		// Determine temperature based on thinking mode
		temperature := s.cfg.Temperature
		if (s.cfg.ThinkingBudget > 0 || (s.cfg.ThinkingMode != "" && s.cfg.ThinkingMode != "none")) && s.cfg.Backend == "anthropic" {
			// Anthropic requires temperature=1 when thinking is enabled
			temperature = 1.0
		}

		// Validate max_tokens > budget_tokens constraint for Anthropic
		maxTokens := s.cfg.MaxTokens
		if s.cfg.Backend == "anthropic" && (s.cfg.ThinkingBudget > 0 || (s.cfg.ThinkingMode != "" && s.cfg.ThinkingMode != "none")) {
			// Determine effective thinking budget
			effectiveBudget := s.cfg.ThinkingBudget
			if effectiveBudget == 0 && s.cfg.ThinkingMode != "" && s.cfg.ThinkingMode != "none" {
				// API will use default based on mode, but minimum is 1024
				effectiveBudget = 1024
			}
			// Normalize to API minimum if user set a value below 1024
			if effectiveBudget > 0 && effectiveBudget < 1024 {
				effectiveBudget = 1024
			}

			// Ensure max_tokens > budget_tokens
			if maxTokens <= effectiveBudget {
				// Auto-adjust max_tokens to be greater than budget
				maxTokens = effectiveBudget + 1000
				// Log the adjustment when it happens
				fmt.Fprintf(s.Stderr, "Note: Auto-adjusted max_tokens from %d to %d (must be > thinking budget of %d)\n",
					s.cfg.MaxTokens, maxTokens, effectiveBudget)
			}
		}

		callOpts := []llms.CallOption{
			llms.WithMaxTokens(maxTokens),
			llms.WithTemperature(temperature),
		}
		if s.useLegacyMaxTokens {
			callOpts = append(callOpts, openai.WithLegacyMaxTokensField())
		}
		// Add prompt caching if enabled
		if s.cfg.PromptCaching {
			if s.cfg.Backend == "anthropic" {
				callOpts = append(callOpts, anthropic.WithPromptCaching())
			} else {
				callOpts = append(callOpts, llms.WithPromptCaching(true))
			}
		}
		// Add thinking mode or budget if specified (multi-provider support)
		if s.cfg.ThinkingBudget > 0 {
			callOpts = append(callOpts, llms.WithThinkingBudget(s.cfg.ThinkingBudget))
			if s.cfg.ShowReasoning {
				callOpts = append(callOpts, llms.WithReturnThinking(true))
			}
		} else if s.cfg.ThinkingMode != "" && s.cfg.ThinkingMode != "none" {
			callOpts = append(callOpts, llms.WithThinkingMode(llms.ThinkingMode(s.cfg.ThinkingMode)))
			if s.cfg.ShowReasoning {
				callOpts = append(callOpts, llms.WithReturnThinking(true))
			}
		}
		// Add interleaved thinking if enabled (Claude 4+ only)
		if s.cfg.InterleavedThinking && s.cfg.Backend == "anthropic" {
			// Use Anthropic-specific option to set the beta header
			callOpts = append(callOpts, anthropic.WithInterleavedThinking())
		}
		// Add streaming reasoning function if we're showing reasoning (multi-provider support)
	if s.cfg.ShowReasoning && (s.cfg.ThinkingMode != "" && s.cfg.ThinkingMode != "none" || s.cfg.ThinkingBudget > 0) {
		callOpts = append(callOpts, llms.WithStreamingReasoningFunc(func(ctx context.Context, reasoningChunk, chunk []byte) error {
			if len(reasoningChunk) > 0 {
				// Display thinking content as it arrives
				select {
				case ch <- string(reasoningChunk):
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			// If there's regular content alongside, stream it too
			if len(chunk) > 0 {
				select {
				case ch <- string(chunk):
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			return nil
		}))
	}
	callOpts = append(callOpts, llms.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
			if firstChunk {
				prefillCleanup()
				if spinnerStop != nil {
					spinnerStop()
					spinnerStop = nil
				}
				// In debug mode, add small delay to let debug output print first
				if s.cfg.Debug {
					time.Sleep(10 * time.Millisecond)
				}
				firstChunk = false
			}

			// If debug mode and same fd, buffer output instead of streaming
			if s.cfg.Debug && s.sameFileDescriptor() {
				fullResponse.Write(chunk)
				return nil
			}

			select {
			case ch <- string(chunk):
				fullResponse.Write(chunk)
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}))

		resp, err := s.model.GenerateContent(genCtx, payload.Messages, callOpts...)

		if err != nil && !errors.Is(err, context.Canceled) {
			// Format multi-line errors (e.g., rate limit details) nicely
			errStr := err.Error()
			if strings.Contains(errStr, "\n") {
				log.Printf("failed to generate content:\n%s", errStr)
			} else {
				log.Printf("failed to generate content: %v", err)
			}
		}


		// Note: With StreamingReasoningFunc support, thinking content now streams
		// as it arrives (before the main response). The code below handles any
		// thinking content that wasn't streamed (e.g., from models that don't
		// support streaming thinking).
		if resp != nil && len(resp.Choices) > 0 {
			// Check all choices for thinking content and costs
			var hasDisplayedCosts bool
			for _, choice := range resp.Choices {
				// Display reasoning content if available
				if choice.ReasoningContent != "" && (s.cfg.ShowReasoning || s.cfg.ShowUsage) {
					// Check if this is summarized thinking (Claude 4) or full thinking
					label := "Reasoning"
					if choice.GenerationInfo != nil {
						if signature, ok := choice.GenerationInfo["signature"].(string); ok && signature != "" {
							label = "Reasoning (summarized)"
						}
					}
					select {
					case ch <- fmt.Sprintf("\n\n--- %s (streaming limitation: shown after response) ---\n%s\n---\n", label, choice.ReasoningContent):
					case <-ctx.Done():
					}
				}

				// Check for thinking content in GenerationInfo (Anthropic style)
				if s.cfg.ShowReasoning && choice.GenerationInfo != nil {
					if thinkingContent, ok := choice.GenerationInfo["ThinkingContent"].(string); ok && thinkingContent != "" {
						label := "Thinking"
						// Check for signature indicating this is summarized content
						if signature, ok := choice.GenerationInfo["signature"].(string); ok && signature != "" {
							label = "Thinking (summarized)"
						}
						select {
						case ch <- fmt.Sprintf("\n\n--- %s (streaming limitation: shown after response) ---\n%s\n---\n", label, thinkingContent):
						case <-ctx.Done():
						}
					}
					// Handle redacted thinking if present
					if redactedThinking, ok := choice.GenerationInfo["redacted_thinking"].(string); ok && redactedThinking != "" {
						select {
						case ch <- fmt.Sprintf("\n\n--- Thinking (redacted for safety) ---\n%s\n---\n", redactedThinking):
						case <-ctx.Done():
						}
					}
				}

				// Store and display usage info
				if choice.GenerationInfo != nil {
					// Store the generation info for history tracking (prefer the one with thinking tokens)
					if s.lastGenerationInfo == nil {
						s.lastGenerationInfo = choice.GenerationInfo
					} else if _, hasThinking := choice.GenerationInfo["ThinkingTokens"]; hasThinking {
						// This choice has thinking tokens, use it instead
						s.lastGenerationInfo = choice.GenerationInfo
					}

					// Display costs once (they should be the same across choices)
					if !hasDisplayedCosts && s.cfg.ShowUsage {
						s.displayUsage(choice.GenerationInfo)
						hasDisplayedCosts = true
					}
				}
			}
		}

		// Clean up spinner if it's still running
		if spinnerStop != nil {
			spinnerStop()
		}

		// Add the assistant message if we haven't already
		if !addedAssistantMessage {
			payload.addAssistantMessage(fullResponse.String())
		}

		// If stderr and stdout are the same fd, show clean output after debug
		if s.cfg.Debug && s.sameFileDescriptor() {
			fmt.Fprintf(s.Stderr, "\n\033[36m=== Output ===\033[0m\n")
			fmt.Fprintf(s.Stderr, "%s\n", fullResponse.String())
		}

		s.nextCompletionPrefill = ""
	}()
	return ch, nil
}

// PerformCompletion provides a non-streaming version of the completion.
func (s *CompletionService) PerformCompletion(ctx context.Context, payload *ChatCompletionPayload, cfg PerformCompletionConfig) (string, error) {
	var stopSpinner func()
	var spinnerPos int
	addedAssistantMessage := false

	prefillCleanup, spinnerPos := s.handleAssistantPrefill(ctx, payload, cfg)
	defer prefillCleanup()

	if s.nextCompletionPrefill != "" {
		payload.addAssistantMessage(s.nextCompletionPrefill)
		addedAssistantMessage = true
	}

	if cfg.ShowSpinner {
		stopSpinner = spin(spinnerPos)
		defer stopSpinner()
	}

	// Determine temperature based on thinking mode
	temperature := s.cfg.Temperature
	if (s.cfg.ThinkingBudget > 0 || (s.cfg.ThinkingMode != "" && s.cfg.ThinkingMode != "none")) && s.cfg.Backend == "anthropic" {
		// Anthropic requires temperature=1 when thinking is enabled
		temperature = 1.0
	}

	// Validate max_tokens > budget_tokens constraint for Anthropic
	maxTokens := s.cfg.MaxTokens
	if s.cfg.Backend == "anthropic" && (s.cfg.ThinkingBudget > 0 || (s.cfg.ThinkingMode != "" && s.cfg.ThinkingMode != "none")) {
		// Determine effective thinking budget
		effectiveBudget := s.cfg.ThinkingBudget
		if effectiveBudget == 0 && s.cfg.ThinkingMode != "" && s.cfg.ThinkingMode != "none" {
			// API will use default based on mode, but minimum is 1024
			effectiveBudget = 1024
		}
		// Normalize to API minimum if user set a value below 1024
		if effectiveBudget > 0 && effectiveBudget < 1024 {
			effectiveBudget = 1024
		}

		// Ensure max_tokens > budget_tokens
		if maxTokens <= effectiveBudget {
			// Auto-adjust max_tokens to be greater than budget
			maxTokens = effectiveBudget + 1000
			// Log the adjustment when it happens
			fmt.Fprintf(s.Stderr, "Note: Auto-adjusted max_tokens from %d to %d (must be > thinking budget of %d)\n",
				s.cfg.MaxTokens, maxTokens, effectiveBudget)
		}
	}

	callOpts := []llms.CallOption{
		llms.WithMaxTokens(maxTokens),
		llms.WithTemperature(temperature),
	}
	if s.useLegacyMaxTokens {
		callOpts = append(callOpts, openai.WithLegacyMaxTokensField())
	}
	// Add prompt caching if enabled
	if s.cfg.PromptCaching {
		callOpts = append(callOpts, llms.WithPromptCaching(true))
	}
	// Add thinking mode or budget if specified (multi-provider support)
	if s.cfg.ThinkingBudget > 0 {
		callOpts = append(callOpts, llms.WithThinkingBudget(s.cfg.ThinkingBudget))
	} else if s.cfg.ThinkingMode != "" && s.cfg.ThinkingMode != "none" {
		callOpts = append(callOpts, llms.WithThinkingMode(llms.ThinkingMode(s.cfg.ThinkingMode)))
	}
	// Add interleaved thinking if enabled (Claude 4+ only)
	if s.cfg.InterleavedThinking && s.cfg.Backend == "anthropic" {
		// Use Anthropic-specific option to set the beta header
		callOpts = append(callOpts, anthropic.WithInterleavedThinking())
	}
	response, err := s.model.GenerateContent(ctx, payload.Messages, callOpts...)
	if err != nil {
		return "", fmt.Errorf("failed to generate content: %w", err)
	}
	if len(response.Choices) == 0 {
		return "", fmt.Errorf("no response from model")
	}

	// First pass: Display thinking/reasoning before the actual response
	for _, choice := range response.Choices {
		// Display reasoning content if available
		if choice.ReasoningContent != "" && (s.cfg.ShowReasoning || s.cfg.ShowUsage) {
			// Check if this is summarized thinking (Claude 4) or full thinking
			label := "Reasoning"
			if choice.GenerationInfo != nil {
				if signature, ok := choice.GenerationInfo["signature"].(string); ok && signature != "" {
					label = "Reasoning (summarized)"
				}
			}
			fmt.Fprintf(s.Stderr, "\n--- %s ---\n%s\n---\n", label, choice.ReasoningContent)
		}

		// Check for thinking content in GenerationInfo (Anthropic style)
		if s.cfg.ShowReasoning && choice.GenerationInfo != nil {
			if thinkingContent, ok := choice.GenerationInfo["ThinkingContent"].(string); ok && thinkingContent != "" {
				label := "Thinking"
				// Check for signature indicating this is summarized content
				if signature, ok := choice.GenerationInfo["signature"].(string); ok && signature != "" {
					label = "Thinking (summarized)"
				}
				fmt.Fprintf(s.Stderr, "\n--- %s ---\n%s\n---\n", label, thinkingContent)
			}
			// Handle redacted thinking if present
			if redactedThinking, ok := choice.GenerationInfo["redacted_thinking"].(string); ok && redactedThinking != "" {
				fmt.Fprintf(s.Stderr, "\n--- Thinking (redacted for safety) ---\n%s\n---\n", redactedThinking)
			}
		}
	}

	// Second pass: Extract content and display costs
	var content string
	var hasDisplayedCosts bool

	for _, choice := range response.Choices {
		// Collect non-empty content
		if choice.Content != "" {
			content = choice.Content
		}

		// Store and display usage info
		if choice.GenerationInfo != nil {
			// Store the generation info for history tracking (prefer the one with thinking tokens)
			if s.lastGenerationInfo == nil {
				s.lastGenerationInfo = choice.GenerationInfo
			} else if _, hasThinking := choice.GenerationInfo["ThinkingTokens"]; hasThinking {
				// This choice has thinking tokens, use it instead
				s.lastGenerationInfo = choice.GenerationInfo
			}

			// Display costs once
			if !hasDisplayedCosts && s.cfg.ShowUsage {
				s.displayUsage(choice.GenerationInfo)
				hasDisplayedCosts = true
			}
		}
	}

	if !addedAssistantMessage {
		payload.addAssistantMessage(content)
	}


	return content, nil
}

// handleAssistantPrefill handles the assistant prefill message.
// It returns a cleanup function that should be called after the completion is done.
// The second return value is the location where the spinner could start.
// displayUsage shows compact token usage statistics
func (s *CompletionService) displayUsage(generationInfo map[string]any) {
	if generationInfo == nil {
		return
	}

	// Extract basic token usage
	var inputTokens, outputTokens, totalTokens int
	var cachedInputTokens, cachedOutputTokens int

	if v, ok := generationInfo["InputTokens"].(int); ok {
		inputTokens = v
	}
	if v, ok := generationInfo["OutputTokens"].(int); ok {
		outputTokens = v
	}
	if v, ok := generationInfo["TotalTokens"].(int); ok {
		totalTokens = v
	} else {
		// If TotalTokens isn't provided, calculate it
		totalTokens = inputTokens + outputTokens
	}

	// Extract cached tokens
	if v, ok := generationInfo["CachedInputTokens"].(int); ok {
		cachedInputTokens = v
	}
	if v, ok := generationInfo["CachedOutputTokens"].(int); ok {
		cachedOutputTokens = v
	}

	// Extract thinking/reasoning tokens if available
	thinkingUsage := llms.ExtractThinkingTokens(generationInfo)

	// Compact colon-separated format
	var parts []string

	// Core usage
	parts = append(parts, fmt.Sprintf("in:%d", inputTokens))
	parts = append(parts, fmt.Sprintf("out:%d", outputTokens))
	parts = append(parts, fmt.Sprintf("total:%d", totalTokens))

	// Cache info if present
	totalCached := cachedInputTokens + cachedOutputTokens
	// Extract ThinkingCachedTokens directly since ExtractThinkingTokens doesn't include it
	if v, ok := generationInfo["ThinkingCachedTokens"].(int); ok {
		totalCached += v
	}
	if totalCached > 0 {
		percent := 100 * totalCached / totalTokens
		parts = append(parts, fmt.Sprintf("cache:%d(%d%%)", totalCached, percent))
	}

	// Thinking info if present
	if thinkingUsage != nil && thinkingUsage.ThinkingTokens > 0 {
		if thinkingUsage.ThinkingBudgetAllocated > 0 {
			usedPct := 100 * thinkingUsage.ThinkingBudgetUsed / thinkingUsage.ThinkingBudgetAllocated
			parts = append(parts, fmt.Sprintf("think:%d(%d%%)", thinkingUsage.ThinkingTokens, usedPct))
		} else {
			parts = append(parts, fmt.Sprintf("think:%d", thinkingUsage.ThinkingTokens))
		}
	}

	// Cost estimation (always show when useful)
	// Simple cost estimation (rough approximation)
	// Input: $0.003/1K, Output: $0.015/1K for Claude Sonnet
	inCost := float64(inputTokens) * 0.003 / 1000
	outCost := float64(outputTokens) * 0.015 / 1000
	totalCost := inCost + outCost
	if totalCost >= 0.01 {
		parts = append(parts, fmt.Sprintf("cost:$%.2f", totalCost))
	} else {
		parts = append(parts, fmt.Sprintf("cost:$%.3f", totalCost))
	}

	// Cache savings if any cached tokens
	if totalCached > 0 {
		// Calculate savings from cached tokens
		// Cached input tokens save input cost, cached output tokens save output cost
		cachedInSavings := float64(cachedInputTokens) * 0.003 / 1000
		cachedOutSavings := float64(cachedOutputTokens) * 0.015 / 1000

		// Include thinking cached tokens if present (treated as input cost)
		if v, ok := generationInfo["ThinkingCachedTokens"].(int); ok && v > 0 {
			cachedInSavings += float64(v) * 0.003 / 1000
		}

		totalSavings := cachedInSavings + cachedOutSavings
		if totalSavings > 0 {
			if totalSavings >= 0.01 {
				parts = append(parts, fmt.Sprintf("saved:$%.2f", totalSavings))
			} else {
				parts = append(parts, fmt.Sprintf("saved:$%.4f", totalSavings))
			}
		}
	}

	// Output compact format in grey
	const grey = "\033[90m"
	const reset = "\033[0m"
	fmt.Fprintf(s.Stderr, "\n%susage: %s%s\n", grey, strings.Join(parts, " "), reset)
}

// sameFileDescriptor checks if stderr and stdout point to the same file descriptor
func (s *CompletionService) sameFileDescriptor() bool {
	// Get file info for stdout and stderr
	stdoutFile, ok1 := s.Stdout.(*os.File)
	stderrFile, ok2 := s.Stderr.(*os.File)
	if !ok1 || !ok2 {
		return false
	}

	// Get file descriptors
	stdoutFd := int(stdoutFile.Fd())
	stderrFd := int(stderrFile.Fd())

	// Get stat info for both
	var stdoutStat, stderrStat syscall.Stat_t
	if syscall.Fstat(stdoutFd, &stdoutStat) != nil {
		return false
	}
	if syscall.Fstat(stderrFd, &stderrStat) != nil {
		return false
	}

	// Compare device and inode
	return stdoutStat.Dev == stderrStat.Dev && stdoutStat.Ino == stderrStat.Ino
}

func (s *CompletionService) handleAssistantPrefill(ctx context.Context, payload *ChatCompletionPayload, cfg PerformCompletionConfig) (func(), int) {
	spinnerPos := 0
	if s.nextCompletionPrefill == "" {
		return func() {}, spinnerPos
	}

	// Store the current message count to ensure proper cleanup
	initialMessageCount := len(payload.Messages)

	if cfg.EchoPrefill {
		s.Stdout.Write([]byte(s.nextCompletionPrefill))
		spinnerPos = len(s.nextCompletionPrefill) + 1
	}

	payload.addAssistantMessage(s.nextCompletionPrefill)
	s.nextCompletionPrefill = ""

	return func() {
		// Only cleanup if we actually added a message
		if len(payload.Messages) > initialMessageCount {
			payload.Messages = payload.Messages[:initialMessageCount]
		}
	}, spinnerPos
}

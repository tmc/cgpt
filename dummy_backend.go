package cgpt

import (
	"context"
	"strings"
	"time"

	"github.com/tmc/langchaingo/llms"
)

type DummyBackend struct {
	GenerateText func() string
}

func NewDummyBackend() (*DummyBackend, error) {
	return &DummyBackend{
		GenerateText: func() string { return dummyDefaultText },
	}, nil
}

var dummyDefaultText = `This is a dummy backend response. It will stream out a few hundred tokens to simulate a real backend.

The quick brown fox jumps over the lazy dog. This pangram contains every letter of the English alphabet at least once.

Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum.

This concludes the dummy backend response. Thank you for using the dummy backend!`

func (d *DummyBackend) Call(ctx context.Context, prompt string, options ...llms.CallOption) (string, error) {
	messages := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeHuman, prompt),
	}
	response, err := d.GenerateContent(ctx, messages, options...)
	if err != nil {
		return "", err
	}
	if len(response.Choices) > 0 {
		return response.Choices[0].Content, nil
	}
	return "", nil
}

func (d *DummyBackend) GenerateContent(ctx context.Context, messages []llms.MessageContent, options ...llms.CallOption) (*llms.ContentResponse, error) {
	dummyText := d.GenerateText()
	words := strings.Fields(dummyText)

	opts := llms.CallOptions{}
	for _, opt := range options {
		opt(&opts)
	}

	// Respect MaxTokens if set (approximate words as tokens)
	if opts.MaxTokens > 0 && opts.MaxTokens < len(words) {
		words = words[:opts.MaxTokens]
	}

	// Calculate mock token counts
	inputTokenCount := len(messages) * 50 // Assume ~50 tokens per message
	outputTokenCount := len(words)
	cachedInputTokens := 0
	thinkingTokenCount := 0
	thinkingInputTokens := 0
	thinkingOutputTokens := 0
	thinkingCachedTokens := 0
	thinkingBudgetUsed := 0
	thinkingBudgetAllocated := 0

	// Check if prompt caching is enabled in metadata
	promptCachingEnabled := false
	if opts.Metadata != nil {
		if enabled, ok := opts.Metadata["prompt_caching"].(bool); ok {
			promptCachingEnabled = enabled
		}
	}

	// Simulate prompt caching (cache 30% of input if enabled)
	if promptCachingEnabled {
		cachedInputTokens = inputTokenCount * 30 / 100
	}

	// Check if thinking mode is enabled
	var choices []*llms.ContentChoice
	thinkingConfig := llms.GetThinkingConfig(&opts)
	if thinkingConfig != nil && thinkingConfig.Mode != llms.ThinkingModeNone && thinkingConfig.Mode != "" {
		// Calculate thinking tokens based on mode or budget
		if thinkingConfig.BudgetTokens > 0 {
			thinkingBudgetAllocated = thinkingConfig.BudgetTokens
			thinkingBudgetUsed = thinkingConfig.BudgetTokens * 80 / 100 // Use 80% of budget
			thinkingTokenCount = thinkingBudgetUsed
		} else {
			// Calculate based on mode
			thinkingBudgetAllocated = llms.CalculateThinkingBudget(thinkingConfig.Mode, opts.MaxTokens)
			thinkingBudgetUsed = thinkingBudgetAllocated * 75 / 100 // Use 75% of calculated budget
			thinkingTokenCount = thinkingBudgetUsed
		}

		// Split thinking tokens between input and output
		thinkingInputTokens = thinkingTokenCount * 20 / 100  // 20% input
		thinkingOutputTokens = thinkingTokenCount * 80 / 100 // 80% output

		// Simulate some cached thinking tokens if prompt caching is enabled
		if promptCachingEnabled && thinkingTokenCount > 100 {
			thinkingCachedTokens = thinkingTokenCount * 15 / 100
		}

		// Add a mock thinking choice
		choices = append(choices, &llms.ContentChoice{
			Content: "",
			GenerationInfo: map[string]any{
				"ThinkingContent":         "Let me think step by step about developing a metaprompting system:\n1. Define the core components\n2. Design the prompt structure\n3. Implement feedback loops\n4. Add evaluation metrics",
				"InputTokens":             inputTokenCount,
				"OutputTokens":            outputTokenCount + thinkingTokenCount,
				"TotalTokens":             inputTokenCount + outputTokenCount + thinkingTokenCount,
				"CachedInputTokens":       cachedInputTokens,
				"ThinkingTokens":          thinkingTokenCount,
				"ThinkingInputTokens":     thinkingInputTokens,
				"ThinkingOutputTokens":    thinkingOutputTokens,
				"ThinkingCachedTokens":    thinkingCachedTokens,
				"ThinkingBudgetUsed":      thinkingBudgetUsed,
				"ThinkingBudgetAllocated": thinkingBudgetAllocated,
			},
		})
		// Add the main content choice
		choices = append(choices, &llms.ContentChoice{
			Content: strings.Join(words, " "),
			GenerationInfo: map[string]any{
				"InputTokens":       inputTokenCount,
				"OutputTokens":      outputTokenCount + thinkingTokenCount,
				"TotalTokens":       inputTokenCount + outputTokenCount + thinkingTokenCount,
				"CachedInputTokens": cachedInputTokens,
			},
		})
	} else {
		// Normal response without thinking
		choices = []*llms.ContentChoice{
			{
				Content: strings.Join(words, " "),
				GenerationInfo: map[string]any{
					"InputTokens":       inputTokenCount,
					"OutputTokens":      outputTokenCount,
					"TotalTokens":       inputTokenCount + outputTokenCount,
					"CachedInputTokens": cachedInputTokens,
				},
			},
		}
	}

	response := &llms.ContentResponse{
		Choices: choices,
	}

	if opts.StreamingFunc != nil {
		for _, word := range words {
			select {
			case <-ctx.Done():
				return response, ctx.Err()
			default:
				if err := opts.StreamingFunc(ctx, []byte(word+" ")); err != nil {
					return response, err
				}
				time.Sleep(40 * time.Millisecond) // Simulate streaming delay
			}
		}
		return response, nil
	}

	return response, nil
}

func (d *DummyBackend) CreateEmbedding(ctx context.Context, text string) ([]float64, error) {
	// Dummy embedding (just return a fixed-size vector of zeros)
	return make([]float64, 128), nil
}

// Package lmhist provides comprehensive history management for language model conversations.
// It handles session storage, prefill tracking, reasoning metadata, and conversation replay.
package lmhist

import (
	"strings"
	"time"

	"github.com/tmc/langchaingo/llms"
	"gopkg.in/yaml.v3"
)

// Metadata tracks session-level information including reasoning and prefill usage
type Metadata struct {
	Created     time.Time `yaml:"created"`
	Description string    `yaml:"description,omitempty"`
	ForkedFrom  string    `yaml:"forked_from,omitempty"`
	ForkPoint   int       `yaml:"fork_point,omitempty"`

	// Reasoning/thinking mode metadata
	ThinkingMode   string `yaml:"thinking_mode,omitempty"`   // none, low, medium, high, auto
	ThinkingBudget int    `yaml:"thinking_budget,omitempty"` // Token budget for thinking

	// Session characteristics
	Backend     string `yaml:"backend"`
	Model       string `yaml:"model"`
	TotalTokens int    `yaml:"total_tokens,omitempty"`

	// Feature usage tracking
	PrefillUsed    bool `yaml:"prefill_used,omitempty"`    // Whether any prefills were used
	CachingEnabled bool `yaml:"caching_enabled,omitempty"` // Whether prompt caching was enabled
}

// PrefillMetadata tracks prefill usage for a message
type PrefillMetadata struct {
	Text      LiteralString `yaml:"text"`                // The actual prefill used
	Skipped   bool          `yaml:"skipped,omitempty"`   // True if prefill was skipped
	Reason    string        `yaml:"reason,omitempty"`    // Why it was skipped
	Timestamp time.Time     `yaml:"timestamp"`           // When this prefill was attempted
}

// TokenUsage tracks detailed token consumption including reasoning
type TokenUsage struct {
	InputTokens    int `yaml:"input_tokens"`
	OutputTokens   int `yaml:"output_tokens"`
	TotalTokens    int `yaml:"total_tokens"`

	// Caching information
	CachedInputTokens  int `yaml:"cached_input_tokens,omitempty"`
	CachedOutputTokens int `yaml:"cached_output_tokens,omitempty"`

	// Reasoning/thinking tokens
	ThinkingTokens        int `yaml:"thinking_tokens,omitempty"`
	ThinkingInputTokens   int `yaml:"thinking_input_tokens,omitempty"`
	ThinkingOutputTokens  int `yaml:"thinking_output_tokens,omitempty"`
	ThinkingCachedTokens  int `yaml:"thinking_cached_tokens,omitempty"`
	ThinkingBudgetUsed    int `yaml:"thinking_budget_used,omitempty"`
	ThinkingBudgetAllocated int `yaml:"thinking_budget_allocated,omitempty"`
}

// MessageInfo extends message tracking with detailed context
type MessageInfo struct {
	Index     int                   `yaml:"index"`
	Role      llms.ChatMessageType  `yaml:"role"`
	Content   LiteralString         `yaml:"content"`
	Timestamp time.Time             `yaml:"timestamp"`

	// Token tracking per message
	Usage *TokenUsage `yaml:"usage,omitempty"`

	// Reasoning content if available
	ReasoningContent LiteralString `yaml:"reasoning_content,omitempty"`

	// Prefill metadata if this message used prefill
	Prefill *PrefillMetadata `yaml:"prefill,omitempty"`

	// Generation metadata
	GenerationInfo map[string]interface{} `yaml:"generation_info,omitempty"`
}

// LiteralString forces YAML to use literal block style (|) for multiline strings
type LiteralString string

// MarshalYAML implements yaml.Marshaler to force literal block style
func (ls LiteralString) MarshalYAML() (interface{}, error) {
	s := string(ls)
	if strings.Contains(s, "\n") {
		// Use literal block style for multiline content
		return &yaml.Node{
			Kind:  yaml.ScalarNode,
			Style: yaml.LiteralStyle,
			Value: s,
		}, nil
	}
	// Use default style for single-line content
	return s, nil
}

// Session represents a complete conversation history with rich metadata
type Session struct {
	// Core session data
	Metadata *Metadata     `yaml:"metadata"`
	Messages []MessageInfo `yaml:"messages"`

	// Session-level aggregates
	TotalUsage *TokenUsage `yaml:"total_usage,omitempty"`

	// Raw langchaingo messages for compatibility
	RawMessages []llms.MessageContent `yaml:"raw_messages,omitempty"`
}

// SessionSummary provides a lightweight view of session data for listing/indexing
type SessionSummary struct {
	ID          string    `yaml:"id"`
	Created     time.Time `yaml:"created"`
	Description string    `yaml:"description"`
	Backend     string    `yaml:"backend"`
	Model       string    `yaml:"model"`
	MessageCount int      `yaml:"message_count"`
	TotalTokens  int      `yaml:"total_tokens"`
	HasPrefill   bool     `yaml:"has_prefill"`
	HasThinking  bool     `yaml:"has_thinking"`
}
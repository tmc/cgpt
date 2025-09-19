package cgpt

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tmc/langchaingo/llms"
	"sigs.k8s.io/yaml"
)

type usageInfo struct {
	TotalInputTokens   int     `yaml:"total_input_tokens,omitempty"`
	TotalOutputTokens  int     `yaml:"total_output_tokens,omitempty"`
	TotalCachedTokens  int     `yaml:"total_cached_tokens,omitempty"`
	TotalThinkingTokens int    `yaml:"total_thinking_tokens,omitempty"`
	TotalCost          float64 `yaml:"total_cost,omitempty"`
	TotalSaved         float64 `yaml:"total_saved,omitempty"`
	LastUpdated        string  `yaml:"last_updated,omitempty"`
}

type historyMetadata struct {
	Created     string     `yaml:"created,omitempty"`
	Description string     `yaml:"description,omitempty"`
	ForkedFrom  string     `yaml:"forked_from,omitempty"`
	ForkPoint   int        `yaml:"fork_point,omitempty"`
	UsageInfo   *usageInfo `yaml:"usage_info,omitempty"`
}

type history struct {
	Metadata *historyMetadata      `yaml:"metadata,omitempty"`
	Backend  string                `yaml:"backend"`
	Model    string                `yaml:"model"`
	Messages []llms.MessageContent `yaml:"messages"`
}

// loadHistory loads the history from the history file (as yaml)
func (s *CompletionService) loadHistory() error {
	if s.historyIn == nil {
		return nil
	}
	b, err := io.ReadAll(s.historyIn)
	if err != nil {
		return err
	}
	var h history
	if err := yaml.Unmarshal(b, &h); err != nil {
		return err
	}
	if h.Model != "" {
		s.payload.Model = h.Model
	}
	s.payload.Messages = h.Messages

	// Load metadata if present
	if h.Metadata != nil {
		s.historyMetadata = h.Metadata
		// Initialize usage info if not present
		if s.historyMetadata.UsageInfo == nil {
			s.historyMetadata.UsageInfo = &usageInfo{}
		}
	}
	return nil
}

func (s *CompletionService) saveHistory() error {
	if s.disableHistory {
		return nil
	}

	// New history system with file handle
	if s.historyFile != nil {
		return s.saveHistoryToFile(s.historyFile)
	}

	// Legacy history system
	if s.historyOutFile == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get user home directory: %w", err)
		}

		// Use session timestamp instead of generating a new one each time
		defaultSavePath := filepath.Join(home, ".cgpt", fmt.Sprintf("default-history-%s.yaml", s.sessionTimestamp))
		err = createHistoryFile(defaultSavePath, s.cfg.Backend, s.payload, s.payload.Messages)
		if err != nil {
			return err
		}
	} else {
		err := createHistoryFile(s.historyOutFile, s.cfg.Backend, s.payload, s.payload.Messages)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *CompletionService) saveHistoryToFile(f *os.File) error {
	// Generate description on first AI response if we have metadata
	if s.historyMetadata != nil && s.historyMetadata.Description == "" && len(s.payload.Messages) >= 2 {
		s.historyMetadata.Description = s.generateDescription()
	}

	// Update usage info if we have metadata and generation info
	if s.historyMetadata != nil && s.lastGenerationInfo != nil {
		s.updateUsageInfo()
	}

	h := history{
		Metadata: s.historyMetadata,
		Backend:  s.cfg.Backend,
		Model:    s.payload.Model,
		Messages: s.payload.Messages,
	}

	// Marshal to YAML
	ybytes, err := yaml.Marshal(h)
	if err != nil {
		return fmt.Errorf("failed to marshal history: %w", err)
	}

	if f != os.Stdout {
		// Truncate and seek to beginning for updates
		f.Truncate(0)
		f.Seek(0, 0)
	}

	// Write the YAML content
	if _, err := f.Write(ybytes); err != nil {
		return fmt.Errorf("failed to write history: %w", err)
	}

	if f != os.Stdout {
		return f.Sync()
	}
	return nil
}

// saveHistory saves the history to the history file (as yaml)
func createHistoryFile(historyOutFile string, backend string, payload *ChatCompletionPayload, messages []llms.MessageContent) error {
	f, err := os.Create(historyOutFile)
	if err != nil {
		return fmt.Errorf("failed to create history file %q: %w", historyOutFile, err)

	}
	if payload == nil {
		return nil
	}
	h := history{
		Backend:  backend,
		Model:    payload.Model,
		Messages: messages,
	}
	// encode with k8s yaml encoder: which doesn't define NewEncoder:
	ybytes, err := yaml.Marshal(h)
	if err != nil {
		return fmt.Errorf("failed to marshal history: %w", err)
	}
	if _, err := f.Write(ybytes); err != nil {
		return fmt.Errorf("failed to write history file %q: %w", historyOutFile, err)
	}
	return nil
}

// generateHistoryTitle sends the conversation history to the LLM to generate a descriptive title
func (s *CompletionService) generateHistoryTitle(ctx context.Context) (string, error) {
	// Don't try to generate a title if we have no messages
	if len(s.payload.Messages) < 2 {
		return "empty-chat", nil
	}

	prompt := "Generate a kebab case title for the following conversation. An example is debug-rust-code or explain-quantum-mechanics."
	msgLimit := min(len(s.payload.Messages), 10)
	for _, m := range s.payload.Messages[:msgLimit] {
		for _, p := range m.Parts {
			prompt += fmt.Sprint(p)
		}
	}

	completion, err := llms.GenerateFromSinglePrompt(ctx, s.model, prompt)
	if err != nil {
		return "", fmt.Errorf("failed to generate title: %w", err)
	}

	fmt.Println("completion", completion)

	// If title is too long, truncate it
	const maxTitleLength = 50
	if len(completion) > maxTitleLength {
		completion = completion[:maxTitleLength]
	}

	return completion, nil
}

// generateDescription creates a short description from the conversation
func (s *CompletionService) generateDescription() string {
	if len(s.payload.Messages) < 2 {
		return ""
	}

	// Get first user message
	var firstUserMsg string
	for _, msg := range s.payload.Messages {
		if msg.Role == "human" || msg.Role == "user" {
			for _, part := range msg.Parts {
				if text, ok := part.(llms.TextContent); ok {
					firstUserMsg = text.Text
					break
				}
			}
			if firstUserMsg != "" {
				break
			}
		}
	}

	if firstUserMsg == "" {
		return ""
	}

	// Simple keyword extraction (take first 50 chars, clean up)
	desc := strings.TrimSpace(firstUserMsg)
	if len(desc) > 50 {
		desc = desc[:50]
		// Try to break at word boundary
		if idx := strings.LastIndex(desc, " "); idx > 30 {
			desc = desc[:idx]
		}
	}

	// Remove problematic characters
	desc = strings.ReplaceAll(desc, "\n", " ")
	desc = strings.ReplaceAll(desc, "\r", " ")
	desc = strings.ReplaceAll(desc, "\t", " ")

	// Collapse multiple spaces
	for strings.Contains(desc, "  ") {
		desc = strings.ReplaceAll(desc, "  ", " ")
	}

	return strings.TrimSpace(desc)
}

// renameChatHistory generates a title and renames the history file
func (s *CompletionService) renameChatHistory(ctx context.Context) error {
	// Skip auto-naming unless explicitly enabled
	if s.disableHistory || !s.autoNameHistory {
		return nil
	}
	if s.historyOutFile == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get user home directory: %w", err)
		}

		// Get the current history file path
		currentPath := filepath.Join(home, ".cgpt", fmt.Sprintf("default-history-%s.yaml", s.sessionTimestamp))

		// Generate a descriptive title
		title, err := s.generateHistoryTitle(ctx)
		if err != nil {
			return fmt.Errorf("failed to generate title: %w", err)
		}

		// Create new filename with timestamp + title
		newPath := filepath.Join(home, ".cgpt", fmt.Sprintf("%s.yaml", title))

		// Rename the file
		if err := os.Rename(currentPath, newPath); err != nil {
			return fmt.Errorf("failed to rename history file: %w", err)
		}

		fmt.Fprintf(s.Stderr, "\033[38;5;240mcgpt: Renamed history to: %s\033[0m\n", filepath.Base(newPath))

		// Update the historyOutFile to use the new path
		s.historyOutFile = newPath
	}
	return nil
}

// updateUsageInfo accumulates usage statistics from the last generation
func (s *CompletionService) updateUsageInfo() {
	if s.historyMetadata == nil || s.lastGenerationInfo == nil {
		return
	}

	// Ensure usage info is initialized
	if s.historyMetadata.UsageInfo == nil {
		s.historyMetadata.UsageInfo = &usageInfo{}
	}

	usage := s.historyMetadata.UsageInfo

	// Extract token counts
	if v, ok := s.lastGenerationInfo["InputTokens"].(int); ok {
		usage.TotalInputTokens += v
	}
	if v, ok := s.lastGenerationInfo["OutputTokens"].(int); ok {
		usage.TotalOutputTokens += v
	}

	// Extract cached tokens
	var cachedInputTokens, cachedOutputTokens int
	if v, ok := s.lastGenerationInfo["CachedInputTokens"].(int); ok {
		cachedInputTokens = v
		usage.TotalCachedTokens += v
	}
	if v, ok := s.lastGenerationInfo["CachedOutputTokens"].(int); ok {
		cachedOutputTokens = v
		usage.TotalCachedTokens += v
	}

	// Extract thinking tokens
	thinkingUsage := llms.ExtractThinkingTokens(s.lastGenerationInfo)
	if thinkingUsage.ThinkingTokens > 0 {
		usage.TotalThinkingTokens += thinkingUsage.ThinkingTokens
	}
	// Also check for ThinkingCachedTokens directly since it might not be in ExtractThinkingTokens
	if v, ok := s.lastGenerationInfo["ThinkingCachedTokens"].(int); ok {
		usage.TotalCachedTokens += v
	}

	// Calculate cost for this generation
	var inputTokens, outputTokens int
	if v, ok := s.lastGenerationInfo["InputTokens"].(int); ok {
		inputTokens = v
	}
	if v, ok := s.lastGenerationInfo["OutputTokens"].(int); ok {
		outputTokens = v
	}

	// Simple cost estimation (rates from Claude Sonnet)
	inputCost := float64(inputTokens) * 0.003 / 1000
	outputCost := float64(outputTokens) * 0.015 / 1000
	generationCost := inputCost + outputCost

	// Calculate savings
	cachedInSavings := float64(cachedInputTokens) * 0.003 / 1000
	cachedOutSavings := float64(cachedOutputTokens) * 0.015 / 1000
	generationSavings := cachedInSavings + cachedOutSavings

	// Update totals
	usage.TotalCost += generationCost
	usage.TotalSaved += generationSavings
	usage.LastUpdated = time.Now().Format(time.RFC3339)
}

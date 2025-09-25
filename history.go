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

	// For stdout, write directly
	if f == os.Stdout {
		if _, err := f.Write(ybytes); err != nil {
			return fmt.Errorf("failed to write history: %w", err)
		}
		return nil
	}

	// For regular files, use atomic write pattern
	fileName := f.Name()
	tempFile, err := os.CreateTemp(filepath.Dir(fileName), ".cgpt-history-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tempFileName := tempFile.Name()

	// Clean up temp file on error
	defer func() {
		if tempFile != nil {
			tempFile.Close()
			os.Remove(tempFileName)
		}
	}()

	// Write to temp file
	if _, err := tempFile.Write(ybytes); err != nil {
		return fmt.Errorf("failed to write temp history: %w", err)
	}

	// Sync to disk
	if err := tempFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync temp history: %w", err)
	}

	// Close temp file before renaming
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp history: %w", err)
	}
	tempFile = nil // Mark as closed for defer cleanup

	// Atomically rename temp file to target
	if err := os.Rename(tempFileName, fileName); err != nil {
		return fmt.Errorf("failed to rename history file: %w", err)
	}

	return nil
}

// saveHistory saves the history to the history file (as yaml)
func createHistoryFile(historyOutFile string, backend string, payload *ChatCompletionPayload, messages []llms.MessageContent) error {
	if payload == nil {
		return nil
	}

	h := history{
		Backend:  backend,
		Model:    payload.Model,
		Messages: messages,
	}

	// Marshal to YAML
	ybytes, err := yaml.Marshal(h)
	if err != nil {
		return fmt.Errorf("failed to marshal history: %w", err)
	}

	// Create temp file in same directory for atomic write
	dir := filepath.Dir(historyOutFile)
	if dir == "" {
		dir = "."
	}
	tempFile, err := os.CreateTemp(dir, ".cgpt-history-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tempFileName := tempFile.Name()

	// Ensure cleanup on error
	defer func() {
		if tempFile != nil {
			tempFile.Close()
			os.Remove(tempFileName)
		}
	}()

	// Write to temp file
	if _, err := tempFile.Write(ybytes); err != nil {
		return fmt.Errorf("failed to write temp history file: %w", err)
	}

	// Sync to disk
	if err := tempFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync temp history file: %w", err)
	}

	// Close before rename
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp history file: %w", err)
	}
	tempFile = nil // Mark as closed for defer cleanup

	// Atomically rename to target file
	if err := os.Rename(tempFileName, historyOutFile); err != nil {
		return fmt.Errorf("failed to rename history file %q: %w", historyOutFile, err)
	}

	return nil
}

// generateHistoryTitle generates a descriptive title from the conversation without using the LLM
func (s *CompletionService) generateHistoryTitle(ctx context.Context) (string, error) {
	// Don't try to generate a title if we have no messages
	if len(s.payload.Messages) < 2 {
		return "empty-chat", nil
	}

	// Extract keywords from the first user message
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
		return "conversation", nil
	}

	// Generate kebab-case title from first message
	title := s.extractTitleFromText(firstUserMsg)
	return title, nil
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

// extractTitleFromText generates a kebab-case title from text
func (s *CompletionService) extractTitleFromText(text string) string {
	// Clean and normalize the text
	text = strings.TrimSpace(text)
	if text == "" {
		return "conversation"
	}

	// Take first line or sentence (whichever is shorter)
	if idx := strings.IndexAny(text, "\n.?!"); idx > 0 && idx < 100 {
		text = text[:idx]
	} else if len(text) > 100 {
		text = text[:100]
	}

	// Extract meaningful words (skip common stop words)
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "and": true, "or": true, "but": true,
		"in": true, "on": true, "at": true, "to": true, "for": true, "of": true,
		"with": true, "by": true, "from": true, "as": true, "is": true, "are": true,
		"was": true, "were": true, "been": true, "be": true, "have": true, "has": true,
		"had": true, "do": true, "does": true, "did": true, "will": true, "would": true,
		"could": true, "should": true, "may": true, "might": true, "must": true,
		"can": true, "this": true, "that": true, "these": true, "those": true,
		"i": true, "you": true, "we": true, "they": true, "it": true, "my": true,
		"your": true, "our": true, "their": true, "its": true, "me": true,
	}

	// Split into words and filter
	words := strings.Fields(strings.ToLower(text))
	var titleWords []string
	for _, word := range words {
		// Clean word of punctuation
		word = strings.Trim(word, ",.;:!?'\"()[]{}")
		if word != "" && !stopWords[word] && len(titleWords) < 5 {
			titleWords = append(titleWords, word)
		}
	}

	// If we have no meaningful words, try harder
	if len(titleWords) == 0 {
		for _, word := range words[:min(3, len(words))] {
			word = strings.Trim(word, ",.;:!?'\"()[]{}")
			if word != "" {
				titleWords = append(titleWords, word)
			}
		}
	}

	if len(titleWords) == 0 {
		return "conversation"
	}

	// Join with hyphens to make kebab-case
	title := strings.Join(titleWords, "-")

	// Ensure the title is a valid filename
	title = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		return '-'
	}, title)

	// Clean up multiple dashes
	for strings.Contains(title, "--") {
		title = strings.ReplaceAll(title, "--", "-")
	}

	// Trim dashes and limit length
	title = strings.Trim(title, "-")
	if len(title) > 50 {
		title = title[:50]
		// Clean up if we cut in the middle of a word
		if idx := strings.LastIndex(title, "-"); idx > 30 {
			title = title[:idx]
		}
	}

	if title == "" {
		return "conversation"
	}

	return title
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

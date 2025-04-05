package cgpt

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/tmc/langchaingo/llms"
	"sigs.k8s.io/yaml"
)

type history struct {
	Backend  string                `json:"backend"`
	Model    string                `json:"model"`
	Messages []llms.MessageContent `json:"messages"`
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
	return nil
}

func (s *CompletionService) saveHistory() error {
	if s.disableHistory {
		return nil
	}
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

// renameChatHistory generates a title and renames the history file
func (s *CompletionService) renameChatHistory(ctx context.Context) error {
	if s.disableHistory {
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

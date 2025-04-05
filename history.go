package cgpt

import (
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

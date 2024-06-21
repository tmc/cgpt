package cgpt

import (
	"fmt"
	"io"
	"os"

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

// saveHistory saves the history to the history file (as yaml)
func (s *CompletionService) saveHistory() error {
	if s.historyOutFile == "" {
		return nil
	}
	f, err := os.Create(s.historyOutFile)
	if err != nil {
		return fmt.Errorf("failed to create history file %q: %w", s.historyOutFile, err)

	}
	if s.payload == nil {
		return nil
	}
	h := history{
		Backend:  s.cfg.Backend,
		Model:    s.payload.Model,
		Messages: s.payload.Messages,
	}
	// encode with k8s yaml encoder: which doesn't define NewEncoder:
	ybytes, err := yaml.Marshal(h)
	if err != nil {
		return fmt.Errorf("failed to marshal history: %w", err)
	}
	if _, err := f.Write(ybytes); err != nil {
		return fmt.Errorf("failed to write history file %q: %w", s.historyOutFile, err)
	}
	return nil
}

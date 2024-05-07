package main

import (
	"fmt"
	"io"
	"os"

	"github.com/tmc/langchaingo/llms"
	"gopkg.in/yaml.v2"
)

type history struct {
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
		Model:    s.payload.Model,
		Messages: s.payload.Messages,
	}
	return yaml.NewEncoder(f).Encode(h)
}

//go:build js

package interactive

import (
	"context"
	"errors"
	"fmt"
)

// JSSession provides a minimal Session implementation for JS/WASM environments.
// It does not support interactive terminal features.
type JSSession struct {
	config Config
}

// Compile-time check for Session interface
var _ Session = (*JSSession)(nil)

// NewJSSession creates a new JS/WASM session stub.
func NewJSSession(cfg Config) (*JSSession, error) {
	fmt.Println("Warning: Interactive session features are limited in JS/WASM environment.")
	return &JSSession{config: cfg}, nil
}

// Run is a no-op for the JS session, as interactive terminal is not available.
// It might return an error indicating lack of support or wait indefinitely.
func (s *JSSession) Run(ctx context.Context) error {
	// In a real web app, this might integrate with JS interop for input/output.
	// For a simple build, returning an error or blocking might be appropriate.
	return errors.New("interactive terminal session not supported in JS/WASM")
	// Alternatively, block forever:
	// select {}
}

// SetResponseState is a no-op.
func (s *JSSession) SetResponseState(state ResponseState) {
	// No-op: No UI state to update in this stub.
}

// SetStreaming is a no-op.
func (s *JSSession) SetStreaming(streaming bool) {
	// No-op: No UI to update in this stub.
}

// SetLastInput stores the last input in the config (minimal state).
// func (s *JSSession) SetLastInput(input string) {
// 	s.config.LastInput = input
// }

// AddResponsePart is a no-op.
func (s *JSSession) AddResponsePart(part string) {
	// No-op: No UI to update. Could potentially log or buffer if needed.
}

// GetHistory returns the loaded history or an empty slice.
func (s *JSSession) GetHistory() []string {
	// return s.config.LoadedHistory // Return whatever was loaded initially
	return s.config.ConversationHistory // Use ConversationHistory field
}

// GetHistoryFilename returns the configured filename.
func (s *JSSession) GetHistoryFilename() string {
	return s.config.HistoryFile
}

// LoadHistory is a no-op, returning an error.
func (s *JSSession) LoadHistory(filename string) error {
	// No-op: Filesystem access for history is typically not available or desired in WASM.
	return errors.New("history loading not supported in JS/WASM")
}

// SaveHistory is a no-op, returning an error.
func (s *JSSession) SaveHistory(filename string) error {
	// No-op: Filesystem access for history is typically not available or desired in WASM.
	return errors.New("history saving not supported in JS/WASM")
}

// Quit is a no-op.
func (s *JSSession) Quit() {
	// No-op: Nothing to quit in this stub implementation.
}

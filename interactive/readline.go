//go:build !js
// +build !js

package interactive

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/chzyer/readline"
	"golang.org/x/term"
)

// ReadlineSession implements an interactive terminal session using chzyer/readline.
type ReadlineSession struct {
	reader         *readline.Instance
	config         Config
	buffer         strings.Builder
	state          InteractiveState
	responeState   ResponseState
	multiline      bool      // True when in explicit multiline mode (shows "..." prompt)
	pendingSubmit  bool      // True when we've typed a line but need to press Enter again
	lastInput      string    // Track last successful input
	expectingCtrlE bool      // For Ctrl+X, Ctrl+E support
	interruptCount int       // Track consecutive Ctrl+C presses
	lastCtrlCTime  time.Time // Track time of last Ctrl+C press
	isStreaming    bool      // Track streaming state for prompt handling
}

// Compile-time check for Session interface
var _ Session = (*ReadlineSession)(nil)

// GetHistoryFilename returns the configured history filename.
func (s *ReadlineSession) GetHistoryFilename() string {
	return s.config.HistoryFile
}

// LoadHistory is a stub implementation for ReadlineSession.
func (s *ReadlineSession) LoadHistory(filename string) error {
	// Readline handles history loading via HistoryFile config.
	// This method could potentially reload if needed, but is complex.
	fmt.Fprintf(os.Stderr, "Warning: LoadHistory not fully implemented for readline session.\n")
	return nil
}

// SaveHistory is a stub implementation for ReadlineSession.
func (s *ReadlineSession) SaveHistory(filename string) error {
	// Readline handles history saving via HistoryFile config and Close().
	// This method could force a save if needed.
	fmt.Fprintf(os.Stderr, "Warning: SaveHistory not fully implemented for readline session.\n")
	return nil
}

// Quit closes the readline instance.
func (s *ReadlineSession) Quit() {
	if s.reader != nil {
		s.reader.Close()
	}
}

// NewSession creates a new interactive readline session.
func NewSession(cfg Config) (Session, error) {
	if cfg.SingleLineHint == "" {
		cfg.SingleLineHint = DefaultSingleLineHint
	}
	if cfg.MultiLineHint == "" {
		cfg.MultiLineHint = DefaultMultiLineHint
	}

	// Ensure prompts have a trailing space for visibility
	if cfg.Prompt != "" && !strings.HasSuffix(cfg.Prompt, " ") {
		cfg.Prompt = cfg.Prompt + " "
	}
	if cfg.AltPrompt != "" && !strings.HasSuffix(cfg.AltPrompt, " ") {
		cfg.AltPrompt = cfg.AltPrompt + " "
	}

	// Expand tilde for history file path
	historyPath, err := expandTilde(cfg.HistoryFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not expand history file path '%s': %v\n", cfg.HistoryFile, err)
		historyPath = cfg.HistoryFile // Use original path as fallback
	}
	cfg.HistoryFile = historyPath // Update config with expanded path

	session := &ReadlineSession{
		config:        cfg,
		state:         StateSingleLine,
		multiline:     false,
		pendingSubmit: false,
	}

	listener := session.createListener()
	painter := PainterFunc(func(line []rune, pos int) []rune {
		// Painter is called frequently, keep it fast.

		// Don't show any hints while streaming responses
		if session.isStreaming {
			return line
		}

		// Don't modify non-empty lines - let the user see exactly what they type
		if len(line) > 0 {
			return line
		}

		// For empty lines when in pendingSubmit mode - no need for extra indicators
		// since we now show the ⏎ in the prompt itself

		// For initial empty lines (when buffer is also empty)
		if session.buffer.Len() == 0 {
			// No text prompts at all - completely clean interface
			return line
		}

		return line // Return original line
	})

	// Determine if Stdin is a TTY
	stdinFile, stdinIsFile := cfg.Stdin.(*os.File)
	isTerminalFunc := func() bool {
		if stdinIsFile {
			// Ensure stdinFile is not nil before accessing Fd()
			if stdinFile == nil {
				// If stdin is not an os.File or is nil, assume not a terminal
				// This might happen in tests or specific environments.
				// Fallback to checking os.Stdout as a proxy?
				return term.IsTerminal(int(os.Stdout.Fd()))
			}
			return term.IsTerminal(int(stdinFile.Fd()))
		}
		// Fallback: Check if os.Stdout is a TTY, assuming it's the interactive one
		return term.IsTerminal(int(os.Stdout.Fd()))
	}

	readlineConfig := &readline.Config{
		Prompt:            cfg.Prompt, // Base prompt
		InterruptPrompt:   "^C",       // Prompt shown after Ctrl+C clears line
		EOFPrompt:         "exit",     // Shown on Ctrl+D exit
		HistoryFile:       cfg.HistoryFile,
		HistoryLimit:      10000,
		HistorySearchFold: true,                          // Case-insensitive history search
		AutoComplete:      readline.NewPrefixCompleter(), // Basic prefix completer
		// Stdin will be set below based on TTY detection
		Listener:               listener, // Custom key handling
		Painter:                painter,  // Custom hint display
		ForceUseInteractive:    true,     // Try interactive features even if TTY detection fails
		DisableAutoSaveHistory: true,     // We handle saving manually
		FuncIsTerminal:         isTerminalFunc,
		// Consider adding other readline config options if needed
	}

	// Check if Stdin is specifically the TTY file we opened, and pass it directly
	// to readline's TTY fields. Otherwise, let readline use defaults (os.Stdin/out/err).
	if ttyFile, ok := cfg.Stdin.(*os.File); ok && ttyFile.Name() == "/dev/tty" {
		readlineConfig.Stdin = ttyFile
		// Also crucial: Tell readline to use the same TTY for output!
		readlineConfig.Stdout = ttyFile
		readlineConfig.Stderr = ttyFile // Or os.Stderr if you want errors separate
		fmt.Fprintln(os.Stderr, "cgpt(readline): Using provided /dev/tty handle for Stdin/Stdout/Stderr.")
	} else {
		// Let readline use its defaults if Stdin isn't the specific TTY handle
		readlineConfig.Stdin = cfg.Stdin
		fmt.Fprintln(os.Stderr, "cgpt(readline): Using default Stdin/Stdout/Stderr.")
	}

	reader, err := readline.NewEx(readlineConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize readline: %w", err)
	}
	session.reader = reader

	return session, nil
}

// SetStreaming updates the streaming state, affecting the prompt display.
func (s *ReadlineSession) SetStreaming(streaming bool) {
	changed := s.isStreaming != streaming
	s.isStreaming = streaming
	// Force a prompt redraw only if the state actually changed
	if changed && s.reader != nil {
		s.reader.SetPrompt(s.getPrompt())
		s.reader.Refresh() // Redraw the line
	}
}

// AddResponsePart prints the response part directly. Attempts cleaner rendering.
func (s *ReadlineSession) AddResponsePart(part string) {
	if s.reader == nil {
		fmt.Print(part) // Fallback if reader not initialized
		return
	}
	// Simple approach: Print the part and refresh the prompt.
	// This might cause the current input line to flicker or be temporarily cleared.
	s.reader.Clean()
	fmt.Print(part)
	s.reader.Refresh() // Refresh might redraw the prompt and current input line
}

// getPrompt returns the appropriate prompt based on the current state.
func (s *ReadlineSession) getPrompt() string {
	if s.isStreaming {
		return ""
	} // No prompt during streaming
	if s.multiline {
		return s.config.AltPrompt
	}

	// Basic prompt without modifications
	prompt := s.config.Prompt

	// If we're waiting for the second Enter (pendingSubmit), add a simple ↵ symbol
	if s.pendingSubmit {
		// Remove trailing space if it exists
		prompt = strings.TrimSuffix(prompt, " ")

		// Append the enter symbol with no space
		return prompt + ansiDimColor("↵")
	}

	// For normal mode, ensure prompt has a trailing space
	if prompt != "" && !strings.HasSuffix(prompt, " ") {
		prompt += " "
	}

	return prompt
}

// getPlaceHolder returns the hint text with ANSI codes for dim color.
func (s *ReadlineSession) getPlaceHolder() string {
	// We're using a minimal approach now with no initial hints
	// Only showing hints when in pendingSubmit mode
	return ""
}

// ansiDimColor applies dim ANSI color code.
func ansiDimColor(text string) string { return fmt.Sprintf("\x1b[90m%s\x1b[0m", text) }

// createListener returns a listener that handles specific key events.
func (s *ReadlineSession) createListener() readline.Listener {
	return readline.FuncListener(func(line []rune, pos int, key rune) (newLine []rune, newPos int, ok bool) {
		processed := false
		newLine = line
		newPos = pos

		// Handle Up Arrow only when the line is empty to recall last input
		if key == readline.CharPrev && len(line) == 0 && s.buffer.Len() == 0 && s.lastInput != "" {
			newLine = []rune(s.lastInput)
			newPos = len(newLine)
			processed = true
			ok = processed
			return
		}

		// Ctrl+X, Ctrl+E for external editor
		if s.expectingCtrlE {
			s.expectingCtrlE = false
			if key == 5 { // Ctrl+E
				fmt.Fprintln(os.Stderr, "\nEditing in $EDITOR...")
				currentContent := s.buffer.String()
				if s.buffer.Len() > 0 && len(line) > 0 {
					currentContent += "\n"
				}
				currentContent += string(line)
				editedText, err := s.editInEditor(currentContent)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Editor error: %v\n", err)
					s.reader.Refresh()
				} else {
					if strings.Contains(editedText, "\n") {
						s.buffer.Reset()
						s.buffer.WriteString(editedText)
						newLine = []rune{}
						newPos = 0
						s.multiline = true
					} else {
						newLine = []rune(editedText)
						newPos = len(newLine)
						s.buffer.Reset()
						s.multiline = false
					}
					processed = true
				}
				ok = processed
				return // Return directly
			}
		}
		if key == 24 {
			s.expectingCtrlE = true
			ok = true
			return newLine, newPos, ok
		} // Ctrl+X

		ok = processed
		return // Default handling
	})
}

// Run starts the interactive input loop for readline.
func (s *ReadlineSession) Run(ctx context.Context) error {
	closeDone := false
	defer func() {
		if !closeDone && s.reader != nil { // Check reader isn't nil
			s.reader.Close()
		}
	}()

	done := make(chan struct{})
	defer close(done)

	// Only use context cancellation - signals are now handled at the top level
	// We need a more specialized approach to context handling
	// SIGINT has two different behaviors depending on state:
	// 1. During response generation: Just interrupt the response
	// 2. During input: Either clear input or exit if line is empty
	
	// We don't need to close the reader on context cancellation,
	// as we'll handle that specially when it's a processing interrupt
	go func() {
		select {
		case <-ctx.Done():
			// CRITICAL FIX: Different handling for processing vs non-processing states
			
			// If we're processing a response (ResponseStateSubmitted or Streaming)
			// we want the cancellation to be handled in the ProcessFn section
			// and NOT exit the program
			// Get the current state and immediately handle differently based on processing state
			currentState := s.responeState
			s.SetStreaming(false) // First make sure we're not in streaming display mode
			
			if !currentState.IsProcessing() {
				// CASE 1: For non-processing states (like waiting for input):
				// terminate the session by closing the reader
				fmt.Fprintln(os.Stderr, "\nTerminating session...")
				if s.reader != nil {
					s.reader.Close() // Interrupt the blocking Readline call
					closeDone = true
				}
			} else {
				// CASE 2: For processing states (response generation):
				// DO NOT terminate the session, just interrupt the current response
				fmt.Fprintln(os.Stderr, ansiDimColor("\nInterrupting response generation only (press Ctrl+C twice rapidly to force exit)"))
				
				// Critical: explicitly set a flag to never propagate this cancellation
				// This ensures we keep the session alive even if the parent context is cancelled
				s.SetResponseState(ResponseStateSInterrupted)
				
				// Most important: return from this goroutine without doing anything else
				// Let the goroutine monitoring processingDone channel handle cleanup
				return
			}
		case <-done:
			// Loop finished normally
		}
	}()

	inTripleQuoteMode := false
	submitBuffer := false

	for {
		// Check context before blocking Readline call
		if ctx.Err() != nil {
			// If we're in a processing state, we want to return to the prompt
			// rather than exiting the program
			if s.responeState.IsProcessing() {
				// Reset state and continue
				s.SetResponseState(ResponseStateReady)
				fmt.Fprintln(os.Stderr, ansiDimColor("Response interrupted - ready for next input."))
				// Clear any pending operations
				s.buffer.Reset()
				inTripleQuoteMode = false
				s.multiline = false
				s.pendingSubmit = false
				s.expectingCtrlE = false
				continue
			}
			// Otherwise propagate the context error (when not in processing state)
			// This is typically when user presses Ctrl+C while in prompt
			return ctx.Err()
		}

		s.reader.SetPrompt(s.getPrompt())
		line, err := s.reader.Readline() // This blocks

		// Check context *immediately* after Readline returns
		// Skip context cancellation check here - handled above and also in each interrupt case
		// This is crucial to prevent Ctrl+C during response generation from exiting the program

		// --- Handle Readline Errors ---
		if errors.Is(err, readline.ErrInterrupt) { // Ctrl+C
			// If there's text on the line, just clear it instead of exiting
			if len(line) > 0 || s.buffer.Len() > 0 {
				// Print "Input cleared" without adding extra newlines
				fmt.Fprint(os.Stderr, "\r")                           // Carriage return to start of line
				fmt.Fprint(os.Stderr, ansiDimColor("Input cleared"))  // Show message
				fmt.Fprint(os.Stderr, "                         \r")  // Clear remainder of line and reset cursor
				
				// Reset state
				s.buffer.Reset()
				inTripleQuoteMode = false
				s.multiline = false
				s.pendingSubmit = false
				s.expectingCtrlE = false
				continue // Continue the loop
			} else {
				// Exit on Ctrl+C when line is empty (with minimal output)
				fmt.Fprint(os.Stderr, "\r")                       // Carriage return to start of line
				fmt.Fprint(os.Stderr, ansiDimColor("Exiting (Ctrl+C at prompt, or press again to force)"))    // Show clear message
				fmt.Fprint(os.Stderr, "                                      \r")   // Clear remainder of line

				// No need to close reader here, defer and cancellation goroutine handle it
				return err // Return the interrupt error
			}
		} else if errors.Is(err, io.EOF) { // Ctrl+D or reader closed due to context cancel
			if ctx.Err() != nil {
				return ctx.Err() // Prioritize context cancellation
			}
			// Handle Ctrl+D logic
			if s.buffer.Len() > 0 || len(line) > 0 {
				fmt.Fprintln(os.Stderr)
				if s.buffer.Len() == 0 && len(line) > 0 {
					s.buffer.WriteString(line)
				}
				submitBuffer = true // Submit remaining buffer on Ctrl+D
			} else {
				// Exit with Ctrl+D (minimal output)
				fmt.Fprint(os.Stderr, "\r")                       // Carriage return to start of line
				fmt.Fprint(os.Stderr, ansiDimColor("Exiting"))    // Show message without dots
				fmt.Fprint(os.Stderr, "                    \r")   // Clear remainder of line
				return err // Return EOF to signal exit
			}
		} else if err != nil {
			// Handle other potential readline errors
			return fmt.Errorf("readline error: %w", err)
		}

		// --- Process Input ---
		trimmedLine := strings.TrimSpace(line)
		isTripleQuoteMarker := trimmedLine == "\"\"\""

		if isTripleQuoteMarker {
			if inTripleQuoteMode {
				inTripleQuoteMode = false
				s.multiline = false
				s.pendingSubmit = false
				submitBuffer = true
			} else {
				if s.buffer.Len() > 0 {
					s.buffer.Reset()
				}
				inTripleQuoteMode = true
				s.multiline = true
				s.pendingSubmit = false
				continue
			}
		} else if len(line) == 0 {
			// Empty line handling
			// 1. If we're waiting for the second Enter press to submit (pendingSubmit)
			// 2. If we're in multiline mode with content
			if s.pendingSubmit || (s.multiline && s.buffer.Len() > 0) {
				submitBuffer = true
				s.pendingSubmit = false
			} else if inTripleQuoteMode {
				// Add a newline in triple quote mode
				s.buffer.WriteString("\n")
			} else {
				// Empty line at top level - just ignore
				continue
			}
		} else {
			// Special handling for the "exit" or "quit" commands
			lineTrimmed := strings.TrimSpace(line)
			if lineTrimmed == "exit" || lineTrimmed == "quit" {
				// User typed "exit" or "quit" - exit the program cleanly
				fmt.Fprint(os.Stderr, "\r")
				fmt.Fprint(os.Stderr, ansiDimColor("Exiting"))
				fmt.Fprint(os.Stderr, "                    \r")
				return io.EOF // Return EOF to signal exit
			}
			
			// Add non-empty line to buffer
			if s.buffer.Len() > 0 {
				s.buffer.WriteString("\n")
			}
			s.buffer.WriteString(line)

			// If not in triple quote mode, mark that we need a second Enter press
			if !inTripleQuoteMode {
				s.pendingSubmit = true
				// Don't enter multiline mode visually (no "..." prompt)
				// Just remember we're waiting for another Enter while keeping the standard prompt
				s.multiline = false

				// No need for a separate indicator since we now show ⏎ in the prompt
			}
		}

		// --- Handle Submission ---
		if submitBuffer {
			submitBuffer = false
			s.multiline = false
			s.pendingSubmit = false
			inputToProcess := s.buffer.String()
			s.buffer.Reset()

			if strings.TrimSpace(inputToProcess) != "" {
				// Clean the readline display before starting processing
				s.reader.Clean()
				
				// We'll use a different approach to handling interrupts that won't
				// interfere with the main program's signal handling
				
				// First, create a detached context that won't be cancelled by signals
				detachedCtx := context.Background()
				responseCtx, cancelResponse := context.WithCancel(detachedCtx)
				
				// Create a done channel and cancellation monitor flag
				processingDone := make(chan struct{})
				interrupted := false
				
				// Set state to indicate we're processing a response
				s.SetResponseState(ResponseStateStreaming)
				
				// Set up a goroutine to monitor the parent context
				// When it's cancelled (likely by Ctrl+C), we'll handle it specially
				go func() {
					select {
					case <-ctx.Done():
						// The main context was cancelled (likely by Ctrl+C)
						interrupted = true
						
						// Make the interruption visible with a small indicator
						fmt.Fprint(os.Stderr, ansiDimColor(" [interrupted] "))
						
						// Cancel our response context to stop generation
						cancelResponse()
						
						// Update state
						s.SetResponseState(ResponseStateSInterrupted)
						
					case <-processingDone:
						// Normal completion, do nothing
						return
					}
				}()
				
				// Process the input with our detached context
				// This will continue even if the parent context is cancelled
				processErr := s.config.ProcessFn(responseCtx, inputToProcess)
				
				// Signal that processing is done and clean up
				close(processingDone)
				cancelResponse()
				
				// Add newline after interrupted responses
				if interrupted {
					fmt.Fprintln(os.Stderr)
				}
				
				// Reset response state for next prompt
				// IMPORTANT FIX: Always reset to ready state after any interruption or cancellation
				// This ensures we return to prompt rather than exiting
				if processErr == nil || errors.Is(processErr, context.Canceled) || interrupted {
					s.SetResponseState(ResponseStateReady)
					// Clear interrupted flag if we need to handle another cycle
					if interrupted {
						fmt.Fprintln(os.Stderr, ansiDimColor("Ready for next input..."))
					}
				} else {
					s.SetResponseState(ResponseStateError)
				}
				
				// CRITICAL FIX: NEVER propagate context error after ProcessFn if we were in the stream
				// Skip this check entirely if we just handled an interruption
				// This prevents Ctrl+C during generation from exiting the program
				if interrupted {
					// Context was cancelled due to Ctrl+C, but we want to keep the session alive
					// Skip all context error checks and continue the loop
					continue
				} else if ctx.Err() != nil {
					// Only propagate context errors for non-interrupted cases
					return ctx.Err()
				}
				// Handle ProcessFn result
				if lastMsg, ok := processErr.(ErrUseLastMessage); ok {
					fmt.Fprintln(os.Stderr, ansiDimColor("Use Up Arrow to recall last input for editing."))
					s.lastInput = string(lastMsg)
				} else if processErr != nil && !errors.Is(processErr, ErrEmptyInput) {
					// Don't print error if context was cancelled during processing
					if !errors.Is(processErr, context.Canceled) {
						fmt.Fprintf(os.Stderr, "Processing error: %v\n", processErr)
					}
				} else if processErr == nil {
					// Save successful input to readline's history
					if err := s.reader.SaveHistory(inputToProcess); err != nil {
						fmt.Fprintf(os.Stderr, "Warning: failed to save history item: %v\n", err)
					}
					s.lastInput = inputToProcess
				}
			}
			inTripleQuoteMode = false
			s.expectingCtrlE = false
		} else if !s.isStreaming {
			s.reader.Refresh()
		}
	}
	// Unreachable in normal flow, loop exits via return
}

// editInEditor helper using Suspend/Resume.
func (s *ReadlineSession) editInEditor(currentContent string) (string, error) {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim"
	}
	tmpfile, err := os.CreateTemp("", "cgpt_edit_*.txt")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	defer os.Remove(tmpfile.Name())
	if _, err := tmpfile.WriteString(currentContent); err != nil {
		tmpfile.Close()
		return "", fmt.Errorf("write temp file: %w", err)
	}
	if err := tmpfile.Close(); err != nil {
		return "", fmt.Errorf("close temp file: %w", err)
	}

	cmd := exec.Command(editor, tmpfile.Name())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Readline Suspend/Resume might not be available or reliable
	// Suspend/Resume removed as they might not exist on the instance
	runErr := cmd.Run()
	// Resume removed

	if runErr != nil {
		return "", fmt.Errorf("editor command failed: %w", runErr)
	}
	contentBytes, err := os.ReadFile(tmpfile.Name())
	if err != nil {
		return "", fmt.Errorf("read temp file: %w", err)
	}
	return strings.TrimSuffix(string(contentBytes), "\n"), nil
}

func (r *ReadlineSession) SetResponseState(state ResponseState) {
	r.responeState = state
}

// GetHistory retrieves the current history from the readline instance.
func (s *ReadlineSession) GetHistory() []string {
	if s.reader != nil {
		// Readline doesn't expose its internal history slice directly.
		// We could potentially read it from the history file if needed,
		// but it's better if the main application manages the canonical history.
		// Returning the config's initial history as a placeholder.
		return s.config.ConversationHistory
	}
	return nil
}

// Expand tilde in file paths
func expandTilde(path string) (string, error) {
	// Check if path starts with ~/
	if strings.HasPrefix(path, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("get home directory: %w", err)
		}
		return strings.Replace(path, "~", homeDir, 1), nil
	}
	return path, nil
}

// Define LinePos struct (if needed for complex cursor logic)
// type LinePos struct { Line []rune; Pos int; Key rune }

// safeSpinnerWriter is a special io.Writer that prevents spinner
// output from interfering with normal output streams
type safeSpinnerWriter struct {
	mu sync.Mutex
	w  io.Writer
}

// Write implements io.Writer ensuring thread safety
func (sw *safeSpinnerWriter) Write(p []byte) (n int, err error) {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	return sw.w.Write(p)
}

// Define painter type
type PainterFunc func(line []rune, pos int) []rune

func (p PainterFunc) Paint(line []rune, pos int) []rune { return p(line, pos) }

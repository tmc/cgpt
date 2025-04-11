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
	"sync/atomic"
	"time"

	"github.com/chzyer/readline"
	"go.uber.org/zap"
	"golang.org/x/term"
)

// ReadlineSession implements an interactive terminal session using chzyer/readline.
type ReadlineSession struct {
	reader *readline.Instance
	config Config
	log    *zap.SugaredLogger // Added logger field

	// State management
	mu            sync.Mutex // Protects access to shared state below
	buffer        strings.Builder
	state         InteractiveState
	responseState atomic.Value // Use atomic.Value for ResponseState
	multiline     bool         // True when in explicit multiline mode (shows "..." prompt)
	pendingSubmit bool         // True when we've typed a line but need to press Enter again

	// Other state
	lastInput      string    // Track last successful input
	expectingCtrlE bool      // For Ctrl+X, Ctrl+E support
	interruptCount int       // Track consecutive Ctrl+C presses
	lastCtrlCTime  time.Time // Track time of last Ctrl+C press
	isStreaming    bool      // Track streaming state for prompt handling (Redundant? Use responseState)

	// Cancellation for ongoing processing
	currentProcessCancel context.CancelFunc // Stores the cancel func for the current ProcessFn call
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
	s.log.Warn("LoadHistory not fully implemented for readline session.")
	return nil
}

// SaveHistory is a stub implementation for ReadlineSession.
func (s *ReadlineSession) SaveHistory(filename string) error {
	// Readline handles history saving via HistoryFile config and Close().
	// This method could force a save if needed.
	s.log.Warn("SaveHistory not fully implemented for readline session.")
	return nil
}

// Quit closes the readline instance.
func (s *ReadlineSession) Quit() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.reader != nil {
		s.reader.Close() // Close should be thread-safe, but lock for consistency
	}
}

// NewSession creates a new interactive readline session.
func NewSession(cfg Config) (Session, error) {
	// Setup logger
	log := cfg.Logger
	if log == nil {
		log = zap.NewNop().Sugar() // Use Nop logger if none provided
	}
	log = log.Named("readline") // Add name to logger

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
		log.Warnf("Could not expand history file path '%s': %v", cfg.HistoryFile, err)
		historyPath = cfg.HistoryFile // Use original path as fallback
	}
	cfg.HistoryFile = historyPath // Update config with expanded path

	session := &ReadlineSession{
		config: cfg,
		log:    log,
		state:  StateSingleLine,
	}
	session.responseState.Store(ResponseStateReady) // Initialize atomic state

	listener := session.createListener()
	painter := PainterFunc(func(line []rune, pos int) []rune {
		// Don't show any hints while streaming responses
		currentState := session.responseState.Load().(ResponseState)
		if currentState == ResponseStateStreaming {
			return line
		}
		// Don't modify non-empty lines
		if len(line) > 0 {
			return line
		}
		// For empty lines when buffer is empty - no hints
		if session.buffer.Len() == 0 {
			return line
		}
		return line
	})

	// Determine if Stdin is a TTY
	stdinFile, stdinIsFile := cfg.Stdin.(*os.File)
	isTerminalFunc := func() bool {
		if stdinIsFile {
			if stdinFile == nil {
				// Check os.Stdout if stdin isn't a usable os.File
				if cfg.Stdout != nil {
					if f, ok := cfg.Stdout.(*os.File); ok {
						return term.IsTerminal(int(f.Fd()))
					}
				}
				return term.IsTerminal(int(os.Stdout.Fd())) // Fallback
			}
			return term.IsTerminal(int(stdinFile.Fd()))
		}
		// Fallback: Check Stdout TTY status
		if cfg.Stdout != nil {
			if f, ok := cfg.Stdout.(*os.File); ok {
				return term.IsTerminal(int(f.Fd()))
			}
		}
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
		Listener:          listener,                      // Custom key handling
		Painter:           painter,                       // Custom hint display
		// Stdin/out/err will be set below based on config/TTY
		ForceUseInteractive:    true, // Try interactive features even if TTY detection fails
		DisableAutoSaveHistory: true, // We handle saving manually
		FuncIsTerminal:         isTerminalFunc,
	}

	// Set Stdout/Stderr from config if provided
	if cfg.Stdout != nil {
		readlineConfig.Stdout = cfg.Stdout
		log.Debug("Using provided Stdout for readline")
	} else {
		log.Debug("Using default os.Stdout for readline")
	}
	if cfg.Stderr != nil {
		readlineConfig.Stderr = cfg.Stderr
		log.Debug("Using provided Stderr for readline")
	} else {
		log.Debug("Using default os.Stderr for readline")
	}

	// Handle Stdin specifically for TTYs
	if ttyFile, ok := cfg.Stdin.(*os.File); ok && isTerminalFunc() {
		readlineConfig.Stdin = ttyFile
		// If Stdin is the TTY, ensure Stdout/Stderr are also set, preferring config values
		if cfg.Stdout == nil {
			readlineConfig.Stdout = ttyFile
		}
		if cfg.Stderr == nil {
			readlineConfig.Stderr = ttyFile // Or default os.Stderr
		}
		log.Debugf("Using provided TTY handle (%s) for readline Stdin.", ttyFile.Name())
	} else {
		// Stdin is not a TTY (pipe?) or wasn't provided as *os.File
		readlineConfig.Stdin = cfg.Stdin // Use the provided reader directly
		log.Debug("Using provided non-TTY Stdin or default for readline.")
	}

	reader, err := readline.NewEx(readlineConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize readline: %w", err)
	}
	session.reader = reader

	log.Info("Readline session initialized")
	return session, nil
}

// SetResponseState updates the response state atomically and refreshes the prompt.
func (s *ReadlineSession) SetResponseState(state ResponseState) {
	// Store previous state first so we can detect transitions
	prevState := s.responseState.Load().(ResponseState)
	s.responseState.Store(state)
	s.log.Debugf("Response state changed from %s to: %s", prevState, state)

	// Refresh prompt when state changes, especially when becoming ready/interrupted
	if state == ResponseStateReady || state == ResponseStateSInterrupted {
		s.mu.Lock() // Lock needed for reader access
		if s.reader != nil {
			// Clear the current line completely for better visual transition
			if prevState.IsProcessing() && (state == ResponseStateSInterrupted || state == ResponseStateReady) {
				// Only clean line when we're transitioning from processing to interrupted/ready
				fmt.Fprint(s.reader.Config.Stderr, "\r\033[K") // Clear current line
			}
			
			s.reader.SetPrompt(s.getPrompt())
			s.reader.Clean() // Ensure line is clean
			s.reader.Refresh() // Redraw the line and prompt
		}
		s.mu.Unlock()
	}
}

// AddResponsePart prints the response part directly.
// Assumes it's called from the ProcessFn goroutine.
func (s *ReadlineSession) AddResponsePart(part string) {
	s.mu.Lock() // Lock for safe access to reader and its output writer
	defer s.mu.Unlock()
	
	// --- CRITICAL CHECK ---
	// Check state *before* writing. If interrupted or already ready for next prompt, discard the part.
	currentState := s.responseState.Load().(ResponseState)
	if currentState == ResponseStateSInterrupted || currentState == ResponseStateReady {
		s.log.Debugf("Discarding response part in state %s: %q", currentState, part)
		return // Do not print if interrupted or already finished
	}
	// --- END CHECK ---
	
	if s.reader == nil {
		fmt.Print(part) // Fallback if reader not initialized
		return
	}
	// Use Clean/Refresh to minimize prompt interference
	s.reader.Clean()
	// Use the configured Stdout writer for output
	fmt.Fprint(s.reader.Config.Stdout, part)
	s.reader.Refresh() // Refresh might redraw the prompt and current input line
}

// getPrompt returns the appropriate prompt based on the current state. Needs locking.
func (s *ReadlineSession) getPrompt() string {
	currentState := s.responseState.Load().(ResponseState)
	if currentState == ResponseStateStreaming || currentState == ResponseStateSubmitted {
		// Minimal prompt or empty while processing/streaming
		return "" // Or maybe a spinner indicator if desired?
	}
	if s.multiline {
		return s.config.AltPrompt
	}

	prompt := s.config.Prompt
	if s.pendingSubmit {
		prompt = strings.TrimSuffix(prompt, " ")
		return prompt + ansiDimColor("↵")
	}
	if prompt != "" && !strings.HasSuffix(prompt, " ") {
		prompt += " "
	}
	return prompt
}

// ansiDimColor applies dim ANSI color code.
func ansiDimColor(text string) string { return fmt.Sprintf("\x1b[90m%s\x1b[0m", text) }

// createListener returns a listener that handles specific key events. Needs locking.
func (s *ReadlineSession) createListener() readline.Listener {
	return readline.FuncListener(func(line []rune, pos int, key rune) (newLine []rune, newPos int, ok bool) {
		s.mu.Lock() // Lock for state access (lastInput, buffer, expectingCtrlE)
		defer s.mu.Unlock()

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
				s.log.Debug("Ctrl+X, Ctrl+E detected, launching editor")
				// Unlock before calling blocking editor function
				s.mu.Unlock()
				currentContent := s.buffer.String()
				if s.buffer.Len() > 0 && len(line) > 0 {
					currentContent += "\n"
				}
				currentContent += string(line)
				editedText, err := s.editInEditor(currentContent)
				// Re-lock after editor returns
				s.mu.Lock()

				if err != nil {
					s.log.Errorf("Editor error: %v", err)
					fmt.Fprintf(s.reader.Config.Stderr, "Editor error: %v\n", err)
					// No need to refresh here, readline will handle it
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
		if key == 24 { // Ctrl+X
			s.log.Debug("Ctrl+X detected, waiting for Ctrl+E")
			s.expectingCtrlE = true
			ok = true
			return newLine, newPos, ok
		}

		ok = processed
		return // Default handling
	})
}

// Run starts the interactive input loop for readline.
func (s *ReadlineSession) Run(ctx context.Context) error {
	defer func() {
		s.mu.Lock()
		// Ensure any ongoing process is cancelled on exit
		if s.currentProcessCancel != nil {
			s.log.Debug("Cancelling ongoing process on session exit")
			s.currentProcessCancel()
			s.currentProcessCancel = nil
		}
		// Close readline instance
		if s.reader != nil {
			s.log.Info("Closing readline instance")
			s.reader.Close()
			s.reader = nil // Prevent double close
		}
		s.mu.Unlock()
	}()

	// Goroutine to close readline instance when the main context is cancelled
	// This helps unblock the Readline() call if the program is terminated externally.
	contextDone := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			s.log.Infof("Main context cancelled (%v), closing readline.", ctx.Err())
			s.mu.Lock()
			if s.reader != nil {
				s.reader.Close() // Interrupt the blocking Readline call
			}
			s.mu.Unlock()
		case <-contextDone:
			// Normal exit, do nothing
		}
	}()
	defer close(contextDone) // Signal goroutine to exit

	inTripleQuoteMode := false
	submitBuffer := false

	for {
		// Check context before blocking Readline call - allows early exit if cancelled before prompt
		if ctx.Err() != nil {
			s.log.Infof("Context cancelled before Readline call: %v", ctx.Err())
			return ctx.Err()
		}

		s.mu.Lock() // Lock for state and reader access
		if s.reader == nil {
			s.mu.Unlock()
			s.log.Warn("Readline instance is nil, exiting Run loop.")
			return errors.New("readline instance closed unexpectedly")
		}
		currentReader := s.reader // Capture reader instance while locked
		currentReader.SetPrompt(s.getPrompt())
		s.mu.Unlock() // Unlock before blocking call

		line, err := currentReader.Readline() // This blocks

		// Check context *immediately* after Readline returns
		if ctx.Err() != nil {
			s.log.Infof("Context cancelled after Readline call: %v", ctx.Err())
			// Don't return immediately if we were processing, let interrupt handle it
			if s.responseState.Load().(ResponseState).IsProcessing() {
				s.log.Warn("Context cancelled during processing, relying on interrupt logic.")
				// The ErrInterrupt block should handle this if reader was closed by context goroutine
			} else {
				return ctx.Err()
			}
		}

		// --- Handle Readline Errors ---
		s.mu.Lock() // Lock for state modification and potential cancel call

		if errors.Is(err, readline.ErrInterrupt) { // Ctrl+C
			currentState := s.responseState.Load().(ResponseState)
			s.log.Debugf("Ctrl+C received, current state: %s", currentState)

			if currentState.IsProcessing() {
				// --- Interrupt Processing ---
				s.log.Info("Interrupting ongoing response generation.")
				if s.currentProcessCancel != nil {
					s.currentProcessCancel() // Signal the ProcessFn goroutine to stop
					s.currentProcessCancel = nil
				} else {
					s.log.Warn("Interrupt received while processing, but no cancel function found!")
				}
				s.SetResponseState(ResponseStateSInterrupted) // Update state (atomic)
				
				// Ensure we have a clean line before showing feedback
				fmt.Fprint(currentReader.Config.Stderr, "\r\033[K") // Clear current line completely
				// Provide feedback directly to stderr
				fmt.Fprintln(currentReader.Config.Stderr, ansiDimColor("[Interrupted]"))
				
				// Reset buffer/multiline state immediately
				s.buffer.Reset()
				inTripleQuoteMode = false
				s.multiline = false
				s.pendingSubmit = false
				s.expectingCtrlE = false
				
				// Force refresh the prompt after a short delay to ensure proper display
				go func() {
					time.Sleep(50 * time.Millisecond) // Small delay to let other goroutines complete
					s.mu.Lock()
					if s.reader != nil {
						s.reader.Clean()
						s.reader.Refresh()
					}
					s.mu.Unlock()
				}()
				
				s.mu.Unlock() // Unlock before continuing loop
				continue      // Go back to prompt
			} else {
				// --- Interrupt while Idle ---
				// If there's text on the line or in buffer, just clear it
				if len(line) > 0 || s.buffer.Len() > 0 {
					s.log.Debug("Ctrl+C clearing input line/buffer")
					// Print feedback directly
					fmt.Fprint(currentReader.Config.Stderr, "\r"+ansiDimColor("Input cleared")+"        \r")
					s.buffer.Reset()
					inTripleQuoteMode = false
					s.multiline = false
					s.pendingSubmit = false
					s.expectingCtrlE = false
					s.mu.Unlock()
					continue // Continue the loop
				} else {
					// Exit on Ctrl+C when line is empty
					s.log.Info("Exiting on Ctrl+C at empty prompt.")
					fmt.Fprint(currentReader.Config.Stderr, "\r"+ansiDimColor("Exiting...")+"        \r")
					s.mu.Unlock()
					return ErrInterrupted // Return specific interrupt error
				}
			}
		} else if errors.Is(err, io.EOF) { // Ctrl+D or reader closed
			s.log.Debug("EOF received from Readline")
			if ctx.Err() != nil {
				s.log.Infof("EOF received after context cancellation: %v", ctx.Err())
				s.mu.Unlock()
				return ctx.Err() // Prioritize context cancellation
			}
			// Handle Ctrl+D logic
			if s.buffer.Len() > 0 || len(line) > 0 {
				s.log.Debug("Ctrl+D submitting remaining buffer")
				fmt.Fprintln(currentReader.Config.Stderr) // Newline for clarity
				if s.buffer.Len() == 0 && len(line) > 0 {
					s.buffer.WriteString(line)
				}
				submitBuffer = true // Submit remaining buffer on Ctrl+D
				// Let the submit logic handle it below
			} else {
				// Exit cleanly on Ctrl+D at empty prompt
				s.log.Info("Exiting on Ctrl+D at empty prompt.")
				fmt.Fprint(currentReader.Config.Stderr, "\r"+ansiDimColor("Exiting...")+"        \r")
				s.mu.Unlock()
				return io.EOF // Return EOF to signal clean exit
			}
		} else if err != nil {
			// Handle other potential readline errors (e.g., reader closed unexpectedly)
			// Check if the error occurred *because* the context was cancelled
			if ctx.Err() != nil {
				s.log.Warnf("Readline error likely due to context cancellation: %v (context err: %v)", err, ctx.Err())
				s.mu.Unlock()
				return ctx.Err()
			}
			s.log.Errorf("Unexpected readline error: %v", err)
			s.mu.Unlock()
			return fmt.Errorf("readline error: %w", err)
		}

		// --- Process Input (still locked) ---
		trimmedLine := strings.TrimSpace(line)
		isTripleQuoteMarker := trimmedLine == "\"\"\""

		if isTripleQuoteMarker {
			if inTripleQuoteMode {
				s.log.Debug("Exiting triple-quote mode, submitting buffer.")
				inTripleQuoteMode = false
				s.multiline = false
				s.pendingSubmit = false
				submitBuffer = true
			} else {
				s.log.Debug("Entering triple-quote mode.")
				if s.buffer.Len() > 0 { // Clear buffer if entering """ after typing something
					s.buffer.Reset()
				}
				inTripleQuoteMode = true
				s.multiline = true
				s.pendingSubmit = false
				s.mu.Unlock() // Unlock before continue
				continue
			}
		} else if len(line) == 0 && !inTripleQuoteMode { // Empty line handling (only outside ```)
			if s.pendingSubmit {
				s.log.Debug("Empty line confirming submission (pendingSubmit=true).")
				submitBuffer = true
				s.pendingSubmit = false
			} else if s.multiline && s.buffer.Len() > 0 {
				// If we somehow got into multiline=true without pendingSubmit (e.g., editor)
				s.log.Debug("Empty line submitting multiline buffer.")
				submitBuffer = true
				s.multiline = false
			} else {
				// Empty line at top level - just ignore
				s.mu.Unlock() // Unlock before continue
				continue
			}
		} else { // Non-empty line or empty line inside ```
			// Special handling for the "exit" or "quit" commands outside ```
			lineTrimmed := strings.TrimSpace(line)
			if !inTripleQuoteMode && (lineTrimmed == "exit" || lineTrimmed == "quit") {
				s.log.Info("Exit command received, exiting.")
				fmt.Fprint(currentReader.Config.Stderr, "\r"+ansiDimColor("Exiting...")+"        \r")
				s.mu.Unlock()
				return io.EOF // Return EOF to signal exit
			}

			// Add line to buffer
			if s.buffer.Len() > 0 {
				s.buffer.WriteString("\n")
			}
			s.buffer.WriteString(line)

			// If not in triple quote mode, mark that we need a second Enter press
			if !inTripleQuoteMode {
				s.pendingSubmit = true
				s.multiline = false // Visually stay in single-line prompt mode
			} else {
				s.pendingSubmit = false // Don't require double enter in ``` mode
			}
		}

		// --- Handle Submission (still locked) ---
		if submitBuffer {
			submitBuffer = false
			s.multiline = false
			s.pendingSubmit = false
			inputToProcess := s.buffer.String()
			s.buffer.Reset() // Clear buffer *before* starting processing

			if strings.TrimSpace(inputToProcess) != "" {
				s.log.Debugf("Submitting input: %q", inputToProcess)
				s.SetResponseState(ResponseStateSubmitting) // Update state

				// Create context for this specific ProcessFn call
				// Derive from the main context to allow external cancellation
				responseCtx, cancel := context.WithCancel(ctx)
				s.currentProcessCancel = cancel // Store cancel func

				wg := sync.WaitGroup{}
				wg.Add(1)

				// Launch ProcessFn in a goroutine
				go func(procCtx context.Context, input string) {
					defer func() {
						// Cleanup in goroutine
						s.mu.Lock()
						if s.currentProcessCancel != nil {
							// If this goroutine finishes but cancel func still exists,
							// it means it wasn't cancelled externally. Nullify it.
							s.currentProcessCancel = nil
						}
						s.mu.Unlock()
						wg.Done()
						s.log.Debug("ProcessFn goroutine finished.")
					}()

					s.log.Debug("ProcessFn goroutine started.")
					processErr := s.config.ProcessFn(procCtx, input)
					finalState := ResponseStateReady // Assume success initially

					// Handle ProcessFn result
					if errors.Is(processErr, context.Canceled) || errors.Is(processErr, ErrInterrupted) {
						s.log.Infof("ProcessFn cancelled or interrupted: %v", processErr)
						finalState = ResponseStateSInterrupted
						// Don't save history on interrupt
					} else if lastMsg, ok := processErr.(ErrUseLastMessage); ok {
						s.log.Debugf("ProcessFn returned ErrUseLastMessage: %q", string(lastMsg))
						s.mu.Lock()
						s.lastInput = string(lastMsg)
						s.mu.Unlock()
						// Use stderr for user hints
						fmt.Fprintln(currentReader.Config.Stderr, ansiDimColor("Use Up Arrow to recall last input for editing."))
						// Don't save history for /last command
					} else if processErr != nil && !errors.Is(processErr, ErrEmptyInput) {
						s.log.Errorf("ProcessFn error: %v", processErr)
						fmt.Fprintf(currentReader.Config.Stderr, "Processing error: %v\n", processErr)
						finalState = ResponseStateError
						// Optionally save history even on error? Maybe not.
					} else if processErr == nil { // Success
						s.log.Debug("ProcessFn completed successfully.")
						s.mu.Lock()
						s.lastInput = input // Store last *successful* input
						// Save successful input to readline's history
						if err := currentReader.SaveHistory(input); err != nil {
							s.log.Warnf("Failed to save history item: %v", err)
						}
						s.mu.Unlock()
					}
					// Set final state (atomically)
					s.SetResponseState(finalState)
				}(responseCtx, inputToProcess)

				// --- End of submit block ---
			} else {
				s.log.Debug("Submit triggered with empty buffer, ignoring.")
			}
			inTripleQuoteMode = false
			s.expectingCtrlE = false
		}
		s.mu.Unlock() // Unlock before next loop iteration
	}
	// Unreachable in normal flow, loop exits via return
}

// editInEditor helper using Suspend/Resume. Needs locking by caller.
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
	cmd.Stdin = os.Stdin   // Use original OS Stdin
	cmd.Stdout = os.Stdout // Use original OS Stdout
	cmd.Stderr = os.Stderr // Use original OS Stderr

	s.log.Debug("Running external editor...")
	// Note: We don't have proper Suspend/Resume in the readline library,
	// so the terminal might be in a strange state during editing

	runErr := cmd.Run() // Run the editor command

	s.log.Debug("Finished external editor.")

	// Handle editor command errors after resuming readline
	if runErr != nil {
		return "", fmt.Errorf("editor command failed: %w", runErr)
	}

	// Read content after successful editor run
	contentBytes, err := os.ReadFile(tmpfile.Name())
	if err != nil {
		return "", fmt.Errorf("read temp file: %w", err)
	}
	return strings.TrimSuffix(string(contentBytes), "\n"), nil
}

// GetHistory retrieves the current history from the readline instance. Needs locking.
func (s *ReadlineSession) GetHistory() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.reader != nil {
		// Readline doesn't expose its internal history slice directly.
		// Returning the config's initial history as a placeholder.
		return s.config.ConversationHistory
	}
	return nil
}

// Expand tilde in file paths
func expandTilde(path string) (string, error) {
	if path == "" {
		return "", nil
	}
	if !strings.HasPrefix(path, "~") {
		return path, nil
	}
	// Handle "~" or "~/"
	sep := string(os.PathSeparator)
	if path == "~" || strings.HasPrefix(path, "~"+sep) {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("get home directory: %w", err)
		}
		if path == "~" {
			return homeDir, nil
		}
		return strings.Replace(path, "~", homeDir, 1), nil
	}
	// Handle other ~user cases maybe later if needed
	return path, nil // Return original if not recognized format
}

// Define painter type
type PainterFunc func(line []rune, pos int) []rune

func (p PainterFunc) Paint(line []rune, pos int) []rune { return p(line, pos) }

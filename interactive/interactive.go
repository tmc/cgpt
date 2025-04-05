package interactive

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/chzyer/readline"
	"golang.org/x/term"
)

var ErrEmptyInput = errors.New("empty input")

// ErrUseLastMessage is a special error type that carries the last message to edit
type ErrUseLastMessage string

func (e ErrUseLastMessage) Error() string {
	return "use last message: " + string(e)
}

// Config defines the required parameters for creating a new interactive session.
type Config struct {
	Prompt         string
	AltPrompt      string
	HistoryFile    string
	ProcessFn      func(ctx context.Context, input string) error
	Stdin          io.ReadCloser // Optional custom input source (e.g., /dev/tty)
	SingleLineHint string        // Placeholder text for single line mode
	MultiLineHint  string        // Placeholder text for multi-line mode
	LastInput      string        // Last user input for retrieval with up arrow
}

// InteractiveState tracks the current input mode
type InteractiveState int

const (
	StateSingleLine InteractiveState = iota
	StateMultiLine
)

// InteractiveSession implements an interactive terminal session
type InteractiveSession struct {
	reader         *readline.Instance
	config         Config
	buffer         strings.Builder
	state          InteractiveState
	multiline      bool
	lastInput      string // Track last successful input
	expectingCtrlE bool   // For Ctrl+X, Ctrl+E support
	interruptCount int    // Track consecutive Ctrl+C presses
	lastCtrlCTime  time.Time // Track time of last Ctrl+C press

	linePos LinePos // Current line position
}

// LinePos represents the current line and position in the input buffer
type LinePos struct {
	Line []rune // Current line content
	Pos  int    // Current cursor position
	Key  rune   // Last key pressed
}

var (
	defaultSingleLineHint = "Enter your prompt to (press Enter twice submit, or \"\"\" for multi-line)"
	defaultMultiLineHint  = "(\"\"\" to end multi-line input, or Ctrl+D to submit)"
)

// NewSession creates a new interactive session
func NewSession(cfg Config) (*InteractiveSession, error) {
	if cfg.SingleLineHint == "" {
		cfg.SingleLineHint = defaultSingleLineHint
	}
	if cfg.MultiLineHint == "" {
		cfg.MultiLineHint = defaultMultiLineHint
	}

	session := &InteractiveSession{
		config:    cfg,
		state:     StateSingleLine,
		multiline: false,
		lastInput: cfg.LastInput,
	}

	// Create the listener
	listener := session.createListener()
	painter := PainterFunc(session.painter)
	readlineConfig := &readline.Config{
		Prompt:              cfg.Prompt, // Base prompt
		InterruptPrompt:     "^C",
		EOFPrompt:           "exit",
		HistoryFile:         cfg.HistoryFile,
		HistoryLimit:        10000,
		HistorySearchFold:   true,
		AutoComplete:        readline.NewPrefixCompleter(),
		Stdin:               cfg.Stdin,
		Listener:            listener,
		Painter:             painter,
		ForceUseInteractive: true,
		FuncIsTerminal: func() bool {
			return term.IsTerminal(int(os.Stdout.Fd()))
		},
	}

	reader, err := readline.NewEx(readlineConfig)
	if err != nil {
		return nil, err
	}
	session.reader = reader

	return session, nil
}

// SetLastInput sets the last input for retrieval with up arrow
func (s *InteractiveSession) SetLastInput(input string) {
	s.lastInput = input
}

// SetStreaming controls visibility of prompt during streaming
// For the readline implementation, we don't hide the prompt, but this
// ensures we implement the Session interface
func (s *InteractiveSession) SetStreaming(streaming bool) {
	// Nothing to do here for readline - we keep the prompt visible
}

func (session *InteractiveSession) painter(line []rune, pos int) []rune {
	if pos == 0 {
		return []rune(session.getPlaceHolder())
	}
	return line
}

type PainterFunc func(line []rune, pos int) []rune

func (p PainterFunc) Paint(line []rune, pos int) []rune {
	return p(line, pos)
}

// getPrompt returns the appropriate prompt based on the current state
func (s *InteractiveSession) getPrompt() string {
	if s.multiline {
		return s.config.AltPrompt
	}
	return s.config.Prompt
}

// decoratePrompt adds a hint to the prompt
func (s *InteractiveSession) getPlaceHolder() string {
	return fmt.Sprint(
		"\x1b[38;5;245m",
		s.placeholder(),
		ansiMoveLeft(len(s.placeholder())), // move len(s.placeholder()) spaces to the left
		"\x1b[0m",
	)
}

// createListener returns a listener that handles input behavior
func (s *InteractiveSession) createListener() readline.Listener {
	return readline.FuncListener(func(line []rune, pos int, key rune) (newLine []rune, newPos int, ok bool) {
		// Default: Let readline handle it
		processed := false
		newLine = line
		newPos = pos
		
		// Handle Up Arrow (for retrieving last input)
		if key == readline.CharPrev && len(line) == 0 && s.buffer.Len() == 0 && s.lastInput != "" {
			// Only handle up arrow when at empty prompt
			newLine = []rune(s.lastInput)
			newPos = len(newLine)
			processed = true
			ok = processed
			return newLine, newPos, ok
		}
		
		// --- Ctrl+X, Ctrl+E Handling ---
		if s.expectingCtrlE {
			s.expectingCtrlE = false // Reset flag regardless of next key
			if key == 5 {            // Ctrl+E (5)
				fmt.Fprintln(os.Stderr, "\nEditing in $EDITOR...") // Give feedback
				editedLine, err := s.editInEditor(string(line))
				if err != nil {
					fmt.Fprintf(os.Stderr, "Editor error: %v\n", err)
					// Keep original line on error
					s.reader.Refresh()
				} else {
					// Replace buffer with edited content
					newLine = []rune(editedLine)
					newPos = len(newLine) // Place cursor at the end
					processed = true      // We handled this
				}
				ok = processed
				return newLine, newPos, ok
			}
			// If it wasn't Ctrl+E, fall through to process 'key' normally
		}

		if key == 24 { // Ctrl+X (24)
			s.expectingCtrlE = true
			ok = false // Don't process Ctrl+X itself, wait for next key
			return newLine, newPos, ok
		}

		// Allow normal processing by readline
		ok = processed
		return newLine, newPos, ok
	})
}

// Run starts the interactive input loop
func (s *InteractiveSession) Run(ctx context.Context) error {
	// We'll close the reader at the end of this function
	// But we need to do it in a more controlled way for Ctrl+C handling
	closeDone := false
	defer func() {
		if !closeDone {
			s.reader.Close()
		}
	}()

	// Set up a channel to listen for context cancellation
	done := make(chan struct{})
	defer close(done)
	
	// Handle cancellation from the main context
	go func() {
		select {
		case <-ctx.Done():
			// When context is canceled, stay quiet - the completion service
			// will handle messaging to the user about cancellation
			// We don't close the reader here directly as it might cause a race condition
		case <-done:
			// We're done, no need to do anything
		}
	}()

	inTripleQuoteMode := false
	submitBuffer := false
	s.lastInput = ""

	for {
		// Check if context was canceled
		if ctx.Err() != nil {
			// Context has been canceled, exit gracefully
			return ctx.Err()
		}
		
		s.reader.SetPrompt(s.getPrompt())
		line, err := s.reader.Readline()

		// Handle readline errors
		if err == readline.ErrInterrupt {
			fmt.Fprintln(os.Stderr)
			
			// Handle consecutive Ctrl+C presses
			now := time.Now()
			if now.Sub(s.lastCtrlCTime) < 2*time.Second {
				// Less than 2 seconds since last Ctrl+C, increment counter
				s.interruptCount++
			} else {
				// More than 2 seconds since last Ctrl+C, reset counter
				s.interruptCount = 1
			}
			s.lastCtrlCTime = now
			
			// If this is the second consecutive Ctrl+C, exit
			if s.interruptCount >= 2 && s.buffer.Len() == 0 && len(line) == 0 {
				fmt.Fprintln(os.Stderr, "\033[38;5;240mExiting...\033[0m")
				closeDone = true
				s.reader.Close()
				break
			}
			
			// Clear current input
			s.buffer.Reset()
			inTripleQuoteMode = false
			s.multiline = false
			s.expectingCtrlE = false
			
			// Provide user feedback
			if len(line) > 0 {
				fmt.Fprintln(os.Stderr, "\033[38;5;240mInput cleared. Type to continue or press Ctrl+D to exit.\033[0m")
			} else {
				fmt.Fprintln(os.Stderr, "\033[38;5;240mPress Ctrl+C again to exit, or continue typing.\033[0m")
			}
			continue
		} else if err == io.EOF {
			if len(line) > 0 || s.buffer.Len() > 0 {
				fmt.Fprintln(os.Stderr)
				// If we have content in line but not in buffer, add it first
				if len(line) > 0 && s.buffer.Len() == 0 {
					s.buffer.WriteString(line)
				}
				submitBuffer = true // Process remaining buffer on Ctrl+D
			} else {
				// Clean EOF handling when buffer is empty
				fmt.Fprintln(os.Stderr, "\033[38;5;240mExiting...\033[0m")
				break
			}
		} else if err != nil {
			closeDone = true // Mark that we're handling the close ourselves
			s.reader.Close() // Ensure reader is closed before returning
			return err
		}

		// Logic based on input
		trimmedLine := strings.TrimSpace(line)
		isTripleQuote := trimmedLine == "\"\"\""

		if isTripleQuote {
			if inTripleQuoteMode {
				inTripleQuoteMode = false
				s.multiline = false
				submitBuffer = true // Submit accumulated buffer
			} else {
				inTripleQuoteMode = true
				s.multiline = true
				continue // Skip adding the opening quote marker
			}
		} else if len(line) == 0 { // Empty line handling
			if s.buffer.Len() == 0 {
				continue // Ignore empty lines on empty buffer
			}
			if inTripleQuoteMode {
				// Add literal newline inside triple quotes
				s.buffer.WriteString("\n")
			} else {
				// Empty line outside triple quotes submits
				submitBuffer = true
			}
		} else { // Normal line with content
			if s.buffer.Len() > 0 {
				s.buffer.WriteString("\n") // Add newline separator
			}
			s.buffer.WriteString(line) // Add the actual content

			// Only switch to multi-line state if in triple quote mode
			// This ensures we don't show ... prompt unless explicitly in triple quote mode
			if inTripleQuoteMode && !s.multiline {
				s.multiline = true
			}
		}

		// Handle submission
		if submitBuffer {
			submitBuffer = false
			input := s.buffer.String() // Get potentially multi-line input

			if strings.TrimSpace(input) != "" { // Only process non-blank input
				err := s.config.ProcessFn(ctx, input)
				
				// Handle special error types
				if lastMsg, ok := err.(ErrUseLastMessage); ok {
					// Special case for edit last message command
					fmt.Fprintln(os.Stderr, "\033[38;5;240mRetrieving last message for editing...\033[0m")
					// There's no direct way to set the buffer in readline, 
					// but we can use this workaround - set it as the lastInput
					// which will be accessible via up arrow
					s.lastInput = string(lastMsg)
					fmt.Fprintln(os.Stderr, "\033[38;5;240mPress Up Arrow to access the previous input.\033[0m")
				} else if err != nil && err != ErrEmptyInput {
					fmt.Fprintf(os.Stderr, "Processing error: %v\n", err)
				} else {
					// Success - save to history
					s.reader.SaveHistory(input)
					s.lastInput = input // Mark that we just had a successful input
				}
			}

			// Reset for next input
			s.buffer.Reset()
			inTripleQuoteMode = false
			s.multiline = false
			s.expectingCtrlE = false
		}
	}

	return nil
}

func (s *InteractiveSession) placeholder() string {
	if s.multiline {
		return s.config.MultiLineHint
	}
	return s.config.SingleLineHint
}

func ansiMoveLeft(n int) string {
	return fmt.Sprintf("\x1b[%dD", n)
}

// editInEditor opens an external editor to edit the current input
func (s *InteractiveSession) editInEditor(line string) (string, error) {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim" // Default to vim if $EDITOR is not set
	}

	tmpfile, err := os.CreateTemp("", "cgpt_input_*.txt")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpfile.Name())

	// Write current buffer content to the file
	if _, err := tmpfile.WriteString(line); err != nil {
		tmpfile.Close()
		return "", fmt.Errorf("failed to write to temp file: %w", err)
	}
	if err := tmpfile.Close(); err != nil {
		return "", fmt.Errorf("failed to close temp file: %w", err)
	}

	cmd := exec.Command(editor, tmpfile.Name())

	// Ensure editor uses the same terminal as readline if possible
	var stdinFile *os.File = os.Stdin
	var stdoutFile *os.File = os.Stdout
	var stderrFile *os.File = os.Stderr

	if s.reader.Config.Stdin != nil {
		if f, ok := s.reader.Config.Stdin.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
			stdinFile = f
		}
	}

	cmd.Stdin = stdinFile
	cmd.Stdout = stdoutFile
	cmd.Stderr = stderrFile

	// Temporarily exit raw mode for the editor
	s.reader.Terminal.ExitRawMode()
	err = cmd.Run()
	// Re-enter raw mode immediately after editor exits
	s.reader.Terminal.EnterRawMode()

	if err != nil {
		return "", fmt.Errorf("editor command failed: %w", err)
	}

	content, err := os.ReadFile(tmpfile.Name())
	if err != nil {
		return "", fmt.Errorf("failed to read temp file: %w", err)
	}

	// Remove trailing newline often added by editors
	return strings.TrimSuffix(string(content), "\n"), nil
}

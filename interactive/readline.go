//go:build !js

package interactive

import (
	"context"
	"errors" // Import errors package
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath" // Import filepath
	"strings"
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
	multiline      bool
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
func NewSession(cfg Config) (*ReadlineSession, error) {
	if cfg.SingleLineHint == "" {
		cfg.SingleLineHint = DefaultSingleLineHint
	}
	if cfg.MultiLineHint == "" {
		cfg.MultiLineHint = DefaultMultiLineHint
	}

	// Expand tilde for history file path
	historyPath, err := expandTilde(cfg.HistoryFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not expand history file path '%s': %v\n", cfg.HistoryFile, err)
		historyPath = cfg.HistoryFile // Use original path as fallback
	}
	cfg.HistoryFile = historyPath // Update config with expanded path

	session := &ReadlineSession{
		config:    cfg,
		state:     StateSingleLine,
		multiline: false,
	}

	listener := session.createListener()
	painter := PainterFunc(func(line []rune, pos int) []rune {
		// Painter is called frequently, keep it fast.
		// Only show hint on empty line at pos 0 when NOT streaming.
		if len(line) == 0 && pos == 0 && !session.isStreaming {
			// Removed submitReady check
			return []rune(session.getPlaceHolder())
		}
		return line // Return original line otherwise
	})

	// Determine if Stdin is a TTY
	stdinFile, stdinIsFile := cfg.Stdin.(*os.File)
	isTerminalFunc := func() bool {
		if stdinIsFile {
			return term.IsTerminal(int(stdinFile.Fd()))
		}
		// Fallback: Check if os.Stdout is a TTY, assuming it's the interactive one
		return term.IsTerminal(int(os.Stdout.Fd()))
	}

	readlineConfig := &readline.Config{
		Prompt:                 cfg.Prompt, // Base prompt
		InterruptPrompt:        "^C",       // Prompt shown after Ctrl+C clears line
		EOFPrompt:              "exit",     // Shown on Ctrl+D exit
		HistoryFile:            cfg.HistoryFile,
		HistoryLimit:           10000,
		HistorySearchFold:      true,                          // Case-insensitive history search
		AutoComplete:           readline.NewPrefixCompleter(), // Basic prefix completer
		Stdin:                  cfg.Stdin,
		Listener:               listener, // Custom key handling
		Painter:                painter,  // Custom hint display
		ForceUseInteractive:    true,     // Try interactive features even if TTY detection fails
		DisableAutoSaveHistory: true,     // We handle saving manually
		FuncIsTerminal:         isTerminalFunc,
		// Consider adding other readline config options if needed
	}

	reader, err := readline.NewEx(readlineConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize readline: %w", err)
	}
	session.reader = reader

	return session, nil
}

// SetLastInput sets the last input for retrieval with up arrow.
func (s *ReadlineSession) SetLastInput(input string) {
	s.lastInput = input
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
	return s.config.Prompt
}

// getPlaceHolder returns the hint text with ANSI codes for dim color.
func (s *ReadlineSession) getPlaceHolder() string {
	hint := s.config.SingleLineHint
	if s.multiline {
		hint = s.config.MultiLineHint
	}
	return ansiDimColor(hint)
}

// ansiDimColor applies dim ANSI color code.
func ansiDimColor(text string) string { return fmt.Sprintf("\x1b[90m%s\x1b[0m", text) }

// createListener returns a listener that handles specific key events.
func (s *ReadlineSession) createListener() readline.Listener {
	return readline.FuncListener(func(line []rune, pos int, key rune) (newLine []rune, newPos int, ok bool) {
		processed := false
		newLine = line
		newPos = pos

		// Removed submitReady usage

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
		if !closeDone {
			s.reader.Close()
		}
	}()
	done := make(chan struct{})
	defer close(done)
	go func() { // Context cancellation handling
		select {
		case <-ctx.Done():
			if s.reader != nil {
				pid := os.Getpid()
				p, _ := os.FindProcess(pid)
				if p != nil {
					_ = p.Signal(os.Interrupt)
				}
			}
		case <-done:
		}
	}()

	inTripleQuoteMode := false
	submitBuffer := false
	submitReady := false // Local variable for double-enter logic

	for {
		s.reader.SetPrompt(s.getPrompt()) // Update prompt
		line, err := s.reader.Readline()

		// --- Handle Errors ---
		if errors.Is(err, readline.ErrInterrupt) { // Ctrl+C
			fmt.Fprintln(os.Stderr) // Newline
			// TODO: If streaming/processing, attempt context cancellation here
			now := time.Now()
			if now.Sub(s.lastCtrlCTime) < 1*time.Second && s.interruptCount > 0 {
				fmt.Fprintln(os.Stderr, ansiDimColor("Exiting..."))
				closeDone = true
				s.reader.Close()
				return err
			}
			s.interruptCount++
			s.lastCtrlCTime = now
			s.buffer.Reset()
			inTripleQuoteMode = false
			s.multiline = false
			s.expectingCtrlE = false
			submitReady = false // Reset local submitReady
			msg := "Input cleared."
			if len(line) == 0 {
				msg = "Press Ctrl+C again quickly to exit."
			}
			fmt.Fprintln(os.Stderr, ansiDimColor(msg))
			continue
		} else if errors.Is(err, io.EOF) { // Ctrl+D
			if s.buffer.Len() > 0 || len(line) > 0 {
				fmt.Fprintln(os.Stderr)
				if s.buffer.Len() == 0 && len(line) > 0 {
					s.buffer.WriteString(line)
				}
				submitBuffer = true
			} else {
				fmt.Fprintln(os.Stderr, ansiDimColor("Exiting..."))
				closeDone = true
				s.reader.Close()
				return err
			}
		} else if err != nil {
			closeDone = true
			s.reader.Close()
			return fmt.Errorf("readline error: %w", err)
		}

		if ctx.Err() != nil {
			return ctx.Err()
		} // Check context

		// --- Process Input ---
		trimmedLine := strings.TrimSpace(line)
		isTripleQuoteMarker := trimmedLine == "\"\"\""

		if isTripleQuoteMarker {
			if inTripleQuoteMode {
				inTripleQuoteMode = false
				s.multiline = false
				submitBuffer = true
				submitReady = false // Reset local submitReady
			} else {
				if s.buffer.Len() > 0 {
					s.buffer.Reset()
				}
				inTripleQuoteMode = true
				s.multiline = true
				submitReady = false // Reset local submitReady
				continue
			}
		} else if len(line) == 0 {
			if !inTripleQuoteMode && s.buffer.Len() > 0 {
				if submitReady {
					submitBuffer = true
					submitReady = false // Reset local submitReady
				} else {
					submitReady = true // Set local submitReady
					s.reader.Refresh()
					continue
				}
			} else if inTripleQuoteMode {
				s.buffer.WriteString("\n")
				submitReady = false // Reset local submitReady
			} else {
				submitReady = false // Reset local submitReady
				continue
			}
		} else {
			submitReady = false // Reset local submitReady
			if s.buffer.Len() > 0 {
				s.buffer.WriteString("\n")
			}
			s.buffer.WriteString(line)
			if !s.multiline && !isTripleQuoteMarker {
				s.multiline = true
			}
		}

		// --- Handle Submission ---
		if submitBuffer {
			submitBuffer = false
			s.multiline = false
			submitReady = false // Reset local submitReady
			inputToProcess := s.buffer.String()
			s.buffer.Reset()

			if strings.TrimSpace(inputToProcess) != "" {
				// Clean line *before* calling potentially blocking ProcessFn
				s.reader.Clean()
				processErr := s.config.ProcessFn(ctx, inputToProcess)

				if lastMsg, ok := processErr.(ErrUseLastMessage); ok {
					fmt.Fprintln(os.Stderr, ansiDimColor("Use Up Arrow to recall last input for editing."))
					s.lastInput = string(lastMsg)
				} else if processErr != nil && !errors.Is(processErr, ErrEmptyInput) && !errors.Is(processErr, context.Canceled) {
					fmt.Fprintf(os.Stderr, "Processing error: %v\n", processErr)
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
		} // Refresh prompt if not submitting/streaming
	}
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
	if path == "" || path[0] != '~' {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	return filepath.Join(home, path[1:]), nil
}

// Define LinePos struct (if needed for complex cursor logic)
// type LinePos struct { Line []rune; Pos int; Key rune }

// Define painter type
type PainterFunc func(line []rune, pos int) []rune

func (p PainterFunc) Paint(line []rune, pos int) []rune { return p(line, pos) }

func (s *ReadlineSession) SetResponeState(state ResponseState) {}

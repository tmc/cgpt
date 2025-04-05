package interactive

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

type Session interface {
	Run(ctx context.Context) error
	SetStreaming(streaming bool) // Control visibility of prompt during streaming
}

// BubbleSession implements an interactive terminal session using Bubble Tea
type BubbleSession struct {
	config         Config
	buffer         strings.Builder
	multiline      bool
	lastInput      string
	program        *tea.Program
	interruptCount int
	lastCtrlCTime  time.Time
	isStreaming    bool // Track if streaming is currently in progress
}

// bubbleModel is the Bubble Tea model for our interactive session
type bubbleModel struct {
	session      *BubbleSession
	textInput    textinput.Model
	ctx          context.Context
	quitting     bool
	tripleMark   bool
	isProcessing bool // Tracks if currently processing a request (streaming)
}

func (m bubbleModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m bubbleModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case streamingMsg:
		// Update processing state based on message
		m.isProcessing = bool(msg)
		if m.isProcessing {
			// Disable input during processing
			m.textInput.Blur()
		} else {
			// Re-enable input after processing
			m.textInput.Focus()
		}
		return m, nil
	case tea.KeyMsg:
		// Handle up arrow for last message
		if msg.String() == "up" && m.textInput.Value() == "" && m.session.buffer.Len() == 0 && m.session.lastInput != "" {
			m.textInput.SetValue(m.session.lastInput)
			return m, nil
		}
		
		switch msg.String() {
		case "ctrl+c":
			// Handle consecutive Ctrl+C presses
			now := time.Now()
			if now.Sub(m.session.lastCtrlCTime) < 2*time.Second {
				// Less than 2 seconds since last Ctrl+C, increment counter
				m.session.interruptCount++
			} else {
				// More than 2 seconds since last Ctrl+C, reset counter
				m.session.interruptCount = 1
			}
			m.session.lastCtrlCTime = now
			
			// If this is the second consecutive Ctrl+C, exit
			if m.session.interruptCount >= 2 && m.session.buffer.Len() == 0 && m.textInput.Value() == "" {
				fmt.Fprintln(os.Stderr, "\033[38;5;240mExiting...\033[0m")
				m.quitting = true
				return m, tea.Quit
			}
			
			// Reset buffers and state
			m.session.buffer.Reset()
			m.session.multiline = false
			m.tripleMark = false
			m.textInput.SetValue("")
			
			// Provide feedback to the user
			if m.textInput.Value() != "" {
				fmt.Fprintln(os.Stderr, "\033[38;5;240mInput cleared. Type to continue or press Ctrl+D to exit.\033[0m")
			} else {
				fmt.Fprintln(os.Stderr, "\033[38;5;240mPress Ctrl+C again to exit, or continue typing.\033[0m")
			}
			return m, nil

		case "ctrl+d":
			if m.textInput.Value() == "" && m.session.buffer.Len() > 0 {
				// Submit buffer on Ctrl+D when line is empty and buffer has content
				input := m.session.buffer.String()
				if strings.TrimSpace(input) != "" {
					err := m.session.config.ProcessFn(m.ctx, input)
					
					// Handle special error types
					if lastMsg, ok := err.(ErrUseLastMessage); ok {
						// Special case for edit last message command
						fmt.Fprintln(os.Stderr, "\033[38;5;240mRetrieving last message for editing...\033[0m")
						m.textInput.SetValue(string(lastMsg))
					} else if err != nil && err != ErrEmptyInput {
						fmt.Fprintf(os.Stderr, "Processing error: %v\n", err)
					} else {
						m.session.lastInput = input
					}
				}
				m.session.buffer.Reset()
				m.tripleMark = false
				m.session.multiline = false
				return m, nil
			} else if m.textInput.Value() == "" && m.session.buffer.Len() == 0 {
				// Exit on Ctrl+D when everything is empty
				fmt.Fprintln(os.Stderr, "\033[38;5;240mExiting...\033[0m")
				m.quitting = true
				return m, tea.Quit
			} else if m.textInput.Value() != "" {
				// When there's content in current line but user presses Ctrl+D, submit it
				value := m.textInput.Value()
				if strings.TrimSpace(value) != "" {
					// Add content to buffer
					if m.session.buffer.Len() > 0 {
						m.session.buffer.WriteString("\n")
					}
					m.session.buffer.WriteString(value)
					
					// Process the buffer
					input := m.session.buffer.String()
					err := m.session.config.ProcessFn(m.ctx, input)
					
					// Handle special error types
					if lastMsg, ok := err.(ErrUseLastMessage); ok {
						// Special case for edit last message command
						fmt.Fprintln(os.Stderr, "\033[38;5;240mRetrieving last message for editing...\033[0m")
						m.textInput.SetValue(string(lastMsg))
					} else if err != nil && err != ErrEmptyInput {
						fmt.Fprintf(os.Stderr, "Processing error: %v\n", err)
					} else {
						m.session.lastInput = input
					}
					
					// Reset for next input
					m.session.buffer.Reset()
					m.tripleMark = false
					m.session.multiline = false
					m.textInput.SetValue("")
				}
				return m, nil
			}

		case "ctrl+x":
			// Start the Ctrl+X, Ctrl+E sequence for external editor
			if m.textInput.Value() != "" {
				if editedText, err := m.session.editInEditor(m.textInput.Value()); err == nil {
					m.textInput.SetValue(editedText)
				}
			}
			return m, nil

		case "enter":
			value := m.textInput.Value()
			trimmedValue := strings.TrimSpace(value)

			// Handle triple quotes
			if trimmedValue == "\"\"\"" {
				if m.tripleMark {
					// End triple quote mode and submit
					m.tripleMark = false
					m.session.multiline = false
					input := m.session.buffer.String()
					
					if strings.TrimSpace(input) != "" {
						err := m.session.config.ProcessFn(m.ctx, input)
						
						// Handle special error types
						if lastMsg, ok := err.(ErrUseLastMessage); ok {
							// Special case for edit last message command
							fmt.Fprintln(os.Stderr, "\033[38;5;240mRetrieving last message for editing...\033[0m")
							m.textInput.SetValue(string(lastMsg))
						} else if err != nil && err != ErrEmptyInput {
							fmt.Fprintf(os.Stderr, "Processing error: %v\n", err)
						} else {
							m.session.lastInput = input
						}
					}
					m.session.buffer.Reset()
				} else {
					// Start triple quote mode
					m.tripleMark = true
					m.session.multiline = true
				}
				m.textInput.SetValue("")
				return m, nil
			}

			// Handle empty line (double enter to submit outside triple quote mode)
			if trimmedValue == "" {
				if m.tripleMark {
					// In triple quote mode, add literal newline
					if m.session.buffer.Len() > 0 {
						m.session.buffer.WriteString("\n")
					}
				} else if m.session.buffer.Len() > 0 {
					// Submit on empty line when not in triple quote mode
					input := m.session.buffer.String()
					if strings.TrimSpace(input) != "" {
						err := m.session.config.ProcessFn(m.ctx, input)
						
						// Handle special error types
						if lastMsg, ok := err.(ErrUseLastMessage); ok {
							// Special case for edit last message command
							fmt.Fprintln(os.Stderr, "\033[38;5;240mRetrieving last message for editing...\033[0m")
							m.textInput.SetValue(string(lastMsg))
						} else if err != nil && err != ErrEmptyInput {
							fmt.Fprintf(os.Stderr, "Processing error: %v\n", err)
						} else {
							m.session.lastInput = input
						}
					}
					m.session.buffer.Reset()
				}
				m.textInput.SetValue("")
				return m, nil
			}

			// Add content to buffer
			if m.session.buffer.Len() > 0 {
				m.session.buffer.WriteString("\n")
			}
			m.session.buffer.WriteString(value)
			m.textInput.SetValue("")
		}
	}

	// Update the prompt based on the state
	if m.session.multiline {
		m.textInput.Placeholder = m.session.config.MultiLineHint
		m.textInput.PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("213"))
		m.textInput.Prompt = m.session.config.AltPrompt
	} else {
		m.textInput.Placeholder = m.session.config.SingleLineHint
		m.textInput.PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("86"))
		m.textInput.Prompt = m.session.config.Prompt
	}

	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m bubbleModel) View() string {
	if m.quitting {
		return ""
	}
	
	// If processing, hide the input
	if m.isProcessing {
		return "" // Hide the prompt completely while processing
	}
	
	return m.textInput.View()
}

// NewBubbleSession creates a new interactive session using BubbleTea
func NewBubbleSession(cfg Config) (*BubbleSession, error) {
	if cfg.SingleLineHint == "" {
		cfg.SingleLineHint = defaultSingleLineHint
	}
	if cfg.MultiLineHint == "" {
		cfg.MultiLineHint = defaultMultiLineHint
	}

	session := &BubbleSession{
		config:    cfg,
		multiline: false,
		lastInput: cfg.LastInput,
	}

	return session, nil
}

// SetLastInput sets the last input for retrieval
func (s *BubbleSession) SetLastInput(input string) {
	s.lastInput = input
}

// SetStreaming sets the streaming state
func (s *BubbleSession) SetStreaming(streaming bool) {
	s.isStreaming = streaming
	// If we have a running program, update the processing state
	if s.program != nil {
		s.program.Send(streamingMsg(streaming))
	}
}

// streamingMsg is a message to update the streaming state
type streamingMsg bool

// Run starts the interactive input loop
func (s *BubbleSession) Run(ctx context.Context) error {
	ti := textinput.New()
	ti.Placeholder = s.config.SingleLineHint
	ti.Focus()
	ti.Prompt = s.config.Prompt
	ti.Width = 80
	ti.PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("86"))

	m := bubbleModel{
		session:   s,
		textInput: ti,
		ctx:       ctx,
	}

	var options []tea.ProgramOption
	if s.config.Stdin != nil {
		if f, ok := s.config.Stdin.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
			options = append(options, tea.WithInput(f))
		}
	}

	p := tea.NewProgram(m, options...)
	s.program = p
	
	// Create a channel to listen for context cancellation
	done := make(chan struct{})
	
	// Handle cancellation from the main context
	go func() {
		select {
		case <-ctx.Done():
			// When context is canceled, stay quiet - the completion service
			// will handle messaging to the user about cancellation
			p.Quit()
			close(done)
		case <-done:
			// We're done, no need to do anything
		}
	}()
	
	_, err := p.Run()
	close(done)
	
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}

// editInEditor opens an external editor to edit the current input
func (s *BubbleSession) editInEditor(line string) (string, error) {
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

	// Ensure editor uses the same terminal if possible
	var stdinFile *os.File = os.Stdin
	var stdoutFile *os.File = os.Stdout
	var stderrFile *os.File = os.Stderr

	cmd.Stdin = stdinFile
	cmd.Stdout = stdoutFile
	cmd.Stderr = stderrFile

	// Need to exit bubble tea mode temporarily
	s.program.Quit()

	// Give terminal a moment to reset
	time.Sleep(100 * time.Millisecond)

	err = cmd.Run()
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
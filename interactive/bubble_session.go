package interactive

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	// Use local ui paths
	"github.com/tmc/cgpt/ui/debug"
	"github.com/tmc/cgpt/ui/editor"  // Use local editor package
	"github.com/tmc/cgpt/ui/help"    // Use local help package
	"github.com/tmc/cgpt/ui/history" // Corrected history import path
	"github.com/tmc/cgpt/ui/keymap"
	"github.com/tmc/cgpt/ui/message"
	"github.com/tmc/cgpt/ui/spinner"
	"github.com/tmc/cgpt/ui/statusbar"
	"github.com/tmc/cgpt/ui/textinput" // For command mode input
)

// Helper function to create a command that sends a message
func msgCmd(msg tea.Msg) tea.Cmd {
	return func() tea.Msg { return msg }
}

// --- Compile-time check for Session interface ---
var _ Session = (*BubbleSession)(nil)

// --- Message Types ---
// Used for internal communication within the Bubble Tea model

type errMsg struct{ err error } // Reports errors back to the Update loop
type streamingMsg bool          // Signals start/stop of response streaming
type processingMsg bool         // Signals start/stop of backend processing
type addResponsePartMsg struct{ part string }
type submitBufferMsg struct { // Carries submitted input from editor/command
	input       string
	clearEditor bool // Whether to clear the editor after submission
}
type editorFinishedMsg struct { // Result from external $EDITOR
	content string
	err     error
}
type commandResultMsg struct { // Result from executing a command mode command
	output string
	err    error
}

// Message history updates
type addUserMessageMsg struct{ content string }
type addModelMessageMsg struct{ content string }
type addSystemMessageMsg struct{ content string }

// --- Sub-Models ---
// These structs group related state for different parts of the UI

// InputModel manages the editor, command input, and current mode.
type InputModel struct {
	editor       editor.Model    // Main multi-line editor
	commandInput textinput.Model // Single-line input for command mode
	mode         editorMode      // Current mode (Insert or Command)
	keyMap       keymap.KeyMap
}

// ConversationViewModel manages the display of messages.
type ConversationViewModel struct {
	conversation []message.Msg   // Holds the history of messages
	respBuffer   strings.Builder // Accumulates streaming response parts
	// Add viewport management if needed for scrolling?
}

// StatusModel manages UI elements like spinner, help, errors, and state flags.
type StatusModel struct {
	spinner        spinner.Model
	help           help.Model
	currentErr     error     // Last error encountered
	isProcessing   bool      // Backend is processing input
	isStreaming    bool      // Receiving streaming response
	interruptCount int       // For Ctrl+C double-press detection
	lastCtrlCTime  time.Time // Timestamp of last Ctrl+C press
}

// --- Main Bubble Tea Model ---

// bubbleModel holds the complete state for the interactive UI.
type bubbleModel struct {
	session        *BubbleSession        // Reference back to session for config/callbacks
	ctx            context.Context       // For cancellation propagation
	width, height  int                   // Current terminal dimensions
	input          InputModel            // Input-related state
	conversationVM ConversationViewModel // Conversation display state
	status         StatusModel           // Status indicators and flags
	debug          *debug.DebugView      // Corrected type to pointer to DebugView
	handlers       eventHandlers         // Map of key/event handlers
	quitting       bool                  // Flag to signal exit
}

// --- Event Handlers ---
// Defines functions to handle specific events/key presses.
type eventHandlers struct {
	onCtrlC      func(*bubbleModel) (tea.Model, tea.Cmd)
	onCtrlD      func(*bubbleModel) (tea.Model, tea.Cmd)
	onCtrlX      func(*bubbleModel) (tea.Model, tea.Cmd)
	onCtrlZ      func(*bubbleModel) (tea.Model, tea.Cmd)
	onEnter      func(*bubbleModel) (tea.Model, tea.Cmd)
	onUpArrow    func(*bubbleModel) (tea.Model, tea.Cmd)
	onWindowSize func(*bubbleModel, tea.WindowSizeMsg) (tea.Model, tea.Cmd)
	// Add other handlers as needed
}

// --- Editor Mode Enum ---
type editorMode int

const (
	modeInsert  editorMode = iota // Normal text editing
	modeCommand                   // Command mode (after Escape)
)

// --- BubbleSession Implementation ---

// BubbleSession implements the Session interface using Bubble Tea.
type BubbleSession struct {
	config  Config
	model   *bubbleModel
	program *tea.Program

	ResponseState ResponseState // Holds the state of the response
}

var _ Session = (*BubbleSession)(nil) // Compile-time check for Session interface

// NewBubbleSession creates a new Bubble Tea based session.
func NewBubbleSession(cfg Config) (*BubbleSession, error) {
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

	session := &BubbleSession{config: cfg}
	return session, nil
}

// Run starts the Bubble Tea application loop.
func (s *BubbleSession) Run(ctx context.Context) error {
	// Initialize components that will be part of sub-models
	editorModel := editor.New()
	editorModel.SetHistory(s.config.ConversationHistory)
	editorModel.Focus()

	cmdInput := textinput.New()
	cmdInput.Prompt = ":"
	cmdInput.CharLimit = 200
	cmdInput.Width = 50 // Initial width

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	helpModel := help.New(keymap.DefaultKeyMap())

	// Initialize the main model with embedded sub-models
	s.model = &bubbleModel{
		session: s,
		ctx:     ctx, // Use the passed context
		input: InputModel{
			editor:       editorModel,
			commandInput: cmdInput,
			mode:         modeInsert, // Start in insert mode
			keyMap:       keymap.DefaultKeyMap(),
		},
		conversationVM: ConversationViewModel{
			conversation: []message.Msg{}, // Initialize empty conversation
			respBuffer:   strings.Builder{},
		},
		status: StatusModel{
			spinner: sp,
			help:    helpModel,
			// Other fields like currentErr, isProcessing etc. default to zero values
		},
		debug:    debug.NewView(),
		handlers: createEventHandlers(),
		// quitting defaults to false
	}

	var options []tea.ProgramOption
	options = append(options, tea.WithAltScreen(), tea.WithMouseAllMotion())
	if s.config.Stdin != nil {
		if f, ok := s.config.Stdin.(*os.File); ok {
			options = append(options, tea.WithInput(f))
		}
	}

	s.program = tea.NewProgram(s.model, options...)

	// No local signal handling - rely on context cancellation from main
	progDone := make(chan error, 1)
	go func() { _, runErr := s.program.Run(); progDone <- runErr }()

	// Handle context cancellation or normal program completion
	select {
	case <-ctx.Done(): // Check if the parent context was cancelled
		s.model.debug.Log(" > Context cancelled, quitting program...")
		s.program.Quit()     // Tell bubbletea to quit
		runErr := <-progDone // Wait for Run() to actually finish
		s.model.debug.Log(" > Program finished after context cancel (err: %v)", runErr)
		// BubbleTea's Run might return an error even after Quit is called
		// Prioritize returning the context error
		return ctx.Err() // Return the cancellation error

	case err := <-progDone: // Run finished on its own or via internal Quit
		if ctx.Err() != nil { // Check if context was cancelled *during* Run
			s.model.debug.Log(" > Program finished after context cancellation.")
			return ctx.Err() // Prioritize context error
		}
		s.model.debug.Log(" > Program finished.")
		// If tea.Quit was called internally, err might be nil or a specific error
		// Check if model requested quit and propagate ErrInterrupted if so
		if s.model.quitting && err == nil {
			return ErrInterrupted // Return our specific interrupt error
		}
		return err // Return error from Run() itself (can be nil)
	}
}

// SetStreaming updates the UI state for streaming via a message.
func (s *BubbleSession) SetStreaming(streaming bool) {
	if s.program != nil {
		s.program.Send(streamingMsg(streaming))
	}
}

// SetResponseState updates the response state.
func (s *BubbleSession) SetResponseState(state ResponseState) {
	s.ResponseState = state
	if s.program != nil {
		s.program.Send(processingMsg(state.IsProcessing()))
		s.program.Send(streamingMsg(state == ResponseStateStreaming))
	}
}

// AddResponsePart sends a part of the response to the Bubble Tea model via a message.
func (s *BubbleSession) AddResponsePart(part string) {
	if s.program != nil {
		s.program.Send(addResponsePartMsg{part: part})
	}
}

// GetHistory retrieves the final history from the editor model.
func (s *BubbleSession) GetHistory() []string {
	if s.model != nil {
		return s.model.input.editor.GetHistory() // Get from input model
	}
	return s.config.ConversationHistory
}

// GetHistoryFilename retrieves the configured history filename.
func (s *BubbleSession) GetHistoryFilename() string {
	return s.config.HistoryFile // Return path from config
}

// LoadHistory delegates history loading to the editor model.
func (s *BubbleSession) LoadHistory(filename string) error {
	if s.model == nil {
		return errors.New("session model not initialized")
	}
	h, err := history.Load(filename)
	if err != nil {
		s.model.debug.Log(" > Failed to load history: %v", err)
		return nil // Non-fatal error
	}
	s.model.input.editor.SetHistory(h) // Update through input model
	s.config.HistoryFile = filename
	s.model.debug.Log(" > History loaded from %s", filename)
	return nil
}

// SaveHistory delegates history saving to the history package.
func (s *BubbleSession) SaveHistory(filename string) error {
	if s.model == nil {
		return errors.New("session model not initialized")
	}
	h := s.model.input.editor.GetHistory() // Get from input model
	if err := history.Save(h, filename); err != nil {
		s.model.debug.Log(" > Failed to save history: %v", err)
		return nil // Non-fatal error
	}
	s.config.HistoryFile = filename
	s.model.debug.Log(" > History saved to %s", filename)
	return nil
}

// Quit signals the Bubble Tea program to quit.
func (s *BubbleSession) Quit() {
	if s.program != nil {
		s.program.Quit()
	}
}

// --- Sub-Models ---

// Removed duplicated InputModel definition from within bubbleModel
// Removed incorrect closing brace here

// --- bubbleModel Methods ---

// Init initializes the model. Required by bubbletea.
func (m *bubbleModel) Init() tea.Cmd {
	// Return initial command if needed, e.g., focus editor
	return m.input.editor.Focus()
}

// Update handles messages and updates the model state. Required by bubbletea.
func (m *bubbleModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	// Handle context cancellation first - if context is done, trigger quit
	if m.ctx.Err() != nil && !m.quitting {
		m.debug.Log(" > Context done in Update, setting quit flag")
		m.quitting = true
		return m, tea.Quit
	}

	// Handle messages that affect the model regardless of mode first
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		var newModel tea.Model
		newModel, cmd = handleWindowSize(m, msg) // Use existing handler
		m = newModel.(*bubbleModel)              // Add type assertion
		// We might need to update component sizes here too
		// e.g., m.input.editor.SetWidth(m.width)
		cmds = append(cmds, cmd)
		return m, tea.Batch(cmds...) // Return early as size affects layout

	case errMsg:
		m.debug.Log(" > Error Received: %v", msg.err)
		m.status.currentErr = msg.err // Update status model
		m.status.isProcessing = false
		m.status.isStreaming = false
		// Focus appropriate input based on mode
		if m.input.mode == modeInsert {
			cmd = m.input.editor.Focus()
		} else {
			cmd = m.input.commandInput.Focus()
		}
		cmds = append(cmds, cmd)
		// Maybe add a command to clear the error after a delay?
		return m, tea.Batch(cmds...) // Return early

	// --- Processing/Streaming State Updates ---
	case processingMsg:
		m.status.isProcessing = bool(msg) // Update status model
		m.status.currentErr = nil
		if m.status.isProcessing {
			m.debug.Log(" > Processing started")
			m.conversationVM.respBuffer.Reset() // Reset conversationVM
			cmds = append(cmds, m.status.spinner.Tick)
		} else {
			// This block was misplaced before
			m.debug.Log(" > Processing finished")
			if m.status.isStreaming {
				m.status.isStreaming = false // Ensure streaming stops if processing finishes
			}
			if m.conversationVM.respBuffer.Len() > 0 {
				// Add the final buffered content as a model message
				cmds = append(cmds, msgCmd(addModelMessageMsg{content: m.conversationVM.respBuffer.String()}))
			}
			// Ensure editor gets focus back after processing
			if m.input.mode == modeInsert { // Check mode before focusing
				cmds = append(cmds, m.input.editor.Focus()) // Update input model
			}
		}

	case streamingMsg:
		m.status.isStreaming = bool(msg) // Update status model
		m.debug.Log(" > Streaming state: %v", m.status.isStreaming)
		if m.status.isStreaming {
			m.status.currentErr = nil
			cmds = append(cmds, m.status.spinner.Tick)
		} else {
			m.debug.Log(" > Streaming finished")
			if m.conversationVM.respBuffer.Len() > 0 {
				cmds = append(cmds, msgCmd(addModelMessageMsg{content: m.conversationVM.respBuffer.String()}))
			}
			if !m.status.isProcessing {
				// Only focus if not still processing (e.g., error occurred)
				if m.input.mode == modeInsert {
					cmds = append(cmds, m.input.editor.Focus()) // Update input model
				}
			}
		}
	case addResponsePartMsg:
		m.conversationVM.respBuffer.WriteString(msg.part) // Update conversationVM
		m.debug.Log(" > Received response part (%d bytes)", len(msg.part))
		// Tick spinner if processing or streaming
		if m.status.isProcessing || m.status.isStreaming {
			cmds = append(cmds, m.status.spinner.Tick)
		}

	case submitBufferMsg:
		trimmedInput := strings.TrimSpace(msg.input)
		// Only submit if not already processing
		if trimmedInput != "" && !m.status.isProcessing { // Check status model
			cmds = append(cmds, msgCmd(addUserMessageMsg{content: trimmedInput}))
			m.status.isProcessing = true // Update status model
			m.status.isStreaming = true  // Assume streaming for now
			m.status.currentErr = nil
			m.debug.Log(" > Submitting input (from msg): '%s'", trimmedInput)
			cmds = append(cmds, m.status.spinner.Tick, m.triggerProcessFn(trimmedInput))
			if msg.clearEditor { // Clear editor if requested by the message
				m.input.editor.Reset()
			}
		}

	case editorFinishedMsg: // From external editor ($EDITOR)
		m.debug.Log(" > Editor finished msg received")
		if msg.err != nil {
			m.debug.Log(" > Editor error: %v", msg.err)
			m.status.currentErr = msg.err // Update status model
		} else {
			m.debug.Log(" > Applying editor content")
			m.input.editor.SetValue(msg.content) // Update input model
			// Maybe auto-submit here? Or just focus?
		}
		// Ensure editor gets focus back
		cmds = append(cmds, m.input.editor.Focus())

	case commandResultMsg: // From command mode execution
		m.debug.Log(" > Command result received")
		if msg.err != nil {
			m.status.currentErr = msg.err // Update status model
		} else if msg.output != "" {
			// Display command output as a system message
			cmds = append(cmds, msgCmd(addSystemMessageMsg{content: msg.output}))
		}
		// Switch back to insert mode and focus editor
		m.input.mode = modeInsert // Update input model
		cmds = append(cmds, m.input.editor.Focus())

	// --- Message History Updates ---
	case addUserMessageMsg:
		m.conversationVM.conversation = append(m.conversationVM.conversation, message.Msg{Type: message.MsgTypeUser, Content: msg.content, Time: time.Now()}) // Update conversationVM
		m.debug.Log(" > Added user message")
	case addModelMessageMsg:
		m.conversationVM.conversation = append(m.conversationVM.conversation, message.Msg{Type: message.MsgTypeAssistant, Content: msg.content, Time: time.Now()}) // Update conversationVM
		m.conversationVM.respBuffer.Reset()                                                                                                                        // Clear buffer after adding full message
		m.debug.Log(" > Added model message")
	case addSystemMessageMsg:
		m.conversationVM.conversation = append(m.conversationVM.conversation, message.Msg{Type: message.MsgTypeSystem, Content: msg.content, Time: time.Now()}) // Update conversationVM
		m.debug.Log(" > Added system message")

	// --- Spinner ---
	case spinner.TickMsg:
		// Only update spinner if processing or streaming
		if m.status.isProcessing || m.status.isStreaming { // Check status model
			var spinnerCmd tea.Cmd
			m.status.spinner, spinnerCmd = m.status.spinner.Update(msg) // Update status model
			cmds = append(cmds, spinnerCmd)
		}

	// --- Key Messages (delegate based on mode) ---
	// We handle specific keys like Esc, Enter, Ctrl+C etc within the mode handlers
	case tea.KeyMsg:
		// Delegate to mode-specific handlers below
		// No action needed here, handled by mode switch below

	// --- Other message types ---
	case clearCtrlCHintMsg: // Handle the message to clear the Ctrl+C hint
		if m.status.currentErr != nil && m.status.currentErr.Error() == "Press Ctrl+C again to exit." {
			m.status.currentErr = nil
		}

		// --- Other message types ---
		// case someOtherMsg:
		// Handle other custom messages

	} // End of main switch

	// --- Mode-Specific Updates (handles KeyMsgs and potentially others) ---
	// Note: We pass the original msg here, allowing modes to handle keys etc.
	switch m.input.mode {
	case modeInsert:
		cmd = m.updateInsertMode(msg) // This will handle keys, submit on Enter etc.
		cmds = append(cmds, cmd)
	case modeCommand:
		cmd = m.updateCommandMode(msg) // This will handle keys, execute on Enter etc.
		cmds = append(cmds, cmd)
	}

	// Update help model (might depend on mode/focus)
	m.status.help, cmd = m.status.help.Update(msg)
	cmds = append(cmds, cmd)

	// --- Final Batch ---
	// Check for quit condition triggered by mode updates
	if m.quitting {
		cmds = append(cmds, tea.Quit)
	}

	return m, tea.Batch(cmds...)
}

// --- Mode Update Helpers ---

// updateInsertMode handles messages when in insert/editing mode.
func (m *bubbleModel) updateInsertMode(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	// Handle specific key presses relevant to insert mode
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch {
		// Switch to command mode on Escape
		case keyMsg.Type == tea.KeyEscape:
			m.input.mode = modeCommand // Update input model
			m.input.editor.Blur()
			m.input.commandInput.Focus()
			m.input.commandInput.Reset()
			m.status.currentErr = nil // Update status model
			m.debug.Log(" > Switched to Command Mode")
			return textinput.Blink // Return command for command input blink

		// Handle Ctrl+C within the main Update loop or specific handler
		case key.Matches(keyMsg, m.input.keyMap.Interrupt):
			var newModel tea.Model
			// Let the main handler deal with Ctrl+C logic (cancel/clear/quit)
			newModel, cmd = handleCtrlC(m) // Use existing handler
			m = newModel.(*bubbleModel)    // Add type assertion
			return cmd

		// Handle Ctrl+D: Delegate to the specific handler function
		// The editor component itself should handle io.EOF when it receives KeyCtrlD
		case keyMsg.Type == tea.KeyCtrlD: // Check for the specific key type
			var newModel tea.Model
			newModel, cmd = handleCtrlD(m) // Use existing handler
			m = newModel.(*bubbleModel)    // Add type assertion
			return cmd

		// Handle other keys by delegating to the editor component
		default:
			// Delegate general key presses to the editor
			m.input.editor, cmd = m.input.editor.Update(msg)
			cmds = append(cmds, cmd)

			// Check if editor input is complete (e.g., Enter pressed in single-line mode or Ctrl+D in multi-line)
			if m.input.editor.InputIsComplete() && m.input.editor.Err == nil {
				submittedInput := m.input.editor.Value()
				// Send a message to trigger the actual processing
				// Request editor clear *after* submission is processed
				cmds = append(cmds, msgCmd(submitBufferMsg{input: submittedInput, clearEditor: true}))
			} else if m.input.editor.Err != nil {
				// Check for specific errors like EOF or Aborted
				if errors.Is(m.input.editor.Err, io.EOF) || errors.Is(m.input.editor.Err, editor.ErrInputAborted) {
					// This might be redundant if Ctrl+D is handled above, but good fallback
					m.debug.Log(" > Editor signaled EOF/Abort: Quitting")
					m.quitting = true
					// No need to append tea.Quit here, handled in main Update loop
				} else {
					// Store other editor errors
					m.status.currentErr = m.input.editor.Err // Update status model
				}
			}
		}
	} else {
		// If not a KeyMsg, still update the editor (e.g., for Paste)
		m.input.editor, cmd = m.input.editor.Update(msg)
		cmds = append(cmds, cmd)
		// Re-check completion/error state after non-key updates if necessary
		if m.input.editor.InputIsComplete() && m.input.editor.Err == nil {
			submittedInput := m.input.editor.Value()
			cmds = append(cmds, msgCmd(submitBufferMsg{input: submittedInput, clearEditor: true}))
		} else if m.input.editor.Err != nil {
			if errors.Is(m.input.editor.Err, io.EOF) || errors.Is(m.input.editor.Err, editor.ErrInputAborted) {
				m.quitting = true
			} else {
				m.status.currentErr = m.input.editor.Err
			}
		}
	}

	return tea.Batch(cmds...)
}

// updateCommandMode handles messages when in command mode.
func (m *bubbleModel) updateCommandMode(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Use keyMap from input model
		switch {
		// Switch back to insert mode on Escape or Interrupt
		case msg.Type == tea.KeyEscape, key.Matches(msg, m.input.keyMap.Interrupt):
			m.input.mode = modeInsert // Update input model
			m.input.commandInput.Blur()
			m.input.editor.Focus()
			m.status.currentErr = nil // Update status model
			m.debug.Log(" > Switched to Insert Mode")
			return nil // No command needed, focus handled by editor

		// Execute command on Enter
		case key.Matches(msg, m.input.keyMap.Submit):
			commandStr := strings.TrimSpace(m.input.commandInput.Value())
			m.input.commandInput.Reset() // Clear command input
			if commandStr != "" {
				m.debug.Log(" > Executing command: :%s", commandStr)
				// Create a command to execute the function and return a result message
				cmd = executeCommandCmd(commandStr, m)
				cmds = append(cmds, cmd)
				// Switch back to insert mode *after* command execution (handled by commandResultMsg)
			} else {
				// If Enter pressed on empty command, just switch back to insert mode
				m.input.mode = modeInsert // Update input model
				m.input.editor.Focus()
				m.debug.Log(" > Empty command, switching to Insert Mode")
			}
			return tea.Batch(cmds...) // Return command(s)
		}
	}

	// Delegate other messages (like typing) to the command text input
	m.input.commandInput, cmd = m.input.commandInput.Update(msg) // Update input model's commandInput
	cmds = append(cmds, cmd)

	return tea.Batch(cmds...)
}

// triggerProcessFn creates a tea.Cmd to run the session's ProcessFn.
func (m *bubbleModel) triggerProcessFn(input string) tea.Cmd {
	return func() tea.Msg {
		// Check context before starting potentially long operation
		if m.ctx.Err() != nil {
			m.debug.Log(" > Context done before starting ProcessFn")
			// Signal processing stopped due to context cancellation before start
			return tea.Batch(msgCmd(errMsg{m.ctx.Err()}), msgCmd(processingMsg(false)))()
		}

		m.debug.Log(" > Starting ProcessFn for: '%s'", input)
		// Pass the model's context down to ProcessFn
		err := m.session.config.ProcessFn(m.ctx, input)
		m.debug.Log(" > ProcessFn finished.")

		// Check context again after ProcessFn returns
		if m.ctx.Err() != nil {
			m.debug.Log(" > Context done during/after ProcessFn")
			// If context was cancelled, prioritize that error unless ProcessFn had a different specific error
			if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, ErrInterrupted) {
				// ProcessFn failed *before* or *differently* than context cancellation/interrupt was detected
				m.debug.Log(" > ProcessFn error before/besides context cancel: %v", err)
				return tea.Batch(msgCmd(errMsg{err}), msgCmd(processingMsg(false)))()
			}
			// If error was cancellation/interrupt, just signal processing stopped
			return processingMsg(false)
		}

		// Handle other errors from ProcessFn
		if err != nil {
			m.debug.Log(" > ProcessFn error: %v", err)
			// Special handling for ErrInterrupted from ProcessFn itself
			if errors.Is(err, ErrInterrupted) {
				// Don't treat interrupt as an error message, just stop processing
				return processingMsg(false)
			}
			// Return both an error message and a processing finished message for other errors
			return tea.Batch(
				msgCmd(errMsg{err}),
				msgCmd(processingMsg(false)),
			)() // Execute batch immediately to get combined message
		}
		// If no error, just signal processing is finished
		return processingMsg(false)
	} // Close the inner anonymous function
} // Close triggerProcessFn

// executeCommandCmd creates a tea.Cmd to run the session's CommandFn.
func executeCommandCmd(command string, m *bubbleModel) tea.Cmd {
	return func() tea.Msg {
		m.debug.Log(" > Executing CommandFn for: :%s", command)
		// TODO: How should command output be captured?
		// For now, assume CommandFn might return an error but not direct output.
		// If CommandFn needs to return output, its signature and this handler need adjustment.
		// Check context before executing command
		if m.ctx.Err() != nil {
			return errMsg{err: fmt.Errorf("command execution cancelled: %w", m.ctx.Err())}
		}
		err := m.session.config.CommandFn(m.ctx, command)
		output := "" // Placeholder for potential future output capture
		if err != nil {
			m.debug.Log(" > CommandFn error: %v", err)
		} else {
			m.debug.Log(" > CommandFn executed successfully.")
		}
		return commandResultMsg{output: output, err: err}
	}
}

// View renders the UI based on the current mode.
func (m *bubbleModel) View() string {
	if m.quitting {
		return "" // Don't render if quitting
	}

	var view strings.Builder // Corrected variable declaration

	// --- Calculate Heights --- // Corrected comment typo
	statusBarHeight := 1 // Corrected variable name
	spinnerHeight := 0
	// Show spinner only when actively processing *and* not streaming visible content
	if m.status.isProcessing && (!m.status.isStreaming || m.conversationVM.respBuffer.Len() == 0) {
		spinnerHeight = 1
	}
	errorHeight := 0
	if m.status.currentErr != nil { // Use status model
		errorHeight = lipgloss.Height(renderError(m))
	}
	debugHeight := 0
	debugContent := m.debug.View()
	if debugContent != "" {
		debugHeight = lipgloss.Height(debugContent) + 1
	}
	helpHeight := 0
	helpContent := m.status.help.View() // Use status model
	if helpContent != "" {
		helpHeight = lipgloss.Height(helpContent)
	}
	headerHeight := 0 // Placeholder for potential header

	// Calculate available height for the main content area (conversation + editor/input)
	mainContentHeight := m.height - headerHeight - debugHeight - statusBarHeight - spinnerHeight - errorHeight - helpHeight - 1 // -1 for potential newline buffer
	if mainContentHeight < 1 {
		mainContentHeight = 1
	}

	// --- Render Sections ---
	if debugContent != "" {
		view.WriteString(debugContent + "\n")
	}

	// Render Conversation History
	conversationContent := renderConversation(m)
	view.WriteString(conversationContent)
	view.WriteString("\n") // Separator line

	conversationHeight := lipgloss.Height(conversationContent)
	inputAreaHeight := mainContentHeight - conversationHeight - 1 // -1 for separator
	if inputAreaHeight < 1 {
		inputAreaHeight = 1 // Ensure minimum height for input
	}

	// Render Editor or Command Input (using input model)
	if m.input.mode == modeInsert {
		m.input.editor.SetHeight(inputAreaHeight) // Set editor height
		view.WriteString(m.input.editor.View())
		// Render streaming response *after* the editor view if streaming
		if m.status.isStreaming && m.conversationVM.respBuffer.Len() > 0 {
			view.WriteString("\n" + renderStreamingResponse(m)) // Prepend newline
		}
	} else { // modeCommand
		cmdInputWidth := m.width - lipgloss.Width(m.input.commandInput.Prompt) - 1
		if cmdInputWidth < 10 {
			cmdInputWidth = 10
		}
		m.input.commandInput.Width = cmdInputWidth
		// Render command input in the allocated input area
		// Potentially adjust height if command input needs more space (unlikely)
		view.WriteString(m.input.commandInput.View())
		// Pad remaining lines in input area if needed
		view.WriteString(strings.Repeat("\n", max(0, inputAreaHeight-lipgloss.Height(m.input.commandInput.View()))))
	}

	// Spinner or Error (using status model)
	// Only show spinner if actively processing and not showing streaming output prominently
	if m.status.isProcessing && (!m.status.isStreaming || m.conversationVM.respBuffer.Len() == 0) {
		view.WriteString("\n" + m.status.spinner.View() + " Processing...")
	}

	if m.status.currentErr != nil {
		view.WriteString("\n" + renderError(m))
	}

	// Help View (using status model)
	if helpContent != "" {
		view.WriteString("\n" + helpContent)
	}

	// Status Bar (Bottom)
	statusData := statusbar.StatusData{}
	if m.input.mode == modeInsert { // Use input model
		statusData.Mode = m.input.editor.InputMode()
	} else {
		statusData.Mode = "COMMAND"
	}
	view.WriteString("\n")
	view.WriteString(statusbar.Render(m.width, statusData))

	// Trim only trailing newlines to preserve internal structure
	return strings.TrimRight(view.String(), "\n")
}

// renderConversation renders conversation history.
func renderConversation(m *bubbleModel) string {
	var lines []string
	// TODO: Implement viewport logic if conversation gets too long
	// For now, render all messages
	for _, msg := range m.conversationVM.conversation { // Use conversationVM
		rendered := message.Render(msg, m.width-2) // -2 for potential border/padding
		lines = append(lines, rendered)
	}
	return strings.Join(lines, "\n")
}

// renderStreamingResponse renders the currently streaming response.
func renderStreamingResponse(m *bubbleModel) string {
	style := message.AssistantStyle
	prefix := "Assistant: "
	indicator := "█"
	if int(time.Now().UnixMilli()/500)%2 == 0 {
		indicator = " "
	}
	content := m.conversationVM.respBuffer.String() // Use conversationVM
	// Use full width for streaming response rendering initially
	contentWidth := max(10, m.width-lipgloss.Width(prefix)-lipgloss.Width(indicator)-3) // Use model width

	renderedContent := style.Width(contentWidth).Render(content)
	lines := strings.Split(renderedContent, "\n")
	firstLine := prefix + lines[0] // Apply prefix only to first line

	subsequentLines := ""
	if len(lines) > 1 {
		prefixWidth := lipgloss.Width(prefix)
		indent := strings.Repeat(" ", prefixWidth)
		for i := 1; i < len(lines); i++ {
			lines[i] = indent + lines[i] // Indent subsequent lines
		}
		subsequentLines = "\n" + strings.Join(lines[1:], "\n")
	}
	// Append indicator to the *last* line (after potential indenting)
	return firstLine + subsequentLines + indicator
}

// renderError renders the current error message.
func renderError(m *bubbleModel) string {
	if m.status.currentErr == nil { // Use status model
		return ""
	}
	errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9")) // Red
	// Render with full width for clarity
	return errStyle.Width(m.width).Render(fmt.Sprintf("Error: %v", m.status.currentErr)) // Use status model
}

// --- Event Handlers ---

func createEventHandlers() eventHandlers {
	return eventHandlers{
		onCtrlC:      handleCtrlC,
		onCtrlD:      handleCtrlD,
		onCtrlX:      handleCtrlX,
		onCtrlZ:      handleCtrlZ,
		onEnter:      handleEnter,
		onUpArrow:    handleUpArrow,
		onWindowSize: handleWindowSize,
	}
}

// handleUpArrow delegates to editor.
func handleUpArrow(m *bubbleModel) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.debug.Log(" > Up Arrow (delegated to editor)")
	m.input.editor, cmd = m.input.editor.Update(tea.KeyMsg{Type: tea.KeyUp}) // Update input model
	return m, cmd
}

// handleWindowSize updates model and component dimensions.
func handleWindowSize(m *bubbleModel, msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	m.width = msg.Width
	m.height = msg.Height
	m.debug.UpdateDimensions(msg.Width)
	m.status.help.SetWidth(msg.Width) // Update status model
	// Recalculate editor width based on new window size
	m.input.editor.SetWidth(msg.Width)
	m.debug.Log(" > Window Size: %dx%d", m.width, m.height)
	return m, nil
}

// handleCtrlC handles interrupt/exit logic based on mode.
func handleCtrlC(m *bubbleModel) (tea.Model, tea.Cmd) {
	if m.input.mode == modeCommand { // Use input model
		m.debug.Log(" > Ctrl+C in Command Mode: Switching to Insert Mode")
		m.input.mode = modeInsert // Update input model
		m.input.commandInput.Blur()
		m.input.commandInput.Reset()
		m.input.editor.Focus()
		m.status.currentErr = nil // Update status model
		return m, nil
	}

	now := time.Now()
	doublePressDuration := 1 * time.Second

	if m.status.isProcessing || m.status.isStreaming { // Use status model
		m.debug.Log(" > Ctrl+C: Attempting to cancel processing")
		// Signal cancellation to the ProcessFn via context
		// The triggerProcessFn command captures the context and handles cancellation
		// Here, we just update the UI state and potentially signal tea.Quit if needed later
		m.status.isProcessing = false // Update status model
		m.status.isStreaming = false
		m.conversationVM.respBuffer.Reset() // Update conversationVM
		m.input.editor.Focus()
		m.status.currentErr = nil // Update status model
		// Return a system message indicating cancellation
		// Let the triggerProcessFn command handle returning ErrInterrupted
		return m, msgCmd(addSystemMessageMsg{content: "[Processing cancelled by user]"})
	}

	if m.input.editor.Value() != "" { // Use input model
		m.debug.Log(" > Ctrl+C: Clearing editor input")
		m.input.editor.SetValue("") // Update input model
		m.status.interruptCount = 0 // Update status model
		m.status.currentErr = nil   // Update status model
	} else {
		// Use status model for time/count
		if now.Sub(m.status.lastCtrlCTime) < doublePressDuration && m.status.interruptCount > 0 {
			m.debug.Log(" > Ctrl+C double press on empty editor: Quitting")
			m.quitting = true
			return m, tea.Quit // Send Quit command immediately
		}

		m.debug.Log(" > Ctrl+C on empty editor: Press again to exit")
		m.status.currentErr = errors.New("Press Ctrl+C again to exit.") // Update status model
		m.status.interruptCount++                                       // Update status model
		m.status.lastCtrlCTime = now                                    // Update status model

		// Command to clear the error message after a delay
		clearErrCmd := tea.Tick(doublePressDuration, func(t time.Time) tea.Msg {
			// Return a message that the main Update loop can handle
			return clearCtrlCHintMsg{}
		})
		return m, clearErrCmd
	}
	return m, nil
}

// Define a new message type for clearing the Ctrl+C hint
type clearCtrlCHintMsg struct{}

// handleCtrlD handles EOF/quit logic, delegating to editor first.
func handleCtrlD(m *bubbleModel) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.debug.Log(" > Ctrl+D (delegated to editor)")
	m.input.editor, cmd = m.input.editor.Update(tea.KeyMsg{Type: tea.KeyCtrlD}) // Update input model

	if m.input.editor.Err != nil && errors.Is(m.input.editor.Err, io.EOF) { // Check input model
		m.debug.Log(" > Ctrl+D on empty editor: Quitting")
		m.quitting = true
		return m, tea.Quit // Send Quit command immediately
	}
	m.debug.Log(" > Ctrl+D handled by editor")
	return m, cmd
}

// handleCtrlX delegates to the editor.
func handleCtrlX(m *bubbleModel) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.debug.Log(" > Ctrl+X: Delegating to editor")
	m.input.editor, cmd = m.input.editor.Update(tea.KeyMsg{Type: tea.KeyCtrlX}) // Update input model
	return m, cmd
}

// handleCtrlZ triggers suspend.
func handleCtrlZ(m *bubbleModel) (tea.Model, tea.Cmd) {
	m.debug.Log(" > Ctrl+Z: Suspending (may not work in WASM)")
	return m, tea.Suspend
}

// handleEnter delegates to the editor.
func handleEnter(m *bubbleModel) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.debug.Log(" > Enter: Delegating to editor")
	m.input.editor, cmd = m.input.editor.Update(tea.KeyMsg{Type: tea.KeyEnter}) // Update input model

	if m.input.editor.InputIsComplete() && m.input.editor.Err == nil { // Check input model
		submittedInput := m.input.editor.Value()
		cmd = tea.Batch(cmd, msgCmd(submitBufferMsg{input: submittedInput, clearEditor: true})) // Request clear
	}
	return m, cmd
}

// max returns the larger of x or y.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

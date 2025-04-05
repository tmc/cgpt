package interactive

import (
	"context"
	"errors"
	"fmt"
	"io" // Added for io.EOF check
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	// "golang.org/x/term" // Removed for WASM compatibility

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
}

// NewBubbleSession creates a new Bubble Tea based session.
func NewBubbleSession(cfg Config) (*BubbleSession, error) {
	if cfg.SingleLineHint == "" {
		cfg.SingleLineHint = DefaultSingleLineHint
	}
	if cfg.MultiLineHint == "" {
		cfg.MultiLineHint = DefaultMultiLineHint
	}

	session := &BubbleSession{config: cfg}
	return session, nil
}

// Run starts the Bubble Tea application loop.
func (s *BubbleSession) Run(ctx context.Context) error {
	// Initialize editor component
	editorModel := editor.New()
	editorModel.SetHistory(s.config.LoadedHistory)
	editorModel.Focus()

	// Initialize command input
	cmdInput := textinput.New()
	cmdInput.Prompt = ":"
	cmdInput.CharLimit = 200
	cmdInput.Width = 50 // Initial width

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	// Use default keymap for help, as editor's is unexported
	helpModel := help.New(keymap.DefaultKeyMap())

	s.model = &bubbleModel{
		editor:       editorModel,
		commandInput: cmdInput, // Add command input model
		spinner:      sp,
		help:         helpModel,
		session:      s,
		ctx:          ctx,
		handlers:     createEventHandlers(),
		debug:        debug.NewView(),
		keyMap:       keymap.DefaultKeyMap(), // Use default keymap
		lastInput:    s.config.LastInput,
		mode:         modeInsert, // Start in insert mode
	}

	var options []tea.ProgramOption
	options = append(options, tea.WithAltScreen(), tea.WithMouseAllMotion())
	if s.config.Stdin != nil {
		// Check if Stdin is a terminal file descriptor before using tea.WithInput
		if f, ok := s.config.Stdin.(*os.File); ok {
			// Note: terminal detection is platform-specific
			// For cross-platform concerns, this could be further refined with build tags
			options = append(options, tea.WithInput(f))
		}
	}

	s.program = tea.NewProgram(s.model, options...)

	progDone := make(chan error, 1)
	go func() { _, runErr := s.program.Run(); progDone <- runErr }()

	select {
	case <-ctx.Done():
		s.model.debug.Log(" > Context cancelled, quitting program...") // Use s.model.debug
		s.program.Quit()
		<-progDone
		return ctx.Err()
	case err := <-progDone:
		if ctx.Err() != nil {
			s.model.debug.Log(" > Program finished after context cancellation.") // Use s.model.debug
			return ctx.Err()
		}
		s.model.debug.Log(" > Program finished.") // Use s.model.debug
		return err
	}
}

// SetStreaming updates the UI state for streaming via a message.
func (s *BubbleSession) SetStreaming(streaming bool) {
	if s.program != nil {
		s.program.Send(streamingMsg(streaming))
	}
}

// SetLastInput updates the last input for history recall in the editor model.
func (s *BubbleSession) SetLastInput(input string) {
	if s.model != nil {
		s.model.editor.SetLastInput(input)
	}
	s.config.LastInput = input
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
		return s.model.editor.GetHistory()
	}
	return s.config.LoadedHistory // Fallback
}

// GetHistoryFilename retrieves the configured history filename.
func (s *BubbleSession) GetHistoryFilename() string {
	return s.config.HistoryFile // Return path from config
}

// LoadHistory delegates history loading to the editor model.
// Note: Filesystem access might be restricted in WASM.
func (s *BubbleSession) LoadHistory(filename string) error {
	if s.model == nil {
		return errors.New("session model not initialized")
	}
	// Load history from file using the history package
	h, err := history.Load(filename) // Use imported history package
	if err != nil {
		// Log the error but continue - history loading is not critical
		s.model.debug.Log(" > Failed to load history: %v", err)
		return nil // Non-fatal error
	}
	s.model.editor.SetHistory(h)    // Update editor's history
	s.config.HistoryFile = filename // Update config path
	s.model.debug.Log(" > History loaded from %s", filename)
	return nil
}

// SaveHistory delegates history saving to the history package.
// Note: Filesystem access might be restricted in WASM.
func (s *BubbleSession) SaveHistory(filename string) error {
	if s.model == nil {
		return errors.New("session model not initialized")
	}
	h := s.model.editor.GetHistory()                  // Get current history from editor
	if err := history.Save(h, filename); err != nil { // Use imported history package
		// Log the error but continue - history saving is not critical
		s.model.debug.Log(" > Failed to save history: %v", err)
		return nil // Non-fatal error
	}
	s.config.HistoryFile = filename // Update config path
	s.model.debug.Log(" > History saved to %s", filename)
	return nil
}

// Quit signals the Bubble Tea program to quit.
func (s *BubbleSession) Quit() {
	if s.program != nil {
		s.program.Quit()
	}
}

// --- Bubble Tea Model ---

// bubbleModel holds the state for the Bubble Tea UI.
type bubbleModel struct {
	width, height  int
	session        *BubbleSession
	editor         editor.Model    // Main multi-line editor
	commandInput   textinput.Model // Single-line input for command mode
	spinner        spinner.Model
	help           help.Model
	debug          *debug.DebugView
	conversation   []message.Msg
	respBuffer     strings.Builder
	lastInput      string
	currentErr     error
	ctx            context.Context
	mode           editorMode // Current mode (Insert or Command)
	quitting       bool
	isProcessing   bool
	isStreaming    bool
	handlers       eventHandlers
	keyMap         keymap.KeyMap
	interruptCount int
	lastCtrlCTime  time.Time
}

// Event handlers struct definition (moved from session.go)
type eventHandlers struct {
	onCtrlC      func(m *bubbleModel) (tea.Model, tea.Cmd)
	onCtrlD      func(m *bubbleModel) (tea.Model, tea.Cmd)
	onCtrlX      func(m *bubbleModel) (tea.Model, tea.Cmd)
	onCtrlZ      func(m *bubbleModel) (tea.Model, tea.Cmd)
	onEnter      func(m *bubbleModel) (tea.Model, tea.Cmd)
	onUpArrow    func(m *bubbleModel) (tea.Model, tea.Cmd)
	onWindowSize func(m *bubbleModel, msg tea.WindowSizeMsg) (tea.Model, tea.Cmd)
}

// --- Bubble Tea Messages ---
type (
	errMsg          struct{ err error }
	processingMsg   bool
	streamingMsg    bool
	submitBufferMsg struct {
		input       string
		clearEditor bool
	} // Renamed clearBuffer
	addResponsePartMsg struct{ part string }
	editorFinishedMsg  struct {
		content string
		err     error
	} // From external editor
	commandResultMsg struct {
		output string
		err    error
	} // Result of executing a command mode command
	// Message history updates
	addUserMessageMsg   struct{ content string }
	addModelMessageMsg  struct{ content string }
	addSystemMessageMsg struct{ content string }
)

func (e errMsg) Error() string { return e.err.Error() }

// --- Bubble Tea Init/Update/View ---

func (m *bubbleModel) Init() tea.Cmd { // Changed receiver to pointer
	m.debug.Log(" > Bubble Tea Init") // Use m.debug
	// return tea.Batch(m.editor.Init(), m.spinner.Tick) // editor.Init() removed
	return m.spinner.Tick
}

func (m *bubbleModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) { // Changed receiver to pointer
	var cmds []tea.Cmd
	var cmd tea.Cmd

	m.debug.AddEvent(msg) // Log event - Use m.debug

	// --- Global Handling & Key Interception ---
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Intercept specific keys before mode-specific handlers
		// For Ctrl+C, we need special handling based on state:
		if key.Matches(msg, m.keyMap.Interrupt) {
			// Always handle Ctrl+C with our custom handler to ensure
			// consistent behavior, especially for tests
			return handleCtrlC(m)
		}
		// If not intercepted, fall through to mode-specific handling below

	case tea.WindowSizeMsg:
		// Pass m (pointer) directly, not &m (pointer-to-pointer)
		return m.handlers.onWindowSize(m, msg)
	case errMsg:
		m.debug.Log(" > Error Received: %v", msg.err) // Use m.debug
		m.currentErr = msg.err
		m.isProcessing = false
		m.isStreaming = false
		if m.mode == modeInsert {
			m.editor.Focus()
		} else {
			m.commandInput.Focus()
		} // Focus correct input
		return m, nil // Return m (pointer) which implements tea.Model
	}

	// --- Mode-Specific Handling ---
	// Only called if the key wasn't intercepted above
	switch m.mode {
	case modeInsert:
		cmd = m.updateInsertMode(msg) // updateInsertMode should modify m in place and return only cmd
		cmds = append(cmds, cmd)
	case modeCommand:
		cmd = m.updateCommandMode(msg) // updateCommandMode should modify m in place and return only cmd
		cmds = append(cmds, cmd)
	}

	// --- Shared Message Handling (after mode-specific updates) ---
	// Need to re-check the type as the original msg might have been processed already
	switch msg := msg.(type) {
	case processingMsg: // Signals ProcessFn start/end
		m.isProcessing = bool(msg)
		m.currentErr = nil
		if m.isProcessing {
			m.debug.Log(" > Processing started") // Use m.debug
			m.respBuffer.Reset()
			cmds = append(cmds, m.spinner.Tick)
		} else {
			m.debug.Log(" > Processing finished") // Use m.debug
			if m.isStreaming {
				m.isStreaming = false
			}
			if m.respBuffer.Len() > 0 {
				// Send message directly instead of using helper cmd
				cmds = append(cmds, msgCmd(addModelMessageMsg{content: m.respBuffer.String()}))
			}
			m.editor.Focus() // Ensure editor is focused after processing
		}
	case streamingMsg: // Signals AddResponsePart start/end
		m.isStreaming = bool(msg)
		m.debug.Log(" > Streaming state: %v", m.isStreaming) // Use m.debug
		if m.isStreaming {
			m.currentErr = nil
			cmds = append(cmds, m.spinner.Tick)
		} else {
			m.debug.Log(" > Streaming finished") // Use m.debug
			if m.respBuffer.Len() > 0 {
				// Send message directly instead of using helper cmd
				cmds = append(cmds, msgCmd(addModelMessageMsg{content: m.respBuffer.String()}))
			}
			if !m.isProcessing {
				m.editor.Focus()
			}
		}
	case addResponsePartMsg: // Append streaming data
		m.respBuffer.WriteString(msg.part)
		m.debug.Log(" > Received response part (%d bytes)", len(msg.part)) // Use m.debug
		// m.editor.SetViewportContent(renderConversation(&m) + "\n" + renderStreamingResponse(&m)) // Removed
		// m.editor.GotoBottom() // Removed
		if m.isProcessing || m.isStreaming {
			cmds = append(cmds, m.spinner.Tick)
		}

	case submitBufferMsg: // Triggered by editor completion or command execution
		trimmedInput := strings.TrimSpace(msg.input)
		if trimmedInput != "" && !m.isProcessing {
			// Send message directly instead of using helper cmd
			cmds = append(cmds, msgCmd(addUserMessageMsg{content: trimmedInput}))
			m.isProcessing = true
			m.currentErr = nil
			m.debug.Log(" > Submitting input (from msg): '%s'", trimmedInput) // Use m.debug
			cmds = append(cmds, m.spinner.Tick, m.triggerProcessFn(trimmedInput))
		}
		// Editor should clear itself if clearEditor was true (now handled by editor)

	case editorFinishedMsg: // Result from external editor ($EDITOR)
		m.debug.Log(" > Editor finished msg received") // Use m.debug
		if msg.err != nil {
			m.debug.Log(" > Editor error: %v", msg.err) // Use m.debug
			m.currentErr = msg.err
		} else {
			m.debug.Log(" > Applying editor content") // Use m.debug
			m.editor.SetValue(msg.content)
		}
		cmds = append(cmds, m.editor.Focus()) // Re-focus editor, removed Blink

	case commandResultMsg: // Result from executing a ':' command
		m.debug.Log(" > Command result received") // Use m.debug
		if msg.err != nil {
			m.currentErr = msg.err
		} else if msg.output != "" {
			// Display command output as a system message or status?
			// Send message directly instead of using helper cmd
			cmds = append(cmds, msgCmd(addSystemMessageMsg{content: msg.output}))
		}
		m.mode = modeInsert                   // Switch back to insert mode
		cmds = append(cmds, m.editor.Focus()) // Removed Blink

	// --- Message History Updates ---
	case addUserMessageMsg:
		m.conversation = append(m.conversation, message.Msg{Type: message.MsgTypeUser, Content: msg.content, Time: time.Now()})
		// m.conversation = limitHistory(m.conversation, 50); // Removed limitHistory
		m.debug.Log(" > Added user message") // Use m.debug
		// m.editor.SetViewportContent(renderConversation(&m)); m.editor.GotoBottom() // Removed
	case addModelMessageMsg:
		m.conversation = append(m.conversation, message.Msg{Type: message.MsgTypeAssistant, Content: msg.content, Time: time.Now()})
		m.respBuffer.Reset()
		// m.conversation = limitHistory(m.conversation, 50); // Removed limitHistory
		m.debug.Log(" > Added model message") // Use m.debug
		// m.editor.SetViewportContent(renderConversation(&m)); m.editor.GotoBottom() // Removed
	case addSystemMessageMsg:
		m.conversation = append(m.conversation, message.Msg{Type: message.MsgTypeSystem, Content: msg.content, Time: time.Now()})
		// m.conversation = limitHistory(m.conversation, 50); // Removed limitHistory
		m.debug.Log(" > Added system message") // Use m.debug
		// m.editor.SetViewportContent(renderConversation(&m)); m.editor.GotoBottom() // Removed

	// --- Spinner ---
	case spinner.TickMsg:
		if m.isProcessing || m.isStreaming {
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

// updateInsertMode handles updates when in normal editor input mode.
// It now modifies m directly and returns only the command.
func (m *bubbleModel) updateInsertMode(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	// Special handling for Escape to enter command mode
	if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.Type == tea.KeyEscape { // Use tea.KeyEscape
		m.mode = modeCommand
		m.editor.Blur()                            // Blur main editor
		m.commandInput.Focus()                     // Focus command input
		m.commandInput.Reset()                     // Clear command input
		m.currentErr = nil                         // Clear any previous error
		m.debug.Log(" > Switched to Command Mode") // Use m.debug
		return textinput.Blink                     // Start command input blink
	}

	// Delegate other messages to the editor component
	// Assign result directly back to m.editor
	m.editor, cmd = m.editor.Update(msg)
	cmds = append(cmds, cmd)

	// Check if the editor signaled completion
	if m.editor.InputIsComplete() && m.editor.Err == nil {
		submittedInput := m.editor.Value() // Get value *before* editor resets
		// No need to reset editor here, it should reset itself on completion signal
		m.lastInput = submittedInput // Store last successful input
		// Send submitBufferMsg as a message, not a command
		cmds = append(cmds, msgCmd(submitBufferMsg{input: submittedInput, clearEditor: false}))
	} else if m.editor.Err != nil {
		// Propagate editor errors (like EOF, Abort)
		if errors.Is(m.editor.Err, io.EOF) || errors.Is(m.editor.Err, editor.ErrInputAborted) { // Use imported io
			m.quitting = true
			cmds = append(cmds, tea.Quit)
		} else {
			m.currentErr = m.editor.Err // Store other editor errors
		}
	}

	// Update help model as well (for window size changes primarily)
	m.help, cmd = m.help.Update(msg)
	cmds = append(cmds, cmd)

	return tea.Batch(cmds...)
}

// updateCommandMode handles updates when in ':' command mode.
// It now modifies m directly and returns only the command.
func (m *bubbleModel) updateCommandMode(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		// Use standard Escape and Ctrl+C keys
		case msg.Type == tea.KeyEscape, key.Matches(msg, m.keyMap.Interrupt):
			// Exit command mode on Escape or Ctrl+C
			m.mode = modeInsert
			m.commandInput.Blur()                     // Blur command input
			m.editor.Focus()                          // Focus main editor
			m.currentErr = nil                        // Clear error
			m.debug.Log(" > Switched to Insert Mode") // Use m.debug
			return nil                                // Removed Blink

		case key.Matches(msg, m.keyMap.Submit): // Enter executes the command
			commandStr := strings.TrimSpace(m.commandInput.Value())
			m.commandInput.Reset() // Clear command input for next time
			if commandStr != "" {
				m.debug.Log(" > Executing command: :%s", commandStr) // Use m.debug
				// Send command to execute in background
				cmd = executeCommandCmd(commandStr, m) // Pass model to access context/config
				cmds = append(cmds, cmd)
				// Optionally show a "processing command" indicator?
			} else {
				// Empty command, just switch back to insert mode
				m.mode = modeInsert
				m.editor.Focus()
				m.debug.Log(" > Empty command, switching to Insert Mode") // Use m.debug
				// cmds = append(cmds, editor.Blink) // Removed Blink
			}
			return tea.Batch(cmds...) // Return immediately, result comes via msg

			// TODO: Add command history navigation (Up/Down in commandInput)?
			// TODO: Add command autocompletion (Tab in commandInput)?
		}
	}

	// Delegate other messages to the command text input
	m.commandInput, cmd = m.commandInput.Update(msg)
	cmds = append(cmds, cmd)

	return tea.Batch(cmds...)
}

// triggerProcessFn creates a tea.Cmd to run the session's ProcessFn.
func (m *bubbleModel) triggerProcessFn(input string) tea.Cmd {
	return func() tea.Msg {
		err := m.session.config.ProcessFn(m.ctx, input) // Pass model's context
		if errors.Is(err, context.Canceled) || m.ctx.Err() != nil {
			m.debug.Log(" > Processing cancelled by context") // Use m.debug
			return processingMsg(false)
		}
		if err != nil {
			m.debug.Log(" > ProcessFn error: %v", err) // Use m.debug
			// Ensure error and processingMsg(false) are sent
			return tea.Batch( // Use tea.Batch to send multiple messages
				msgCmd(errMsg{err}),
				msgCmd(processingMsg(false)),
			)
		}
		m.debug.Log(" > ProcessFn finished successfully") // Use m.debug
		return processingMsg(false)                       // Signal normal completion
	}
}

// executeCommandCmd creates a tea.Cmd to run the session's CommandFn.
// It now takes the model to access context and config.
func executeCommandCmd(command string, m *bubbleModel) tea.Cmd {
	return func() tea.Msg {
		if m.session == nil || m.session.config.CommandFn == nil {
			return errMsg{errors.New("command function not configured")}
		}

		// Use the model's context and session's CommandFn
		err := m.session.config.CommandFn(m.ctx, command)
		output := "" // CommandFn might provide output via AddResponsePart or return it

		// Logging removed as 'm' is not accessible here. Error is returned in the message.
		// Send result back
		return commandResultMsg{output: output, err: err}
	}
}

// View renders the UI based on the current mode.
func (m *bubbleModel) View() string { // Changed receiver to pointer
	if m.quitting {
		return ""
	}

	var view strings.Builder

	// --- Calculate Heights ---
	statusBarHeight := 1
	// editorViewHeight := 0 // Removed unused variable
	spinnerHeight := 0
	if m.isProcessing && !m.isStreaming {
		spinnerHeight = 1
	}
	errorHeight := 0
	if m.currentErr != nil {
		errorHeight = lipgloss.Height(renderError(m)) // Pass m directly
	}
	debugHeight := 0
	debugContent := m.debug.View() // Use m.debug
	if debugContent != "" {
		debugHeight = lipgloss.Height(debugContent) + 1
	}
	helpHeight := 0
	helpContent := m.help.View()
	if helpContent != "" {
		helpHeight = lipgloss.Height(helpContent)
	}
	headerHeight := 0

	// Height available for the main content area (editor/command input)
	availableHeight := m.height - headerHeight - debugHeight - statusBarHeight - spinnerHeight - errorHeight - helpHeight - 1 // -1 for spacing
	if availableHeight < 1 {
		availableHeight = 1
	}

	// --- Render Sections ---
	if debugContent != "" {
		view.WriteString(debugContent + "\n")
	}

	// Render Editor or Command Input
	if m.mode == modeInsert {
		m.editor.SetHeight(availableHeight) // Update editor height
		// Update editor viewport content before rendering
		convAndStream := renderConversation(m) // Pass m directly
		if m.isStreaming && m.respBuffer.Len() > 0 {
			if convAndStream != "" {
				convAndStream += "\n"
			}
			convAndStream += renderStreamingResponse(m) // Pass m directly
		}
		// m.editor.SetViewportContent(convAndStream) // Removed
		view.WriteString(m.editor.View())
		// editorViewHeight = lipgloss.Height(m.editor.View()) // Removed unused variable
	} else { // modeCommand
		// Command input takes minimal height (usually 1 line)
		cmdInputWidth := m.width - lipgloss.Width(m.commandInput.Prompt) - 1
		if cmdInputWidth < 10 {
			cmdInputWidth = 10
		}
		m.commandInput.Width = cmdInputWidth
		// Render conversation history *above* command input
		// convHeight := availableHeight - 1 // Removed unused variable
		// if convHeight < 1 { convHeight = 1 }
		view.WriteString(renderConversation(m)) // Pass m directly, Render history in remaining space - removed height arg
		view.WriteString("\n")
		view.WriteString(m.commandInput.View()) // Render command input line
		// editorViewHeight = lipgloss.Height(renderConversation(&m)) + 1 + lipgloss.Height(m.commandInput.View()) // Removed unused variable
	}

	// Spinner or Error (Below editor/command input)
	if m.isProcessing && !m.isStreaming {
		view.WriteString("\n" + m.spinner.View() + " Processing...")
	}
	if m.currentErr != nil {
		view.WriteString("\n" + renderError(m)) // Pass m directly
	}

	// Help View
	if helpContent != "" {
		view.WriteString("\n" + helpContent)
	}

	// Status Bar (Bottom)
	statusData := statusbar.StatusData{Mode: "INSERT" /* TODO: Populate */} // Default
	// cursorLine, cursorCol := 0, 0 // Removed unused variables
	if m.mode == modeInsert {
		statusData.Mode = m.editor.InputMode() // Get mode from editor (NORMAL/INSERT if implemented)
		// cursorLine, cursorCol = m.editor.GetCursor() // Removed unused assignment
	} else {
		statusData.Mode = "COMMAND"
		// Get cursor from commandInput if needed, though less common for status line
		// cursorCol = m.commandInput.Position()
	}
	// statusData.CursorLine = cursorLine + 1 // Field removed from statusbar.StatusData
	// statusData.CursorCol = cursorCol + 1   // Field removed from statusbar.StatusData

	view.WriteString("\n")
	view.WriteString(statusbar.Render(m.width, statusData))

	// Trim potentially excessive newlines at the end
	return strings.TrimRight(view.String(), "\n")
}

// renderConversation renders conversation history for editor viewport or above command line.
func renderConversation(m *bubbleModel) string {
	// This function now just prepares the content string for the editor's viewport
	// or direct rendering. Height limiting is handled by the caller (View or editor).
	var lines []string
	// contentWidth := m.editor.ViewportWidth() // Removed ViewportWidth usage

	for _, msg := range m.conversation {
		// Render with model width as fallback, editor width might not be reliable
		rendered := message.Render(msg, m.width-2) // Use message package
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
	content := m.respBuffer.String()
	// contentWidth := max(10, m.editor.ViewportWidth()-lipgloss.Width(prefix)-lipgloss.Width(indicator)-1) // Removed ViewportWidth usage
	contentWidth := max(10, m.width-lipgloss.Width(prefix)-lipgloss.Width(indicator)-3) // Use model width
	renderedContent := style.Width(contentWidth).Render(content)
	lines := strings.Split(renderedContent, "\n")
	if len(lines) > 1 {
		prefixWidth := lipgloss.Width(prefix)
		indent := strings.Repeat(" ", prefixWidth)
		for i := 1; i < len(lines); i++ {
			lines[i] = indent + lines[i]
		}
	}
	return prefix + strings.Join(lines, "\n") + indicator
}

// renderError renders the current error message.
// Receiver changed to pointer for consistency, though not strictly needed here.
func renderError(m *bubbleModel) string {
	if m.currentErr == nil {
		return ""
	}
	errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9")) // Red
	return errStyle.Width(m.width - 2).Render(fmt.Sprintf("Error: %v", m.currentErr))
}

// --- Event Handlers ---

func createEventHandlers() eventHandlers {
	return eventHandlers{
		onCtrlC:      handleCtrlC,
		onCtrlD:      handleCtrlD,
		onCtrlX:      handleCtrlX,
		onCtrlZ:      handleCtrlZ,
		onEnter:      handleEnter,   // Should now be handled primarily by editor
		onUpArrow:    handleUpArrow, // Should now be handled primarily by editor
		onWindowSize: handleWindowSize,
	}
}

// handleUpArrow delegates to editor.
func handleUpArrow(m *bubbleModel) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.debug.Log(" > Up Arrow (delegated to editor)")             // Use m.debug
	m.editor, cmd = m.editor.Update(tea.KeyMsg{Type: tea.KeyUp}) // Assign to declared cmd
	return m, cmd                                                // Return m (pointer)
}

// handleWindowSize updates model and component dimensions.
func handleWindowSize(m *bubbleModel, msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	m.width = msg.Width
	m.height = msg.Height
	m.debug.UpdateDimensions(msg.Width) // Use m.debug
	m.help.SetWidth(msg.Width)
	// Editor and commandInput width/height are set dynamically in View based on available space
	m.debug.Log(" > Window Size: %dx%d", m.width, m.height) // Use m.debug
	return m, nil                                           // Return m (pointer), Trigger re-render
}

// handleCtrlC handles interrupt/exit logic based on mode.
func handleCtrlC(m *bubbleModel) (tea.Model, tea.Cmd) {
	if m.mode == modeCommand { // In command mode, Ctrl+C cancels command and returns to insert
		m.debug.Log(" > Ctrl+C in Command Mode: Switching to Insert Mode") // Use m.debug
		m.mode = modeInsert
		m.commandInput.Blur()
		m.commandInput.Reset()
		m.editor.Focus()
		m.currentErr = nil
		return m, nil // Return m (pointer), Removed Blink
	}

	// In Insert Mode (logic from previous version)
	now := time.Now()
	doublePressDuration := 1 * time.Second

	// 1. Handle cancellation during processing/streaming first
	if m.isProcessing || m.isStreaming {
		m.debug.Log(" > Ctrl+C: Attempting to cancel processing") // Use m.debug
		// TODO: Signal cancellation to ProcessFn goroutine (needs context cancellation)
		m.isProcessing = false
		m.isStreaming = false
		m.respBuffer.Reset()
		m.editor.Focus()
		m.currentErr = nil
		// Send message directly
		return m, msgCmd(addSystemMessageMsg{content: "[Processing cancelled by user]"}) // Return m (pointer)
	}

	// 2. Check if editor has content
	if m.editor.Value() != "" {
		// If editor has content, clear it on first Ctrl+C
		m.debug.Log(" > Ctrl+C: Clearing editor input") // Use m.debug
		m.editor.SetValue("")
		m.interruptCount = 0 // Reset interrupt count after clearing
		m.currentErr = nil   // Clear any previous error hint
	} else {
		// If editor is empty, check for double press to quit
		if now.Sub(m.lastCtrlCTime) < doublePressDuration && m.interruptCount > 0 {
			m.debug.Log(" > Ctrl+C double press on empty editor: Quitting") // Use m.debug
			m.quitting = true
			return m, tea.Quit // Return m (pointer)
		}

		// First Ctrl+C on empty editor: Show hint
		m.debug.Log(" > Ctrl+C on empty editor: Press again to exit") // Use m.debug
		m.currentErr = errors.New("Press Ctrl+C again to exit.")
		m.interruptCount++ // Increment count *only* when showing hint
		m.lastCtrlCTime = now

		// Command to clear the hint after a delay
		clearErrCmd := tea.Tick(doublePressDuration, func(t time.Time) tea.Msg {
			// Need to capture the model state at the time the tick is created
			// or find a way to access the current model state when the tick fires.
			// This closure approach might capture an outdated model.
			// A safer way might involve sending a specific message type that the
			// Update function handles to clear the error if it matches.
			// For now, keeping the original logic, but be aware of potential staleness.
			return func(currentModel tea.Model) tea.Msg { // Closure
				// The closure receives tea.Model, so we still need the type assertion here.
				modelInstance, ok := currentModel.(*bubbleModel)
				if !ok {
					return nil
				} // Should not happen
				if modelInstance.currentErr != nil && modelInstance.currentErr.Error() == "Press Ctrl+C again to exit." {
					return errMsg{nil} // Clear specific exit hint
				}
				return nil
			}(m) // Pass m (pointer) into closure
		})

		return m, clearErrCmd // Return m (pointer)
	}
	return m, nil // Return m (pointer)
}

// handleCtrlD handles EOF/quit logic, delegating to editor first.
func handleCtrlD(m *bubbleModel) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.debug.Log(" > Ctrl+D (delegated to editor)")                  // Use m.debug
	m.editor, cmd = m.editor.Update(tea.KeyMsg{Type: tea.KeyCtrlD}) // Assign to declared cmd

	// Check if editor signaled EOF
	if m.editor.Err != nil && errors.Is(m.editor.Err, io.EOF) { // Use imported io
		m.debug.Log(" > Ctrl+D on empty editor: Quitting") // Use m.debug
		m.quitting = true
		return m, tea.Quit // Return m (pointer)
	}
	// Otherwise, editor handled it (e.g., delete char)
	m.debug.Log(" > Ctrl+D handled by editor") // Use m.debug
	return m, cmd                              // Return m (pointer)
}

// handleCtrlX delegates to the editor.
func handleCtrlX(m *bubbleModel) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.debug.Log(" > Ctrl+X: Delegating to editor")                  // Use m.debug
	m.editor, cmd = m.editor.Update(tea.KeyMsg{Type: tea.KeyCtrlX}) // Assign to declared cmd
	return m, cmd                                                   // Return m (pointer)
}

// handleCtrlZ triggers suspend.
// Note: Suspend might not be meaningful in a WASM/browser context.
func handleCtrlZ(m *bubbleModel) (tea.Model, tea.Cmd) {
	// var cmd tea.Cmd // No command needed for Suspend
	m.debug.Log(" > Ctrl+Z: Suspending (may not work in WASM)") // Use m.debug
	return m, tea.Suspend                                       // Return m (pointer)
}

// handleEnter delegates to the editor.
func handleEnter(m *bubbleModel) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.debug.Log(" > Enter: Delegating to editor")                   // Use m.debug
	m.editor, cmd = m.editor.Update(tea.KeyMsg{Type: tea.KeyEnter}) // Assign to declared cmd

	// Check if editor completed input
	if m.editor.InputIsComplete() && m.editor.Err == nil {
		submittedInput := m.editor.Value() // Get value *before* editor resets
		m.lastInput = submittedInput       // Store last successful input
		// Send submitBufferMsg as a message
		cmd = tea.Batch(cmd, msgCmd(submitBufferMsg{input: submittedInput, clearEditor: false}))
		// Editor should reset itself
	}
	return m, cmd // Return m (pointer)
}

// max returns the larger of x or y.
// Added helper as math.Max requires float64
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

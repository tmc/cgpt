package editor

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea" // Use standard textarea
	tea "github.com/charmbracelet/bubbletea"

	// Use local paths
	// "github.com/tmc/cgpt/interactive" // Removed to break import cycle
	"github.com/tmc/cgpt/ui/completion"

	// "github.com/tmc/cgpt/ui/debug" // Removed, handled differently
	// "github.com/tmc/cgpt/ui/editor/internal/textarea" // Removed internal import
	"github.com/tmc/cgpt/ui/keymap"
)

// --- Interfaces ---
// These types are expected by the editor from the parent session
type AutoCompleteFn func(entireInput [][]rune, line, col int) (msg string, comp Completions)
type Completions interface {
	completion.Values
	Candidate(e completion.Entry) Candidate
}
type Candidate interface {
	Replacement() string
	MoveRight() int
	DeleteLeft() int
}

// --- Errors ---
var ErrInputAborted = errors.New("input aborted") // Specific error for Ctrl+C clear

// --- Messages ---
// Messages emitted by the editor model
type InputCompleteMsg struct{}      // Input is ready for processing
type RequestCompletionsMsg struct { // Request parent to run AutoCompleteFn
	Runes [][]rune
	Line  int
	Col   int
}
type ApplyCompletionMsg struct { // Tell parent a completion was applied (optional)
	Completion Candidate
}
type SwitchToCommandModeMsg struct{} // Tell parent to switch mode

// editorFinishedMsg holds the content and error from the external editor.
type editorFinishedMsg struct {
	content string
	err     error
}

// --- Model ---

const (
	minHeight = 1 // Minimum lines for the editor/viewport
)

// Model is the Bubble Tea model for the enhanced input editor.
type Model struct {
	textarea textarea.Model // Underlying text area - Reverted alias
	keyMap   keymap.KeyMap

	// History
	history         []string
	historyCursor   int    // -1 means not navigating
	valueBeforeHist string // Value before starting history nav

	// Completion
	completion     completion.Model // Completion UI component
	showCompletion bool             // Whether completion UI is active
	autoCompleteFn AutoCompleteFn   // Callback to get completions

	// Editor State
	checkInputCompleteFn  func(entireInput [][]rune, line, col int) bool
	lastInput             string // Last submitted line (for recall)
	maxHistorySize        int
	dedupHistory          bool
	externalEditorExt     string
	externalEditorEnabled bool

	// Viewport (managed by textarea, but we need dimensions)
	width  int
	height int

	// Error state specific to editor operations
	Err error

	// Internal state
	inputCompleted bool // Flag set when Enter/Ctrl+D signals completion
}

// DefaultSingleLineHint defines the default placeholder text.
const DefaultSingleLineHint = `Enter prompt (""" for multi-line, ESC for command mode)`

// New creates a new editor model.
func New() Model {
	ta := textarea.New()                   // Use the adapted internal textarea - Reverted alias
	ta.Placeholder = DefaultSingleLineHint // Use local default
	ta.Prompt = "> "
	ta.ShowLineNumbers = false // Keep it simple for REPL

	km := keymap.DefaultKeyMap() // Get default keymap

	// Configure textarea's internal keymap using our KeyMap struct
	// TODO: Review keymap assignments based on actual keymap.KeyMap fields
	ta.KeyMap = textarea.KeyMap{ // Reverted alias
		// CharacterBackward: km.Left, CharacterForward: km.Right, // Assuming these might not exist directly
		// DeleteAfterCursor: km.DeleteLineEnd, DeleteBeforeCursor: km.DeleteLineStart,
		// DeleteCharacterBackward: km.DeleteCharBackward, DeleteCharacterForward: km.DeleteCharForward,
		// DeleteWordBackward: km.DeleteWordBackward, DeleteWordForward: km.DeleteWordForward,
		InsertNewline: km.Submit, // Enter key initially mapped to submit/newline logic
		// LineEnd:       km.End, LineNext: km.Down, LinePrevious: km.Up, LineStart: km.Home,
		// Paste: km.Paste, WordBackward: km.WordBackward, WordForward: km.WordForward,
		// InputBegin: km.Top, InputEnd: km.Bottom,
	}

	compModel := completion.New(km) // Pass keymap

	return Model{
		textarea:       ta,
		keyMap:         km,
		completion:     compModel,
		historyCursor:  -1,
		maxHistorySize: 1000,
		dedupHistory:   true,
	}
}

// --- Getters / Setters ---

func (m *Model) SetValue(s string)  { m.textarea.SetValue(s); m.resetHistoryNavigation() } // Reset history nav on set
func (m Model) Value() string       { return m.textarea.Value() }
func (m Model) ViewportWidth() int  { return m.textarea.Width() }
func (m Model) ViewportHeight() int { return m.textarea.Height() }
func (m *Model) SetWidth(w int)     { m.textarea.SetWidth(w) /* ; m.completion.SetWidth(w) */ }  // Update textarea, completion width might be handled internally or differently
func (m *Model) SetHeight(h int)    { m.textarea.SetHeight(h); m.recalculateCompletionHeight() } // Update textarea height
// func (m *Model) SetViewportContent(s string) { m.textarea.Viewport().SetContent(s) } // Removed, textarea manages viewport
// func (m *Model) GotoBottom()                 { m.textarea.Viewport().GotoBottom() } // Removed, textarea manages viewport
func (m Model) Line() int             { return m.textarea.Line() }
func (m Model) CursorCol() int        { return m.textarea.LineInfo().ColumnOffset }  // Use LineInfo().ColumnOffset
func (m Model) ValueRunes() [][]rune  { return runesFromString(m.textarea.Value()) } // Use Value() and convert
func (m Model) Multiline() bool       { return m.textarea.LineCount() > 1 }
func (m Model) InputIsComplete() bool { return m.inputCompleted }
func (m *Model) Focus() tea.Cmd       { m.Err = nil; return m.textarea.Focus() } // Clear error on focus
func (m *Model) Blur()                { m.textarea.Blur() }
func (m *Model) Reset() {
	m.textarea.Reset()
	m.resetHistoryNavigation()
	m.completion.Reset()
	m.showCompletion = false
	m.inputCompleted = false
	m.Err = nil
}
func (m *Model) SetHistory(h []string)     { m.history = h; m.resetHistoryNavigation() }
func (m *Model) GetHistory() []string      { return m.history }
func (m *Model) SetLastInput(input string) { m.lastInput = input }
func (m *Model) GetCursor() (int, int)     { return m.textarea.Line(), m.textarea.LineInfo().ColumnOffset } // Use Line() and LineInfo().ColumnOffset
func (m *Model) InputMode() string         { return "INSERT" }                                              // Placeholder, more modes needed for full Vim

// SetExternalEditorEnabled configures the external editor keybinding.
func (m *Model) SetExternalEditorEnabled(enable bool, extension string) {
	m.keyMap.Editor.SetEnabled(enable)
	m.externalEditorEnabled = enable
	m.externalEditorExt = extension
	if extension == "" {
		m.externalEditorExt = "txt"
	}
}

// recalculateCompletionHeight adjusts completion list height based on editor height.
// recalculateCompletionHeight adjusts completion list height based on editor height.
func (m *Model) recalculateCompletionHeight() {
	// Example: Max 1/3 of editor height, minimum 3 lines
	// compHeight := clamp(m.textarea.Height()/3, 3, 10)
	// m.completion.SetHeight(compHeight) // Assuming completion handles its own height or doesn't need explicit setting
}

// --- Main Update Logic ---

func (m Model) Init() tea.Cmd { return nil } // Textarea doesn't need Init

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	// Reset completion flag for next cycle
	m.inputCompleted = false
	m.Err = nil // Clear previous error on new message

	// 1. Handle Completion Model first if it's active
	if m.showCompletion {
		// Update returns tea.Model, which is the updated completion model
		_, compCmd := m.completion.Update(msg) // Use blank identifier for updatedCompModel
		// Assign back if the type is indeed completion.Model
		// if cm, ok := updatedCompModel.(completion.Model); ok { // Commented out due to compiler error
		// 	m.completion = cm
		// } // TODO: Handle case where it's not completion.Model?
		cmds = append(cmds, compCmd)

		compErr := m.completion.Error()
		selectedComp := m.completion.Selected()

		if compErr != nil { // Completion was cancelled or errored
			m.showCompletion = false
			m.completion.Reset()
			m.Err = compErr // Store completion error if any
			// Don't consume the key msg, let textarea potentially handle Esc/Ctrl+C
		} else if selectedComp != nil { // Completion was selected
			// Assuming completion.Values() returns the interface directly or has a method
			// If Values is unexported 'values', this needs adjustment based on actual API
			// valuesInterface := m.completion.Values() // Commented out: .Values() seems undefined
			// m.applyCompletion(valuesInterface.Candidate(selectedComp)) // Depends on Values()
			m.showCompletion = false
			m.completion.Reset()
			// Consume the key press (Enter/Tab) that selected the completion
			return m, tea.Batch(cmds...) // Return immediately
		} else {
			// Completion still active, consume the message
			return m, tea.Batch(cmds...)
		}
	}

	// 2. Handle Editor-Specific Keybindings & Messages
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keyMap.RecallLast): // Up Arrow on Empty Line
			if m.textarea.Value() == "" && m.textarea.Line() == 0 && m.lastInput != "" {
				return m.navigateHistory(false) // Navigate up
			}
			// Otherwise, fall through to textarea's Up key handling

		// case key.Matches(msg, m.keyMap.HistoryPrevious): // Explicit History Up - TODO: Re-enable with correct keymap field
		// 	return m.navigateHistory(false)
		// case key.Matches(msg, m.keyMap.HistoryNext): // Explicit History Down - TODO: Re-enable with correct keymap field
		// 	return m.navigateHistory(true)

		// case key.Matches(msg, m.keyMap.AutoComplete): // Tab - TODO: Re-enable with correct keymap field
		// 	if m.autoCompleteFn != nil {
		// 		// Use GetCursor helper method
		// 		row, col := m.GetCursor()
		// 		return m, RequestCompletionsCmd(runesFromString(m.textarea.Value()), row, col)
		// 	}
		// If no handler, fall through to textarea (might insert tab char)

		case key.Matches(msg, m.keyMap.Editor): // Ctrl+X
			return m.launchExternalEditor()

		case key.Matches(msg, m.keyMap.Submit): // Enter
			if m.shouldSubmitInput() {
				m.inputCompleted = true
				m.Err = nil // Clear error on successful submit intent
				// Value is retrieved by the parent model after this Update returns
				// We don't return tea.Quit here, parent decides that
				return m, func() tea.Msg { return InputCompletionCmd() } // Wrap in Cmd
			}
			// If not submitting, fall through to let textarea handle newline insertion

		case key.Matches(msg, m.keyMap.Quit): // Ctrl+D
			// Check if editor is truly empty before signalling EOF
			// Use GetCursor helper method
			row, col := m.GetCursor()
			if m.textarea.Value() == "" && row == 0 && col == 0 {
				m.Err = io.EOF                                           // Signal EOF for parent to handle quit
				m.inputCompleted = true                                  // Mark as complete for parent check
				return m, func() tea.Msg { return InputCompletionCmd() } // Wrap in Cmd
			}
			// Otherwise, fall through to let textarea handle (delete forward)

		case key.Matches(msg, m.keyMap.Interrupt): // Ctrl+C
			// If editor is empty, signal abort immediately
			if m.textarea.Value() == "" {
				m.Err = ErrInputAborted
				m.inputCompleted = true
				return m, func() tea.Msg { return InputCompletionCmd() } // Wrap in Cmd
			}
			// Otherwise, clear the editor content
			m.textarea.Reset()
			m.resetHistoryNavigation()
			return m, nil // Consume the key

			// case key.Matches(msg, m.keyMap.EnterCommandMode): // Escape
			// 	return m, func() tea.Msg { return SwitchToCommandModeMsg{} } // Signal parent
		}

	// Handle completion request from ourselves (e.g., after Tab)
	case RequestCompletionsMsg:
		if m.autoCompleteFn != nil {
			// Note: msg.Runes was generated using runesFromString, which might differ from internal textarea state if conversion isn't perfect.
			compMsg, comps := m.autoCompleteFn(msg.Runes, msg.Line, msg.Col)
			// hasPrefill, mvR, delL, prefill, newComps := computePrefill(comps) // TODO: Re-enable computePrefill
			hasPrefill := false // Assume no prefill for now

			// if hasPrefill { // TODO: Re-enable computePrefill
			// 	m.textarea.CursorRight(mvR)
			// 	m.textarea.DeleteCharactersBackward(delL)
			// 	m.textarea.InsertString(prefill)
			// 	// Use adjusted completions if prefill occurred
			// 	if newComps != nil {
			// 		comps = newComps
			// 	}
			// }

			// Show completion UI if there are suitable candidates
			if comps != nil && (comps.NumCategories() > 1 || (comps.NumCategories() == 1 && comps.NumEntries(0) > 1) || (comps.NumCategories() == 1 && comps.NumEntries(0) == 1 && !hasPrefill)) {
				m.completion.SetValues(compMsg, comps) // Assuming SetValues exists
				m.showCompletion = true
				m.recalculateCompletionHeight() // Adjust layout for completion list
			} else {
				m.showCompletion = false
				m.completion.Reset()            // Hide if no/single completion after prefill
				m.recalculateCompletionHeight() // Adjust layout back
			}
		}

	// Handle result from external editor
	case editorFinishedMsg: // Assuming this type is defined correctly within the package
		if msg.err != nil {
			m.Err = fmt.Errorf("editor failed: %w", msg.err) // Store error
		} else {
			m.textarea.SetValue(msg.content) // Apply content
		}
		// Try using textarea's Blink command if available
		cmds = append(cmds, m.textarea.Focus()) // Refocus, removed Blink
	}

	// 3. Pass message to underlying textarea if not handled above
	m.textarea, cmd = m.textarea.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// shouldSubmitInput checks if the current state warrants input completion.
func (m *Model) shouldSubmitInput() bool {
	// Use custom check function if provided
	if m.checkInputCompleteFn != nil {
		// Use GetCursor helper method
		row, col := m.GetCursor()
		return m.checkInputCompleteFn(runesFromString(m.textarea.Value()), row, col)
	}

	// Default logic: Submit only if the cursor is on the last line of a non-empty buffer.
	// This prevents submitting when adding newlines in the middle of input.
	isLastLine := m.textarea.Line() == m.textarea.LineCount()-1
	bufferNotEmpty := m.textarea.Value() != ""

	return isLastLine && bufferNotEmpty
}

// navigateHistory moves up or down in the history.
func (m *Model) navigateHistory(down bool) (Model, tea.Cmd) {
	if len(m.history) == 0 {
		return *m, nil
	} // No history

	if m.historyCursor == -1 { // Entering history navigation
		m.valueBeforeHist = m.textarea.Value() // Store current value
	}

	if down {
		m.historyCursor++
	} else {
		m.historyCursor--
	}

	// Clamp cursor
	m.historyCursor = clamp(m.historyCursor, -1, len(m.history)-1)

	newValue := m.valueBeforeHist // Default to original value
	if m.historyCursor != -1 {    // If navigating within history
		newValue = m.history[m.historyCursor]
	}

	m.textarea.SetValue(newValue)
	m.textarea.CursorEnd() // Move cursor to end after setting value
	return *m, nil
}

// launchExternalEditor prepares and returns the command to launch $EDITOR.
func (m *Model) launchExternalEditor() (Model, tea.Cmd) {
	if !m.externalEditorEnabled {
		return *m, tea.Println("External editor is disabled.")
	}

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim"
	}
	currentInput := m.textarea.Value()

	tmpDir := os.TempDir()
	pattern := "cgpt-edit-*."
	if m.externalEditorExt != "" {
		pattern += m.externalEditorExt
	}
	tmpFile, err := os.CreateTemp(tmpDir, pattern)
	if err != nil {
		m.Err = fmt.Errorf("temp file: %w", err)
		return *m, nil
	}
	tempFilePath := tmpFile.Name()

	if _, err := tmpFile.WriteString(currentInput); err != nil {
		tmpFile.Close()
		os.Remove(tempFilePath)
		m.Err = fmt.Errorf("write temp: %w", err)
		return *m, nil
	}
	if err := tmpFile.Close(); err != nil {
		os.Remove(tempFilePath)
		m.Err = fmt.Errorf("close temp: %w", err)
		return *m, nil
	}

	editorCmd := createEditorCmd(tempFilePath) // Use shared helper

	m.textarea.Blur() // Blur editor while external editor runs
	// Callback will send editorFinishedMsg
	return *m, tea.ExecProcess(editorCmd, editorCallback(tempFilePath)) // Removed debug logger
}

// View renders the editor and potentially the completion menu.
func (m Model) View() string {
	var b strings.Builder
	b.WriteString(m.textarea.View()) // Render the core text area

	if m.showCompletion {
		b.WriteString("\n")
		// Render completion menu below, potentially adjusting position/style
		// based on available space and cursor position (more advanced).
		// For now, just render it directly.
		b.WriteString(m.completion.View())
	}
	return b.String()
}

// --- Command Creators ---
func InputCompletionCmd() tea.Msg { return InputCompleteMsg{} }

// runesFromString converts a string to [][]rune, handling newlines.
// Note: This might not perfectly replicate internal textarea state for complex inputs.
func runesFromString(s string) [][]rune {
	lines := strings.Split(s, "\n")
	runes := make([][]rune, len(lines))
	for i, line := range lines {
		runes[i] = []rune(line)
	}
	return runes
}

// AddHistory adds an entry if autosave is off or manually called.
func (m *Model) AddHistory(line string) {
	if m.maxHistorySize > 0 && len(m.history) >= m.maxHistorySize {
		copy(m.history, m.history[1:])
		m.history = m.history[:m.maxHistorySize-1]
	}
	if !(m.dedupHistory && len(m.history) > 0 && m.history[len(m.history)-1] == line) {
		m.history = append(m.history, line)
	}
	m.resetHistoryNavigation() // Reset nav cursor after adding
}

// resetHistoryNavigation resets the history cursor.
func (m *Model) resetHistoryNavigation() {
	m.historyCursor = -1
	m.valueBeforeHist = ""
}

// --- Helpers ---
// (clamp, min, max remain the same)
func clamp(v, low, high int) int {
	if high < low {
		low, high = high, low
	}
	return min(high, max(low, v))
}
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// editorCallback handles the result of the external editor process.
func editorCallback(tempFilePath string) func(error) tea.Msg {
	return func(execErr error) tea.Msg {
		// debug.Log(" > Editor process finished") // Logging removed
		defer func() { // Ensure cleanup runs
			err := os.Remove(tempFilePath)
			if err != nil && !os.IsNotExist(err) {
				// debug.Log(" > Failed to remove editor temp file %s: %v", tempFilePath, err) // Logging removed
				fmt.Fprintf(os.Stderr, "Warning: Failed to remove editor temp file %s: %v\n", tempFilePath, err) // Basic stderr log
			} // else if err == nil {
			// debug.Log(" > Removed editor temp file %s", tempFilePath) // Logging removed
			// }
		}()
		if execErr != nil {
			// debug.Log(" > Editor command execution failed: %v", execErr) // Logging removed
			return editorFinishedMsg{err: fmt.Errorf("editor command failed: %w", execErr)}
		}
		// debug.Log(" > Reading editor temp file %s", tempFilePath) // Logging removed
		contentBytes, readErr := os.ReadFile(tempFilePath)
		if readErr != nil {
			// debug.Log(" > Failed to read editor temp file: %v", readErr) // Logging removed
			return editorFinishedMsg{err: fmt.Errorf("read editor temp file: %w", readErr)}
		}
		editedContent := strings.TrimSuffix(string(contentBytes), "\n")
		// debug.Log(" > Read %d bytes from editor", len(editedContent)) // Logging removed
		return editorFinishedMsg{content: editedContent}
	}
}

// Placeholder for createEditorCmd helper logic
func createEditorCmd(tempFilePath string) *exec.Cmd {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim"
	}
	quotedPath := "'" + strings.ReplaceAll(tempFilePath, "'", "'\\''") + "'"
	fullCmd := editor + " " + quotedPath
	cmd := exec.Command("sh", "-c", fullCmd)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}

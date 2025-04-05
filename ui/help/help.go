package help

import (
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/tmc/cgpt/ui/keymap" // Use local keymap
)

// Ensure Model implements help.KeyMap
var _ help.KeyMap = (*Model)(nil)

// Model wraps the bubbles/help model for integration.
type Model struct {
	inner  help.Model
	keyMap keymap.KeyMap // Keep a reference to the main keymap
	Show   bool          // Whether the help view is currently visible
}

// New creates a new help model.
func New(mainKeyMap keymap.KeyMap) Model {
	h := help.New()
	h.ShowAll = false // Start with short help
	return Model{
		inner:  h,
		keyMap: mainKeyMap,
		Show:   false, // Hidden by default
	}
}

// Init does nothing.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles messages, primarily toggling help visibility.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Toggle help visibility on '?'
		if key.Matches(msg, m.keyMap.ToggleHelp) {
			m.Show = !m.Show
			// Optionally toggle between short/full help
			// m.inner.ShowAll = !m.inner.ShowAll
		}
	case tea.WindowSizeMsg:
		// Update the width of the underlying help model
		m.inner.Width = msg.Width
	}
	// Note: The underlying help.Model doesn't have stateful updates
	// other than ShowAll and Width.
	return m, nil
}

// View renders the help.
func (m Model) View() string {
	if !m.Show {
		return ""
	}
	// Pass the keymap (which implements help.KeyMap via ShortHelp/FullHelp)
	return m.inner.View(m)
}

// ShortHelp returns the bindings for the short help view.
// It delegates to the main keymap.
func (m Model) ShortHelp() []key.Binding {
	// Selectively choose which keys to show in short help
	// This requires the main keymap to have a method or logic for this,
	// or we define it here based on the fields.
	// Selectively choose which keys to show in short help
	return []key.Binding{
		m.keyMap.Submit,
		m.keyMap.Editor,
		m.keyMap.Interrupt,
		m.keyMap.Quit,
		m.keyMap.ToggleHelp,
	}
	// Alternatively, if keymap.KeyMap implemented help.KeyMap:
	// return m.keyMap.ShortHelp()
}

// FullHelp returns the bindings for the full help view.
// It delegates to the main keymap, grouping bindings logically.
func (m Model) FullHelp() [][]key.Binding {
	// Group bindings logically for the full help view
	// Group bindings logically for the full help view based on keymap.KeyMap
	return [][]key.Binding{
		{ // Navigation / Scrolling
			m.keyMap.Up, m.keyMap.Down, m.keyMap.PageUp, m.keyMap.PageDown, m.keyMap.Top, m.keyMap.Bottom,
			// m.keyMap.Left, m.keyMap.Right, // These might be editor-internal
		},
		{ // Input Actions
			m.keyMap.Submit, m.keyMap.RecallLast, m.keyMap.Editor,
		},
		{ // App Control
			m.keyMap.Interrupt, m.keyMap.Quit, m.keyMap.Suspend, m.keyMap.ToggleHelp,
		},
	}
	// Alternatively, if keymap.KeyMap implemented help.KeyMap:
	// return m.keyMap.FullHelp()
}

// SetWidth updates the width for the help view.
func (m *Model) SetWidth(w int) {
	m.inner.Width = w
}

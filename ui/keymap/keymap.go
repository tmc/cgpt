package keymap

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines the application's keybindings.
// Using bubbles/key allows for help generation and context-aware enabling.
type KeyMap struct {
	// Navigation / Basic Editing (delegated to editor/viewport)
	Up       key.Binding
	Down     key.Binding
	PageUp   key.Binding
	PageDown key.Binding
	Top      key.Binding
	Bottom   key.Binding
	Left     key.Binding // Ctrl+Left
	Right    key.Binding // Ctrl+Right

	// Input Actions
	Submit     key.Binding // Enter (context-dependent: submit/newline)
	RecallLast key.Binding // Up arrow on empty input
	Editor     key.Binding // Ctrl+X
	// Paste handled by terminal/editor component

	// Application Control
	Quit       key.Binding // Ctrl+D on empty / Second Ctrl+C
	Interrupt  key.Binding // Ctrl+C during processing / clear input
	Suspend    key.Binding // Ctrl+Z
	ToggleHelp key.Binding // ?
}

// DefaultKeyMap returns a default set of key bindings focusing on session control.
// Editor-specific keys (like word navigation, deletion) are managed by the editor component itself.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		// Viewport / Scrolling (Names match viewport's default for easy delegation)
		Up:       key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		Down:     key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
		PageUp:   key.NewBinding(key.WithKeys("pgup", "b"), key.WithHelp("pgup/b", "page up")),
		PageDown: key.NewBinding(key.WithKeys("pgdown", "f", " "), key.WithHelp("pgdn/f/space", "page down")),
		Top:      key.NewBinding(key.WithKeys("home"), key.WithHelp("home", "go to top")),
		Bottom:   key.NewBinding(key.WithKeys("end"), key.WithHelp("end", "go to bottom")),
		// HalfPageUp/Down often overlap with editor keys, delegate to viewport if needed

		// Input Actions
		Submit:     key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "submit / newline")),
		RecallLast: key.NewBinding(key.WithKeys("up"), key.WithHelp("↑ (empty)", "recall last")),
		Editor:     key.NewBinding(key.WithKeys("ctrl+x"), key.WithHelp("ctrl+x", "open editor")),

		// Application Control
		Interrupt: key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("ctrl+c", "interrupt / clear / quit")),
		Quit:      key.NewBinding(key.WithKeys("ctrl+d"), key.WithHelp("ctrl+d (empty)", "quit")),
		Suspend:   key.NewBinding(key.WithKeys("ctrl+z"), key.WithHelp("ctrl+z", "suspend")),

		// Help
		ToggleHelp: key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "toggle help")),
	}
}

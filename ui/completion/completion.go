package completion

import (
	// Using original bubbles/list for the completion menu view
	"errors"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	// For debug
	"github.com/tmc/cgpt/ui/keymap" // Use local keymap
)

// --- Interfaces (Mirrors bubbline/complete/complete.go) ---

// Values provides the data for the completion menu.
type Values interface {
	NumCategories() int
	CategoryTitle(catIdx int) string
	NumEntries(catIdx int) int
	Entry(catIdx, entryIdx int) Entry
}

// Entry represents a single item shown in the completion menu.
type Entry interface {
	Title() string       // The primary text displayed for the item.
	Description() string // Additional descriptive text shown below the list.
}

// --- Model ---

// Model holds the state for the completion suggestion UI.
type Model struct {
	list   list.Model    // Use bubbles/list to render candidates
	keyMap keymap.KeyMap // Reference to main keymap for context

	completionMsg string // Informational message (e.g., "Completing '...'")
	active        bool   // Is the completion view currently visible and active?
	values        Values // The current set of completion values being displayed
	selectedEntry Entry  // The entry chosen by the user
	err           error  // Stores errors like io.EOF or cancellation
	width, height int    // Dimensions for layout

	// TODO: Add styles if needed
}

// New creates a new completion model.
func New(appKeyMap keymap.KeyMap) Model {
	l := list.New([]list.Item{}, NewItemDelegate(), 0, 0) // Start empty
	l.SetShowHelp(false)                                  // Usually don't need list's built-in help
	l.SetShowTitle(false)                                 // Titles managed externally if needed
	l.SetShowStatusBar(false)                             // No status bar needed
	l.SetShowPagination(true)                             // Show pagination if list is long
	l.SetFilteringEnabled(false)                          // Filtering handled externally by editor/AutoCompleteFn

	// TODO: Customize list styles (Title, Pagination, Item styles)
	// Use lipgloss.AdaptiveColor for theme support

	return Model{
		list:   l,
		keyMap: appKeyMap, // Use shared keymap
		active: false,
	}
}

// --- Item Delegate for bubbles/list ---

// completionItem wraps our completion Entry to satisfy list.Item.
type completionItem struct{ Entry }

func (i completionItem) FilterValue() string { return i.Title() } // Basic filter on title

// NewItemDelegate creates a delegate for rendering completion items.
func NewItemDelegate() list.DefaultDelegate {
	d := list.NewDefaultDelegate()

	// Customize how selected vs normal items look
	// TODO: Use theme colors
	d.Styles.SelectedTitle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("212")). // Example selection color
		Background(lipgloss.Color("236")).
		PaddingLeft(1)
	d.Styles.NormalTitle = lipgloss.NewStyle().PaddingLeft(1)

	// Hide descriptions in the list itself (show below)
	d.ShowDescription = false
	d.SetHeight(1) // Single line per item

	return d
}

// --- Core Methods ---

// IsActive returns true if the completion view is visible.
func (m Model) IsActive() bool { return m.active }

// Reset hides the view and clears selection/error state.
func (m *Model) Reset() {
	m.active = false
	m.selectedEntry = nil
	m.err = nil
	m.completionMsg = ""
	// Don't reset list items here, SetValues does that
}

// SetValues updates the list with new completion candidates.
func (m *Model) SetValues(msg string, v Values) {
	m.Reset() // Reset state before showing new values
	m.values = v
	m.completionMsg = msg

	if v == nil || v.NumCategories() == 0 || (v.NumCategories() == 1 && v.NumEntries(0) == 0) {
		// No values to show, remain inactive
		m.list.SetItems([]list.Item{})
		return
	}

	// For simplicity, only handle the first category for now
	// TODO: Implement multi-category support if needed (like tabs or sections)
	catIdx := 0
	numEntries := v.NumEntries(catIdx)
	listItems := make([]list.Item, numEntries)
	for i := 0; i < numEntries; i++ {
		listItems[i] = completionItem{v.Entry(catIdx, i)}
	}

	m.list.SetItems(listItems)
	m.active = true  // Activate the view
	m.list.Select(0) // Select the first item by default
	// m.list.Title = v.CategoryTitle(catIdx) // Set title if showing it
}

// Selected returns the entry chosen by the user, or nil if none/cancelled.
func (m Model) Selected() Entry { return m.selectedEntry }

// Error returns any error that occurred during selection (e.g., cancellation).
func (m Model) Error() error { return m.err }

// --- Bubble Tea Interface ---

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !m.active {
		return m, nil
	} // Ignore updates if not active

	var cmds []tea.Cmd
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle keys specifically for completion navigation/selection
		switch {
		// Selection/Acceptance
		case key.Matches(msg, m.keyMap.Submit): // Use main submit key
			selected := m.list.SelectedItem()
			if selected != nil {
				m.selectedEntry = selected.(completionItem).Entry
				m.err = nil // Clear any previous error on selection
			} else {
				// Handle case where Enter is pressed but nothing is selectable
				// (e.g., list is empty after filtering) - maybe just reset?
				m.err = errors.New("no completion selected")
			}
			m.active = false // Deactivate on selection/error
			return m, nil    // No further command needed from here

		// Cancellation
		case key.Matches(msg, m.keyMap.Interrupt): // Use main interrupt key
			m.err = errors.New("completion cancelled") // Set error state
			m.active = false
			return m, nil

		// Navigation (delegate to list)
		// Let the list handle its default navigation keys (Up, Down, PgUp, PgDown, Home, End)
		default:
			m.list, cmd = m.list.Update(msg)
			cmds = append(cmds, cmd)
		}
	case tea.WindowSizeMsg:
		// Update list dimensions
		m.width = msg.Width // Store full width if needed elsewhere
		// Calculate list height/width based on available space or fixed size
		listHeight := min(10, msg.Height/3) // Example: max 10 lines or 1/3 screen
		listWidth := min(60, msg.Width-4)   // Example: max 60 chars wide
		m.list.SetSize(listWidth, listHeight)
	}

	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	if !m.active || m.values == nil {
		return ""
	}

	var b strings.Builder

	// 1. Optional Completion Message
	if m.completionMsg != "" {
		// TODO: Style this message
		style := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
		b.WriteString(style.Render(m.completionMsg))
		b.WriteString("\n")
	}

	// 2. Render the List
	b.WriteString(m.list.View())

	// 3. Render Description of Selected Item
	selected := m.list.SelectedItem()
	if selected != nil {
		desc := selected.(completionItem).Entry.Description()
		if desc != "" {
			// TODO: Style and wrap description
			descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Italic(true)
			// Ensure description wraps within the available width
			descWidth := m.list.Width() // Use list width for consistency
			wrappedDesc := lipgloss.NewStyle().Width(descWidth).Render(desc)
			b.WriteString("\n")
			b.WriteString(descStyle.Render(wrappedDesc))
		}
	}

	return b.String()
}

// Helper for min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// --- Interface Implementations for list.Item ---
// (completionItem already defined and implements FilterValue)

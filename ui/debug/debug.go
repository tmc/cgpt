package debug

import (
	"fmt"
	"os" // For checking DEBUG_UI env var
	"reflect"
	"strings"

	// Need to import to get type for filtering - Removed spinner
	// Need for filtering - Removed textinput
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	// Import message types from interactive package to resolve undefined types
	// "github.com/tmc/cgpt/interactive" // Removed import to break cycle
)

// DebugView is a component for displaying debug information.
type DebugView struct {
	log           strings.Builder
	events        []string
	eventCounter  int
	maxEvents     int
	ignoredEvents map[string]bool // Store type names as strings
	width         int
	columnWidth   int
	Visible       bool // Control visibility externally
}

// NewView creates a new debug view component.
func NewView() *DebugView {
	// Use string literals for type names to avoid import issues and undefined types
	ignored := map[string]bool{
		"spinner.TickMsg":        true,
		"textinput.BlinkMsg":     true,
		"tea.FrameMsg":           true,
		"tea.SequenceMsg":        true, // Internal Bubble Tea message
		"tea.UntypedMsg":         true,
		"tea.SuspendMsg":         true,
		"tea.ResumeMsg":          true,
		"tea.ClearScrollAreaMsg": true,
		// Add pointer versions if needed
		"*spinner.TickMsg":    true,
		"*textinput.BlinkMsg": true,
		// Add other common noisy messages as needed
	}

	return &DebugView{
		maxEvents:     5,
		ignoredEvents: ignored,
		width:         80,
		columnWidth:   35,
		Visible:       os.Getenv("DEBUG_UI") == "1",
	}
}

// AddEvent adds an event, respecting filtering and capping.
func (dv *DebugView) AddEvent(msg tea.Msg) {
	if !dv.Visible {
		return
	}

	msgTypeStr := reflect.TypeOf(msg).String()
	if dv.ignoredEvents[msgTypeStr] {
		return
	}

	displayType := msgTypeStr
	if dotIndex := strings.LastIndex(msgTypeStr, "."); dotIndex != -1 {
		displayType = msgTypeStr[dotIndex+1:]
	}
	if strings.HasPrefix(displayType, "*") {
		displayType = displayType[1:]
	}

	eventStr := fmt.Sprintf("%s", displayType)
	// Optional value preview (truncated)
	// eventValStr := fmt.Sprintf("%v", msg) ...

	if dv.columnWidth > 3 && lipgloss.Width(eventStr) > dv.columnWidth {
		runes := []rune(eventStr)
		if len(runes) > dv.columnWidth-3 {
			eventStr = string(runes[:dv.columnWidth-3]) + "..."
		}
	}

	dv.eventCounter++
	eventWithID := fmt.Sprintf("%04d:%s", dv.eventCounter, eventStr)
	dv.events = append(dv.events, eventWithID)

	if len(dv.events) > dv.maxEvents {
		dv.events = dv.events[len(dv.events)-dv.maxEvents:]
	}
}

// Log adds a log entry.
func (dv *DebugView) Log(format string, args ...interface{}) {
	if !dv.Visible {
		return
	}
	logEntry := fmt.Sprintf(format, args...)
	dv.log.WriteString(logEntry)
	if !strings.HasSuffix(logEntry, "\n") {
		dv.log.WriteString("\n")
	}
}

// UpdateDimensions updates the width parameters.
func (dv *DebugView) UpdateDimensions(width int) {
	if width <= 0 {
		return
	}
	dv.width = width
	dv.columnWidth = width/2 - 4
	if dv.columnWidth < 20 {
		dv.columnWidth = 20
	}
}

// View renders the debug view if visible.
func (dv *DebugView) View() string {
	if !dv.Visible || dv.width < 40 {
		return ""
	}

	logSection := ""
	eventsSection := ""
	logStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Width(dv.columnWidth).Align(lipgloss.Left).Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("238")).Padding(0, 1)

	if dv.log.Len() > 0 {
		allLogLines := strings.Split(strings.TrimRight(dv.log.String(), "\n"), "\n")
		maxLogLines := 10
		startIdx := 0
		if len(allLogLines) > maxLogLines {
			startIdx = len(allLogLines) - maxLogLines
		}
		linesToShow := allLogLines[startIdx:]
		var formattedLines []string
		for _, line := range linesToShow {
			if dv.columnWidth > 3 && lipgloss.Width(line) > dv.columnWidth {
				runes := []rune(line)
				line = string(runes[:dv.columnWidth-3]) + "..."
			}
			formattedLines = append(formattedLines, line)
		}
		logSection = logStyle.Render("Logs:\n" + strings.Join(formattedLines, "\n"))
	}

	if len(dv.events) > 0 {
		eventsSection = logStyle.Render("Events:\n" + strings.Join(dv.events, "\n"))
	}

	if logSection != "" && eventsSection != "" {
		return lipgloss.JoinHorizontal(lipgloss.Top, logSection, "  ", eventsSection)
	}
	return logSection + eventsSection
}

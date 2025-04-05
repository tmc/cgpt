package message

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// Msg represents a message in the conversation.
type Msg struct {
	// UUID    string    `json:"uuid"` // Consider adding if needed for unique identification
	Type    MsgType   `json:"type"` // user, assistant, system
	Content string    `json:"content"`
	Time    time.Time `json:"time"`

	// Optional fields based on type
	ToolUseResult *ToolResult `json:"toolUseResult,omitempty"`
	CostUSD       float64     `json:"costUSD,omitempty"`
	DurationMs    int         `json:"durationMs,omitempty"`
	ToolUses      []ToolUse   `json:"toolUses,omitempty"`
	IsApiErrorMsg bool        `json:"isApiErrorMsg,omitempty"`
	ToolUseID     string      `json:"toolUseId,omitempty"` // Relevant for progress/results
}

type MsgType string

const (
	MsgTypeUser      MsgType = "user"
	MsgTypeAssistant MsgType = "assistant"
	MsgTypeSystem    MsgType = "system"
	// MsgTypeProgress could be added if differentiation is needed vs assistant
)

// ToolUse represents a requested tool use by the assistant.
type ToolUse struct {
	ID    string         `json:"id"`
	Name  string         `json:"name"`
	Input map[string]any `json:"input"`
}

// ToolResult represents the result of a tool execution (part of a UserMessage).
type ToolResult struct {
	ToolUseID string `json:"tool_use_id"`
	Content   string `json:"content"` // Simplified content string
	IsError   bool   `json:"is_error,omitempty"`
}

// --- Styles ---
var (
	// Define base styles - consider making these configurable via a theme
	UserStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("39")) // Bright blue

	AssistantStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("86")) // Cyan

	SystemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")). // Lighter Gray
			Italic(true)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")) // Bright Red

	TimestampStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")). // Dark gray
			Faint(true)                        // Use Faint for less emphasis

	ToolResultStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("242")) // Gray

	ToolUseStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("208")) // Orange-ish

	MessagePadding = lipgloss.NewStyle().PaddingLeft(1) // Base padding for message content
)

// --- Message Rendering ---

// Render formats a message for display, considering terminal width.
func Render(msg Msg, width int) string {
	timeStr := msg.Time.Format("15:04") // Shorter time format
	timeDisplay := TimestampStyle.Render(timeStr)
	timeWidth := lipgloss.Width(timeDisplay)

	var prefix string
	var content string
	var baseStyle lipgloss.Style // Style for the main content text

	switch msg.Type {
	case MsgTypeUser:
		if msg.ToolUseResult != nil {
			prefix = ToolResultStyle.Render("↳ Tool: ")
			content = msg.ToolUseResult.Content
			if msg.ToolUseResult.IsError {
				baseStyle = ErrorStyle
			} else {
				baseStyle = ToolResultStyle // Keep dim for success too
			}
		} else {
			prefix = UserStyle.Bold(true).Render("You:") // Bold prefix only
			content = msg.Content
			baseStyle = UserStyle // Regular style for content
		}
	case MsgTypeAssistant:
		if msg.IsApiErrorMsg {
			prefix = ErrorStyle.Bold(true).Render("Error:")
			content = msg.Content
			baseStyle = ErrorStyle
		} else if len(msg.ToolUses) > 0 {
			prefix = ToolUseStyle.Bold(true).Render("Tool Call:")
			var toolDescs []string
			for _, tu := range msg.ToolUses {
				// TODO: Improve tool rendering if needed (e.g., show args concisely)
				toolDescs = append(toolDescs, fmt.Sprintf("%s(...)", tu.Name))
			}
			content = strings.Join(toolDescs, ", ")
			baseStyle = ToolUseStyle
		} else {
			prefix = AssistantStyle.Bold(true).Render("Assistant:")
			content = msg.Content // TODO: Apply markdown rendering here if desired
			baseStyle = AssistantStyle
		}
	case MsgTypeSystem:
		prefix = SystemStyle.Render("System:")
		content = msg.Content
		baseStyle = SystemStyle
	default:
		// Fallback for unknown types
		prefix = lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Render("Unknown:")
		content = msg.Content
		baseStyle = lipgloss.NewStyle()
	}

	// Calculate available width for content rendering
	prefixWidth := lipgloss.Width(prefix)
	// Available width = Total Width - Time Width - Space - Prefix Width - Base Padding
	availableWidth := width - timeWidth - 1 - prefixWidth - MessagePadding.GetPaddingLeft()
	if availableWidth < 10 {
		availableWidth = 10
	} // Minimum content width

	// Render content with wrapping, applying base style and width limit
	// Using Width() on the style handles wrapping automatically.
	renderedContent := baseStyle.Width(availableWidth).Render(content)

	// Combine parts, handling multi-line content indentation
	lines := strings.Split(renderedContent, "\n")
	firstLine := timeDisplay + " " + prefix + MessagePadding.Render(lines[0])
	subsequentLines := ""
	if len(lines) > 1 {
		// Indent subsequent lines relative to the start of the content on the first line
		// Indentation = Time Width + Space + Prefix Width + Base Padding
		indentWidth := timeWidth + 1 + prefixWidth + MessagePadding.GetPaddingLeft()
		indent := strings.Repeat(" ", indentWidth)
		subsequentLines = "\n" + indent + strings.Join(lines[1:], "\n"+indent)
	}

	return firstLine + subsequentLines
}

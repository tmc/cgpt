package message

import (
	// "fmt" // Removed unused import
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// Msg represents a message in the conversation.
type Msg struct {
	// UUID    string    `json:"uuid"` // Consider adding if needed
	Type    MsgType   `json:"type"`
	Content string    `json:"content"`
	Time    time.Time `json:"time"`

	// Optional fields
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

// ToolUse represents an intended tool call by the assistant.
type ToolUse struct {
	ID    string         `json:"id"`
	Name  string         `json:"name"`
	Input map[string]any `json:"input"` // Consider specific types if known
}

// ToolResult represents the outcome of a tool execution.
type ToolResult struct {
	ToolUseID string `json:"tool_use_id"`
	Content   string `json:"content"` // Result content as a string
	IsError   bool   `json:"is_error,omitempty"`
}

// --- Styles ---
var (
	UserStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("39")) // Bright blue

	AssistantStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("86")) // Cyan

	SystemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")). // Lighter Gray
			Italic(true)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")). // Bright Red
			Bold(true)

	TimestampStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")). // Dark gray
			Faint(true)

	ToolResultStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("242")) // Gray

	ToolUseStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("208")) // Orange-ish

	MessagePadding = lipgloss.NewStyle().PaddingLeft(1) // Base padding for message content
)

// --- Message Rendering ---

// Render formats a message for display, considering terminal width.
func Render(msg Msg, width int) string {
	timeStr := msg.Time.Format("15:04") // HH:MM format
	timeDisplay := TimestampStyle.Render(timeStr)
	timeWidth := lipgloss.Width(timeDisplay)

	var prefix string
	var content string
	var style lipgloss.Style // Style for the main content

	switch msg.Type {
	case MsgTypeUser:
		if msg.ToolUseResult != nil {
			prefix = ToolResultStyle.Render("↳ Tool: ")
			content = msg.ToolUseResult.Content
			if msg.ToolUseResult.IsError {
				style = ErrorStyle
			} else {
				style = ToolResultStyle
			}
		} else {
			prefix = UserStyle.Bold(true).Render("You:")
			content = msg.Content
			style = UserStyle
		}
	case MsgTypeAssistant:
		if msg.IsApiErrorMsg {
			prefix = ErrorStyle.Render("API Error:")
			content = msg.Content
			style = ErrorStyle
		} else if len(msg.ToolUses) > 0 {
			prefix = ToolUseStyle.Bold(true).Render("Tool Call:")
			var toolDescs []string
			for _, tu := range msg.ToolUses {
				toolDescs = append(toolDescs, fmt.Sprintf("%s(...)", tu.Name))
			}
			content = strings.Join(toolDescs, ", ") + " ..."
			style = ToolUseStyle
		} else {
			prefix = AssistantStyle.Bold(true).Render("Assistant:")
			content = msg.Content
			style = AssistantStyle
		}
	case MsgTypeSystem:
		prefix = SystemStyle.Render("System:")
		content = msg.Content
		style = SystemStyle
	default:
		prefix = lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Render("Unknown:")
		content = msg.Content
		style = lipgloss.NewStyle()
	}

	prefixWidth := lipgloss.Width(prefix)
	paddingWidth := MessagePadding.GetPaddingLeft() + MessagePadding.GetPaddingRight()
	availableWidth := width - timeWidth - 1 - prefixWidth - paddingWidth
	if availableWidth < 10 {
		availableWidth = 10
	}

	renderedContent := style.Width(availableWidth).Render(content)

	lines := strings.Split(renderedContent, "\n")
	firstLine := timeDisplay + " " + prefix + MessagePadding.Render(lines[0])

	subsequentLines := ""
	if len(lines) > 1 {
		indentWidth := timeWidth + 1 + prefixWidth + MessagePadding.GetPaddingLeft()
		indent := strings.Repeat(" ", indentWidth)
		subsequentLines = "\n" + indent + strings.Join(lines[1:], "\n"+indent)
	}

	return firstLine + subsequentLines
}

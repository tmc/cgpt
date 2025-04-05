package message

import (
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

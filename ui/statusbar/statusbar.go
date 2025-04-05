package statusbar

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Style definitions for the status bar
var (
	statusBarStyle = lipgloss.NewStyle().
			Reverse(true) // Invert colors for status bar look

	// Normal text style within the status bar
	statusTextStyle = lipgloss.NewStyle().Inherit(statusBarStyle)

	// Separator style
	separatorStyle = statusTextStyle.Copy().Foreground(lipgloss.Color("240")) // Dim gray

	// Specific styles (optional)
	modeStyle = statusTextStyle.Copy().Bold(true)
	// costStyle = statusTextStyle.Copy().Foreground(lipgloss.Color("220")) // Yellow
	// tokenStyle = statusTextStyle.Copy().Foreground(lipgloss.Color("245")) // Light gray
	// updateStyle = statusTextStyle.Copy().Foreground(lipgloss.Color("82")) // Green
)

// StatusData holds the information for the status bar
type StatusData struct {
	Cost           float64
	Tokens         int
	Mode           string // e.g., "Input", "Processing", "Multi-line", "Editor"
	UpdateStatus   string // e.g., "✓ Up to date", "Update available!", "Checking..."
	CustomMessages []string
	// Add other relevant status info here (e.g., current file, git branch)
}

// Render creates the status bar string
func Render(width int, data StatusData) string {
	if width <= 0 {
		return ""
	}

	sep := separatorStyle.Render(" │ ")

	// Format required parts (add padding within the strings)
	modeStr := modeStyle.Render(fmt.Sprintf(" %s ", data.Mode))
	// costStr := costStyle.Render(fmt.Sprintf(" Cost: $%.4f ", data.Cost))
	// tokenStr := tokenStyle.Render(fmt.Sprintf(" Tokens: ~%dk ", data.Tokens/1000))
	updateStr := fmt.Sprintf(" %s ", data.UpdateStatus) // Add padding here too
	customStr := strings.Join(data.CustomMessages, sep)

	// Assemble sections
	leftSections := []string{modeStr /*, costStr, tokenStr */}
	left := strings.Join(leftSections, sep)
	right := updateStr

	// Calculate available space for custom messages + padding
	leftWidth := lipgloss.Width(left)
	rightWidth := lipgloss.Width(right)
	customWidth := lipgloss.Width(customStr)

	// Total space used by fixed elements and potential custom message separator
	fixedWidth := leftWidth + rightWidth
	if customWidth > 0 {
		fixedWidth += lipgloss.Width(sep) + customWidth
	}

	// Calculate padding needed
	paddingWidth := width - fixedWidth
	if paddingWidth < 0 {
		paddingWidth = 0
	} // Ensure non-negative padding

	// Construct final string - place padding between left and custom/right
	var middle string
	if customWidth > 0 {
		// Place padding, then custom message
		middle = strings.Repeat(" ", paddingWidth) + sep + customStr
	} else {
		// Just padding
		middle = strings.Repeat(" ", paddingWidth)
	}

	finalStr := left + middle + right

	// Ensure the final string doesn't exceed the width (can happen with rounding)
	// Render the final string within the specified width
	return statusBarStyle.Width(width).Render(finalStr)
}

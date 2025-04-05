package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// RenderLogo renders a simplified application header.
func RenderLogo(width int /*, other data */) string {
	title := " LLM CLI (Go/BubbleTea) " // Add padding within the title
	style := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FFFFFF")). // White text
		Background(lipgloss.Color("63"))       // Purple-ish background

	// Center the title, ensuring it doesn't exceed width
	titleWidth := lipgloss.Width(title)
	if titleWidth > width {
		// Truncate title if necessary (unlikely with short title)
		title = title[:width-3] + "..."
		titleWidth = width
	}

	sidePadding := (width - titleWidth) / 2
	leftPad := strings.Repeat(" ", sidePadding)
	// Adjust padding for odd widths to ensure total width is correct
	rightPad := strings.Repeat(" ", width-titleWidth-sidePadding)

	return style.Render(leftPad + title + rightPad)
}

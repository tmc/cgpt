package spinner

import (
	// Keep original spinner implementation
	"github.com/charmbracelet/bubbles/spinner"
)

// Re-export or wrap the original spinner types/functions if needed,
// or use the original directly. For simplicity, let's assume direct use.

// Model is an alias for the original spinner model.
type Model = spinner.Model

// New creates a new spinner model.
var New = spinner.New

// TickMsg is an alias for the original spinner tick message.
type TickMsg = spinner.TickMsg

// Option is an alias for the original spinner option type.
type Option = spinner.Option

// Spinner is an alias for the original Spinner struct.
type Spinner = spinner.Spinner

// Re-export constants
var (
	Line      = spinner.Line
	Dot       = spinner.Dot
	MiniDot   = spinner.MiniDot
	Jump      = spinner.Jump
	Pulse     = spinner.Pulse
	Points    = spinner.Points
	Globe     = spinner.Globe
	Moon      = spinner.Moon
	Monkey    = spinner.Monkey
	Meter     = spinner.Meter
	Hamburger = spinner.Hamburger
	Ellipsis  = spinner.Ellipsis
)

// Re-export options
var (
	WithSpinner = spinner.WithSpinner
	WithStyle   = spinner.WithStyle
)

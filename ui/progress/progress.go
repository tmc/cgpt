package progress

import (
	// Keep original progress implementation
	"github.com/charmbracelet/bubbles/progress"
)

// Re-export or wrap the original progress types/functions.

// Model is an alias for the original progress model.
type Model = progress.Model

// New creates a new progress model.
var New = progress.New

// Messages and Commands
type FrameMsg = progress.FrameMsg

// ... add others if needed

// Options
type Option = progress.Option

var (
	WithDefaultGradient       = progress.WithDefaultGradient
	WithGradient              = progress.WithGradient
	WithDefaultScaledGradient = progress.WithDefaultScaledGradient
	WithScaledGradient        = progress.WithScaledGradient
	WithSolidFill             = progress.WithSolidFill
	WithFillCharacters        = progress.WithFillCharacters
	WithoutPercentage         = progress.WithoutPercentage
	WithWidth                 = progress.WithWidth
	WithSpringOptions         = progress.WithSpringOptions
	WithColorProfile          = progress.WithColorProfile
)

package viewport

import (
	// Keep original viewport implementation
	"github.com/charmbracelet/bubbles/viewport"
)

// Re-export or wrap the original viewport types/functions.

// Model is an alias for the original viewport model.
type Model = viewport.Model

// New creates a new viewport model.
var New = viewport.New

// KeyMap is an alias.
type KeyMap = viewport.KeyMap

// DefaultKeyMap is an alias.
var DefaultKeyMap = viewport.DefaultKeyMap

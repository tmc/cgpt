//go:build !js

package textinput

import (
	// Keep original textinput implementation
	"github.com/charmbracelet/bubbles/textinput"
)

// Re-export or wrap the original textinput types/functions.

// Model is an alias for the original textinput model.
type Model = textinput.Model

// New creates a new textinput model.
var New = textinput.New

// Messages and Commands can also be re-exported if needed
// type PasteMsg = textinput.PasteMsg // Commented out - likely removed upstream
// type PasteErrMsg = textinput.PasteErrMsg // Commented out - likely removed upstream

// var Paste = textinput.Paste // Commented out - likely removed upstream
var Blink = textinput.Blink

// Constants like EchoMode can be re-exported
const (
	EchoNormal   = textinput.EchoNormal
	EchoPassword = textinput.EchoPassword
	EchoNone     = textinput.EchoNone
)

type EchoMode = textinput.EchoMode

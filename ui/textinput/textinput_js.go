//go:build js

package textinput

import (
	// Keep original textinput implementation (will be replaced by WASM fork during generate)
	"github.com/charmbracelet/bubbles/textinput"
)

// Re-export or wrap the original textinput types/functions for JS/WASM.

// Model is an alias for the original textinput model.
type Model = textinput.Model

// New creates a new textinput model.
var New = textinput.New

// Exclude Paste-related messages and commands as they are likely not
// available or implemented differently in the WASM environment.
// type PasteMsg = textinput.PasteMsg
// type PasteErrMsg = textinput.PasteErrMsg
// var Paste = textinput.Paste

var Blink = textinput.Blink

// Constants like EchoMode can be re-exported
const (
	EchoNormal   = textinput.EchoNormal
	EchoPassword = textinput.EchoPassword
	EchoNone     = textinput.EchoNone
)

type EchoMode = textinput.EchoMode

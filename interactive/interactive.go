package interactive

import (
	"context"
	"errors"
	"io"

	"github.com/tmc/cgpt/ui/completion" // Use local completion package
	// "github.com/tmc/cgpt/ui/computil" // Removed unused import
)

var ErrEmptyInput = errors.New("empty input")

// ErrUseLastMessage signals recalling the last message for editing.
type ErrUseLastMessage string

func (e ErrUseLastMessage) Error() string { return "use last message: " + string(e) }

// --- Autocompletion Types ---
// Defined here for clarity, implemented in completion_helpers.go

// AutoCompleteFn is the callback signature for autocomplete logic.
type AutoCompleteFn func(entireInput [][]rune, line, col int) (msg string, comp Completions)

// Completions provides completion candidates and metadata.
type Completions interface {
	completion.Values // Embed the core values interface
	Candidate(e completion.Entry) Candidate
}

// Candidate defines how a chosen completion entry replaces text.
type Candidate interface {
	Replacement() string
	MoveRight() int
	DeleteLeft() int
}

// --- End Autocompletion Types ---

// --- Command Mode Types ---

// CommandFn is the callback signature for handling command mode input.
type CommandFn func(ctx context.Context, command string) error

// --- End Command Mode Types ---

// Config defines parameters for creating an interactive session.
type Config struct {
	Prompt             string
	AltPrompt          string
	HistoryFile        string                                        // Path for loading/saving history
	LoadedHistory      []string                                      // Pre-loaded history
	ProcessFn          func(ctx context.Context, input string) error // Handles normal input submission
	CommandFn          CommandFn                                     // Handles command mode submission
	AutoCompleteFn     AutoCompleteFn
	CheckInputComplete func(entireInput [][]rune, line, col int) bool // Optional: Custom submit logic
	Stdin              io.ReadCloser
	SingleLineHint     string
	MultiLineHint      string
	LastInput          string
}

// InteractiveState (remains internal)
type InteractiveState int

const (
	StateSingleLine InteractiveState = iota
	StateMultiLine
)

// Defaults
var (
	DefaultSingleLineHint  = `Enter prompt (""" for multi-line, ESC for command mode)`  // Updated hint
	DefaultMultiLineHint   = `(End with """ or Ctrl+D to submit, ESC for command mode)` // Updated hint
	SubmitReadyPlaceholder = "Press Enter again to submit..."
)

// Session defines the interface for an interactive session implementation.
type Session interface {
	Run(ctx context.Context) error
	SetStreaming(streaming bool)
	SetLastInput(input string)
	AddResponsePart(part string)
	GetHistory() []string              // Retrieve current history
	GetHistoryFilename() string        // Get the configured history filename
	LoadHistory(filename string) error // Load history from a file
	SaveHistory(filename string) error // Save history to a file
	Quit()                             // Add a method to signal quitting
}

// NewSession is defined in platform-specific files
// func NewSession(cfg Config) (Session, error)

// --- Completion Helpers ---
// Moved to completion_helpers.go to keep this file focused on interfaces/config.

// SimpleWordsCompletion moved to completion_helpers.go
// SingleWordCompletion moved to completion_helpers.go

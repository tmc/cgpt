package interactive

import (
	"context"
	"errors"
	"io"

	"github.com/tmc/cgpt/ui/completion" // Use local completion package
)

var ErrEmptyInput = errors.New("empty input")

// ErrInterrupted signals that the interactive session was interrupted.
var ErrInterrupted = errors.New("session interrupted")

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
	Stdin io.ReadCloser

	ConversationHistory []string
	ProcessFn           func(ctx context.Context, input string) error // Handles normal input submission

	HistoryFile string // Disk path for loading/saving history

	Prompt    string
	AltPrompt string

	CommandFn          CommandFn // Handles command mode submission
	AutoCompleteFn     AutoCompleteFn
	CheckInputComplete func(entireInput [][]rune, line, col int) bool // Optional: Custom submit logic

	SingleLineHint string
	MultiLineHint  string
}

// InteractiveState (remains internal)
type InteractiveState int

const (
	StateSingleLine InteractiveState = iota
	StateMultiLine
)

type ResponseState int

const (
	ResponseStateUndefined ResponseState = iota
	ResponseStateReady
	ResponseStateSubmitting
	ResponseStateSubmitted
	ResponseStateStreaming
	ResponseStateSInterrupted
	ResponseStateError
)

// Defaults
var (
	DefaultSingleLineHint  = `Enter prompt (""" for multi-line, ESC for command mode)`
	DefaultMultiLineHint   = `(End with """ or Ctrl+D to submit, ESC for command mode)` // Updated hint
	SubmitReadyPlaceholder = "Press Enter again to submit..."
)

// Session defines the interface for an interactive session implementation.
type Session interface {
	Run(ctx context.Context) error
	SetResponseState(state ResponseState)
	AddResponsePart(part string)
}

type historyManager interface { // TODO: move history concepts elsewhere.
	GetHistory() []string              // Retrieve current history
	GetHistoryFilename() string        // Get the configured history filename
	LoadHistory(filename string) error // Load history from a file
	SaveHistory(filename string) error // Save history to a file
}

func (r ResponseState) String() string {
	switch r {
	case ResponseStateUndefined:
		return "undefined"
	case ResponseStateReady:
		return "ready"
	case ResponseStateSubmitting:
		return "submitting"
	case ResponseStateSubmitted:
		return "submitted"
	case ResponseStateStreaming:
		return "streaming"
	case ResponseStateSInterrupted:
		return "stream interrupted"
	case ResponseStateError:
		return "error"
	default:
		return "unknown state"
	}
}

func (r ResponseState) IsProcessing() bool {
	if r == ResponseStateUndefined {
		return false
	}
	if r == ResponseStateReady {
		return false
	}
	if r == ResponseStateError {
		return false
	}
	if r == ResponseStateSInterrupted {
		return false
	}
	return true
}

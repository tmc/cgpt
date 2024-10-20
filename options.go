package cgpt

import (
	"context"
	"io"
	"os"
	"strings"
	"time"
)

// RunOptions contains all the options that are relevant to run cgpt.
type RunOptions struct {
	// Config options
	*Config `json:"config,omitempty" yaml:"config,omitempty"`
	// Input options
	InputStrings   []string `json:"inputStrings,omitempty" yaml:"inputStrings,omitempty"`
	InputFiles     []string `json:"inputFiles,omitempty" yaml:"inputFiles,omitempty"`
	PositionalArgs []string `json:"positionalArgs,omitempty" yaml:"positionalArgs,omitempty"`
	Prefill        string   `json:"prefill,omitempty" yaml:"prefill,omitempty"`

	// Output options
	Continuous   bool `json:"continuous,omitempty" yaml:"continuous,omitempty"`
	StreamOutput bool `json:"streamOutput,omitempty" yaml:"streamOutput,omitempty"`
	ShowSpinner  bool `json:"showSpinner,omitempty" yaml:"showSpinner,omitempty"`
	EchoPrefill  bool `json:"echoPrefill,omitempty" yaml:"echoPrefill,omitempty"`

	// Verbosity options
	Verbose   bool `json:"verbose,omitempty" yaml:"verbose,omitempty"`
	DebugMode bool `json:"debugMode,omitempty" yaml:"debugMode,omitempty"`

	// History options
	HistoryIn           string `json:"historyIn,omitempty" yaml:"historyIn,omitempty"`
	HistoryOut          string `json:"historyOut,omitempty" yaml:"historyOut,omitempty"`
	ReadlineHistoryFile string `json:"readlineHistoryFile,omitempty" yaml:"readlineHistoryFile,omitempty"`
	NCompletions        int    `json:"nCompletions,omitempty" yaml:"nCompletions,omitempty"`

	// I/O
	Stdout io.Writer `json:"-" yaml:"-"`
	Stderr io.Writer `json:"-" yaml:"-"`
	Stdin  io.Reader `json:"-" yaml:"-"`

	// Timing
	MaximumTimeout time.Duration `json:"maximumTimeout,omitempty" yaml:"maximumTimeout,omitempty"`

	ConfigPath string `json:"configPath,omitempty" yaml:"configPath,omitempty"`
}

// GetCombinedInputReader returns an io.Reader that combines all input sources.
func (ro *RunOptions) GetCombinedInputReader(ctx context.Context) (io.Reader, error) {
	handler := &InputHandler{
		Files:   ro.InputFiles,
		Strings: ro.InputStrings,
		Args:    ro.PositionalArgs,
		Stdin:   ro.Stdin,
	}
	return handler.Process(ctx)
}

// InputSourceType represents the type of input source.
type InputSourceType string

const (
	InputSourceStdin  InputSourceType = "stdin"
	InputSourceFile   InputSourceType = "file"
	InputSourceString InputSourceType = "string"
	InputSourceArg    InputSourceType = "arg"
)

// InputSource represents a single input source.
type InputSource struct {
	Type   InputSourceType
	Reader io.Reader
}

// InputHandler manages multiple input sources.
type InputHandler struct {
	Files   []string
	Strings []string
	Args    []string
	Stdin   io.Reader
}

// InputSources is a slice of InputSource.
type InputSources []InputSource

// Readers returns a slice of io.Reader from InputSources.
func (is InputSources) Readers() []io.Reader {
	readers := make([]io.Reader, len(is))
	for i, s := range is {
		readers[i] = s.Reader
	}
	return readers
}

// Process reads the set of inputs, this will block on stdin if it is included.
// The order of precedence is:
// 1. Files
// 2. Strings
// 3. Args
func (h *InputHandler) Process(ctx context.Context) (io.Reader, error) {
	var readers []io.Reader
	stdinReader := h.getStdinReader()

	for _, file := range h.Files {
		if file == "-" {
			if stdinReader != nil {
				readers = append(readers, stdinReader)
			} else {
				readers = append(readers, strings.NewReader(""))
			}
		} else {
			f, err := os.Open(file)
			if err != nil {
				return nil, err
			}
			readers = append(readers, f)
		}
	}

	for _, s := range h.Strings {
		readers = append(readers, strings.NewReader(s))
	}

	for _, arg := range h.Args {
		readers = append(readers, strings.NewReader(arg))
	}

	return io.MultiReader(readers...), nil
}

func (h *InputHandler) getStdinReader() io.Reader {
	if h.Stdin == nil {
		return nil
	}
	return h.Stdin
}

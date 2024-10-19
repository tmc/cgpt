package cgpt

import (
	"io"
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

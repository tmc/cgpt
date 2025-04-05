package options

import (
	"io"
	"time"
)

// RunOptions contains all the options that are relevant to run cgpt.
type RunOptions struct {
	// Config options
	*Config `json:"config,omitempty" yaml:"config,omitempty"`

	// --- Input source flags ---
	InputStrings   []string `json:"inputStrings,omitempty" yaml:"inputStrings,omitempty"`
	InputFiles     []string `json:"inputFiles,omitempty" yaml:"inputFiles,omitempty"`
	PositionalArgs []string `json:"positionalArgs,omitempty" yaml:"positionalArgs,omitempty"`
	Prefill        string   `json:"prefill,omitempty" yaml:"prefill,omitempty"`

	// Output options
	Continuous   bool `json:"continuous,omitempty" yaml:"continuous,omitempty"`
	StreamOutput bool `json:"streamOutput,omitempty" yaml:"streamOutput,omitempty"`
	ShowSpinner  bool `json:"showSpinner,omitempty" yaml:"showSpinner,omitempty"`
	EchoPrefill  bool `json:"echoPrefill,omitempty" yaml:"echoPrefill,omitempty"`
	UseTUI       bool `json:"useTUI,omitempty" yaml:"useTUI,omitempty"` // Use BubbleTea UI for interactive mode
	PrintUsage   bool

	// Verbosity options
	Verbose   bool `json:"verbose,omitempty" yaml:"verbose,omitempty"`
	DebugMode bool `json:"debugMode,omitempty" yaml:"debugMode,omitempty"`

	// History options
	HistoryIn           string `json:"historyIn,omitempty" yaml:"historyIn,omitempty"`
	HistoryOut          string `json:"historyOut,omitempty" yaml:"historyOut,omitempty"`
	ReadlineHistoryFile string `json:"readlineHistoryFile,omitempty" yaml:"readlineHistoryFile,omitempty"`
	NCompletions        int    `json:"nCompletions,omitempty" yaml:"nCompletions,omitempty"`

	// --- I/O handles passed in ---
	Stdout io.Writer `json:"-" yaml:"-"`
	Stderr io.Writer `json:"-" yaml:"-"`
	Stdin  io.Reader `json:"-" yaml:"-"` // Passed during initFlags

	// Timing
	CompletionTimeout time.Duration `json:"completionTimeout,omitempty" yaml:"completionTimeout,omitempty"` // Renamed from MaximumTimeout

	ConfigPath string `json:"configPath,omitempty" yaml:"configPath,omitempty"`

	// Backend/Provider-specific options.
	OpenAIUseLegacyMaxTokens bool `json:"openaiUseLegacyMaxTokens,omitempty"`

	// Interactive mode placeholder text
	SingleLineHint string // Placeholder text for single line input
	MultiLineHint  string // Placeholder text for multi-line input
}

// Legacy file, import from the completion package instead
package cgpt

import (
	"context"
	"io"
	"time"

	"github.com/tmc/cgpt/completion"
	"github.com/tmc/langchaingo/llms"
	"go.uber.org/zap"
)

// CompletionService is the main entry point for completions.
// Use completion.Service instead for new code.
type CompletionService struct {
	service *completion.Service
}

// CompletionOptions is the configuration for the legacy CompletionService.
// Use completion.Options instead for new code.
type CompletionOptions = completion.Options

// CompletionServiceOption configures a CompletionService.
// Use completion.ServiceOption instead for new code.
type CompletionServiceOption func(*CompletionService)

// NewCompletionService creates a new CompletionService with the given configuration.
// Use completion.New instead for new code.
func NewCompletionService(cfg *Config, model llms.Model, opts ...CompletionServiceOption) (*CompletionService, error) {
	compCfg := &completion.Config{
		MaxTokens:         cfg.MaxTokens,
		Temperature:       cfg.Temperature,
		SystemPrompt:      cfg.SystemPrompt,
		CompletionTimeout: cfg.CompletionTimeout,
	}
	
	compService, err := completion.New(compCfg, model)
	if err != nil {
		return nil, err
	}
	
	s := &CompletionService{
		service: compService,
	}
	
	for _, opt := range opts {
		opt(s)
	}
	
	return s, nil
}

// WithOptions sets the whole options struct
func WithOptions(opts CompletionOptions) CompletionServiceOption {
	return func(s *CompletionService) {
		completion.WithOptions(opts)(s.service)
	}
}

// WithStdout sets the stdout writer
func WithStdout(w io.Writer) CompletionServiceOption {
	return func(s *CompletionService) {
		completion.WithStdout(w)(s.service)
	}
}

// WithStderr sets the stderr writer
func WithStderr(w io.Writer) CompletionServiceOption {
	return func(s *CompletionService) {
		completion.WithStderr(w)(s.service)
	}
}

// WithShowSpinner enables or disables the spinner
func WithShowSpinner(show bool) CompletionServiceOption {
	return func(s *CompletionService) {
		completion.WithShowSpinner(show)(s.service)
	}
}

// WithEchoPrefill enables or disables echoing the prefill
func WithEchoPrefill(echo bool) CompletionServiceOption {
	return func(s *CompletionService) {
		completion.WithEchoPrefill(echo)(s.service)
	}
}

// WithHistoryIn sets the history input file
func WithHistoryIn(path string) CompletionServiceOption {
	return func(s *CompletionService) {
		completion.WithHistoryIn(path)(s.service)
	}
}

// WithHistoryOut sets the history output file
func WithHistoryOut(path string) CompletionServiceOption {
	return func(s *CompletionService) {
		completion.WithHistoryOut(path)(s.service)
	}
}

// WithReadlineHistoryFile sets the readline history file
func WithReadlineHistoryFile(path string) CompletionServiceOption {
	return func(s *CompletionService) {
		completion.WithReadlineHistoryFile(path)(s.service)
	}
}

// WithPrefill sets the prefill content
func WithPrefill(prefill string) CompletionServiceOption {
	return func(s *CompletionService) {
		completion.WithPrefill(prefill)(s.service)
	}
}

// WithContinuous enables or disables continuous mode
func WithContinuous(continuous bool) CompletionServiceOption {
	return func(s *CompletionService) {
		completion.WithContinuous(continuous)(s.service)
	}
}

// WithStreamOutput enables or disables streaming output
func WithStreamOutput(stream bool) CompletionServiceOption {
	return func(s *CompletionService) {
		completion.WithStreamOutput(stream)(s.service)
	}
}

// WithVerbose enables or disables verbose logging
func WithVerbose(verbose bool) CompletionServiceOption {
	return func(s *CompletionService) {
		completion.WithVerbose(verbose)(s.service)
	}
}

// WithDebugMode enables or disables debug mode
func WithDebugMode(debug bool) CompletionServiceOption {
	return func(s *CompletionService) {
		completion.WithDebugMode(debug)(s.service)
	}
}

// WithCompletionTimeout sets the completion timeout
func WithCompletionTimeout(timeout time.Duration) CompletionServiceOption {
	return func(s *CompletionService) {
		completion.WithCompletionTimeout(timeout)(s.service)
	}
}

// WithLogger sets the logger for the completion service.
func WithLogger(l *zap.SugaredLogger) CompletionServiceOption {
	return func(s *CompletionService) {
		completion.WithLogger(l)(s.service)
	}
}

// Run executes a completion using the service's options
func (s *CompletionService) Run(ctx context.Context, runCfg RunOptions) error {
	// Convert cgpt.RunOptions to completion.RunOptions
	compRunOpts := completion.RunOptions{
		Config: &completion.Config{
			MaxTokens:         runCfg.Config.MaxTokens,
			Temperature:       runCfg.Config.Temperature,
			SystemPrompt:      runCfg.Config.SystemPrompt,
			CompletionTimeout: runCfg.Config.CompletionTimeout,
		},
		InputStrings:             runCfg.InputStrings,
		InputFiles:               runCfg.InputFiles,
		PositionalArgs:           runCfg.PositionalArgs,
		Prefill:                  runCfg.Prefill,
		Continuous:               runCfg.Continuous,
		StreamOutput:             runCfg.StreamOutput,
		ShowSpinner:              runCfg.ShowSpinner,
		EchoPrefill:              runCfg.EchoPrefill,
		PrintUsage:               runCfg.PrintUsage,
		Verbose:                  runCfg.Verbose,
		DebugMode:                runCfg.DebugMode,
		HistoryIn:                runCfg.HistoryIn,
		HistoryOut:               runCfg.HistoryOut,
		ReadlineHistoryFile:      runCfg.ReadlineHistoryFile,
		NCompletions:             runCfg.NCompletions,
		Stdout:                   runCfg.Stdout,
		Stderr:                   runCfg.Stderr,
		Stdin:                    runCfg.Stdin,
		MaximumTimeout:           runCfg.MaximumTimeout,
		ConfigPath:               runCfg.ConfigPath,
		OpenAIUseLegacyMaxTokens: runCfg.OpenAIUseLegacyMaxTokens,
	}
	return s.service.Run(ctx, compRunOpts)
}

// RunOptionsToCompletionOptions converts RunOptions to CompletionOptions
func RunOptionsToCompletionOptions(runOpts RunOptions) CompletionOptions {
	// Create a completion.Options from cgpt.RunOptions
	return completion.Options{
		Stdout:             runOpts.Stdout,
		Stderr:             runOpts.Stderr,
		EchoPrefill:        runOpts.EchoPrefill,
		ShowSpinner:        runOpts.ShowSpinner,
		PrintUsage:         runOpts.PrintUsage,
		CompletionTimeout:  runOpts.Config.CompletionTimeout,
		HistoryIn:          runOpts.HistoryIn,
		HistoryOut:         runOpts.HistoryOut,
		ReadlineHistoryFile: runOpts.ReadlineHistoryFile,
		Prefill:            runOpts.Prefill,
		Verbose:            runOpts.Verbose,
		DebugMode:          runOpts.DebugMode,
		StreamOutput:       runOpts.StreamOutput,
		Continuous:         runOpts.Continuous,
	}
}

// NewCompletionOptions creates a new CompletionOptions with defaults.
func NewCompletionOptions() CompletionOptions {
	return completion.NewOptions()
}

// SetNextCompletionPrefill sets the next completion prefill message.
func (s *CompletionService) SetNextCompletionPrefill(content string) {
	s.service.SetNextCompletionPrefill(content)
}
package cgpt

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/tmc/cgpt/interactive"
	"github.com/tmc/langchaingo/llms"
	"go.uber.org/zap"
)

type CompletionService struct {
	cfg *Config

	loggerCfg zap.Config
	logger    *zap.SugaredLogger

	model llms.Model

	payload *ChatCompletionPayload

	completionTimeout time.Duration

	historyIn           io.Reader
	historyOutFile      string
	readlineHistoryFile string

	performCompletionConfig PerformCompletionConfig

	// nextCompletionPrefill is the message to prefill the assistant with for the next completion.
	nextCompletionPrefill string
}

// NewCompletionService creates a new CompletionService with the given configuration.
func NewCompletionService(cfg *Config) (*CompletionService, error) {
	model, err := initializeModel(cfg.Backend, cfg.Model, cfg.Debug, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize model: %w", err)
	}
	loggerCfg := zap.NewDevelopmentConfig()
	logger, err := loggerCfg.Build()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}
	return &CompletionService{
		cfg:               cfg,
		loggerCfg:         loggerCfg,
		logger:            logger.Sugar(),
		model:             model,
		payload:           newCompletionPayload(cfg),
		completionTimeout: cfg.CompletionTimeout,
	}, nil
}

// RunConfig is the configuration for the Run method.
type RunConfig struct {
	// InputString is the input text to complete. If present, this will be used instead of reading from a file.
	// This will only be used for the first completion when either running in continuous mode or when running multiple completions.
	InputString string

	// InputFile is the file to read input from. Use "-" for stdin.
	InputFile string

	// Continuous will run the completion API in a loop, using the previous output as the input for the next request.
	Continuous bool

	// StreamOutput will stream results as they come in.
	StreamOutput bool

	// Prefill is the message to prefill the assistant with.
	// This will only be used for the first completion if more than one completion is run.
	// By default, it will be printed before the assistant's response.
	Prefill string
	// EchoPrefill is a flag that, when set to true, will echo the prefill message to the user.
	EchoPrefill bool

	// HistoryIn is the file to read cgpt history from.
	HistoryIn string
	// HistoryOut is the file to store cgpt history in.
	HistoryOut string

	// NCompletions is the number of completions to complete in a history-enabled context.
	NCompletions int

	// Verbose will enable verbose output.
	Verbose bool

	// DebugMode will enable debug output.
	DebugMode bool

	// ReadlineHistoryFile is the file to store readline history in.
	ReadlineHistoryFile string

	// MaximumTimeout is the maximum time to wait for a response.
	MaximumTimeout time.Duration

	// ShowSpinner is a flag that, when set to true, shows a spinner while waiting for the completion.
	ShowSpinner bool
}

// PerformCompletionConfig is the configuration for the PerformCompletion method, it controls the behavior of the completion with regard to user interaction.
type PerformCompletionConfig struct {
	// EchoPrefill is a flag that, when set to true, will echo the prefill message to the user.
	EchoPrefill bool
	// ShowSpinner is a flag that, when set to true, shows a spinner while waiting for the completion.
	ShowSpinner bool
}

// Run runs the completion service with the given configuration.
func (s *CompletionService) Run(ctx context.Context, runCfg RunConfig) error {
	s.readlineHistoryFile = runCfg.ReadlineHistoryFile
	s.loggerCfg.Level.SetLevel(zap.WarnLevel)
	if runCfg.Verbose {
		s.loggerCfg.Level.SetLevel(zap.InfoLevel)
	}
	if runCfg.DebugMode {
		s.loggerCfg.Level.SetLevel(zap.DebugLevel)
	}
	if err := s.handleHistory(runCfg.HistoryIn, runCfg.HistoryOut); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	if !s.loadedWithHistory() && s.cfg.SystemPrompt != "" {
		s.payload.addSystemMessage(s.cfg.SystemPrompt)
	}
	if runCfg.Prefill != "" {
		s.SetNextCompletionPrefill(runCfg.Prefill)
	}
	if runCfg.Continuous {
		if runCfg.StreamOutput {
			return s.runContinuousCompletionStreaming(ctx, runCfg)
		} else {
			return s.runContinuousCompletion(ctx, runCfg)
		}
	}
	if runCfg.StreamOutput {
		return s.runOneShotCompletionStreaming(ctx, runCfg)
	} else {
		return s.runOneShotCompletion(ctx, runCfg)
	}
}

func (s *CompletionService) loadedWithHistory() bool {
	return s.historyIn != nil
}

func (s *CompletionService) handleHistory(historyIn, historyOut string) error {
	s.historyOutFile = historyOut
	if historyIn != "" {
		f, err := os.Open(historyIn)
		if err != nil {
			return fmt.Errorf("issue reading input history file: %w", err)
		}
		s.historyIn = f
		defer f.Close()
	}
	err := s.loadHistory()
	if err != nil {
		return fmt.Errorf("failed to load history: %w", err)
	}
	if err := s.saveHistory(); err != nil {
		return fmt.Errorf("failed to save history: %w", err)
	}
	return nil
}

func (s *CompletionService) getLastUserMessage() string {
	if len(s.payload.Messages) == 0 {
		return ""
	}
	last := s.payload.Messages[len(s.payload.Messages)-1]
	var parts []string
	for _, m := range last.Parts {
		parts = append(parts, fmt.Sprint(m))
	}

	return strings.Join(parts, "\n")
}

func (s *CompletionService) runOneShotCompletionStreaming(ctx context.Context, runCfg RunConfig) error {
	var (
		input io.Reader = os.Stdin
		err   error
	)
	s.logger.Debug("running one-shot completion with streaming")
	inputString := runCfg.InputString
	inputFile := runCfg.InputFile

	var contents string
	if inputString != "" {
		contents = inputString
	} else {
		if inputFile != "-" {
			input, err = os.Open(inputFile)
			if err != nil {
				return fmt.Errorf("failed to open input file %q: %w", inputFile, err)
			}
		}
		b, err := io.ReadAll(input)
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}
		contents = string(b)
	}

	s.payload.Stream = true
	s.payload.addUserMessage(contents)
	streamPayloads, err := s.PerformCompletionStreaming(ctx, s.payload, PerformCompletionConfig{
		ShowSpinner: runCfg.ShowSpinner,
		EchoPrefill: !runCfg.EchoPrefill,
	})
	if err != nil {
		return err
	}
	content := strings.Builder{}
	for r := range streamPayloads {
		content.WriteString(r)
		fmt.Print(r)
	}
	s.payload.addAssistantMessage(content.String())
	if err := s.saveHistory(); err != nil {
		return fmt.Errorf("failed to save history: %w", err)
	}
	return nil
}

// Non-streaming version of one-shot completion.
func (s *CompletionService) runOneShotCompletion(ctx context.Context, runCfg RunConfig) error {
	var (
		input io.Reader = os.Stdin
		err   error
	)
	s.logger.Debug("running one-shot completion")
	inputString := runCfg.InputString
	inputFile := runCfg.InputFile

	var contents string
	if inputString != "" {
		contents = inputString
	} else {
		if inputFile != "-" {
			input, err = os.Open(inputFile)
			if err != nil {
				return fmt.Errorf("failed to open input file %q: %w", inputFile, err)
			}
		}
		b, err := io.ReadAll(input)
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}
		contents = string(b)
	}

	s.payload.Stream = false
	s.payload.addUserMessage(contents)
	response, err := s.PerformCompletion(ctx, s.payload, PerformCompletionConfig{
		ShowSpinner: runCfg.ShowSpinner,
		EchoPrefill: !runCfg.EchoPrefill,
	})
	if err != nil {
		return err
	}
	fmt.Print(response)
	s.payload.addAssistantMessage(response)
	if err := s.saveHistory(); err != nil {
		return fmt.Errorf("failed to save history: %w", err)
	}
	return nil
}

// Enhanced function to run continuous streaming completion mode.
func (s *CompletionService) runContinuousCompletionStreaming(ctx context.Context, runCfg RunConfig) error {
	fmt.Fprintln(os.Stderr, "Running in continuous mode. Press ctrl+c to exit.")
	processFn := func(input string) error {
		s.payload.addUserMessage(input)
		streamPayloads, err := s.PerformCompletionStreaming(ctx, s.payload, PerformCompletionConfig{
			ShowSpinner: runCfg.ShowSpinner,
			EchoPrefill: !runCfg.EchoPrefill,
		})
		if err != nil {
			return err
		}
		content := strings.Builder{}
		for r := range streamPayloads {
			content.WriteString(r)
			fmt.Print(r)
		}
		s.payload.addAssistantMessage(content.String())
		fmt.Println()
		if err := s.saveHistory(); err != nil {
			return fmt.Errorf("failed to save history: %w", err)
		}
		return nil
	}

	sessionConfig := interactive.Config{
		Prompt:      ">>> ",
		AltPrompt:   "... ",
		HistoryFile: expandTilde(s.readlineHistoryFile),
		ProcessFn:   processFn,
	}

	session, err := interactive.NewInteractiveSession(sessionConfig)
	if err != nil {
		return err
	}

	return session.Run()
}

// Non-streaming version of continuous completion.
func (s *CompletionService) runContinuousCompletion(ctx context.Context, runCfg RunConfig) error {
	fmt.Fprintln(os.Stderr, "Running in continuous mode. Press ctrl+c to exit.")
	processFn := func(input string) error {
		s.payload.addUserMessage(input)
		response, err := s.PerformCompletion(ctx, s.payload, PerformCompletionConfig{
			ShowSpinner: runCfg.ShowSpinner,
			EchoPrefill: !runCfg.EchoPrefill,
		})
		if err != nil {
			return err
		}
		fmt.Print(response)
		s.payload.addAssistantMessage(response)
		fmt.Println()
		if err := s.saveHistory(); err != nil {
			return fmt.Errorf("failed to save history: %w", err)
		}
		return nil
	}

	sessionConfig := interactive.Config{
		Prompt:      ">>> ",
		AltPrompt:   "... ",
		HistoryFile: expandTilde(s.readlineHistoryFile),
		ProcessFn:   processFn,
	}

	session, err := interactive.NewInteractiveSession(sessionConfig)
	if err != nil {
		return err
	}

	return session.Run()
}

func expandTilde(path string) string {
	if strings.HasPrefix(path, "~/") {
		return strings.Replace(path, "~", os.Getenv("HOME"), 1)
	}
	return path
}

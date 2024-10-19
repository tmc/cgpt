package cgpt

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/tmc/cgpt/interactive"
	"github.com/tmc/langchaingo/llms"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
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

	// Stdout is the writer for standard output. If nil, os.Stdout will be used.
	Stdout io.Writer
	// Stderr is the writer for standard error. If nil, os.Stderr will be used.
	Stderr io.Writer
}

type CompletionServiceOption func(*CompletionService)

func WithStdout(w io.Writer) CompletionServiceOption {
	return func(s *CompletionService) {
		s.Stdout = w
	}
}

func WithStderr(w io.Writer) CompletionServiceOption {
	return func(s *CompletionService) {
		s.Stderr = w
	}
}

// WithLogger sets the logger for the completion service.
func WithLogger(l *zap.SugaredLogger) CompletionServiceOption {
	return func(s *CompletionService) {
		s.logger = l
	}
}

// NewCompletionService creates a new CompletionService with the given configuration.
func NewCompletionService(cfg *Config, model llms.Model, opts ...CompletionServiceOption) (*CompletionService, error) {
	if cfg == nil {
		return nil, errors.New("config cannot be nil")
	}
	if model == nil {
		return nil, errors.New("model cannot be nil")
	}

	s := &CompletionService{
		cfg:               cfg,
		model:             model,
		payload:           newCompletionPayload(cfg),
		completionTimeout: cfg.CompletionTimeout,
		Stdout:            os.Stdout,
		Stderr:            os.Stderr,
	}
	for _, opt := range opts {
		opt(s)
	}
	s.loggerCfg = zap.NewDevelopmentConfig()
	if s.logger == nil {
		// Create custom WriteSyncer for Stderr only
		stderrSyncer := zapcore.AddSync(os.Stderr)

		// Create custom Core with the WriteSyncer
		core := zapcore.NewCore(
			zapcore.NewConsoleEncoder(s.loggerCfg.EncoderConfig),
			stderrSyncer,
			s.loggerCfg.Level,
		)

		// Create logger with the custom Core
		logger := zap.New(core)
		s.logger = logger.Sugar()
	}

	return s, nil
}

// PerformCompletionConfig is the configuration for the PerformCompletion method, it controls the behavior of the completion with regard to user interaction.
type PerformCompletionConfig struct {
	Stdout      io.Writer
	EchoPrefill bool
	ShowSpinner bool
}

// Run runs the completion service with the given configuration.
func (s *CompletionService) Run(ctx context.Context, runCfg RunOptions) error {
	s.readlineHistoryFile = runCfg.ReadlineHistoryFile
	s.loggerCfg.Level.SetLevel(zap.WarnLevel)
	if runCfg.Verbose {
		s.loggerCfg.Level.SetLevel(zap.InfoLevel)
	}
	if runCfg.DebugMode {
		s.loggerCfg.Level.SetLevel(zap.DebugLevel)
	}
	if err := s.handleHistory(runCfg.HistoryIn, runCfg.HistoryOut); err != nil {
		fmt.Fprintln(s.Stderr, err)
	}
	if !s.loadedWithHistory() && s.cfg.SystemPrompt != "" {
		s.payload.addSystemMessage(s.cfg.SystemPrompt)
	}
	if runCfg.Prefill != "" {
		s.SetNextCompletionPrefill(runCfg.Prefill)
	}
	if runCfg.Stdout == nil {
		runCfg.Stdout = os.Stdout
	}

	if err := s.processInputs(ctx, runCfg); err != nil {
		return fmt.Errorf("failed to process inputs: %w", err)
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

func readStdin() (string, error) {
	var input strings.Builder
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		input.WriteString(scanner.Text())
		input.WriteString("\n")
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("failed to read from stdin: %w", err)
	}
	return input.String(), nil
}

func createInputProcessor(input string) (func() (string, error), error) {
	if input == "-" {
		return readStdin, nil
	}

	if _, err := os.Stat(input); err == nil {
		return func() (string, error) {
			content, err := os.ReadFile(input)
			return string(content), err
		}, nil
	}

	return func() (string, error) {
		return input, nil
	}, nil
}

func (s *CompletionService) processInputs(_ context.Context, cfg RunOptions) error {
	inputs := make([]string, 0, len(cfg.InputStrings)+len(cfg.InputFiles)+len(cfg.PositionalArgs))
	inputs = append(inputs, cfg.InputStrings...)
	inputs = append(inputs, cfg.InputFiles...)
	inputs = append(inputs, cfg.PositionalArgs...)

	var combinedInput strings.Builder

	for _, input := range inputs {
		processor, err := createInputProcessor(input)
		if err != nil {
			return err
		}
		content, err := processor()
		if err != nil {
			return fmt.Errorf("failed to process input %s: %w", input, err)
		}
		combinedInput.WriteString(content)
		combinedInput.WriteString("\n")
	}

	if combinedInput.Len() > 0 {
		s.payload.addUserMessage(combinedInput.String())
	}

	return nil
}

func (s *CompletionService) runOneShotCompletionStreaming(ctx context.Context, runCfg RunOptions) error {
	s.logger.Debug("running one-shot completion with streaming")

	s.payload.Stream = true
	streamPayloads, err := s.PerformCompletionStreaming(ctx, s.payload, PerformCompletionConfig{
		ShowSpinner: runCfg.ShowSpinner,
		EchoPrefill: !runCfg.EchoPrefill,
	})
	if err != nil {
		return fmt.Errorf("failed to perform completion streaming: %w", err)
	}
	content := strings.Builder{}
	for r := range streamPayloads {
		content.WriteString(r)
		runCfg.Stdout.Write([]byte(r))
	}
	s.payload.addAssistantMessage(content.String())
	if err := s.saveHistory(); err != nil {
		return fmt.Errorf("failed to save history: %w", err)
	}
	return nil
}

// Non-streaming version of one-shot completion.
func (s *CompletionService) runOneShotCompletion(ctx context.Context, runCfg RunOptions) error {
	s.logger.Debug("running one-shot completion")

	s.payload.Stream = false
	response, err := s.PerformCompletion(ctx, s.payload, PerformCompletionConfig{
		ShowSpinner: runCfg.ShowSpinner,
		EchoPrefill: !runCfg.EchoPrefill,
	})
	if err != nil {
		return err
	}
	runCfg.Stdout.Write([]byte(response))
	s.payload.addAssistantMessage(response)
	if err := s.saveHistory(); err != nil {
		return fmt.Errorf("failed to save history: %w", err)
	}
	return nil
}

// Enhanced function to run continuous streaming completion mode.
func (s *CompletionService) runContinuousCompletionStreaming(ctx context.Context, runCfg RunOptions) error {
	fmt.Fprintf(s.Stderr, "\033[38;5;240mcgpt: Running in continuous mode. Press ctrl+c to exit.\033[0m\n")

	// If we have processed inputs, generate an initial response
	if len(s.payload.Messages) > 0 && s.payload.Messages[len(s.payload.Messages)-1].Role == llms.ChatMessageTypeHuman {
		if err := s.generateResponse(ctx, runCfg); err != nil {
			return fmt.Errorf("failed to generate initial response: %w", err)
		}
	}

	processFn := func(input string) error {
		s.payload.addUserMessage(input)
		return s.generateResponse(ctx, runCfg)
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
func (s *CompletionService) runContinuousCompletion(ctx context.Context, runCfg RunOptions) error {
	fmt.Fprintln(s.Stderr, "Running in continuous mode. Press ctrl+c to exit.")
	processFn := func(input string) error {
		s.payload.addUserMessage(input)
		response, err := s.PerformCompletion(ctx, s.payload, PerformCompletionConfig{
			ShowSpinner: runCfg.ShowSpinner,
			EchoPrefill: !runCfg.EchoPrefill,
		})
		if err != nil {
			return err
		}
		runCfg.Stdout.Write([]byte(response))
		s.payload.addAssistantMessage(response)
		runCfg.Stdout.Write([]byte("\n"))
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

func (s *CompletionService) generateResponse(ctx context.Context, runCfg RunOptions) error {
	s.payload.Stream = runCfg.StreamOutput
	if runCfg.StreamOutput {
		streamPayloads, err := s.PerformCompletionStreaming(ctx, s.payload, PerformCompletionConfig{
			ShowSpinner: runCfg.ShowSpinner,
			EchoPrefill: !runCfg.EchoPrefill,
		})
		if err != nil {
			return fmt.Errorf("failed to perform completion streaming: %w", err)
		}
		content := strings.Builder{}
		for r := range streamPayloads {
			content.WriteString(r)
			runCfg.Stdout.Write([]byte(r))
		}
		s.payload.addAssistantMessage(content.String())
	} else {
		response, err := s.PerformCompletion(ctx, s.payload, PerformCompletionConfig{
			ShowSpinner: runCfg.ShowSpinner,
			EchoPrefill: !runCfg.EchoPrefill,
		})
		if err != nil {
			return err
		}
		runCfg.Stdout.Write([]byte(response))
		s.payload.addAssistantMessage(response)
	}
	if err := s.saveHistory(); err != nil {
		return fmt.Errorf("failed to save history: %w", err)
	}
	return nil
}

// SetNextCompletionPrefill sets the next completion prefill message.
// Note that not all inference engines support prefill messages.
// Whitespace is trimmed from the end of the message.
func (s *CompletionService) SetNextCompletionPrefill(content string) {
	s.nextCompletionPrefill = strings.TrimRight(content, " \t\n")
}

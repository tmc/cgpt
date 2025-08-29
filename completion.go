package cgpt

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
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
	historyFile         *os.File      // File handle for new history system
	historyManager      *historyManager
	historyMetadata     *historyMetadata // Metadata for the history file
	readlineHistoryFile string
	disableHistory      bool
	autoNameHistory     bool
	autoHistory         bool  // Whether this is an auto-generated session

	performCompletionConfig PerformCompletionConfig

	// nextCompletionPrefill is the message to prefill the assistant with for the next completion.
	nextCompletionPrefill string

	// Stdout is the writer for standard output. If nil, os.Stdout will be used.
	Stdout io.Writer
	// Stderr is the writer for standard error. If nil, os.Stderr will be used.
	Stderr io.Writer

	// sessionTimestamp is used to create a consistent history file name for the entire session
	sessionTimestamp string

	// useLegacyMaxTokens uses the legacy max_tokens field for OpenAI-compatible backends
	useLegacyMaxTokens bool
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

// WithDisableHistory sets whether history saving is disabled
func WithDisableHistory(disable bool) CompletionServiceOption {
	return func(s *CompletionService) {
		s.disableHistory = disable
	}
}

// WithUseLegacyMaxTokens sets whether to use the legacy max_tokens field for OpenAI-compatible backends
func WithUseLegacyMaxTokens(useLegacy bool) CompletionServiceOption {
	return func(s *CompletionService) {
		s.useLegacyMaxTokens = useLegacy
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
		sessionTimestamp:  time.Now().Format("20060102150405"),
	}
	for _, opt := range opts {
		opt(s)
	}
	// Ensure Stdout and Stderr are never nil
	if s.Stdout == nil {
		s.Stdout = os.Stdout
	}
	if s.Stderr == nil {
		s.Stderr = os.Stderr
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

func (s *CompletionService) Run(ctx context.Context, runCfg RunOptions) error {
	if err := s.configure(runCfg); err != nil {
		return fmt.Errorf("configuration error: %w", err)
	}
	if err := s.setupSystemPrompt(); err != nil {
		return fmt.Errorf("system prompt setup error: %w", err)
	}
	if err := s.handleInput(ctx, runCfg); err != nil {
		return fmt.Errorf("input handling error: %w", err)
	}
	err := s.executeCompletion(ctx, runCfg)
	
	// Handle post-completion tasks (naming, cleanup)
	s.finalizeHistory(ctx, runCfg)
	
	return err
}

func (s *CompletionService) configure(runCfg RunOptions) error {
	s.readlineHistoryFile = runCfg.ReadlineHistoryFile
	s.configureLogLevel(runCfg)

	// Initialize history manager
	var err error
	s.historyManager, err = newHistoryManager()
	if err != nil {
		return fmt.Errorf("failed to initialize history manager: %w", err)
	}

	// Setup history based on flags
	if err := s.setupHistory(runCfg); err != nil {
		fmt.Fprintln(s.Stderr, err)
	}
	
	if runCfg.Prefill != "" {
		s.SetNextCompletionPrefill(runCfg.Prefill)
	}
	if runCfg.Stdout == nil {
		runCfg.Stdout = os.Stdout
	}
	return nil
}

func (s *CompletionService) configureLogLevel(runCfg RunOptions) {
	s.loggerCfg.Level.SetLevel(zap.WarnLevel)
	if runCfg.Verbose {
		s.loggerCfg.Level.SetLevel(zap.InfoLevel)
	}
	if runCfg.DebugMode {
		s.loggerCfg.Level.SetLevel(zap.DebugLevel)
	}
}

func (s *CompletionService) setupSystemPrompt() error {
	if s.loadedWithHistory() || s.cfg.SystemPrompt == "" {
		return nil
	}

	s.payload.Messages = append([]llms.MessageContent(nil), s.payload.Messages...)
	sysMsg := llms.TextParts(llms.ChatMessageTypeSystem, s.cfg.SystemPrompt)

	sysIdx := slices.IndexFunc(s.payload.Messages, func(m llms.MessageContent) bool {
		return m.Role == "system"
	})

	if sysIdx >= 0 {
		s.payload.Messages[sysIdx] = sysMsg
	} else {
		s.payload.Messages = append([]llms.MessageContent{sysMsg}, s.payload.Messages...)
	}

	return nil
}

func (s *CompletionService) handleInput(ctx context.Context, runCfg RunOptions) error {
	r, err := runCfg.GetCombinedInputReader(ctx)
	if err != nil {
		return fmt.Errorf("failed to get inputs: %w", err)
	}

	input, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("failed to read inputs: %w", err)
	}

	if len(input) != 0 {
		s.payload.addUserMessage(string(input))
	}

	return nil
}

func (s *CompletionService) executeCompletion(ctx context.Context, runCfg RunOptions) error {
	if runCfg.Continuous {
		if runCfg.StreamOutput {
			return s.runContinuousCompletionStreaming(ctx, runCfg)
		}
		return s.runContinuousCompletion(ctx, runCfg)
	}

	if runCfg.StreamOutput {
		return s.runOneShotCompletionStreaming(ctx, runCfg)
	}
	return s.runOneShotCompletion(ctx, runCfg)
}

func (s *CompletionService) loadedWithHistory() bool {
	return s.historyIn != nil
}


// setupHistory handles all history flag combinations
func (s *CompletionService) setupHistory(runCfg RunOptions) error {
	// Validate flag combinations
	flagCount := 0
	if runCfg.History != "" {
		flagCount++
	}
	if runCfg.HistoryIn != "" || runCfg.HistoryOut != "" {
		flagCount++
	}
	if runCfg.Continue {
		flagCount++
	}
	
	if flagCount > 1 {
		return fmt.Errorf("cannot combine -H/--history with -I/-O or -C flags")
	}
	
	// Handle each flag type
	if runCfg.Continue {
		return s.setupContinue()
	}
	
	if runCfg.History != "" {
		return s.setupHistoryFile(runCfg.History)
	}
	
	if runCfg.HistoryIn != "" || runCfg.HistoryOut != "" {
		return s.setupExplicitHistory(runCfg.HistoryIn, runCfg.HistoryOut)
	}
	
	// No history flags = no history
	s.disableHistory = true
	return nil
}

// setupContinue finds and continues the most recent session
func (s *CompletionService) setupContinue() error {
	sessionsDir := filepath.Join(s.historyManager.homeDir, ".cgpt", "history", "sessions")
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		return fmt.Errorf("failed to create sessions directory: %w", err)
	}
	
	pattern := filepath.Join(sessionsDir, "*.yaml")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("failed to find sessions: %w", err)
	}
	
	if len(matches) == 0 {
		return fmt.Errorf("no previous sessions found")
	}
	
	// Find most recent
	var latest string
	var latestTime time.Time
	for _, match := range matches {
		info, err := os.Stat(match)
		if err != nil {
			continue
		}
		if info.ModTime().After(latestTime) {
			latest = match
			latestTime = info.ModTime()
		}
	}
	
	return s.setupHistoryFile(latest)
}

// setupHistoryFile handles -H flag (read and write same file or auto)
func (s *CompletionService) setupHistoryFile(historySpec string) error {
	if historySpec == "auto" {
		// Generate auto path and write-only
		path, file, _, err := s.historyManager.resolveHistoryPath("auto")
		if err != nil {
			return fmt.Errorf("failed to create auto history path: %w", err)
		}
		s.historyOutFile = path
		s.historyFile = file
		s.autoHistory = true  // Mark this as an auto-generated session
		
		// Initialize metadata for new session
		s.historyMetadata = &historyMetadata{
			Created: time.Now().Format(time.RFC3339),
		}
		return nil
	}
	
	// Explicit file - read from it if exists, write to it
	if _, err := os.Stat(historySpec); err == nil {
		// File exists, load it
		f, err := os.Open(historySpec)
		if err != nil {
			return fmt.Errorf("failed to open history file for reading: %w", err)
		}
		s.historyIn = f
		defer f.Close()
		
		if err := s.loadHistory(); err != nil {
			return fmt.Errorf("failed to load history: %w", err)
		}
	}
	
	// Setup for writing
	f, err := os.OpenFile(historySpec, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to open history file for writing: %w", err)
	}
	
	s.historyOutFile = historySpec
	s.historyFile = f
	return nil
}

// setupExplicitHistory handles separate -I and -O flags
func (s *CompletionService) setupExplicitHistory(historyIn, historyOut string) error {
	// Setup input if specified
	if historyIn != "" {
		f, err := os.Open(historyIn)
		if err != nil {
			return fmt.Errorf("failed to open input history file: %w", err)
		}
		s.historyIn = f
		defer f.Close()
		
		if err := s.loadHistory(); err != nil {
			return fmt.Errorf("failed to load history: %w", err)
		}
		
		// Track fork if outputting to a different file
		if historyOut != "" && historyOut != historyIn && historyOut != "-" {
			if s.historyMetadata == nil {
				s.historyMetadata = &historyMetadata{}
			}
			s.historyMetadata.ForkedFrom = historyIn
			s.historyMetadata.ForkPoint = len(s.payload.Messages)
			if s.historyMetadata.Created == "" {
				s.historyMetadata.Created = time.Now().Format(time.RFC3339)
			}
		}
	}
	
	// Setup output if specified
	if historyOut != "" {
		if historyOut == "-" {
			s.historyFile = os.Stdout
			s.historyOutFile = "-"
		} else {
			f, err := os.OpenFile(historyOut, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
			if err != nil {
				return fmt.Errorf("failed to open output history file: %w", err)
			}
			s.historyFile = f
			s.historyOutFile = historyOut
		}
	} else if historyIn != "" {
		// If only input specified, disable saving
		s.disableHistory = true
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

func (s *CompletionService) runOneShotCompletionStreaming(ctx context.Context, runCfg RunOptions) error {
	s.logger.Debug("running one-shot completion with streaming")

	s.payload.Stream = true
	streamPayloads, err := s.PerformCompletionStreaming(ctx, s.payload, PerformCompletionConfig{
		ShowSpinner: runCfg.ShowSpinner,
		EchoPrefill: runCfg.EchoPrefill,
	})
	if err != nil {
		return fmt.Errorf("failed to perform completion streaming: %w", err)
	}
	content := strings.Builder{}
	for r := range streamPayloads {
		content.WriteString(r)
		// Only write response to stdout if we're not outputting history to stdout
		if s.historyOutFile != "-" {
			s.Stdout.Write([]byte(r))
		}
	}
	if err := s.saveHistory(); err != nil {
		return fmt.Errorf("failed to save history: %w", err)
	}

	if err := s.renameChatHistory(ctx); err != nil {
		return fmt.Errorf("failed to rename history: %w", err)
	}
	return nil
}

// Non-streaming version of one-shot completion.
func (s *CompletionService) runOneShotCompletion(ctx context.Context, runCfg RunOptions) error {
	s.logger.Debug("running one-shot completion")

	s.payload.Stream = false
	response, err := s.PerformCompletion(ctx, s.payload, PerformCompletionConfig{
		ShowSpinner: runCfg.ShowSpinner,
		EchoPrefill: runCfg.EchoPrefill,
	})
	if err != nil {
		return err
	}
	// Only write response to stdout if we're not outputting history to stdout
	if s.historyOutFile != "-" {
		s.Stdout.Write([]byte(response))
	}
	if err := s.saveHistory(); err != nil {
		return fmt.Errorf("failed to save history: %w", err)
	}
	if err := s.renameChatHistory(ctx); err != nil {
		return fmt.Errorf("failed to rename history: %w", err)
	}
	return nil
}

// Enhanced function to run continuous streaming completion mode.
func (s *CompletionService) runContinuousCompletionStreaming(ctx context.Context, runCfg RunOptions) error {
	fmt.Fprintf(s.Stderr, "\033[38;5;240mcgpt: Running in continuous mode. Press ctrl+c to exit.\033[0m\n")

	// Setup context with cancellation
	ctxWithCancel, cancel := context.WithCancel(ctx)
	defer cancel()

	// Set up shutdown handler to catch ctrl+c and generate title
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		fmt.Fprintf(s.Stderr, "\n\033[38;5;240mcgpt: Generating descriptive title for chat history...\033[0m\n")

		// Create a new timeout context for title generation
		titleCtx, titleCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer titleCancel()

		// Generate a title and rename the history file
		if err := s.renameChatHistory(titleCtx); err != nil {
			fmt.Fprintf(s.Stderr, "\033[38;5;240mcgpt: Failed to rename history: %v\033[0m\n", err)
		}

		// Now cancel the main context
		cancel()
	}()

	// If we have processed inputs, generate an initial response
	if len(s.payload.Messages) > 0 && s.payload.Messages[len(s.payload.Messages)-1].Role == llms.ChatMessageTypeHuman {
		if err := s.generateResponse(ctxWithCancel, runCfg); err != nil {
			return fmt.Errorf("failed to generate initial response: %w", err)
		}
	}

	processFn := func(input string) error {
		input = strings.TrimSpace(input)
		if input == "" {
			return interactive.ErrEmptyInput
		}
		s.payload.addUserMessage(input)
		return s.generateResponse(ctxWithCancel, runCfg)
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

	err = session.Run()

	// Before returning, try to rename the history file with a descriptive title
	if ctxWithCancel.Err() == nil { // Only if we haven't already done it in signal handler
		rctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if renameErr := s.renameChatHistory(rctx); renameErr != nil {
			fmt.Fprintf(s.Stderr, "\033[38;5;240mcgpt: Failed to rename history: %v\033[0m\n", renameErr)
		}
	}

	return err
}

// Non-streaming version of continuous completion.
func (s *CompletionService) runContinuousCompletion(ctx context.Context, runCfg RunOptions) error {
	fmt.Fprintln(s.Stderr, "Running in continuous mode. Press ctrl+c to exit.")
	processFn := func(input string) error {
		input = strings.TrimSpace(input)
		if input == "" {
			return interactive.ErrEmptyInput
		}
		s.payload.addUserMessage(input)
		response, err := s.PerformCompletion(ctx, s.payload, PerformCompletionConfig{
			ShowSpinner: runCfg.ShowSpinner,
			EchoPrefill: runCfg.EchoPrefill,
		})
		if err != nil {
			return err
		}
		// Only write response to stdout if we're not outputting history to stdout
		if s.historyOutFile != "-" {
			s.Stdout.Write([]byte(response))
			s.Stdout.Write([]byte("\n"))
		}
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
			EchoPrefill: runCfg.EchoPrefill,
		})
		if err != nil {
			return fmt.Errorf("failed to perform completion streaming: %w", err)
		}
		content := strings.Builder{}
		for r := range streamPayloads {
			content.WriteString(r)
			// Only write response to stdout if we're not outputting history to stdout
			if s.historyOutFile != "-" {
				s.Stdout.Write([]byte(r))
			}
		}
		if s.historyOutFile != "-" {
			s.Stdout.Write([]byte("\n"))
		}
	} else {
		response, err := s.PerformCompletion(ctx, s.payload, PerformCompletionConfig{
			ShowSpinner: runCfg.ShowSpinner,
			EchoPrefill: runCfg.EchoPrefill,
		})
		if err != nil {
			return err
		}
		// Only write response to stdout if we're not outputting history to stdout
		if s.historyOutFile != "-" {
			s.Stdout.Write([]byte(response))
		}
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

func (s *CompletionService) finalizeHistory(ctx context.Context, runCfg RunOptions) {
	// Close history file if open
	if s.historyFile != nil && s.historyFile != os.Stdout {
		s.historyFile.Close()
		
		// Note: Named session functionality removed for simplicity
	}
}

func (s *CompletionService) generateHistoryName(ctx context.Context) string {
	// Generate a name based on the conversation
	// For now, use a simple approach - can be enhanced with AI later
	if len(s.payload.Messages) > 0 {
		firstMsg := s.payload.Messages[0]
		for _, part := range firstMsg.Parts {
			if text, ok := part.(llms.TextContent); ok {
				// Take first 50 chars, clean up
				name := strings.TrimSpace(text.Text)
				if len(name) > 50 {
					name = name[:50]
				}
				// Replace problematic characters
				name = strings.ReplaceAll(name, "/", "-")
				name = strings.ReplaceAll(name, "\n", " ")
				name = strings.ReplaceAll(name, "\t", " ")
				// Collapse multiple spaces
				name = strings.Join(strings.Fields(name), "-")
				name = strings.ToLower(name)
				return name
			}
		}
	}
	return "conversation"
}

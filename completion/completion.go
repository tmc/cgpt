package completion

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
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

// Service is the main entry point for the completion service.
type Service struct {
	cfg *Config

	loggerCfg zap.Config
	logger    *zap.SugaredLogger

	model llms.Model

	payload *ChatCompletionPayload

	completionTimeout time.Duration

	historyIn           io.Reader
	historyOutFile      string
	readlineHistoryFile string

	opts *Options

	// nextCompletionPrefill is the message to prefill the assistant with for the next completion.
	nextCompletionPrefill string

	// sessionTimestamp is used to create a consistent history file name for the entire session
	sessionTimestamp string

	// activeSession is the interactive session being used (for controlling UI state)
	activeSession interactive.Session
}

// Config holds the static configuration for the Service.
type Config struct {
	// MaxTokens is the maximum number of tokens to generate.
	MaxTokens int
	// Temperature controls randomness in generation (0.0-1.0)
	Temperature float64
	// System prompt for the model
	SystemPrompt string
	// CompletionTimeout is the timeout for completion generation
	CompletionTimeout time.Duration
}

// Options is the configuration for the Service.
type Options struct {
	// Stdout is the writer for standard output. If nil, os.Stdout will be used.
	Stdout io.Writer
	// Stderr is the writer for standard error. If nil, os.Stderr will be used.
	Stderr io.Writer

	EchoPrefill bool
	ShowSpinner bool
	PrintUsage  bool

	// CompletionTimeout is the timeout for the completion.
	CompletionTimeout time.Duration

	// History options
	HistoryIn           string
	HistoryOut          string
	ReadlineHistoryFile string

	// Completion prefill
	Prefill string

	// Verbosity options
	Verbose   bool
	DebugMode bool

	// StreamOutput controls whether to stream the output.
	StreamOutput bool

	// Continuous controls whether to run in continuous mode.
	Continuous bool

	// UseTUI controls whether to use BubbleTea UI for interactive mode.
	UseTUI bool
}

type ServiceOption func(*Service)

// WithOptions is a ServiceOption that sets the whole options struct
func WithOptions(opts Options) ServiceOption {
	return func(s *Service) {
		s.opts = &opts
	}
}

// WithStdout sets the stdout writer
func WithStdout(w io.Writer) ServiceOption {
	return func(s *Service) {
		s.opts.Stdout = w
	}
}

// WithStderr sets the stderr writer
func WithStderr(w io.Writer) ServiceOption {
	return func(s *Service) {
		s.opts.Stderr = w
	}
}

// WithLogger sets the logger for the completion service.
func WithLogger(l *zap.SugaredLogger) ServiceOption {
	return func(s *Service) {
		s.logger = l
	}
}

// New creates a new Service with the given configuration.
func New(cfg *Config, model llms.Model, opts ...ServiceOption) (*Service, error) {
	if cfg == nil {
		return nil, errors.New("config cannot be nil")
	}
	if model == nil {
		return nil, errors.New("model cannot be nil")
	}

	// Create default options
	defaultOpts := NewOptions()
	defaultOpts.CompletionTimeout = cfg.CompletionTimeout

	s := &Service{
		cfg:               cfg,
		model:             model,
		payload:           newCompletionPayload(cfg),
		completionTimeout: cfg.CompletionTimeout,
		opts:              &defaultOpts,
		sessionTimestamp:  time.Now().Format("20060102150405"),
	}

	// Apply options
	for _, opt := range opts {
		opt(s)
	}

	// Setup logger
	s.loggerCfg = zap.NewDevelopmentConfig()
	if s.logger == nil {
		// Create custom WriteSyncer for Stderr only
		stderrSyncer := zapcore.AddSync(s.opts.Stderr)

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

	// Configure log level
	s.loggerCfg.Level.SetLevel(zap.WarnLevel)
	if s.opts.Verbose {
		s.loggerCfg.Level.SetLevel(zap.InfoLevel)
	}
	if s.opts.DebugMode {
		s.loggerCfg.Level.SetLevel(zap.DebugLevel)
	}

	// Handle history files
	if err := s.handleHistory(s.opts.HistoryIn, s.opts.HistoryOut); err != nil {
		fmt.Fprintln(s.opts.Stderr, err)
	}

	// Set prefill if needed
	if s.opts.Prefill != "" {
		s.SetNextCompletionPrefill(s.opts.Prefill)
	}
	return s, nil
}

// NewOptions creates a new Options with defaults.
func NewOptions() Options {
	return Options{
		Stdout:              os.Stdout,
		Stderr:              os.Stderr,
		ShowSpinner:         true,
		EchoPrefill:         false,
		PrintUsage:          false,
		UseTUI:              false, // Default to classic readline UI
		CompletionTimeout:   30 * time.Second,
		ReadlineHistoryFile: "~/.cgpt_history",
	}
}

// RunOptions contains the options for a single run of the completion service.
type RunOptions struct {
	// Config options
	*Config
	// Input options
	InputStrings   []string
	InputFiles     []string
	PositionalArgs []string
	Prefill        string

	// Output options
	Continuous   bool
	StreamOutput bool
	ShowSpinner  bool
	EchoPrefill  bool
	UseTUI       bool // Use BubbleTea UI for interactive mode
	PrintUsage   bool

	// Verbosity options
	Verbose   bool
	DebugMode bool

	// History options
	HistoryIn           string
	HistoryOut          string
	ReadlineHistoryFile string
	NCompletions        int

	// I/O
	Stdout io.Writer
	Stderr io.Writer
	Stdin  io.Reader

	// Timing
	MaximumTimeout time.Duration

	ConfigPath string

	// Backend/Provider-specific options.
	OpenAIUseLegacyMaxTokens bool
}

// Run executes a completion using the service's options
func (s *Service) Run(ctx context.Context, runCfg RunOptions) error {
	// Update Options from RunOptions
	// (this will eventually go away when we fully migrate to Options)
	tempOpts := RunOptionsToOptions(runCfg)

	// Apply input handling from RunOptions (this will go away too)
	if err := s.setupSystemPrompt(); err != nil {
		return fmt.Errorf("system prompt setup error: %w", err)
	}
	if err := s.handleInput(ctx, runCfg); err != nil {
		return fmt.Errorf("input handling error: %w", err)
	}

	// Execute the completion based on options
	if tempOpts.Continuous {
		if tempOpts.StreamOutput {
			return s.runContinuousCompletionStreaming(ctx, runCfg)
		}
		return s.runContinuousCompletion(ctx, runCfg)
	}

	if tempOpts.StreamOutput {
		return s.runOneShotCompletionStreaming(ctx, runCfg)
	}
	return s.runOneShotCompletion(ctx, runCfg)
}

// RunOptionsToOptions converts RunOptions to Options
func RunOptionsToOptions(runOpts RunOptions) Options {
	options := NewOptions()

	// Only override non-nil/non-zero values
	if runOpts.Stdout != nil {
		options.Stdout = runOpts.Stdout
	}
	if runOpts.Stderr != nil {
		options.Stderr = runOpts.Stderr
	}

	options.ShowSpinner = runOpts.ShowSpinner
	options.EchoPrefill = runOpts.EchoPrefill
	options.PrintUsage = runOpts.PrintUsage
	options.StreamOutput = runOpts.StreamOutput
	options.Continuous = runOpts.Continuous
	options.UseTUI = runOpts.UseTUI
	options.Verbose = runOpts.Verbose
	options.DebugMode = runOpts.DebugMode

	options.HistoryIn = runOpts.HistoryIn
	options.HistoryOut = runOpts.HistoryOut
	options.ReadlineHistoryFile = runOpts.ReadlineHistoryFile
	options.Prefill = runOpts.Prefill

	if runOpts.CompletionTimeout > 0 {
		options.CompletionTimeout = runOpts.CompletionTimeout
	}

	return options
}

func (s *Service) PerformCompletionStreaming(ctx context.Context, payload *ChatCompletionPayload) (<-chan string, error) {
	ch := make(chan string)
	go func() {
		defer close(ch)
		fullResponse := strings.Builder{}
		firstChunk := true
		addedAssistantMessage := false

		prefillCleanup, spinnerPos := s.handleAssistantPrefill(ctx, payload)

		// Send prefill immediately if it exists
		if s.nextCompletionPrefill != "" {
			if s.opts.EchoPrefill {
				spinnerPos = len(s.nextCompletionPrefill) + 1
			}
			select {
			case ch <- s.nextCompletionPrefill + " ":
			case <-ctx.Done():
				prefillCleanup()
				return
			}
			payload.addAssistantMessage(s.nextCompletionPrefill)
			addedAssistantMessage = true
			fullResponse.WriteString(s.nextCompletionPrefill)
		}

		// Start spinner on the last character
		var spinnerStop func()
		if s.opts.ShowSpinner {
			spinnerStop = spin(spinnerPos)
		}

		// Create a cancellable context for the generation
		genCtx, cancel := context.WithCancel(ctx)
		defer cancel()

		// Handle ctrl-c by cancelling the generation context
		go func() {
			select {
			case <-ctx.Done():
				cancel()
			case <-genCtx.Done():
			}
		}()

		_, err := s.model.GenerateContent(genCtx, payload.Messages,
			llms.WithMaxTokens(s.cfg.MaxTokens),
			llms.WithTemperature(s.cfg.Temperature),
			llms.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
				if firstChunk {
					prefillCleanup()
					if spinnerStop != nil {
						spinnerStop()
						spinnerStop = nil
					}
					firstChunk = false
				}

				select {
				case ch <- string(chunk):
					fullResponse.Write(chunk)
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			}))

		if err != nil && !errors.Is(err, context.Canceled) {
			log.Printf("failed to generate content: %v", err)
		}

		// Clean up spinner if it's still running
		if spinnerStop != nil {
			spinnerStop()
		}

		// Add the assistant message if we haven't already
		if !addedAssistantMessage {
			payload.addAssistantMessage(fullResponse.String())
		}

		s.nextCompletionPrefill = ""
	}()
	return ch, nil
}

// PerformCompletion provides a non-streaming version of the completion.
func (s *Service) PerformCompletion(ctx context.Context, payload *ChatCompletionPayload) (string, error) {
	var stopSpinner func()
	var spinnerPos int
	addedAssistantMessage := false

	prefillCleanup, spinnerPos := s.handleAssistantPrefill(ctx, payload)
	defer prefillCleanup()

	if s.nextCompletionPrefill != "" {
		payload.addAssistantMessage(s.nextCompletionPrefill)
		addedAssistantMessage = true
	}

	if s.opts.ShowSpinner {
		stopSpinner = spin(spinnerPos)
		defer stopSpinner()
	}

	response, err := s.model.GenerateContent(ctx, payload.Messages,
		llms.WithMaxTokens(s.cfg.MaxTokens),
		llms.WithTemperature(s.cfg.Temperature))
	if err != nil {
		return "", fmt.Errorf("failed to generate content: %w", err)
	}
	if len(response.Choices) == 0 {
		return "", fmt.Errorf("no response from model")
	}

	content := response.Choices[0].Content
	if !addedAssistantMessage {
		payload.addAssistantMessage(content)
	}

	return content, nil
}

// handleAssistantPrefill handles the assistant prefill message.
// It returns a cleanup function that should be called after the completion is done.
// The second return value is the location where the spinner could start.
func (s *Service) handleAssistantPrefill(ctx context.Context, payload *ChatCompletionPayload) (func(), int) {
	spinnerPos := 0
	if s.nextCompletionPrefill == "" {
		return func() {}, spinnerPos
	}

	// Store the current message count to ensure proper cleanup
	initialMessageCount := len(payload.Messages)

	if s.opts.EchoPrefill {
		s.opts.Stdout.Write([]byte(s.nextCompletionPrefill))
		spinnerPos = len(s.nextCompletionPrefill) + 1
	}

	payload.addAssistantMessage(s.nextCompletionPrefill)
	s.nextCompletionPrefill = ""

	return func() {
		// Only cleanup if we actually added a message
		if len(payload.Messages) > initialMessageCount {
			payload.Messages = payload.Messages[:initialMessageCount]
		}
	}, spinnerPos
}

// GetInputReader returns a reader for the given input strings and files.
func GetInputReader(ctx context.Context, files []string, strings []string, args []string, stdin io.Reader) (io.Reader, error) {
	handler := &InputHandler{
		Files:   files,
		Strings: strings,
		Args:    args,
		Stdin:   stdin,
	}
	return handler.Process(ctx)
}

func (s *Service) setupSystemPrompt() error {
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

// AddInput reads input from the provided RunOptions and adds it to the payload
func (s *Service) handleInput(ctx context.Context, runCfg RunOptions) error {
	r, err := GetInputReader(ctx, runCfg.InputFiles, runCfg.InputStrings, runCfg.PositionalArgs, runCfg.Stdin)
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

func (s *Service) loadedWithHistory() bool {
	return s.historyIn != nil
}

func (s *Service) handleHistory(historyIn, historyOut string) error {
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
		fmt.Println("")
		fmt.Println("issue in save history:", err)
		// return fmt.Errorf("failed to save history: %w", err) // TODO: handle
	}
	return nil
}

func (s *Service) getLastUserMessage() string {
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

func (s *Service) runOneShotCompletionStreaming(ctx context.Context, runCfg RunOptions) error {
	s.logger.Debug("running one-shot completion with streaming")
	s.payload.Stream = true
	streamPayloads, err := s.PerformCompletionStreaming(ctx, s.payload)
	if err != nil {
		return fmt.Errorf("failed to perform completion streaming: %w", err)
	}
	content := strings.Builder{}
	for r := range streamPayloads {
		content.WriteString(r)
		runCfg.Stdout.Write([]byte(r))
	}
	if err := s.saveHistory(); err != nil {
		fmt.Println("")
		fmt.Println("issue in save history:", err)
		// return fmt.Errorf("failed to save history: %w", err) // TODO: handle
	}
	return nil
}

// Non-streaming version of one-shot completion.
func (s *Service) runOneShotCompletion(ctx context.Context, runCfg RunOptions) error {
	s.logger.Debug("running one-shot completion")

	s.payload.Stream = false
	response, err := s.PerformCompletion(ctx, s.payload)
	if err != nil {
		return err
	}
	runCfg.Stdout.Write([]byte(response))
	if err := s.saveHistory(); err != nil {
		return fmt.Errorf("failed to save history: %w", err)
	}
	return nil
}

// Enhanced function to run continuous streaming completion mode.
func (s *Service) runContinuousCompletionStreaming(ctx context.Context, runCfg RunOptions) error {
	fmt.Fprintf(s.opts.Stderr, "\033[38;5;240mcgpt: Running in continuous mode. Press ctrl+c to exit, use up-arrow to edit last message.\033[0m\n")

	// Setup context with cancellation
	ctxWithCancel, cancel := context.WithCancel(ctx)
	defer cancel()

	// If we have processed inputs, generate an initial response
	if len(s.payload.Messages) > 0 && s.payload.Messages[len(s.payload.Messages)-1].Role == llms.ChatMessageTypeHuman {
		if err := s.generateResponse(ctxWithCancel, runCfg); err != nil {
			return fmt.Errorf("failed to generate initial response: %w", err)
		}
	}

	processFn := func(ctx context.Context, input string) error {
		input = strings.TrimSpace(input)
		if input == "" {
			return interactive.ErrEmptyInput
		}

		// Special command handling
		if input == "/last" {
			// Get the last user message, if available
			if lastMsg := s.getLastUserMessage(); lastMsg != "" {
				return interactive.ErrUseLastMessage(lastMsg)
			}
			fmt.Fprintf(s.opts.Stderr, "\033[38;5;240mNo previous message to edit.\033[0m\n")
			return interactive.ErrEmptyInput
		}

		// Add message to payload and generate response
		s.payload.addUserMessage(input)
		if err := s.generateResponse(ctxWithCancel, runCfg); err != nil {
			return fmt.Errorf("issue with response: %w", err)
		}
		if err := s.saveHistory(); err != nil {
			fmt.Println("")
			fmt.Println("issue in saving history:", err)
		}
		return nil
	}

	// Configure the interactive session
	sessionConfig := interactive.Config{
		Prompt:      ">>> ",
		AltPrompt:   "... ",
		HistoryFile: expandTilde(s.readlineHistoryFile), // TODO: decouple and tease apart history concerns.
		ProcessFn:   processFn,
	}

	// Check if we should use the TUI interface
	var session interactive.Session
	var err error

	// Use the platform-agnostic NewSession which will select the appropriate implementation
	// (BubbleSession for native, potentially WASM-compatible BubbleSession for js)
	session, err = interactive.NewSession(sessionConfig)
	if err != nil {
		return fmt.Errorf("failed to create interactive session: %w", err)
	}

	// Store session in the service to allow the generateResponse to manipulate prompt visibility
	s.activeSession = session

	err = s.activeSession.Run(ctxWithCancel)

	// If context was canceled, just return nil (clean exit)
	if ctxWithCancel.Err() != nil {
		return nil
	}

	// Before returning, try to rename the history file with a descriptive title
	rctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if renameErr := s.renameChatHistory(rctx); renameErr != nil {
		fmt.Fprintf(s.opts.Stderr, "\033[38;5;240mcgpt: Failed to rename history: %v\033[0m\n", renameErr)
	}

	return err
}

// Non-streaming version of continuous completion.
func (s *Service) runContinuousCompletion(ctx context.Context, runCfg RunOptions) error {
	fmt.Fprintf(s.opts.Stderr, "\033[38;5;240mcgpt: Running in continuous mode. Press ctrl+c to exit.\033[0m\n")

	// Setup context with cancellation
	ctxWithCancel, cancel := context.WithCancel(ctx)
	defer cancel()

	// Set up shutdown handler to catch ctrl+c and save history
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		// Add a line break if needed
		fmt.Fprintf(s.opts.Stderr, "\n\033[38;5;240mcgpt: Exiting...\033[0m\n")

		// Create a new timeout context for title generation
		titleCtx, titleCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer titleCancel()

		// Save history first before generating title
		if err := s.saveHistory(); err != nil {
			fmt.Fprintf(s.opts.Stderr, "\033[38;5;240mcgpt: Failed to save history: %v\033[0m\n", err)
		}

		// Generate a title and rename the history file
		if err := s.renameChatHistory(titleCtx); err != nil {
			fmt.Fprintf(s.opts.Stderr, "\033[38;5;240mcgpt: Failed to rename history: %v\033[0m\n", err)
		}

		// Now cancel the main context
		cancel()
	}()
	processFn := func(ctx context.Context, input string) error {
		input = strings.TrimSpace(input)
		if input == "" {
			return interactive.ErrEmptyInput
		}
		s.payload.addUserMessage(input)
		response, err := s.PerformCompletion(ctxWithCancel, s.payload)
		if err != nil {
			if ctxWithCancel.Err() != nil {
				// Context was canceled, just return without error
				return nil
			}
			return err
		}
		runCfg.Stdout.Write([]byte(response))
		runCfg.Stdout.Write([]byte("\n"))
		if err := s.saveHistory(); err != nil {
			fmt.Println("")
			fmt.Println("issue in save history:", err)
			// return fmt.Errorf("failed to save history: %w", err) // TODO: handle
		}
		return nil
	}

	sessionConfig := interactive.Config{
		Prompt:      ">>> ",
		AltPrompt:   "... ",
		HistoryFile: expandTilde(s.readlineHistoryFile),
		ProcessFn:   processFn,
	}

	// Check if we should use the TUI interface
	var session interactive.Session
	var err error

	// Use the platform-agnostic NewSession which will select the appropriate implementation
	session, err = interactive.NewSession(sessionConfig)
	if err == nil && runCfg.UseTUI {
		// Log if TUI was requested
		fmt.Fprintf(s.opts.Stderr, "\033[38;5;240mcgpt: Using Terminal UI mode (if supported by platform)\033[0m\n")
	}

	if err != nil {
		return err
	}

	// Store session in the service to allow the generateResponse to manipulate prompt visibility
	s.activeSession = session

	err = session.Run(ctxWithCancel)

	// If context was canceled, just return nil (clean exit)
	if ctxWithCancel.Err() != nil {
		return nil
	}

	return err
}

func expandTilde(path string) string {
	if strings.HasPrefix(path, "~/") {
		return strings.Replace(path, "~", os.Getenv("HOME"), 1)
	}
	return path
}

func (s *Service) generateResponse(ctx context.Context, runCfg RunOptions) error {
	s.payload.Stream = runCfg.StreamOutput

	if runCfg.StreamOutput {
		streamPayloads, err := s.PerformCompletionStreaming(ctx, s.payload)
		if err != nil {
			if ctx.Err() != nil {
				// Context was canceled (e.g. by ctrl+c)
				return ctx.Err()
			}
			return fmt.Errorf("failed to perform completion streaming: %w", err)
		}

		content := strings.Builder{}
		wasInterrupted := false

		for r := range streamPayloads {
			// Check for context cancellation in the middle of streaming
			if ctx.Err() != nil {
				// Context was canceled, stop processing
				wasInterrupted = true
				break
			}
			content.WriteString(r)
			if s.activeSession != nil { // Add nil check
				s.activeSession.AddResponsePart(r)
			}
			runCfg.Stdout.Write([]byte(r))
		}

		// Handle interruption or normal completion
		if wasInterrupted || ctx.Err() != nil {
			// Print a newline first to ensure clean output
			runCfg.Stdout.Write([]byte("\n"))
			fmt.Fprintf(runCfg.Stderr, "\033[38;5;240mResponse interrupted. Press Ctrl+C again to discard, or continue typing to keep partial response.\033[0m\n")

			// Store the partial response in the message history
			if content.Len() > 0 {
				s.payload.addAssistantMessage(content.String())

				// Save this partial response to history
				if err := s.saveHistory(); err != nil {
					fmt.Fprintf(runCfg.Stderr, "\033[38;5;240mWarning: Failed to save partial response: %v\033[0m\n", err)
				}
			}

			return ctx.Err()
		}

		// Normal completion - add newline and save response
		runCfg.Stdout.Write([]byte("\n"))

		// Add completed response to message history
		if content.Len() > 0 {
			s.payload.addAssistantMessage(content.String())
		}
	} else {
		response, err := s.PerformCompletion(ctx, s.payload)
		if err != nil {
			if ctx.Err() != nil {
				// Context was canceled (e.g. by ctrl+c)
				return ctx.Err()
			}
			return err
		}
		runCfg.Stdout.Write([]byte(response))

		// Add response to message history
		if response != "" {
			s.payload.addAssistantMessage(response)
		}
	}

	// Save history after successful completion
	if err := s.saveHistory(); err != nil {
		s.logger.Error("failed to save history", zap.Error(err))
		return nil
		//return fmt.Errorf("failed to save history: %w", err)
	}

	return nil
}

// SetNextCompletionPrefill sets the next completion prefill message.
// Note that not all inference engines support prefill messages.
// Whitespace is trimmed from the end of the message.
func (s *Service) SetNextCompletionPrefill(content string) {
	s.nextCompletionPrefill = strings.TrimRight(content, " \t\n")
}

// generateHistoryTitle sends the conversation history to the LLM to generate a descriptive title
func (s *Service) generateHistoryTitle(ctx context.Context) (string, error) {
	// Don't try to generate a title if we have no messages
	if len(s.payload.Messages) < 2 {
		return "empty-chat", nil
	}

	prompt := "Generate a kebab case title for the following conversation. An example is debug-rust-code or explain-quantum-mechanics."
	msgLimit := min(len(s.payload.Messages), 10)
	for _, m := range s.payload.Messages[:msgLimit] {
		for _, p := range m.Parts {
			prompt += fmt.Sprint(p)
		}
	}

	completion, err := llms.GenerateFromSinglePrompt(ctx, s.model, prompt)
	if err != nil {
		return "", fmt.Errorf("failed to generate title: %w", err)
	}

	fmt.Println("completion", completion)

	// If title is too long, truncate it
	const maxTitleLength = 50
	if len(completion) > maxTitleLength {
		completion = completion[:maxTitleLength]
	}

	return completion, nil
}

// renameChatHistory generates a title and renames the history file
func (s *Service) renameChatHistory(ctx context.Context) error {
	if s.historyOutFile == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get user home directory: %w", err)
		}

		// Get the current history file path
		currentPath := filepath.Join(home, ".cgpt", fmt.Sprintf("default-history-%s.yaml", s.sessionTimestamp))

		// Generate a descriptive title
		title, err := s.generateHistoryTitle(ctx)
		if err != nil {
			return fmt.Errorf("failed to generate title: %w", err)
		}

		// Create new filename with timestamp + title
		newPath := filepath.Join(home, ".cgpt", fmt.Sprintf("%s.yaml", title))

		// Rename the file
		if err := os.Rename(currentPath, newPath); err != nil {
			s.logger.Error("failed to rename history file", zap.Error(err))
			return nil
		}

		fmt.Fprintf(s.opts.Stderr, "\033[38;5;240mcgpt: Renamed history to: %s\033[0m\n", filepath.Base(newPath))

		// Update the historyOutFile to use the new path
		s.historyOutFile = newPath
	}
	return nil
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

// ChatCompletionPayload holds the messages for a chat completion.
type ChatCompletionPayload struct {
	Messages []llms.MessageContent
	Stream   bool
}

// newCompletionPayload creates a new ChatCompletionPayload.
func newCompletionPayload(cfg *Config) *ChatCompletionPayload {
	return &ChatCompletionPayload{
		Messages: []llms.MessageContent{},
		Stream:   true,
	}
}

// addUserMessage adds a user message to the completion payload.
func (p *ChatCompletionPayload) addUserMessage(content string) {
	p.Messages = append(p.Messages, llms.TextParts(llms.ChatMessageTypeHuman, content))
}

// AddUserMessage adds a user message to the service's payload
func (s *Service) AddUserMessage(content string) {
	s.payload.addUserMessage(content)
}

// addAssistantMessage adds an assistant message to the completion payload.
func (p *ChatCompletionPayload) addAssistantMessage(content string) {
	p.Messages = append(p.Messages, llms.TextParts(llms.ChatMessageTypeAI, content))
}

// loadHistory loads the history from the history file
func (s *Service) loadHistory() error {
	if s.historyIn == nil {
		return nil
	}
	// TODO: Actually implement loading history from the file
	// Placeholder for loading history
	return nil
}

// saveHistory saves the history to the history file
func (s *Service) saveHistory() error {
	if s.historyOutFile == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get user home directory: %w", err)
		}
		// Ensure directory exists
		if err := os.MkdirAll(filepath.Join(home, ".cgpt"), 0755); err != nil {
			return fmt.Errorf("failed to create history directory: %w", err)
		}
		// Set default history file
		s.historyOutFile = filepath.Join(home, ".cgpt", fmt.Sprintf("default-history-%s.yaml", s.sessionTimestamp))
	}

	// TODO: Actually implement saving history to file
	// Placeholder for saving history
	return nil
}

// Spinner implementation
type spinner struct {
	frames []string
	pos    int
	active bool
	done   chan struct{}
}

func newSpinner() *spinner {
	return &spinner{
		frames: []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		done:   make(chan struct{}),
	}
}

func (s *spinner) start() {
	s.active = true
	go func() {
		for s.active {
			select {
			case <-s.done:
				return
			default:
				fmt.Print("\b" + s.frames[s.pos])
				s.pos = (s.pos + 1) % len(s.frames)
				time.Sleep(80 * time.Millisecond)
			}
		}
	}()
}

func (s *spinner) stop() {
	s.active = false
	close(s.done)
	fmt.Print("\b \b") // Clear the spinner
}

// spin starts a spinner at the given character position
func spin(pos int) func() {
	if pos <= 0 {
		fmt.Print(" ")
	}
	s := newSpinner()
	s.start()
	return s.stop
}

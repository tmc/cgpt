package completion

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
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

	historyIn           io.ReadCloser
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
		// Allow nil config, but handle potential nil dereferences later
		// return nil, errors.New("config cannot be nil")
	}
	if model == nil {
		return nil, errors.New("model cannot be nil")
	}

	// Create default options
	defaultOpts := NewOptions()
	if cfg != nil { // Check if cfg is not nil before accessing
		defaultOpts.CompletionTimeout = cfg.CompletionTimeout
	}

	s := &Service{
		cfg:              cfg,
		model:            model,
		payload:          newCompletionPayload(cfg), // Handles nil cfg
		opts:             &defaultOpts,              // Initialize with defaults
		sessionTimestamp: time.Now().Format("20060102150405"),
	}
	if cfg != nil { // Check if cfg is not nil before accessing
		s.completionTimeout = cfg.CompletionTimeout
	}

	// Apply provided options, potentially overriding defaults
	for _, opt := range opts {
		opt(s)
	}

	// Setup logger - ensure s.opts.Stderr is valid
	if s.opts.Stderr == nil {
		s.opts.Stderr = os.Stderr // Default if not set by options
	}
	s.loggerCfg = zap.NewDevelopmentConfig()
	s.loggerCfg.DisableStacktrace = true // Disable stacktraces for cleaner logs
	s.loggerCfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder // Use ISO8601 for timestamps
	s.loggerCfg.EncoderConfig.CallerKey = "" // Disable caller info


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
		// Re-enable stacktrace and caller in debug mode
		s.loggerCfg.DisableStacktrace = false
		s.loggerCfg.EncoderConfig.CallerKey = "caller"
		// Rebuild logger with debug settings
		core := zapcore.NewCore(zapcore.NewConsoleEncoder(s.loggerCfg.EncoderConfig), zapcore.AddSync(s.opts.Stderr), s.loggerCfg.Level)
		logger := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1)) // Add caller info
		s.logger = logger.Sugar()
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
	// Update internal options (keep as is for now, refactor later if needed)
	tempOpts := RunOptionsToOptions(runCfg) // Use local var, don't modify s.opts directly here

	// Apply input handling from RunOptions
	if err := s.setupSystemPrompt(); err != nil {
		return fmt.Errorf("system prompt setup error: %w", err)
	}
	if err := s.handleInput(ctx, runCfg); err != nil {
		// Propagate context cancellation if that caused the error
		if errors.Is(err, context.Canceled) {
			return err
		}
		return fmt.Errorf("input handling error: %w", err)
	}

	// Close history input reader when Run finishes
	if s.historyIn != nil {
		defer s.historyIn.Close()
	}

	// Execute the completion based on options
	if tempOpts.Continuous {
		if tempOpts.StreamOutput {
			runErr = s.runContinuousCompletionStreaming(ctx, runCfg)
		} else {
			runErr = s.runContinuousCompletion(ctx, runCfg)
		}
	} else {
		if tempOpts.StreamOutput {
			runErr = s.runOneShotCompletionStreaming(ctx, runCfg)
		} else {
			runErr = s.runOneShotCompletion(ctx, runCfg)
		}
	}

	// Ensure final history save happens unless cancelled mid-save
	if !errors.Is(runErr, context.Canceled) { // Avoid saving if the main run was cancelled
		if saveErr := s.saveHistory(); saveErr != nil {
			// Log history save error but don't overwrite original runErr
			fmt.Fprintf(s.opts.Stderr, "\nWarning: failed to save final history: %v\n", saveErr)
		}
	}


	return runErr // Return the original error from the run methods
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

	// Use MaximumTimeout from RunOptions if provided, otherwise CompletionTimeout from Config
	if runOpts.MaximumTimeout > 0 {
		options.CompletionTimeout = runOpts.MaximumTimeout
	} else if runOpts.Config != nil && runOpts.Config.CompletionTimeout > 0 {
		options.CompletionTimeout = runOpts.Config.CompletionTimeout
	}
	// If CompletionTimeout is still 0, use the default from NewOptions
	if options.CompletionTimeout == 0 {
		options.CompletionTimeout = NewOptions().CompletionTimeout
	}

	return options
}

// PerformCompletionStreaming needs to respect the passed context
func (s *Service) PerformCompletionStreaming(ctx context.Context, payload *ChatCompletionPayload) (<-chan string, error) {
	ch := make(chan string)
	go func() {
		defer close(ch)
		fullResponse := strings.Builder{}
		firstChunk := true
		addedAssistantMessage := false // Track if message was added (via prefill or full response)

		prefillCleanup, spinnerPos := s.handleAssistantPrefill(ctx, payload) // Pass ctx

		// Send prefill immediately if it exists
		if s.nextCompletionPrefill != "" {
			if s.opts.EchoPrefill {
				spinnerPos = len(s.nextCompletionPrefill) + 1 // Simplified
			}
			select {
			case ch <- s.nextCompletionPrefill + " ":
			case <-ctx.Done(): // Respect context cancellation
				prefillCleanup()
				return
			}
			// Don't add to payload here yet, add the *final* message later
			addedAssistantMessage = true // Mark that prefill *was* used
			fullResponse.WriteString(s.nextCompletionPrefill) // Track prefill in buffer
		}

		// Start spinner
		var spinnerStop func()
		if s.opts.ShowSpinner {
			spinnerStop = spin(spinnerPos, s.opts.Stderr) // Pass Stderr to spinner
		}
		defer func() { // Ensure spinner stops
			if spinnerStop != nil {
				spinnerStop()
			}
		}()

		// Use the passed context directly for the LLM call
		_, err := s.model.GenerateContent(ctx, payload.Messages, // Use ctx directly
			llms.WithMaxTokens(s.cfg.MaxTokens),
			llms.WithTemperature(s.cfg.Temperature),
			llms.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
				if firstChunk {
					prefillCleanup() // Call cleanup function
					if spinnerStop != nil {
						spinnerStop() // Stop spinner on first chunk
						spinnerStop = nil
					}
					firstChunk = false
				}

				select {
				case ch <- string(chunk):
					fullResponse.Write(chunk) // Append chunk to full response
					return nil
				case <-ctx.Done(): // Check context within streaming func
					return ctx.Err() // Return context error to stop streaming
				}
			}))

		// Handle potential errors from GenerateContent
		if err != nil && !errors.Is(err, context.Canceled) {
			// Use logger instead of log.Printf
			s.logger.Errorf("failed to generate content: %v", err)
			// Optionally send error via the channel or handle differently
		}

		// Add the complete assistant message to the payload *once* after streaming finishes
		finalContent := fullResponse.String()
		if finalContent != "" { // Only add if there's content
			payload.addAssistantMessage(finalContent)
		}


		// Reset prefill after completion attempt
		s.nextCompletionPrefill = ""
	}()
	return ch, nil
}

// PerformCompletion needs to respect the passed context
func (s *Service) PerformCompletion(ctx context.Context, payload *ChatCompletionPayload) (string, error) {
	var stopSpinner func()
	var spinnerPos int
	addedAssistantMessage := false

	prefillCleanup, spinnerPos := s.handleAssistantPrefill(ctx, payload) // Pass ctx
	defer prefillCleanup() // Ensure cleanup happens

	if s.nextCompletionPrefill != "" {
		// Don't add to payload here, add the final combined message later
		addedAssistantMessage = true // Mark prefill was used
	}

	if s.opts.ShowSpinner {
		stopSpinner = spin(spinnerPos, s.opts.Stderr) // Pass Stderr
		defer stopSpinner()                           // Ensure spinner stops
	}

	// Use the passed context directly
	response, err := s.model.GenerateContent(ctx, payload.Messages, // Use ctx directly
		llms.WithMaxTokens(s.cfg.MaxTokens),
		llms.WithTemperature(s.cfg.Temperature))

	// Handle context cancellation error cleanly
	if errors.Is(err, context.Canceled) {
		return "", err // Propagate cancellation
	}
	if err != nil {
		return "", fmt.Errorf("failed to generate content: %w", err)
	}

	if len(response.Choices) == 0 {
		// Consider logging this or returning a more specific error
		return "", fmt.Errorf("no response choices from model")
	}

	content := response.Choices[0].Content
	fullContent := s.nextCompletionPrefill + content // Combine prefill and response

	// Add the full message to the payload
	if fullContent != "" {
		payload.addAssistantMessage(fullContent)
	}


	s.nextCompletionPrefill = "" // Reset prefill after completion

	return content, nil // Return only the newly generated part
}

// handleAssistantPrefill - pass context
func (s *Service) handleAssistantPrefill(ctx context.Context, payload *ChatCompletionPayload) (func(), int) {
	spinnerPos := 0
	if s.nextCompletionPrefill == "" {
		return func() {}, spinnerPos
	}

	// Store the current message count to ensure proper cleanup
	// initialMessageCount := len(payload.Messages) // No longer needed here

	if s.opts.EchoPrefill {
		// Write needs context check? Unlikely to block significantly.
		_, _ = s.opts.Stdout.Write([]byte(s.nextCompletionPrefill)) // Ignore write error for echo
		spinnerPos = len(s.nextCompletionPrefill)                    // Position after prefill
	}

	// Don't add assistant message here anymore, handled in Perform*

	// Cleanup function is now a no-op as payload management is centralized
	return func() {}, spinnerPos
}


// GetInputReader - pass context
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
	// Allow setup even with history, if system prompt is explicitly provided
	// and not already present, or differs.
	if s.cfg == nil || s.cfg.SystemPrompt == "" {
		return nil // No system prompt configured
	}

	sysMsg := llms.TextParts(llms.ChatMessageTypeSystem, s.cfg.SystemPrompt)
	sysIdx := slices.IndexFunc(s.payload.Messages, func(m llms.MessageContent) bool {
		return m.Role == llms.ChatMessageTypeSystem
	})

	if sysIdx >= 0 {
		// If system prompt exists, check if it needs updating
		// This simple check assumes single text part for system prompt
		existingPrompt := ""
		if len(s.payload.Messages[sysIdx].Parts) > 0 {
			if part, ok := s.payload.Messages[sysIdx].Parts[0].(llms.TextContent); ok {
				existingPrompt = part.Text
			}
		}
		if existingPrompt != s.cfg.SystemPrompt {
			s.logger.Debugf("Updating system prompt from '%s' to '%s'", existingPrompt, s.cfg.SystemPrompt)
			s.payload.Messages[sysIdx] = sysMsg // Update existing
		}
	} else {
		// Add system prompt at the beginning if it doesn't exist
		s.logger.Debugf("Adding system prompt: '%s'", s.cfg.SystemPrompt)
		s.payload.Messages = append([]llms.MessageContent{sysMsg}, s.payload.Messages...)
	}

	return nil
}


// handleInput - pass context, check context during ReadAll
func (s *Service) handleInput(ctx context.Context, runCfg RunOptions) error {
	r, err := GetInputReader(ctx, runCfg.InputFiles, runCfg.InputStrings, runCfg.PositionalArgs, runCfg.Stdin)
	if err != nil {
		return fmt.Errorf("failed to get inputs: %w", err)
	}

	// If the reader is an *os.File, we might need to manage closing it.
	// However, MultiReader makes this complex. Assume ReadAll is sufficient for now.
	if closer, ok := r.(io.Closer); ok {
		defer closer.Close() // Close if the reader itself is closable (e.g., single file)
	}


	// ReadAll doesn't directly support context, but we check ctx.Err() afterwards
	inputBytes, err := io.ReadAll(r)
	if ctx.Err() != nil {
		return ctx.Err() // Prioritize context cancellation
	}
	if err != nil {
		return fmt.Errorf("failed to read inputs: %w", err)
	}

	if len(inputBytes) != 0 {
		s.payload.addUserMessage(string(inputBytes))
	}

	return nil
}

func (s *Service) loadedWithHistory() bool {
	return s.historyIn != nil
}

func (s *Service) handleHistory(historyIn, historyOut string) error {
	s.historyOutFile = historyOut
	if historyIn != "" {
		expandedPath, err := expandTilde(historyIn)
		if err != nil {
			fmt.Fprintf(s.opts.Stderr,"Warning: could not expand history input path '%s': %v\n", historyIn, err)
			expandedPath = historyIn // Use original path as fallback
		}

		f, err := os.Open(expandedPath)
		if err != nil {
			// Don't return error if file not found, just log warning
			if os.IsNotExist(err) {
				fmt.Fprintf(s.opts.Stderr,"Warning: history input file not found: %s\n", expandedPath)
				return nil // Not an error if history file doesn't exist
			}
			return fmt.Errorf("issue opening input history file %s: %w", expandedPath, err)
		}
		s.historyIn = f // Assign the io.ReadCloser
		// Defer closing until the Service.Run method finishes

		// Load history immediately after opening
		loadErr := s.loadHistory()
		// Close the file reader *after* attempting to load
		// We can close here because loadHistory reads all content now
		if closeErr := f.Close(); closeErr != nil {
			fmt.Fprintf(s.opts.Stderr, "Warning: failed to close history input file %s: %v\n", expandedPath, closeErr)
		}
		s.historyIn = nil // Set reader to nil after closing


		if loadErr != nil {
			// Log warning, but don't return error for load failure
			fmt.Fprintf(s.opts.Stderr, "Warning: failed to load history from %s: %v\n", historyIn, loadErr)
		}
	}

	// Initial save might create the file if historyOut is set but file doesn't exist
	if s.historyOutFile != "" && len(s.payload.Messages) == 0 { // Save empty history if needed
		expandedOutPath, _ := expandTilde(s.historyOutFile) // Ignore error for stat check
		if _, err := os.Stat(expandedOutPath); os.IsNotExist(err) {
			if err := s.saveHistory(); err != nil {
				fmt.Fprintf(s.opts.Stderr, "Warning: failed to create initial history file %s: %v\n", s.historyOutFile, err)
			}
		}
	}
	return nil
}

func (s *Service) getLastUserMessage() string {
	for i := len(s.payload.Messages) - 1; i >= 0; i-- {
		if s.payload.Messages[i].Role == llms.ChatMessageTypeHuman {
			// Assuming simple text parts for user messages
			var parts []string
			for _, p := range s.payload.Messages[i].Parts {
				if textPart, ok := p.(llms.TextContent); ok {
					parts = append(parts, textPart.Text)
				}
			}
			return strings.Join(parts, "\n")
		}
	}
	return "" // No user message found
}


// runOneShotCompletionStreaming - pass context
func (s *Service) runOneShotCompletionStreaming(ctx context.Context, runCfg RunOptions) error {
	s.logger.Debug("running one-shot completion with streaming")
	s.payload.Stream = true
	streamPayloads, err := s.PerformCompletionStreaming(ctx, s.payload)
	if err != nil {
		// Handle context cancellation from PerformCompletionStreaming
		if errors.Is(err, context.Canceled) {
			return err
		}
		return fmt.Errorf("failed to perform completion streaming: %w", err)
	}
	content := strings.Builder{}
	for r := range streamPayloads {
		// Check context *inside* the loop in case PerformCompletionStreaming doesn't error out immediately
		if ctx.Err() != nil {
			// Drain the rest of the channel? Or just break? Break is simpler.
			break
		}
		// content.WriteString(r) // No need to buffer here
		// Check context before potentially blocking write? Unlikely.
		_, writeErr := runCfg.Stdout.Write([]byte(r))
		if writeErr != nil {
			s.logger.Warnf("Error writing to stdout: %v", writeErr)
			// Potentially break or return error depending on severity
		}
	}

	// Check context error after loop finishes or breaks
	if ctx.Err() != nil {
		// Don't save history if cancelled during streaming? Or save partial?
		// Current logic saves *after* loop, so partial won't be saved here.
		return ctx.Err()
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
	_, writeErr := runCfg.Stdout.Write([]byte(response))
	if writeErr != nil {
		s.logger.Warnf("Error writing to stdout: %v", writeErr)
		// Maybe return this error?
	}
	_, _ = runCfg.Stdout.Write([]byte("\n")) // Ensure newline after non-streaming response


	// Save history *after* successful completion
	// Payload was updated by PerformCompletion
	// if err := s.saveHistory(); err != nil { // Moved to Run()
	//	return fmt.Errorf("failed to save history: %w", err)
	// }
	return nil
}

// runContinuousCompletionStreaming - simplify context handling
func (s *Service) runContinuousCompletionStreaming(ctx context.Context, runCfg RunOptions) error {
	fmt.Fprintf(s.opts.Stderr, "\033[38;5;240mcgpt: Running in continuous mode. Press ctrl+c to exit.\033[0m\n")

	// No need for ctxWithCancel, just use ctx directly
	// The interactive session's Run method should respect the passed ctx.

	// Generate initial response if needed
	if len(s.payload.Messages) > 0 && s.payload.Messages[len(s.payload.Messages)-1].Role == llms.ChatMessageTypeHuman {
		// Pass ctx
		if err := s.generateResponse(ctx, runCfg); err != nil {
			if errors.Is(err, context.Canceled) {
				return err
			} // Exit cleanly on cancel
			// Log error but continue to interactive loop if possible
			fmt.Fprintf(s.opts.Stderr, "\nError generating initial response: %v\n", err)
			// return fmt.Errorf("failed to generate initial response: %w", err) // Optionally exit
		}
	}


	processFn := func(ctx context.Context, input string) error {
		input = strings.TrimSpace(input)
		if input == "" {
			return interactive.ErrEmptyInput
		}

		// Special command handling (remains the same)
		if input == "/last" {
			// Get the last user message, if available
			if lastMsg := s.getLastUserMessage(); lastMsg != "" {
				return interactive.ErrUseLastMessage(lastMsg)
			}
			fmt.Fprintf(s.opts.Stderr, "\033[38;5;240mNo previous message to edit.\033[0m\n")
			return interactive.ErrEmptyInput
		}

		s.payload.addUserMessage(input)
		// Pass the context received by processFn (which originates from session.Run)
		if err := s.generateResponse(ctx, runCfg); err != nil {
			// Don't return error on cancellation, let session handle it
			if errors.Is(err, context.Canceled) {
				return nil
			}
			// Log or display the error appropriately
			fmt.Fprintf(s.opts.Stderr, "\nError generating response: %v\n", err)
			// Decide whether to continue the loop or return the error
			// Returning nil allows the loop to continue after an error.
			return nil // Or return err to stop the loop
		}

		// Save history after successful generation moved to generateResponse
		// if err := s.saveHistory(); err != nil {
		// 	fmt.Fprintf(s.opts.Stderr, "\nWarning: issue saving history: %v\n", err)
		// }
		return nil
	}

	// Configure the interactive session
	sessionConfig := interactive.Config{
		Stdin:               runCfg.Stdin, // Pass Stdin from RunOptions
		Prompt:              ">>> ",
		AltPrompt:           "... ",
		HistoryFile:         expandTilde(runCfg.ReadlineHistoryFile),
		ProcessFn:           processFn,
		// Add other necessary fields like CommandFn, AutoCompleteFn if needed
		ConversationHistory: s.payload.MessagesToHistoryStrings(), // Provide current history
	}

	var session interactive.Session
	var err error

	// Select session type based on UseTUI
	if runCfg.UseTUI {
		session, err = interactive.NewBubbleSession(sessionConfig)
		if err == nil {
			fmt.Fprintf(s.opts.Stderr, "\033[38;5;240mcgpt: Using Terminal UI mode (BubbleTea)\033[0m\n")
		} else {
			fmt.Fprintf(s.opts.Stderr, "\033[38;5;240mWarning: Failed to initialize BubbleTea UI: %v. Falling back to readline.\033[0m\n", err)
			// Fall through to readline
		}
	}

	// Fallback or default to ReadlineSession
	if session == nil { // If TUI wasn't requested or failed
		session, err = interactive.NewSession(sessionConfig) // NewSession handles build tags
		if err != nil {
			return fmt.Errorf("failed to create interactive session: %w", err)
		}
	}


	s.activeSession = session

	// Pass the main ctx directly to session.Run
	runErr := s.activeSession.Run(ctx)

	// Check context error after Run returns
	if ctx.Err() != nil {
		// Context was canceled (likely by signal)
		// Perform cleanup *after* Run returns (renameChatHistory moved to Run)
		// renameCtx, cancelRename := context.WithTimeout(context.Background(), 10*time.Second)
		// defer cancelRename()
		// if renameErr := s.renameChatHistory(renameCtx); renameErr != nil {
		//	fmt.Fprintf(s.opts.Stderr, "\033[38;5;240mcgpt: Failed to rename history on exit: %v\033[0m\n", renameErr)
		// }
		return ctx.Err() // Return the cancellation error
	}

	// If Run returned an error other than context cancellation
	if runErr != nil {
		return runErr // Return the error from the session itself
	}

	// Normal exit without cancellation or session error
	// Perform final rename attempt here as well for normal exit (renameChatHistory moved to Run)
	// renameCtx, cancelRename := context.WithTimeout(context.Background(), 10*time.Second)
	// defer cancelRename()
	// if renameErr := s.renameChatHistory(renameCtx); renameErr != nil {
	// 	fmt.Fprintf(s.opts.Stderr, "\033[38;5;240mcgpt: Failed to rename history: %v\033[0m\n", renameErr)
	// }

	return nil // Normal, clean exit
}

// runContinuousCompletion - simplify context handling (similar to streaming version)
func (s *Service) runContinuousCompletion(ctx context.Context, runCfg RunOptions) error {
	fmt.Fprintf(s.opts.Stderr, "\033[38;5;240mcgpt: Running in continuous mode. Press ctrl+c to exit.\033[0m\n")

	// Use ctx directly

	processFn := func(ctx context.Context, input string) error {
		input = strings.TrimSpace(input)
		if input == "" {
			return interactive.ErrEmptyInput
		}
		s.payload.addUserMessage(input)
		// Pass ctx
		response, err := s.PerformCompletion(ctx, s.payload)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return nil // Let session handle cancellation
			}
			fmt.Fprintf(s.opts.Stderr, "\nError generating response: %v\n", err)
			return nil // Allow loop to continue
		}
		_, _ = runCfg.Stdout.Write([]byte(response)) // Ignore write error
		_, _ = runCfg.Stdout.Write([]byte("\n"))    // Add newline

		// History saving moved to generateResponse/PerformCompletion
		// if err := s.saveHistory(); err != nil {
		//	fmt.Fprintf(s.opts.Stderr, "\nWarning: issue saving history: %v\n", err)
		// }
		return nil
	}

	sessionConfig := interactive.Config{
		Stdin:               runCfg.Stdin,
		Prompt:              ">>> ",
		AltPrompt:           "... ",
		HistoryFile:         expandTilde(runCfg.ReadlineHistoryFile),
		ProcessFn:           processFn,
		ConversationHistory: s.payload.MessagesToHistoryStrings(), // Provide current history
		// Add other fields as needed
	}

	var session interactive.Session
	var err error

	// Use NewSession which respects build tags
	session, err = interactive.NewSession(sessionConfig)
	if err != nil {
		return err
	}

	s.activeSession = session

	// Pass ctx directly
	runErr := session.Run(ctx)

	// Handle exit reasons (similar to streaming version, renameChatHistory moved to Run)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if runErr != nil {
		return runErr
	}

	// Normal exit cleanup (renameChatHistory moved to Run)

	return nil
}

func expandTilde(path string) (string, error) {
	if path == "" || path[0] != '~' {
		return path, nil
	}
	if len(path) > 1 && path[1] != '/' && path[1] != '\\' {
		// Handle ~user syntax if needed, otherwise return original
		return path, nil // Or return an error for unsupported syntax
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}

	if len(path) == 1 { // Just "~"
		return home, nil
	}
	// Replace "~/" or "~\"
	return filepath.Join(home, path[2:]), nil
}


// generateResponse - pass context, check context errors, save history on success
func (s *Service) generateResponse(ctx context.Context, runCfg RunOptions) error {
	s.payload.Stream = runCfg.StreamOutput

	if runCfg.StreamOutput {
		// Pass ctx
		streamPayloads, err := s.PerformCompletionStreaming(ctx, s.payload)
		if err != nil {
			return err // Propagates cancellation etc.
		}

		// content := strings.Builder{} // Removed, payload is updated in Perform*
		wasInterrupted := false

		for r := range streamPayloads {
			if ctx.Err() != nil { // Check context inside loop
				wasInterrupted = true
				break
			}
			// content.WriteString(r) // Removed
			if s.activeSession != nil {
				s.activeSession.AddResponsePart(r)
			}
			// Check context before write?
			_, _ = runCfg.Stdout.Write([]byte(r)) // Ignore write error
		}

		// Handle interruption or normal completion
		if wasInterrupted || ctx.Err() != nil {
			_, _ = runCfg.Stdout.Write([]byte("\n")) // Ensure newline on interrupt
			fmt.Fprintf(runCfg.Stderr, "\033[38;5;240mResponse interrupted.\033[0m\n")

			// History is saved implicitly by PerformCompletionStreaming adding partial message
			// if err := s.saveHistory(); err != nil { // Removed explicit save here
			// 	fmt.Fprintf(runCfg.Stderr, "\033[38;5;240mWarning: Failed to save partial response: %v\033[0m\n", err)
			// }

			return ctx.Err() // Return context error
		}

		// Normal completion
		_, _ = runCfg.Stdout.Write([]byte("\n"))
		// History already saved by PerformCompletionStreaming goroutine

	} else { // Non-streaming
		// Pass ctx
		response, err := s.PerformCompletion(ctx, s.payload)
		if err != nil {
			return err // Propagates cancellation etc.
		}
		_, _ = runCfg.Stdout.Write([]byte(response))
		_, _ = runCfg.Stdout.Write([]byte("\n")) // Add newline

		// History already saved by PerformCompletion
	}

	// History saving is now handled within Perform* methods

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
	// Don't try to generate a title if we have no messages or context is done
	if len(s.payload.Messages) < 2 || ctx.Err() != nil {
		return "empty-chat", ctx.Err()
	}

	// Construct prompt carefully, limiting message content
	var promptBuilder strings.Builder
	promptBuilder.WriteString("Generate a short, filesystem-safe, kebab-case title (max 5 words, e.g., debug-rust-code, explain-quantum-mechanics) for the following conversation:\n\n")

	msgLimit := min(len(s.payload.Messages), 6) // Limit messages considered for title
	charLimit := 500                           // Limit total characters in prompt
	currentChars := 0

	for _, m := range s.payload.Messages[:msgLimit] {
		rolePrefix := fmt.Sprintf("%s: ", m.Role)
		promptBuilder.WriteString(rolePrefix)
		currentChars += len(rolePrefix)

		for _, p := range m.Parts {
			partStr := fmt.Sprint(p) // Handle different part types simply
			partLen := len(partStr)
			if currentChars+partLen > charLimit {
				// Truncate part if adding it exceeds limit
				truncateLen := charLimit - currentChars - 3 // Account for "..."
				if truncateLen > 0 {
					promptBuilder.WriteString(partStr[:truncateLen] + "...")
				}
				currentChars = charLimit // Mark as full
				break                    // Stop adding parts for this message
			}
			promptBuilder.WriteString(partStr)
			currentChars += partLen
		}
		promptBuilder.WriteString("\n") // Newline between messages
		currentChars++
		if currentChars >= charLimit {
			break // Stop adding messages if char limit reached
		}
	}
	promptBuilder.WriteString("\nTitle:") // Ask for the title explicitly

	prompt := promptBuilder.String()
	s.logger.Debugf("Generating title with prompt: %s", prompt)

	// Use a separate, short timeout for title generation
	titleCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	completion, err := llms.GenerateFromSinglePrompt(titleCtx, s.model, prompt,
		llms.WithMaxTokens(20), // Limit tokens for title
		llms.WithTemperature(0.2), // Lower temperature for more deterministic title
	)
	if err != nil {
		// Check if the error was due to context cancellation/timeout
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			s.logger.Warnf("Title generation timed out or was cancelled: %v", err)
			return fmt.Sprintf("chat-%s", s.sessionTimestamp), nil // Fallback title
		}
		return "", fmt.Errorf("failed to generate title: %w", err)
	}

	// Sanitize and format the title
	// Remove potential quotes, trim whitespace, replace spaces with hyphens, lowercase
	title := strings.TrimSpace(completion)
	title = strings.Trim(title, `"'`)
	title = strings.ToLower(title)
	title = strings.ReplaceAll(title, " ", "-")
	// Remove characters unsafe for filenames (allow letters, numbers, hyphen)
	reg := regexp.MustCompile(`[^a-z0-9-]+`)
	title = reg.ReplaceAllString(title, "")
	// Limit length and prevent multiple consecutive hyphens
	title = regexp.MustCompile(`-{2,}`).ReplaceAllString(title, "-")
	title = strings.Trim(title, "-")

	const maxTitleLength = 50
	if len(title) > maxTitleLength {
		title = title[:maxTitleLength]
		title = strings.TrimRight(title, "-") // Ensure it doesn't end with hyphen after truncate
	}
	if title == "" { // Fallback if sanitization removed everything
		title = fmt.Sprintf("chat-%s", s.sessionTimestamp)
	}


	s.logger.Infof("Generated history title: %s", title)
	return title, nil
}


// renameChatHistory generates a title and renames the history file
func (s *Service) renameChatHistory(ctx context.Context) error {
	// Expand tilde for history output file path
	expandedOutPath, err := expandTilde(s.historyOutFile)
	if err != nil {
		fmt.Fprintf(s.opts.Stderr, "Warning: could not expand history output path '%s': %v\n", s.historyOutFile, err)
		expandedOutPath = s.historyOutFile // Use original path as fallback
	}
	s.historyOutFile = expandedOutPath // Update service state with expanded path


	if s.historyOutFile == "" || !strings.Contains(filepath.Base(s.historyOutFile), "default-history-") {
		// Only rename default files, or if no file is set yet (though it should be by now)
		s.logger.Debugf("Skipping history rename for file: %s", s.historyOutFile)
		return nil
	}


	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home directory: %w", err)
	}
	historyDir := filepath.Join(home, ".cgpt")

	// Get the current history file path (might be default or already renamed)
	currentPath := s.historyOutFile
	if !filepath.IsAbs(currentPath) { // Ensure path is absolute
		currentPath = filepath.Join(historyDir, currentPath)
	}

	// Check if the current file exists before trying to rename
	if _, err := os.Stat(currentPath); os.IsNotExist(err) {
		s.logger.Warnf("History file %s not found, cannot rename.", currentPath)
		return nil // Don't treat as error if file doesn't exist
	}

	// Generate a descriptive title
	title, err := s.generateHistoryTitle(ctx) // Pass context
	if err != nil {
		s.logger.Errorf("Failed to generate history title: %v", err)
		return nil // Don't block exit on title generation failure
	}

	// Create new filename with timestamp + title
	// Use session timestamp for consistency even if title generation fails/falls back
	newFilename := fmt.Sprintf("%s-%s.yaml", s.sessionTimestamp, title)
	newPath := filepath.Join(historyDir, newFilename)


	// Avoid renaming if paths are the same
	if currentPath == newPath {
		s.logger.Debugf("History file already has the correct name: %s", newFilename)
		return nil
	}


	// Rename the file
	s.logger.Infof("Attempting to rename history file from %s to %s", currentPath, newPath)
	if err := os.Rename(currentPath, newPath); err != nil {
		// Log specific error, but don't fail the exit
		s.logger.Errorf("Failed to rename history file: %v", err)
		return nil // Non-fatal error for rename failure
	}

	fmt.Fprintf(s.opts.Stderr, "\033[38;5;240mcgpt: Renamed history to: %s\033[0m\n", filepath.Base(newPath))

	// Update the historyOutFile path in the service state
	s.historyOutFile = newPath
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
// The order of precedence is: Files, Strings, Args, Stdin (if '-' is not used explicitly).
func (h *InputHandler) Process(ctx context.Context) (io.Reader, error) {
	var readers []io.Reader
	var closers []io.Closer // Keep track of files to close
	stdinUsed := false      // Track if stdin was used via '-'

	cleanup := func() {
		for _, closer := range closers {
			closer.Close() // Attempt to close all opened files
		}
	}
	// Use a flag to signal success/error for cleanup
	success := false
	defer func() {
		if !success {
			cleanup() // Cleanup if function returns error
		}
	}()

	for _, file := range h.Files {
		if file == "-" {
			if h.Stdin != nil {
				// Stdin usually doesn't need closing managed here
				readers = append(readers, h.Stdin)
				stdinUsed = true
			} else {
				// If '-' is used but stdin is nil, treat as empty input
				readers = append(readers, strings.NewReader(""))
			}
		} else {
			expandedFile, err := expandTilde(file)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not expand file path '%s': %v\n", file, err)
				expandedFile = file
			}
			f, err := os.Open(expandedFile)
			if err != nil {
				return nil, fmt.Errorf("opening file %s: %w", expandedFile, err)
			}
			readers = append(readers, f)
			closers = append(closers, f) // Add file to closers list
		}
	}

	for _, s := range h.Strings {
		readers = append(readers, strings.NewReader(s))
	}

	for _, arg := range h.Args {
		readers = append(readers, strings.NewReader(arg))
	}

	// Add stdin reader at the end ONLY if it wasn't explicitly used with '-'
	if h.Stdin != nil && !stdinUsed {
		readers = append(readers, h.Stdin)
	}

	// Handle the case where no inputs were provided at all
	if len(readers) == 0 {
		success = true // Mark as success
		return strings.NewReader(""), nil // Return empty reader instead of nil
	}


	// Return a reader that closes all opened files when it's closed
	multiReader := io.MultiReader(readers...)
	multiCloserReader := struct {
		io.Reader
		io.Closer
	}{
		Reader: multiReader,
		Closer: io.NopCloser(nil), // Default NopCloser
	}

	// Create a custom Closer that closes all tracked files
	multiCloserReader.Closer = multiCloser{closers: closers}

	success = true // Mark as success before returning
	return multiCloserReader, nil
}

// multiCloser helps close multiple io.Closer objects.
type multiCloser struct {
	closers []io.Closer
}

func (mc multiCloser) Close() error {
	var errs []error
	for _, closer := range mc.closers {
		if err := closer.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		// Combine errors if needed, for now return the first one
		return fmt.Errorf("errors closing readers: %v", errs)
	}
	return nil
}



func (h *InputHandler) getStdinReader() io.Reader {
	if h.Stdin == nil {
		return nil
	}
	return h.Stdin
}

// ChatCompletionPayload holds the messages for a chat completion.
type ChatCompletionPayload struct {
	Messages []llms.MessageContent `yaml:"messages"` // Add YAML tag
	Stream   bool                  `yaml:"-"`        // Exclude Stream from YAML
}


// newCompletionPayload creates a new ChatCompletionPayload.
func newCompletionPayload(cfg *Config) *ChatCompletionPayload {
	p := &ChatCompletionPayload{
		Messages: []llms.MessageContent{},
		Stream:   true, // Default to streaming
	}
	// Add system prompt from config if available
	if cfg != nil && cfg.SystemPrompt != "" {
		// Avoid adding if payload already has one (e.g. from loaded history)
		hasSys := false
		for _, msg := range p.Messages {
			if msg.Role == llms.ChatMessageTypeSystem {
				hasSys = true
				break
			}
		}
		if !hasSys {
			p.Messages = append(p.Messages, llms.TextParts(llms.ChatMessageTypeSystem, cfg.SystemPrompt))
		}
	}
	return p
}

// addUserMessage adds a user message to the completion payload.
func (p *ChatCompletionPayload) addUserMessage(content string) {
	// Prevent adding empty user messages
	if strings.TrimSpace(content) == "" {
		return
	}
	// // Simple merge logic - append if last wasn't user, merge if last was user
	// if len(p.Messages) > 0 && p.Messages[len(p.Messages)-1].Role == llms.ChatMessageTypeHuman {
	// 	lastMsg := p.Messages[len(p.Messages)-1]
	// 	existingContent := ""
	// 	if len(lastMsg.Parts) > 0 {
	// 		if textPart, ok := lastMsg.Parts[0].(llms.TextContent); ok {
	// 			existingContent = textPart.Text
	// 		}
	// 	}
	// 	// Use TextParts to ensure correct type
	// 	p.Messages[len(p.Messages)-1] = llms.TextParts(llms.ChatMessageTypeHuman, existingContent+"\n\n"+content)
	// } else {
	p.Messages = append(p.Messages, llms.TextParts(llms.ChatMessageTypeHuman, content))
	// }
}


// AddUserMessage adds a user message to the service's payload
func (s *Service) AddUserMessage(content string) {
	s.payload.addUserMessage(content)
}

// addAssistantMessage adds an assistant message to the completion payload.
func (p *ChatCompletionPayload) addAssistantMessage(content string) {
	// Prevent adding empty assistant messages
	if strings.TrimSpace(content) == "" {
		return
	}
	// Handle potential duplicate consecutive AI messages if needed (e.g., merge?)
	// For now, just append.
	p.Messages = append(p.Messages, llms.TextParts(llms.ChatMessageTypeAI, content))
}

// loadHistory loads the history from the history file reader.
func (s *Service) loadHistory() error {
	if s.historyIn == nil {
		s.logger.Debug("No history input reader provided.")
		return nil
	}
	data, err := io.ReadAll(s.historyIn) // Read all data since historyIn might be closed soon
	if err != nil {
		return fmt.Errorf("reading history input: %w", err)
	}
	if len(data) == 0 {
		s.logger.Debug("History input was empty.")
		return nil // Empty history is not an error
	}

	// Try unmarshaling as YAML first
	var loadedMessages []llms.MessageContent
	if yaml.Unmarshal(data, &loadedMessages) == nil {
		s.logger.Debugf("Loaded %d messages from YAML history", len(loadedMessages))
		// Prepend loaded history to existing payload messages if any
		// Ensure system prompt isn't duplicated or handled correctly
		currentMessages := s.payload.Messages // Messages from flags/stdin
		s.payload.Messages = loadedMessages    // Start with loaded history

		// Re-add any initial messages that weren't part of history (e.g., from flags)
		// Skip system prompt from currentMessages if history already has one
		historyHasSysPrompt := s.payload.HasSystemPrompt()
		for _, msg := range currentMessages {
			if msg.Role == llms.ChatMessageTypeSystem && historyHasSysPrompt {
				continue // Skip initial system prompt if history provided one
			}
			// Simple append for now, might need smarter merging logic
			s.payload.Messages = append(s.payload.Messages, msg)
		}
		// Ensure system prompt from config is applied/updated if needed
		s.setupSystemPrompt() // Re-apply/verify system prompt from config
		return nil
	}

	// Fallback: Try parsing as legacy plain text format (role: content\n)
	s.logger.Debug("History is not valid YAML, attempting legacy plain text parse.")
	lines := strings.Split(string(data), "\n")
	var legacyMessages []llms.MessageContent
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ": ", 2)
		if len(parts) != 2 {
			s.logger.Warnf("Skipping invalid legacy history line: %s", line)
			continue
		}
		role := strings.ToLower(strings.TrimSpace(parts[0]))
		content := strings.TrimSpace(parts[1])
		var msgType llms.ChatMessageType
		switch role {
		case "user", "human":
			msgType = llms.ChatMessageTypeHuman
		case "ai", "assistant":
			msgType = llms.ChatMessageTypeAI
		case "system":
			msgType = llms.ChatMessageTypeSystem
		default:
			s.logger.Warnf("Skipping legacy history line with unknown role '%s'", role)
			continue
		}
		legacyMessages = append(legacyMessages, llms.TextParts(msgType, content))
	}
	if len(legacyMessages) > 0 {
		s.logger.Debugf("Loaded %d messages from legacy history format", len(legacyMessages))
		currentMessages := s.payload.Messages
		s.payload.Messages = legacyMessages
		historyHasSysPrompt := s.payload.HasSystemPrompt()
		for _, msg := range currentMessages {
			if msg.Role == llms.ChatMessageTypeSystem && historyHasSysPrompt {
				continue
			}
			s.payload.Messages = append(s.payload.Messages, msg)
		}
		s.setupSystemPrompt()
		// Consider saving immediately in the new format
		if s.historyOutFile != "" {
			if err := s.saveHistory(); err != nil {
				s.logger.Warnf("Failed to save history in new format after loading legacy: %v", err)
			} else {
				s.logger.Info("Converted legacy history to new YAML format.")
			}
		}
		return nil
	}

	s.logger.Warn("Could not parse history file content (checked YAML and legacy format).")
	return nil // Treat unparseable as non-fatal for now
}


// saveHistory saves the history to the history file in YAML format.
func (s *Service) saveHistory() error {
	// Expand tilde if present in the history output file path
	expandedPath, err := expandTilde(s.historyOutFile)
	if err != nil {
		fmt.Fprintf(s.opts.Stderr, "Warning: could not expand history output path '%s': %v\n", s.historyOutFile, err)
		expandedPath = s.historyOutFile // Use original path as fallback
	}


	if expandedPath == "" {
		// If no output file specified (even after expansion), don't attempt to save.
		return nil
	}

	if len(s.payload.Messages) == 0 {
		// Optionally write an empty file or just do nothing
		// Let's do nothing if there are no messages to save
		// s.logger.Debug("No messages in payload, skipping history save.")
		return nil
	}


	// Ensure directory exists
	historyDir := filepath.Dir(expandedPath)
	if err := os.MkdirAll(historyDir, 0755); err != nil {
		return fmt.Errorf("failed to create history directory %s: %w", historyDir, err)
	}

	// Marshal messages to YAML
	data, err := yaml.Marshal(s.payload.Messages)
	if err != nil {
		return fmt.Errorf("failed to marshal history to YAML: %w", err)
	}

	// Write to file
	err = os.WriteFile(expandedPath, data, 0644) // Use standard file permissions
	if err != nil {
		return fmt.Errorf("failed to write history file %s: %w", expandedPath, err)
	}
	s.logger.Debugf("Saved %d messages to history file: %s", len(s.payload.Messages), expandedPath)

	return nil
}

// Spinner implementation
type spinner struct {
	frames []string
	pos    int
	active bool
	done   chan struct{}
	output io.Writer // Added output writer
}

func newSpinner(output io.Writer) *spinner {
	if output == nil {
		output = os.Stderr // Default to stderr
	}
	return &spinner{
		frames: []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		done:   make(chan struct{}),
		output: output,
	}
}

func (s *spinner) start(initialPos int) {
	s.active = true
	// Print initial spaces if needed
	if initialPos > 0 {
		fmt.Fprint(s.output, strings.Repeat(" ", initialPos))
	}
	fmt.Fprint(s.output, " ") // Print initial space for the spinner frame

	go func() {
		ticker := time.NewTicker(80 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-s.done:
				// Clear the spinner frame before returning
				fmt.Fprint(s.output, "\b \b")
				return
			case <-ticker.C:
				if !s.active { // Double check active flag
					fmt.Fprint(s.output, "\b \b")
					return
				}
				fmt.Fprint(s.output, "\b"+s.frames[s.pos])
				s.pos = (s.pos + 1) % len(s.frames)
			}
		}
	}()
}

func (s *spinner) stop() {
	if !s.active {
		return
	}
	s.active = false
	// Signal the goroutine to stop and clean up
	// Use non-blocking send to avoid deadlock if goroutine already exited
	select {
	case s.done <- struct{}{}:
	default:
	}
}


// spin starts a spinner at the given character position using the specified writer.
func spin(pos int, writer io.Writer) func() {
	if writer == nil {
		writer = os.Stderr // Default to stderr if nil
	}
	// Check if writer is a terminal TTY before starting spinner
	isTerminal := false
	if f, ok := writer.(*os.File); ok {
		isTerminal = term.IsTerminal(int(f.Fd()))
	}
	// Only start spinner if output is a terminal
	if !isTerminal {
		return func() {} // Return no-op function if not a terminal
	}


	s := newSpinner(writer)
	s.start(pos)
	return s.stop
}

// Helper to check if payload has a system prompt
func (p *ChatCompletionPayload) HasSystemPrompt() bool {
	for _, m := range p.Messages {
		if m.Role == llms.ChatMessageTypeSystem {
			return true
		}
	}
	return false
}

// MessagesToHistoryStrings converts payload messages to simple strings for basic history interfaces.
func (p *ChatCompletionPayload) MessagesToHistoryStrings() []string {
	var history []string
	for _, msg := range p.Messages {
		// Simple conversion, might lose multi-part detail
		var contentBuilder strings.Builder
		for _, part := range msg.Parts {
			if textPart, ok := part.(llms.TextContent); ok {
				contentBuilder.WriteString(textPart.Text)
			}
			// Add handling for other part types if necessary
		}
		// Add role prefix for clarity, though readline might not use it
		// history = append(history, fmt.Sprintf("%s: %s", msg.Role, contentBuilder.String()))
		// Or just the content:
		// Skip empty messages
		content := contentBuilder.String()
		if strings.TrimSpace(content) != "" {
			history = append(history, content)
		}

	}
	return history
}

// --- YAML Marshaling/Unmarshaling for llms.MessageContent ---
// We need custom marshaling because llms.MessageContent contains an interface (ContentPart)

// messageContentYAML is a helper struct for YAML serialization/deserialization.
type messageContentYAML struct {
	Role  llms.ChatMessageType `yaml:"role"`
	Parts []messagePartYAML    `yaml:"parts"` // Use a slice of our helper struct
}

// messagePartYAML is a helper struct for individual parts.
// It stores the type explicitly to aid deserialization.
type messagePartYAML struct {
	Type string `yaml:"type"`
	Text string `yaml:"text,omitempty"`
	// Add fields for other part types like ImageURL, ToolCall, ToolCallResponse if needed
}

// MarshalYAML implements the yaml.Marshaler interface for llms.MessageContent.
func (mc llms.MessageContent) MarshalYAML() (interface{}, error) {
	yamlParts := make([]messagePartYAML, 0, len(mc.Parts))
	for _, part := range mc.Parts {
		switch p := part.(type) {
		case llms.TextContent:
			yamlParts = append(yamlParts, messagePartYAML{Type: "text", Text: p.Text})
		// Add cases for other types (ImageURL, ToolCall, etc.) here
		// case llms.ImageURLContent:
		// 	yamlParts = append(yamlParts, messagePartYAML{Type: "image_url", ImageURL: p.ImageURL})
		default:
			// Handle unknown part types or return an error
			// For now, skipping unknown parts during serialization
			log.Printf("Skipping unknown message part type during YAML marshal: %T", p)
		}
	}
	return messageContentYAML{Role: mc.Role, Parts: yamlParts}, nil
}

// UnmarshalYAML implements the yaml.Unmarshaler interface for llms.MessageContent.
func (mc *llms.MessageContent) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var helper messageContentYAML
	if err := unmarshal(&helper); err != nil {
		// Check for simpler string unmarshal (legacy support?)
		var simpleContent string
		if errSimple := unmarshal(&simpleContent); errSimple == nil {
			// Heuristic: Assume it's a user message if just a string
			mc.Role = llms.ChatMessageTypeHuman
			mc.Parts = []llms.ContentPart{llms.TextContent{Text: simpleContent}}
			return nil
		}
		return err // Return original error if it wasn't a simple string
	}


	mc.Role = helper.Role
	mc.Parts = make([]llms.ContentPart, 0, len(helper.Parts))
	for _, yamlPart := range helper.Parts {
		switch yamlPart.Type {
		case "text":
			mc.Parts = append(mc.Parts, llms.TextContent{Text: yamlPart.Text})
		// Add cases for other types (ImageURL, ToolCall, etc.) here
		// case "image_url":
		// 	mc.Parts = append(mc.Parts, llms.ImageURLContent{ImageURL: yamlPart.ImageURL})
		default:
			// Handle unknown part types or return an error
			// For now, skipping unknown parts during deserialization
			log.Printf("Skipping unknown message part type during YAML unmarshal: %s", yamlPart.Type)
		}
	}
	return nil
}

// Ensure our custom types work with the yaml package
var _ yaml.Marshaler = (*llms.MessageContent)(nil)
var _ yaml.Unmarshaler = (*llms.MessageContent)(nil)

// GetPayloadMessages returns the current messages in the payload.
func (s *Service) GetPayloadMessages() []llms.MessageContent {
	if s.payload == nil {
		return nil
	}
	return s.payload.Messages
}

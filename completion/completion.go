package completion

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	// "log" // Standard log can likely be removed if all instances are converted
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/tmc/cgpt/interactive" // Assuming this exists
	"github.com/tmc/langchaingo/llms"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/term"
	"gopkg.in/yaml.v3"
)

// Logging is now externalized to cmd/cgpt/logger.go

// Service is the main entry point for the completion service.
type Service struct {
	cfg *Config

	logger *zap.SugaredLogger // Logger is now injected rather than created internally

	model llms.Model

	payload *ChatCompletionPayload

	completionTimeout time.Duration

	historyIn           io.ReadCloser // Changed to ReadCloser
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

// WithLogger is a ServiceOption that sets the logger
func WithLogger(logger *zap.SugaredLogger) ServiceOption {
	return func(s *Service) {
		s.logger = logger
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

// Note: WithLogger is already defined in the ServiceOption section

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
	defaultOpts := NewOptions() // Already sets Stderr default
	if cfg != nil {             // Check if cfg is not nil before accessing
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

	// --- Logger Setup ---

	// Ensure Stderr is valid (already defaulted in NewOptions)
	if s.opts.Stderr == nil {
		s.opts.Stderr = os.Stderr
	}

	if s.logger == nil { // Only create a default logger if one wasn't provided via options
		// Create a basic default logger that writes to stderr
		logger := zap.NewExample().Sugar() // Simple logger with minimal output
		logger = logger.Named("cgpt")      // Add a name for identification
		s.logger = logger
		
		s.logger.Warn("No logger provided, using basic default logger. Consider using WithLogger().")
	} else {
		s.logger.Debug("Using externally provided logger")
	}

	// --- End Logger Setup ---

	s.logger.Info("cgpt service initialized")       // This line will be green
	s.logger.Debugf("Service options: %+v", s.opts) // This line will be grey (if DebugMode is true)

	// Handle history files
	if err := s.handleHistory(s.opts.HistoryIn, s.opts.HistoryOut); err != nil {
		// Use Fprintln for user-facing warnings about history handling.
		// These don't necessarily need full log format.
		s.logger.Warnf("Error handling history: %v", err)
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
	Stdin  io.ReadCloser

	// Timing
	MaximumTimeout time.Duration

	ConfigPath string

	// Backend/Provider-specific options.
	OpenAIUseLegacyMaxTokens bool
}

// Run executes a completion using the service's options
// Run receives the main context directly
func (s *Service) Run(ctx context.Context, runCfg RunOptions) error {
	// Update internal options (keep as is for now, refactor later if needed)
	s.logger.Debugf("Running with options: %+v", runCfg)
	tempOpts := RunOptionsToOptions(runCfg) // Use local var, don't modify s.opts directly here

	// Apply input handling from RunOptions
	if err := s.setupSystemPrompt(); err != nil {
		return fmt.Errorf("system prompt setup error: %w", err)
	}
	// Pass ctx directly to handleInput
	if err := s.handleInput(ctx, runCfg); err != nil {
		// Propagate context cancellation if that caused the error
		if errors.Is(err, context.Canceled) {
			s.logger.Debug("Input handling cancelled by context")
			return err
		}
		return fmt.Errorf("input handling error: %w", err)
	}

	// Close history input reader when Run finishes (if it was opened)
	// Note: historyIn is set to nil in handleHistory after reading, so this defer might not be strictly necessary anymore.
	if s.historyIn != nil {
		defer s.historyIn.Close()
	}

	// Execute the completion based on options
	// Pass ctx directly to the run methods
	var runErr error
	s.logger.Infof("Running completion... continuous=%v, streamOutput=%v", tempOpts.Continuous, tempOpts.StreamOutput)
	if tempOpts.Continuous {
		if tempOpts.StreamOutput {
			runErr = s.runContinuousCompletionStreaming(ctx, runCfg)
		} else {
			runErr = s.runContinuousCompletion(ctx, runCfg)
		}
		// After continuous mode finishes (normally or by error/cancel), rename history
		renameCtx, cancelRename := context.WithTimeout(context.Background(), 10*time.Second) // Short timeout for rename
		defer cancelRename()
		if renameErr := s.renameChatHistory(renameCtx); renameErr != nil {
			// Use logger for internal warnings
			s.logger.Warnf("Failed to rename history on exit: %v", renameErr)
			// Optionally inform user via Fprintf as well
			// s.logger.Warnf("Failed to rename history on exit: %v", renameErr)
		}

	} else {
		if tempOpts.StreamOutput {
			runErr = s.runOneShotCompletionStreaming(ctx, runCfg)
		} else {
			runErr = s.runOneShotCompletion(ctx, runCfg)
		}
	}

	// Ensure final history save happens unless cancelled mid-save
	// This is crucial for one-shot modes. Continuous mode saves within its loop/generateResponse.
	if !tempOpts.Continuous && !errors.Is(runErr, context.Canceled) { // Avoid saving if the main run was cancelled
		if saveErr := s.saveHistory(); saveErr != nil {
			// Log history save error but don't overwrite original runErr
			s.logger.Warnf("Failed to save final history: %v", saveErr)
			// Use Fprintln for user-facing warning
			s.logger.Warnf("Failed to save final history: %v", saveErr)
		}
	}

	if errors.Is(runErr, context.Canceled) {
		s.logger.Debug("Run finished due to context cancellation")
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
		var err error // Declare error variable

		prefillCleanup, spinnerPos := s.handleAssistantPrefill(ctx, payload) // Pass ctx

		// Send prefill immediately if it exists
		if s.nextCompletionPrefill != "" {
			if s.opts.EchoPrefill {
				spinnerPos = len(s.nextCompletionPrefill) + 1 // Simplified
			}
			select {
			case ch <- s.nextCompletionPrefill + " ": // Add space after prefill if echoing
			case <-ctx.Done(): // Respect context cancellation
				s.logger.Debug("Prefill send cancelled by context")
				prefillCleanup()
				return
			}
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
		_, err = s.model.GenerateContent(ctx, payload.Messages, // Use ctx directly
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
					s.logger.Debug("Streaming function cancelled by context")
					return ctx.Err() // Return context error to stop streaming
				}
			}))

		// Add the complete assistant message to the payload *once* after streaming finishes
		// regardless of error, as long as some content was generated.
		finalContent := fullResponse.String()
		if finalContent != "" { // Only add if there's content
			payload.addAssistantMessage(finalContent)
		}

		// Reset prefill after completion attempt
		s.nextCompletionPrefill = ""

		// Handle potential errors from GenerateContent *after* adding message and resetting prefill
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				s.logger.Debugf("Content generation cancelled or timed out: %v", err)
				// Don't log as error if it's just cancellation
			} else {
				s.logger.Errorf("Failed to generate content: %v", err)
				// Optionally send error via the channel or handle differently
				// For now, just log it. The error is implicitly returned by the outer function scope.
			}
			// The channel will be closed by the defer, signaling completion/error.
		} else {
			s.logger.Debug("Content generation completed successfully.")
		}

	}()
	// Return the channel immediately. Errors during generation will cause the channel to close.
	return ch, nil
}

// PerformCompletion needs to respect the passed context
func (s *Service) PerformCompletion(ctx context.Context, payload *ChatCompletionPayload) (string, error) {
	var stopSpinner func()
	var spinnerPos int

	prefillCleanup, spinnerPos := s.handleAssistantPrefill(ctx, payload) // Pass ctx
	defer prefillCleanup()                                               // Ensure cleanup happens

	// Don't add prefill to payload here, add the final combined message later

	if s.opts.ShowSpinner {
		stopSpinner = spin(spinnerPos, s.opts.Stderr) // Pass Stderr
		defer stopSpinner()                           // Ensure spinner stops
	}

	s.logger.Debugf("Performing non-streaming completion with %d existing messages", len(payload.Messages))

	// Use the passed context directly
	response, err := s.model.GenerateContent(ctx, payload.Messages, // Use ctx directly
		llms.WithMaxTokens(s.cfg.MaxTokens),
		llms.WithTemperature(s.cfg.Temperature))

	// Handle context cancellation error cleanly
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		s.logger.Debugf("Non-streaming completion cancelled or timed out: %v", err)
		// Reset prefill even on cancellation
		s.nextCompletionPrefill = ""
		return "", err // Propagate cancellation/timeout
	}
	if err != nil {
		s.logger.Errorf("Failed to generate non-streaming content: %v", err)
		// Reset prefill even on error
		s.nextCompletionPrefill = ""
		return "", fmt.Errorf("failed to generate content: %w", err)
	}

	if len(response.Choices) == 0 {
		s.logger.Warn("Received response with no choices from model")
		// Reset prefill
		s.nextCompletionPrefill = ""
		return "", fmt.Errorf("no response choices from model")
	}

	content := response.Choices[0].Content
	s.logger.Debugf("Received non-streaming response content (length %d)", len(content))
	fullContent := s.nextCompletionPrefill + content // Combine prefill and response

	// Add the full message to the payload
	if fullContent != "" {
		payload.addAssistantMessage(fullContent)
		s.logger.Debugf("Added assistant message (length %d) to payload", len(fullContent))
	} else {
		s.logger.Debug("No content (prefill+response) to add to payload")
	}

	s.nextCompletionPrefill = "" // Reset prefill after successful completion

	return content, nil // Return only the newly generated part
}

// handleAssistantPrefill manages echoing prefill and preparing spinner position.
// It no longer modifies the payload directly.
func (s *Service) handleAssistantPrefill(ctx context.Context, payload *ChatCompletionPayload) (func(), int) {
	spinnerPos := 0
	cleanupFunc := func() {} // Default no-op cleanup

	if s.nextCompletionPrefill == "" {
		return cleanupFunc, spinnerPos
	}

	s.logger.Debugf("Handling assistant prefill: '%s'", s.nextCompletionPrefill)

	if s.opts.EchoPrefill {
		// Write needs context check? Unlikely to block significantly, but good practice.
		select {
		case <-ctx.Done():
			s.logger.Debug("Context cancelled before echoing prefill")
			// Don't echo if cancelled
		default:
			// Use Fprint for potentially simpler handling than Write with byte slice
			_, err := fmt.Fprint(s.opts.Stdout, s.nextCompletionPrefill)
			if err != nil {
				s.logger.Warnf("Error echoing prefill to stdout: %v", err)
			}
			spinnerPos = len(s.nextCompletionPrefill) // Position after prefill
		}
	}

	// Cleanup function remains a no-op as payload management is centralized
	return cleanupFunc, spinnerPos
}

// GetInputReader - pass context
func GetInputReader(ctx context.Context, files []string, stringsToRead []string, args []string, stdin io.Reader) (io.Reader, error) {
	handler := &InputHandler{
		Files:   files,
		Strings: stringsToRead, // Renamed to avoid conflict with strings package
		Args:    args,
		Stdin:   stdin,
	}
	// Pass context to Process (though current Process doesn't use it extensively yet)
	return handler.Process(ctx)
}

func (s *Service) setupSystemPrompt() error {
	// Allow setup even with history, if system prompt is explicitly provided
	// and not already present, or differs.
	if s.cfg == nil || s.cfg.SystemPrompt == "" {
		s.logger.Debug("No system prompt configured or config is nil.")
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
			s.logger.Infof("Updating system prompt from '%s' to '%s'", existingPrompt, s.cfg.SystemPrompt)
			s.payload.Messages[sysIdx] = sysMsg // Update existing
		} else {
			s.logger.Debug("Existing system prompt matches config, no update needed.")
		}
	} else {
		// Add system prompt at the beginning if it doesn't exist
		s.logger.Infof("Prepending system prompt: '%s'", s.cfg.SystemPrompt)
		s.payload.Messages = append([]llms.MessageContent{sysMsg}, s.payload.Messages...)
	}

	return nil
}

// handleInput - pass context, check context during ReadAll
func (s *Service) handleInput(ctx context.Context, runCfg RunOptions) error {
	// First, handle files and strings using GetInputReader
	inputFiles := runCfg.InputFiles
	inputStrings := runCfg.InputStrings
	posArgs := runCfg.PositionalArgs

	s.logger.Debugf("Handling input: files=%v, strings=%v, args=%v, stdin_present=%t",
		inputFiles, inputStrings, posArgs, runCfg.Stdin != nil)

	// Process file and string inputs first
	// Check if stdin seems available (not foolproof)
	stdinIsPipeOrFile := isStdinAvailable(runCfg.Stdin)
	s.logger.Debugf("Stdin available (detected as pipe/file): %t", stdinIsPipeOrFile)

	if len(inputFiles) > 0 || len(inputStrings) > 0 || stdinIsPipeOrFile {
		s.logger.Debug("Processing input from files, strings, or stdin pipe/file")
		r, err := GetInputReader(ctx, inputFiles, inputStrings, nil, runCfg.Stdin) // Pass stdin here
		if err != nil {
			return fmt.Errorf("failed to get combined input reader: %w", err)
		}

		// If the reader is an io.Closer (like the multiCloser from GetInputReader), defer its closing.
		if closer, ok := r.(io.Closer); ok {
			defer func() {
				if closeErr := closer.Close(); closeErr != nil {
					s.logger.Warnf("Error closing input reader: %v", closeErr)
				}
			}()
		}

		// Read all data, respecting context cancellation
		var inputBytes []byte
		readDone := make(chan struct{})
		var readErr error

		go func() {
			defer close(readDone)
			inputBytes, readErr = io.ReadAll(r) // Read from the combined reader
		}()

		select {
		case <-ctx.Done():
			s.logger.Info("Input reading cancelled by context")
			return ctx.Err() // Return context error
		case <-readDone:
			if readErr != nil {
				// Check if the error is simply EOF, which is expected and not an error
				if errors.Is(readErr, io.EOF) {
					s.logger.Debug("EOF reached while reading input.")
				} else {
					return fmt.Errorf("failed to read combined inputs: %w", readErr)
				}
			}
		}

		// Process the input if we have any
		if len(inputBytes) > 0 {
			inputText := string(inputBytes)
			// Avoid logging potentially huge inputs fully at info level
			s.logger.Debugf("Read %d bytes from files/strings/stdin", len(inputBytes))
			if len(inputText) < 500 { // Log shorter inputs fully at debug
				s.logger.Debugf("Input text: '%s'", inputText)
			}
			s.payload.addUserMessage(inputText)
		} else {
			s.logger.Debug("No input bytes read from files/strings/stdin.")
		}
	} else {
		s.logger.Debug("Skipping read from files/strings/stdin (none provided or stdin is terminal).")
	}

	// Process positional arguments separately - each one becomes a separate message
	if len(posArgs) > 0 {
		s.logger.Debugf("Processing %d positional arguments", len(posArgs))
		for _, arg := range posArgs {
			trimmedArg := strings.TrimSpace(arg)
			if len(trimmedArg) > 0 {
				s.logger.Debugf("Adding positional arg as message: '%s'", trimmedArg)
				s.payload.addUserMessage(trimmedArg)
			} else {
				s.logger.Debug("Skipping empty positional argument.")
			}
		}
	} else {
		s.logger.Debug("No positional arguments to process.")
	}

	// Log final state if useful
	if s.logger.Desugar().Core().Enabled(zapcore.DebugLevel) { // Check if debug is enabled efficiently
		s.logger.Debugf("Payload messages after input handling: %d", len(s.payload.Messages))
		// Optionally log message roles/types if needed for debugging
		// for i, msg := range s.payload.Messages {
		// 	s.logger.Debugf("  Msg %d: Role=%s", i, msg.Role)
		// }
	}

	return nil
}

// isStdinAvailable checks if stdin *might* have data (pipe or file redirection).
// It returns false if stdin is likely the interactive terminal.
func isStdinAvailable(stdin io.Reader) bool {
	if stdin == nil {
		return false
	}
	// Check if it's the actual os.Stdin file descriptor
	if f, ok := stdin.(*os.File); ok && f == os.Stdin {
		stat, err := f.Stat()
		if err != nil {
			// Cannot stat stdin, assume not available for non-interactive use
			return false
		}
		// If stdin is a character device (terminal), it's not "available" for non-interactive reading.
		// If it's anything else (pipe, file), it is available.
		return (stat.Mode() & os.ModeCharDevice) == 0
	}
	// If it's not os.Stdin but some other reader, assume it's intended for reading.
	// This could be a bytes.Buffer in tests, etc.
	return true
}

func (s *Service) loadedWithHistory() bool {
	// Consider history loaded if HistoryIn was specified *and* successfully read.
	// Check if payload has more than just potentially a system prompt.
	if len(s.payload.Messages) == 0 {
		return false
	}
	if len(s.payload.Messages) == 1 && s.payload.Messages[0].Role == llms.ChatMessageTypeSystem {
		return false // Only system prompt doesn't count as loaded history
	}
	return true // Assume history was loaded if there are non-system messages initially
}

func (s *Service) handleHistory(historyIn, historyOut string) error {
	// Expand historyOut first to potentially create the default file early
	expandedOutPath, expandOutErr := expandTilde(historyOut)
	if expandOutErr != nil {
		s.logger.Warnf("Could not expand history output path '%s': %v", historyOut, expandOutErr)
		expandedOutPath = historyOut // Use original path as fallback
	}
	s.historyOutFile = expandedOutPath // Store potentially expanded path

	// Attempt to load history if historyIn is provided
	if historyIn != "" {
		s.logger.Infof("Attempting to load history from: %s", historyIn)
		expandedPath, err := expandTilde(historyIn)
		if err != nil {
			// Log warning, but proceed with original path
			s.logger.Warnf("Could not expand history input path '%s', using original: %v", historyIn, err)
			expandedPath = historyIn
		}

		f, err := os.Open(expandedPath)
		if err != nil {
			// Don't return error if file not found, just log warning
			if os.IsNotExist(err) {
				s.logger.Warnf("History input file not found: %s", expandedPath)
				// File not found is not an error preventing startup
			} else {
				// Other errors opening the file are more problematic
				return fmt.Errorf("issue opening input history file %s: %w", expandedPath, err)
			}
		} else {
			// File opened successfully
			s.historyIn = f            // Assign the io.ReadCloser
			loadErr := s.loadHistory() // Load history immediately

			// Close the file reader *after* attempting to load
			if closeErr := s.historyIn.Close(); closeErr != nil {
				s.logger.Warnf("Failed to close history input file %s: %v", expandedPath, closeErr)
			}
			s.historyIn = nil // Set reader to nil after closing

			if loadErr != nil {
				// Log warning, but don't return error for load failure, allow proceeding without history
				s.logger.Warnf("Failed to load history from %s: %v", expandedPath, loadErr)
			} else {
				s.logger.Infof("Successfully loaded history from %s", expandedPath)
			}
		}
	} else {
		s.logger.Debug("No history input file specified.")
	}

	// Initial save might create the file if historyOut is set but file doesn't exist
	// This helps ensure the directory exists and the file can be written to later.
	// Only do this if we *didn't* load any history (to avoid overwriting just-loaded history
	// with potentially only a system prompt).
	if s.historyOutFile != "" && !s.loadedWithHistory() {
		if _, statErr := os.Stat(s.historyOutFile); os.IsNotExist(statErr) {
			s.logger.Infof("History output file %s does not exist, attempting initial save.", s.historyOutFile)
			if err := s.saveHistory(); err != nil {
				// Log failure to create initial history, but don't prevent startup
				s.logger.Warnf("Failed to create initial history file %s: %v", s.historyOutFile, err)
			}
		}
	}
	return nil // Return nil even if loading had warnings
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
			return strings.Join(parts, "\n") // Join parts if multi-part exists
		}
	}
	s.logger.Debug("No previous user message found in history.")
	return "" // No user message found
}

// runOneShotCompletionStreaming - pass context
func (s *Service) runOneShotCompletionStreaming(ctx context.Context, runCfg RunOptions) error {
	s.logger.Debug("Running one-shot completion with streaming")
	s.payload.Stream = true

	// Use the context directly - no local signal handling
	// Pass the context to the streaming function
	streamPayloads, err := s.PerformCompletionStreaming(ctx, s.payload)
	if err != nil {
		// Handle context cancellation from PerformCompletionStreaming setup (unlikely)
		if errors.Is(err, context.Canceled) {
			s.logger.Debug("Stream setup cancelled by context.")
			return err
		}
		// Other setup errors
		return fmt.Errorf("failed to initiate completion streaming: %w", err)
	}

	// Reading from the stream
	for r := range streamPayloads {
		// Check context *inside* the loop
		select {
		case <-ctx.Done():
			s.logger.Info("Streaming interrupted by context cancellation.")
			_, _ = runCfg.Stdout.Write([]byte("\n")) // Ensure newline on interrupt
			// History is saved implicitly by PerformCompletionStreaming goroutine adding partial message
			return ctx.Err() // Return context error
		default:
			// Write the chunk to output
			_, writeErr := runCfg.Stdout.Write([]byte(r))
			if writeErr != nil {
				// Don't stop streaming for a write error, just log it
				s.logger.Warnf("Error writing stream chunk to stdout: %v", writeErr)
			}
		}
	}

	// Check context error after loop finishes (channel closed by PerformCompletionStreaming)
	// This handles cases where GenerateContent itself returned a context error.
	if ctx.Err() != nil {
		s.logger.Debug("Streaming finished due to context cancellation during generation.")
		_, _ = runCfg.Stdout.Write([]byte("\n")) // Ensure newline
		return ctx.Err()
	}

	s.logger.Debug("One-shot streaming finished normally.")
	_, _ = runCfg.Stdout.Write([]byte("\n")) // Ensure newline after successful stream

	// History saving is now handled in Run() for one-shot modes

	return nil
}

// runOneShotCompletion - pass context
func (s *Service) runOneShotCompletion(ctx context.Context, runCfg RunOptions) error {
	s.logger.Debug("Running one-shot completion without streaming")
	s.payload.Stream = false

	// Use the context directly - no local signal handling
	// Pass the context to the completion function
	response, err := s.PerformCompletion(ctx, s.payload)
	if err != nil {
		// Check for cancellation/timeout handled within PerformCompletion
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			s.logger.Info("One-shot completion cancelled or timed out.")
			return err // Return the context error
		}
		// Other errors
		return fmt.Errorf("failed to perform completion: %w", err)
	}

	// Check if we've been cancelled *after* completion but *before* writing response
	// (less likely but possible)
	if ctx.Err() != nil {
		s.logger.Info("Context cancelled after completion but before writing response.")
		return ctx.Err()
	}

	_, writeErr := runCfg.Stdout.Write([]byte(response))
	if writeErr != nil {
		// Log warning but don't fail the operation for write error
		s.logger.Warnf("Error writing response to stdout: %v", writeErr)
	}
	_, _ = runCfg.Stdout.Write([]byte("\n")) // Ensure newline after non-streaming response

	s.logger.Debug("One-shot completion finished normally.")
	// History saving is now handled in Run() for one-shot modes

	return nil
}

// runContinuousCompletionStreaming - simplify context handling
func (s *Service) runContinuousCompletionStreaming(ctx context.Context, runCfg RunOptions) error {
	// Use grey color for status messages via Fprintf
	s.logger.Info("Running in continuous mode (streaming). Press ctrl+c to exit.")

	// Generate initial response if needed (e.g., if input was provided via flags/files)
	// Check if the last message is from the user, indicating we need an initial response.
	if len(s.payload.Messages) > 0 && s.payload.Messages[len(s.payload.Messages)-1].Role == llms.ChatMessageTypeHuman {
		s.logger.Info("Generating initial response for continuous mode...")
		// Pass ctx for the initial generation
		if err := s.generateResponse(ctx, runCfg); err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				s.logger.Info("Initial response generation cancelled or timed out.")
				return err // Exit cleanly on cancel/timeout
			}
			// Log error but continue to interactive loop if possible
			s.logger.Errorf("Failed to generate initial response: %v", err)
			// Error already logged
			// Decide whether to proceed to interactive loop or exit
			// return fmt.Errorf("failed to generate initial response: %w", err) // Option to exit
		}
	}

	processFn := func(loopCtx context.Context, input string) error {
		// loopCtx is the context provided by the interactive session for this specific input loop
		s.logger.Debugf("Processing user input: '%s'", input)
		input = strings.TrimSpace(input)
		if input == "" {
			s.logger.Debug("Empty input received, prompting again.")
			return interactive.ErrEmptyInput // Signal session to reprompt
		}

		// Special command handling
		if input == "/last" {
			if lastMsg := s.getLastUserMessage(); lastMsg != "" {
				s.logger.Debug("User requested /last, providing previous message.")
				return interactive.ErrUseLastMessage(lastMsg)
			}
			s.logger.Debug("User requested /last, but no previous message found.")
			s.logger.Info("No previous message to edit.")
			return interactive.ErrEmptyInput // Reprompt
		}

		// Add user message and generate response
		s.payload.addUserMessage(input)
		// Pass the loopCtx to generateResponse
		if err := s.generateResponse(loopCtx, runCfg); err != nil {
			// Don't return error on cancellation, let session handle it
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				s.logger.Info("Response generation cancelled or timed out during interactive loop.")
				// Let session handle exit, return nil here to allow potential cleanup in session
				return nil
			}
			// Log other errors and inform user, but allow loop to continue
			s.logger.Errorf("Failed to generate response during interactive loop: %v", err)
			s.logger.Errorf("Error generating response: %v", err)
			// Returning nil allows the loop to continue after an error.
			return nil
		}

		// History saving is now handled within generateResponse/Perform* methods
		// No explicit save needed here.

		return nil // Signal success for this input loop
	}

	// Configure the interactive session
	historyFilePath, expandErr := expandTilde(runCfg.ReadlineHistoryFile)
	if expandErr != nil {
		s.logger.Warnf("Could not expand readline history path '%s': %v", runCfg.ReadlineHistoryFile, expandErr)
		historyFilePath = runCfg.ReadlineHistoryFile // Use original path
	} else {
		s.logger.Debugf("Using expanded readline history path: %s", historyFilePath)
	}

	sessionConfig := interactive.Config{
		Stdin:               runCfg.Stdin, // Pass Stdin from RunOptions
		Prompt:              ">>> ",
		AltPrompt:           "... ",
		HistoryFile:         historyFilePath,
		ProcessFn:           processFn,
		ConversationHistory: s.payload.MessagesToHistoryStrings(), // Provide current history
	}
	var session interactive.Session
	var sessionErr error

	// Select session type based on UseTUI
	if runCfg.UseTUI {
		s.logger.Info("Attempting to initialize BubbleTea UI session.")
		session, sessionErr = interactive.NewBubbleSession(sessionConfig)
		if sessionErr == nil {
			s.logger.Info("Using Terminal UI mode (BubbleTea)")
		} else {
			s.logger.Warnf("Failed to initialize BubbleTea UI: %v. Falling back to readline.", sessionErr)
			s.logger.Warnf("Failed to initialize BubbleTea UI: %v. Falling back to readline.", sessionErr)
			// Fall through to readline by leaving session == nil
		}
	} else {
		s.logger.Info("Using classic readline interactive session.")
	}

	// Fallback or default to ReadlineSession
	if session == nil { // If TUI wasn't requested or failed
		session, sessionErr = interactive.NewSession(sessionConfig) // NewSession handles build tags
		if sessionErr != nil {
			return fmt.Errorf("failed to create interactive session: %w", sessionErr)
		}
	}

	s.activeSession = session // Store the active session

	// Pass the main ctx (from Run) directly to session.Run
	// The session itself will manage its internal context based on this parent context.
	s.logger.Debug("Starting interactive session...")
	runErr := s.activeSession.Run(ctx)
	s.logger.Debug("Interactive session finished.")

	// Check the reason for Run returning
	if errors.Is(runErr, context.Canceled) || errors.Is(runErr, context.DeadlineExceeded) || ctx.Err() != nil {
		// Context was canceled (likely by signal handled by main or session)
		s.logger.Info("Interactive session exited due to context cancellation or deadline.")
		// History renaming is handled in the main Run function's defer for continuous mode
		return ctx.Err() // Return the cancellation error
	}

	// If Run returned an error other than context cancellation
	if runErr != nil && !errors.Is(runErr, interactive.ErrInterrupted) && !errors.Is(runErr, io.EOF) {
		s.logger.Errorf("Interactive session exited with error: %v", runErr)
		// History renaming is handled in the main Run function's defer
		return runErr // Return the error from the session itself
	}

	// Normal exit without cancellation or session error (or just ErrInterrupted)
	s.logger.Info("Interactive session exited normally.")
	// History renaming is handled in the main Run function's defer

	return nil // Normal, clean exit
}

// runContinuousCompletion - simplify context handling (similar to streaming version)
func (s *Service) runContinuousCompletion(ctx context.Context, runCfg RunOptions) error {
	// Use grey color for status messages via Fprintf
	s.logger.Info("Running in continuous mode (non-streaming). Press ctrl+c to exit.")

	// Generate initial response if needed (similar to streaming version)
	if len(s.payload.Messages) > 0 && s.payload.Messages[len(s.payload.Messages)-1].Role == llms.ChatMessageTypeHuman {
		s.logger.Info("Generating initial response for continuous mode...")
		// Generate non-streaming response
		// Need to adapt generateResponse or call PerformCompletion directly
		if response, err := s.PerformCompletion(ctx, s.payload); err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				s.logger.Info("Initial response generation cancelled or timed out.")
				return err // Exit cleanly
			}
			s.logger.Errorf("Failed to generate initial response: %v", err)
			// Error already logged
			// Optionally exit or continue to loop
		} else {
			// Print initial response
			_, _ = runCfg.Stdout.Write([]byte(response))
			_, _ = runCfg.Stdout.Write([]byte("\n"))
			// Save history after initial response (handled by PerformCompletion updating payload, Run will save)
		}
	}

	processFn := func(loopCtx context.Context, input string) error {
		s.logger.Debugf("Processing user input: '%s'", input)
		input = strings.TrimSpace(input)
		if input == "" {
			s.logger.Debug("Empty input received, prompting again.")
			return interactive.ErrEmptyInput
		}

		// Special command handling (same as streaming)
		if input == "/last" {
			// ... (same as streaming version)
			if lastMsg := s.getLastUserMessage(); lastMsg != "" {
				return interactive.ErrUseLastMessage(lastMsg)
			}
			s.logger.Info("No previous message to edit.")
			return interactive.ErrEmptyInput
		}

		s.payload.addUserMessage(input)
		// Pass loopCtx to PerformCompletion
		response, err := s.PerformCompletion(loopCtx, s.payload)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				s.logger.Info("Response generation cancelled or timed out during interactive loop.")
				return nil // Let session handle exit
			}
			s.logger.Errorf("Failed to generate response during interactive loop: %v", err)
			s.logger.Errorf("Error generating response: %v", err)
			return nil // Allow loop to continue
		}

		// Write response
		_, _ = runCfg.Stdout.Write([]byte(response)) // Ignore write error
		_, _ = runCfg.Stdout.Write([]byte("\n"))     // Add newline

		// History is updated by PerformCompletion. Final save by Run defer.

		return nil // Success for this input loop
	}

	// Configure and run interactive session (same as streaming version)
	historyFilePath, expandErr := expandTilde(runCfg.ReadlineHistoryFile)
	if expandErr != nil {
		s.logger.Warnf("Could not expand readline history path: %v", expandErr)
		historyFilePath = runCfg.ReadlineHistoryFile
	} else {
		s.logger.Debugf("Using expanded readline history path: %s", historyFilePath)
	}

	sessionConfig := interactive.Config{
		Stdin:               runCfg.Stdin,
		Prompt:              ">>> ",
		AltPrompt:           "... ",
		HistoryFile:         historyFilePath,
		ProcessFn:           processFn,
		ConversationHistory: s.payload.MessagesToHistoryStrings(),
	}

	session, err := interactive.NewSession(sessionConfig)
	if err != nil {
		return fmt.Errorf("failed to create interactive session: %w", err)
	}

	s.activeSession = session

	// Pass the main ctx directly to session.Run
	s.logger.Debug("Starting interactive session...")
	runErr := s.activeSession.Run(ctx)
	s.logger.Debug("Interactive session finished.")

	// Handle exit reasons (same as streaming version)
	if errors.Is(runErr, context.Canceled) || errors.Is(runErr, context.DeadlineExceeded) || ctx.Err() != nil {
		s.logger.Info("Interactive session exited due to context cancellation or deadline.")
		// History rename/save handled in Run defer
		return ctx.Err()
	}
	if runErr != nil && !errors.Is(runErr, interactive.ErrInterrupted) {
		s.logger.Errorf("Interactive session exited with error: %v", runErr)
		// History rename/save handled in Run defer
		return runErr
	}

	s.logger.Info("Interactive session exited normally.")
	// History rename/save handled in Run defer

	return nil // Normal, clean exit
}

func expandTilde(path string) (string, error) {
	if path == "" {
		return "", nil
	}
	if path[0] != '~' {
		return path, nil
	}
	// Handle "~" or "~/"
	if len(path) == 1 || (len(path) > 1 && path[1] == filepath.Separator) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get user home dir: %w", err)
		}
		if len(path) == 1 { // Just "~"
			return home, nil
		}
		// Replace "~/"
		return filepath.Join(home, path[2:]), nil
	}
	// Handle other cases like ~user (unsupported for now) or invalid paths
	// For simplicity, return original path if format is not recognized/supported
	return path, nil
}

// generateResponse handles calling the appropriate Perform* method and outputting results.
// It now relies on Perform* methods to update the payload.
// History saving is handled by the caller (Run) or Perform* internals.
func (s *Service) generateResponse(ctx context.Context, runCfg RunOptions) error {
	s.payload.Stream = runCfg.StreamOutput // Ensure payload reflects current mode

	var err error
	if runCfg.StreamOutput {
		s.logger.Debug("Generating streaming response...")
		streamPayloads, streamErr := s.PerformCompletionStreaming(ctx, s.payload)
		if streamErr != nil {
			// Handle context cancellation from setup (unlikely)
			if errors.Is(streamErr, context.Canceled) {
				return streamErr
			}
			return fmt.Errorf("failed to initiate streaming: %w", streamErr)
		}

		wasInterrupted := false
		for r := range streamPayloads {
			select {
			case <-ctx.Done(): // Check context inside loop
				wasInterrupted = true
				break // Exit the loop
			default:
				// If using TUI, update TUI instead of direct write
				if s.activeSession != nil && runCfg.UseTUI {
					s.activeSession.AddResponsePart(r)
				} else {
					// Write directly to stdout if not using TUI or no active session
					_, writeErr := runCfg.Stdout.Write([]byte(r))
					if writeErr != nil {
						s.logger.Warnf("Error writing stream chunk to stdout: %v", writeErr)
						// Optionally break or continue on write error
					}
				}
			}
			if wasInterrupted {
				break
			}
		} // End range streamPayloads

		// Final newline and status message
		if !runCfg.UseTUI { // Don't add extra newline if TUI is managing output
			_, _ = runCfg.Stdout.Write([]byte("\n")) // Ensure newline
		}

		if wasInterrupted || ctx.Err() != nil {
			s.logger.Info("Response generation interrupted by context.")
			if !runCfg.UseTUI { // TUI might handle its own interrupt message
				s.logger.Info("Response interrupted.")
			}
			// Payload was updated with partial response by PerformCompletionStreaming
			err = ctx.Err() // Return context error
		} else {
			s.logger.Debug("Streaming response generation finished normally.")
			// Payload was updated with full response by PerformCompletionStreaming
		}

	} else { // Non-streaming
		s.logger.Debug("Generating non-streaming response...")
		response, performErr := s.PerformCompletion(ctx, s.payload)
		if performErr != nil {
			err = performErr // Assign error to return
			// Error/cancellation logged within PerformCompletion
		} else {
			// If using TUI, add the whole response at once
			if s.activeSession != nil && runCfg.UseTUI {
				s.activeSession.AddResponsePart(response + "\n") // Add newline for TUI
			} else {
				// Write directly if not TUI
				_, writeErr := runCfg.Stdout.Write([]byte(response))
				if writeErr != nil {
					s.logger.Warnf("Error writing non-streaming response to stdout: %v", writeErr)
				}
				_, _ = runCfg.Stdout.Write([]byte("\n")) // Add newline
			}
			s.logger.Debug("Non-streaming response generation finished normally.")
			// Payload was updated by PerformCompletion
		}
	}

	// History saving is now handled outside this function (in Run or by Perform*)
	// If we are in continuous mode, we might want to save history after each successful generation here?
	// Let's stick to saving in Run for one-shot, and rely on Perform* updating payload for continuous.
	// Final save/rename for continuous happens after the loop exits in Run.

	return err // Return any error encountered during generation/streaming
}

// SetNextCompletionPrefill sets the next completion prefill message.
// Note that not all inference engines support prefill messages.
// Whitespace is trimmed from the end of the message.
func (s *Service) SetNextCompletionPrefill(content string) {
	trimmedContent := strings.TrimRight(content, " \t\n")
	s.logger.Debugf("Setting next completion prefill to: '%s'", trimmedContent)
	s.nextCompletionPrefill = trimmedContent
}

// generateHistoryTitle sends the conversation history to the LLM to generate a descriptive title
func (s *Service) generateHistoryTitle(ctx context.Context) (string, error) {
	defaultTitle := fmt.Sprintf("chat-%s", s.sessionTimestamp)

	// Don't try to generate a title if we have insufficient messages or context is done
	// Require at least one user and one AI message ideally. Let's say >= 2 messages total.
	if len(s.payload.Messages) < 2 {
		s.logger.Debug("Insufficient messages for title generation, using default.")
		return defaultTitle, nil
	}
	if ctx.Err() != nil {
		s.logger.Warn("Context cancelled before title generation, using default.")
		return defaultTitle, ctx.Err()
	}

	// Construct prompt carefully, limiting message content
	var promptBuilder strings.Builder
	promptBuilder.WriteString("Generate a short, filesystem-safe, kebab-case title (max 5 words, e.g., debug-rust-code, explain-quantum-mechanics) for the following conversation. Output ONLY the title string.\n\nCONVERSATION:\n")

	msgLimit := min(len(s.payload.Messages), 8) // Limit messages considered for title (increased slightly)
	charLimit := 800                            // Limit total characters in prompt (increased slightly)
	currentChars := 0
	addedMessages := 0

	// Iterate backwards to prioritize more recent messages? Or forwards? Let's stick to forwards.
	for _, m := range s.payload.Messages {
		// Skip system messages for title generation prompt
		if m.Role == llms.ChatMessageTypeSystem {
			continue
		}

		rolePrefix := fmt.Sprintf("%s: ", m.Role) // Use actual role (Human, AI)
		promptBuilder.WriteString(rolePrefix)
		currentChars += len(rolePrefix)

		partStr := ""
		for _, p := range m.Parts {
			if textPart, ok := p.(llms.TextContent); ok {
				partStr += textPart.Text // Concatenate text parts
			}
		}
		partStr = strings.TrimSpace(partStr) // Trim whitespace from combined parts

		// Truncate individual message part string if needed
		maxPartLen := (charLimit - currentChars) / (msgLimit - addedMessages + 1) // Dynamic limit
		maxPartLen = max(maxPartLen, 50)                                          // Min length per part
		if len(partStr) > maxPartLen {
			partStr = partStr[:maxPartLen] + "..."
		}

		partLen := len(partStr)
		if currentChars+partLen > charLimit {
			// If adding this truncated part still exceeds limit, stop adding messages
			s.logger.Debugf("Character limit %d reached while building title prompt.", charLimit)
			break
		}
		promptBuilder.WriteString(partStr)
		currentChars += partLen
		promptBuilder.WriteString("\n") // Newline between messages
		currentChars++
		addedMessages++

		if addedMessages >= msgLimit {
			s.logger.Debugf("Message limit %d reached for title prompt.", msgLimit)
			break // Stop adding messages if message limit reached
		}
	}
	promptBuilder.WriteString("\nTITLE:") // Ask for the title explicitly

	prompt := promptBuilder.String()
	s.logger.Debugf("Generating title with prompt (length %d)", len(prompt))
	// For extreme debug: s.logger.Debugf("Title prompt:\n%s", prompt)

	// Use a separate, short timeout for title generation
	// Use the passed context as the parent for the timeout context
	titleCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	// Use the main model configured for the service
	completion, err := llms.GenerateFromSinglePrompt(titleCtx, s.model, prompt,
		llms.WithMaxTokens(25),    // Limit tokens for title (slightly more generous)
		llms.WithTemperature(0.2), // Lower temperature for more deterministic title
		// llms.WithStopWords([]string{"\n"}), // Stop generation at newline?
	)
	if err != nil {
		// Check if the error was due to context cancellation/timeout of titleCtx *or* the parent ctx
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || ctx.Err() != nil {
			s.logger.Warnf("Title generation timed out or was cancelled: %v. Using default title.", err)
			return defaultTitle, nil // Fallback title, not an error for the caller
		}
		// Log other LLM errors but fallback to default title
		s.logger.Errorf("Failed to generate title via LLM: %v. Using default title.", err)
		return defaultTitle, nil // Fallback title, not an error for the caller
	}

	// Sanitize and format the title
	title := strings.TrimSpace(completion)
	// Remove potential quotes, trim final newlines that might sneak in
	title = strings.Trim(title, `"'`)
	title = strings.TrimRight(title, "\n")
	// Lowercase, replace spaces/underscores with hyphens
	title = strings.ToLower(title)
	title = strings.ReplaceAll(title, " ", "-")
	title = strings.ReplaceAll(title, "_", "-")
	// Remove characters unsafe for filenames (allow letters, numbers, hyphen)
	reg := regexp.MustCompile(`[^a-z0-9-]+`)
	title = reg.ReplaceAllString(title, "")
	// Condense multiple consecutive hyphens
	title = regexp.MustCompile(`-{2,}`).ReplaceAllString(title, "-")
	// Trim leading/trailing hyphens
	title = strings.Trim(title, "-")

	const maxTitleLength = 60 // Slightly longer max length
	if len(title) > maxTitleLength {
		title = title[:maxTitleLength]
		// Trim again in case truncation left a hyphen
		title = strings.TrimRight(title, "-")
	}
	if title == "" { // Fallback if sanitization removed everything
		s.logger.Warn("Title sanitization resulted in empty string, using default title.")
		title = defaultTitle
	} else {
		s.logger.Infof("Generated and sanitized history title: %s", title)
	}

	return title, nil
}

// renameChatHistory generates a title and renames the history file. Called after continuous mode exits.
func (s *Service) renameChatHistory(ctx context.Context) error {
	if s.historyOutFile == "" {
		s.logger.Debug("No history output file set, skipping rename.")
		return nil
	}

	// Expand tilde just in case it wasn't expanded before or contains variables
	// Though s.historyOutFile should ideally be the fully resolved path by now.
	currentPath, err := expandTilde(s.historyOutFile)
	if err != nil {
		// Log warning but try to proceed with the original path
		s.logger.Warnf("Could not expand history output path '%s' during rename: %v", s.historyOutFile, err)
		currentPath = s.historyOutFile
	}
	s.historyOutFile = currentPath // Update service state with potentially re-expanded path

	// Check if the file looks like a default, timestamped file eligible for renaming.
	// Example default pattern: ~/.cgpt/default-history-YYYYMMDDHHMMSS.yaml
	historyDir := filepath.Dir(currentPath)
	baseName := filepath.Base(currentPath)
	isDefaultFormat := strings.HasPrefix(baseName, "default-history-") && strings.HasSuffix(baseName, ".yaml")

	// Or maybe check if it contains the specific session timestamp?
	// isSessionTimestampFormat := strings.Contains(baseName, s.sessionTimestamp) && strings.HasSuffix(baseName, ".yaml")
	// Let's stick to renaming files named like "default-history-*" for now.
	// This avoids renaming user-specified explicit filenames.

	if !isDefaultFormat {
		s.logger.Debugf("History file '%s' does not match default pattern, skipping rename.", baseName)
		return nil
	}

	// Check if the current file exists before trying to rename
	if _, err := os.Stat(currentPath); os.IsNotExist(err) {
		s.logger.Warnf("History file %s not found, cannot rename.", currentPath)
		return nil // Don't treat as error if file doesn't exist (maybe it was never saved)
	}

	// Generate a descriptive title using the parent context passed down
	title, titleErr := s.generateHistoryTitle(ctx) // Pass context
	if titleErr != nil {
		// generateHistoryTitle already logged the error and returned a default title
		// We can proceed with the default title. Check if context was cancelled.
		if errors.Is(titleErr, context.Canceled) || errors.Is(titleErr, context.DeadlineExceeded) {
			s.logger.Info("History rename cancelled during title generation.")
			return titleErr // Propagate cancellation
		}
		// Otherwise, continue with default title even if LLM call failed
	}

	// Create new filename with timestamp + title
	// Use session timestamp for consistency
	newFilename := fmt.Sprintf("%s-%s.yaml", s.sessionTimestamp, title)
	newPath := filepath.Join(historyDir, newFilename)

	// Avoid renaming if paths are the same (e.g., if default title produced same name)
	if currentPath == newPath {
		s.logger.Debugf("Generated title results in the same filename, skipping rename: %s", newFilename)
		return nil
	}

	// Rename the file
	s.logger.Infof("Attempting to rename history file from '%s' to '%s'", baseName, newFilename)
	if err := os.Rename(currentPath, newPath); err != nil {
		// Log specific error, but don't fail the exit flow
		s.logger.Errorf("Failed to rename history file '%s' to '%s': %v", currentPath, newPath, err)
		s.logger.Errorf("Failed to rename history file: %v", err)
		return nil // Non-fatal error for rename failure
	}

	s.logger.Infof("Renamed history to: %s", newFilename)

	// Update the historyOutFile path in the service state to the new name
	s.historyOutFile = newPath
	return nil
}

// --- Input Handling ---

// InputSourceType represents the type of input source.
type InputSourceType string

const (
	InputSourceStdin  InputSourceType = "stdin"
	InputSourceFile   InputSourceType = "file"
	InputSourceString InputSourceType = "string"
	InputSourceArg    InputSourceType = "arg" // Keep if needed, though args processed separately now
)

// InputSource represents a single input source.
type InputSource struct {
	Type   InputSourceType
	Reader io.Reader
	Source string // Store filename or identifier
}

// InputHandler manages multiple input sources.
type InputHandler struct {
	Files   []string
	Strings []string
	Args    []string // Keep for potential future use? Currently handled in handleInput directly.
	Stdin   io.Reader
	Logger  *zap.SugaredLogger // Added logger
}

// InputSources is a slice of InputSource.
type InputSources []InputSource

// Process reads the set of inputs, this will block on stdin if it is included.
// The order of precedence is: Files, Strings, Stdin (if '-' is used explicitly or implicitly).
// Args are now handled separately in the main handleInput function.
func (h *InputHandler) Process(ctx context.Context) (io.Reader, error) {
	var readers []io.Reader
	var closers []io.Closer      // Keep track of files to close
	stdinExplicitlyUsed := false // Track if stdin was used via '-'

	log := h.Logger
	if log == nil {
		// Fallback to a Nop logger if none provided
		log = zap.NewNop().Sugar()
	}

	cleanup := func() {
		log.Debugf("Closing %d input file readers.", len(closers))
		for i, closer := range closers {
			if err := closer.Close(); err != nil {
				log.Warnf("Error closing input reader #%d: %v", i, err)
			}
		}
	}
	// Use a flag to signal success/error for cleanup
	success := false
	defer func() {
		if !success {
			log.Debug("Cleaning up input readers due to error or incomplete processing.")
			cleanup() // Cleanup if function returns error or panics
		}
		// If successful, the closer returned by Process will handle cleanup.
	}()

	// Process Files
	for _, file := range h.Files {
		if file == "-" {
			if h.Stdin != nil && !stdinExplicitlyUsed {
				log.Debug("Using stdin explicitly via '-' argument.")
				readers = append(readers, h.Stdin)
				stdinExplicitlyUsed = true
			} else if stdinExplicitlyUsed {
				log.Warn("Stdin ('-') specified multiple times, ignoring subsequent instances.")
			} else {
				log.Warn("Stdin ('-') specified but no stdin reader available.")
				// Add an empty reader to represent the intent? Or ignore?
				// Let's ignore for now.
			}
		} else {
			expandedFile, err := expandTilde(file)
			if err != nil {
				log.Warnf("Could not expand file path '%s', using original: %v", file, err)
				expandedFile = file
			}
			log.Debugf("Opening file input: %s", expandedFile)
			f, err := os.Open(expandedFile)
			if err != nil {
				// Don't need cleanup here as defer will handle it
				return nil, fmt.Errorf("opening file %s: %w", expandedFile, err)
			}
			readers = append(readers, f)
			closers = append(closers, f) // Add file to closers list
		}
	}

	// Process Strings
	if len(h.Strings) > 0 {
		log.Debugf("Adding %d string inputs.", len(h.Strings))
		for _, s := range h.Strings {
			readers = append(readers, strings.NewReader(s))
		}
	}

	// Process Args - Currently handled in handleInput, so ignore h.Args here.
	// if len(h.Args) > 0 {
	// 	log.Debugf("Adding %d arg inputs.", len(h.Args))
	// 	for _, arg := range h.Args {
	// 		readers = append(readers, strings.NewReader(arg))
	// 	}
	// }

	// Add stdin reader implicitly at the end ONLY if it wasn't explicitly used with '-'
	// AND if it seems like it's piped/redirected.
	if h.Stdin != nil && !stdinExplicitlyUsed && isStdinAvailable(h.Stdin) {
		log.Debug("Adding implicit stdin (pipe/redirect).")
		readers = append(readers, h.Stdin)
	} else if h.Stdin != nil && !stdinExplicitlyUsed && !isStdinAvailable(h.Stdin) {
		log.Debug("Stdin is terminal and was not explicitly requested ('-'), skipping.")
	}

	// Handle the case where no inputs were provided at all
	if len(readers) == 0 {
		log.Debug("No input sources found (files, strings, or applicable stdin).")
		success = true                    // Mark as success
		return strings.NewReader(""), nil // Return empty reader instead of nil
	}

	log.Debugf("Combining %d input readers.", len(readers))
	// Return a reader that closes all opened files when it's closed
	multiReader := io.MultiReader(readers...)
	multiCloserReader := struct {
		io.Reader
		io.Closer
	}{
		Reader: multiReader,
		// Create a custom Closer that closes all tracked files
		Closer: multiCloser{closers: closers, Logger: log},
	}

	success = true // Mark as success before returning the closable reader
	return multiCloserReader, nil
}

// multiCloser helps close multiple io.Closer objects.
type multiCloser struct {
	closers []io.Closer
	Logger  *zap.SugaredLogger
}

func (mc multiCloser) Close() error {
	var errs []error
	log := mc.Logger
	if log == nil {
		log = zap.NewNop().Sugar()
	}

	log.Debugf("MultiCloser closing %d readers.", len(mc.closers))
	for i, closer := range mc.closers {
		if err := closer.Close(); err != nil {
			log.Warnf("Error closing reader #%d: %v", i, err)
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		// Combine errors? For now, return a generic error indicating failures.
		return fmt.Errorf("errors closing %d readers: %v", len(errs), errs) // Report all errors
	}
	log.Debug("MultiCloser finished closing readers.")
	return nil
}

// --- Payload Handling ---

// ChatCompletionPayload holds the messages for a chat completion.
type ChatCompletionPayload struct {
	Messages []llms.MessageContent `yaml:"messages"` // Add YAML tag
	Stream   bool                  `yaml:"-"`        // Exclude Stream from YAML
}

// newCompletionPayload creates a new ChatCompletionPayload.
func newCompletionPayload(cfg *Config) *ChatCompletionPayload {
	p := &ChatCompletionPayload{
		Messages: []llms.MessageContent{},
		Stream:   true, // Default to streaming, can be overridden
	}
	// System prompt is added later by setupSystemPrompt to handle history loading correctly
	return p
}

// addUserMessage adds a user message to the completion payload.
func (p *ChatCompletionPayload) addUserMessage(content string) {
	trimmedContent := strings.TrimSpace(content)
	if trimmedContent == "" {
		// Optionally log skipping empty message
		// s.logger.Debug("Skipping empty user message.")
		return
	}
	// Simple append logic. Merging consecutive user messages can sometimes be complex
	// depending on desired behavior (e.g., should they be one turn or multiple?).
	// Sticking to append simplifies history representation.
	p.Messages = append(p.Messages, llms.TextParts(llms.ChatMessageTypeHuman, trimmedContent))
}

// AddUserMessage adds a user message to the service's payload
func (s *Service) AddUserMessage(content string) {
	s.logger.Debugf("Adding user message (length %d)", len(content))
	s.payload.addUserMessage(content)
}

// addAssistantMessage adds an assistant message to the completion payload.
func (p *ChatCompletionPayload) addAssistantMessage(content string) {
	trimmedContent := strings.TrimSpace(content)
	if trimmedContent == "" {
		// Optionally log skipping empty message
		// s.logger.Debug("Skipping empty assistant message.")
		return
	}
	// Append assistant message. Merging consecutive AI messages is usually not needed.
	p.Messages = append(p.Messages, llms.TextParts(llms.ChatMessageTypeAI, trimmedContent))
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
		// Skip system messages for readline history? Usually yes.
		if msg.Role == llms.ChatMessageTypeSystem {
			continue
		}

		// Simple conversion, might lose multi-part detail
		var contentBuilder strings.Builder
		for _, part := range msg.Parts {
			if textPart, ok := part.(llms.TextContent); ok {
				if contentBuilder.Len() > 0 {
					contentBuilder.WriteString("\n") // Add newline between text parts if needed
				}
				contentBuilder.WriteString(textPart.Text)
			}
			// Add handling for other part types if necessary for history display
		}

		content := contentBuilder.String()
		// Include role for potential context? Readline might ignore it.
		// Let's just add the content for now.
		if strings.TrimSpace(content) != "" {
			history = append(history, content)
		}
	}
	return history
}

// GetPayloadMessages returns a copy of the current messages in the payload.
func (s *Service) GetPayloadMessages() []llms.MessageContent {
	if s.payload == nil {
		return nil
	}
	// Return a copy to prevent external modification
	msgsCopy := make([]llms.MessageContent, len(s.payload.Messages))
	copy(msgsCopy, s.payload.Messages)
	return msgsCopy
}

// --- History Loading/Saving ---

// loadHistory loads the history from the history file reader (s.historyIn).
// s.historyIn should be closed by the caller after this function returns.
func (s *Service) loadHistory() error {
	if s.historyIn == nil {
		s.logger.Error("loadHistory called with nil historyIn reader.") // Should not happen
		return errors.New("internal error: history reader is nil")
	}
	s.logger.Debug("Reading history data from input reader...")
	data, err := io.ReadAll(s.historyIn) // Read all data
	if err != nil {
		return fmt.Errorf("reading history input: %w", err)
	}
	if len(data) == 0 {
		s.logger.Debug("History input was empty.")
		return nil // Empty history is not an error
	}

	s.logger.Debugf("Read %d bytes of history data. Attempting YAML parse.", len(data))
	// Try unmarshaling as YAML first using our wrapper
	var wrappedMessages []MessageContentWrapper
	if err := yaml.Unmarshal(data, &wrappedMessages); err == nil && len(wrappedMessages) > 0 {
		// YAML unmarshal succeeded and yielded messages
		loadedMessages := make([]llms.MessageContent, len(wrappedMessages))
		for i, wrapped := range wrappedMessages {
			loadedMessages[i] = wrapped.Content
		}
		s.logger.Infof("Loaded %d messages from YAML history", len(loadedMessages))

		// Prepend loaded history to any existing payload messages (e.g., from flags before history load)
		currentMessages := s.payload.Messages // Messages from flags/stdin added before history load
		s.payload.Messages = loadedMessages   // Start with loaded history

		// Re-add any initial messages that weren't part of history
		historyHasSysPrompt := s.payload.HasSystemPrompt()
		s.logger.Debugf("History has system prompt: %t. Current initial messages: %d", historyHasSysPrompt, len(currentMessages))
		for _, msg := range currentMessages {
			if msg.Role == llms.ChatMessageTypeSystem && historyHasSysPrompt {
				s.logger.Debug("Skipping initial system prompt from args/config as history already has one.")
				continue // Skip initial system prompt if history provided one
			}
			// Simple append for now. Could implement smarter merging if needed.
			s.logger.Debugf("Appending initial message (Role: %s) after loaded history.", msg.Role)
			s.payload.Messages = append(s.payload.Messages, msg)
		}
		// Ensure system prompt from config is applied/updated if needed, potentially overwriting loaded one
		if err := s.setupSystemPrompt(); err != nil {
			// Log error but don't fail loading history
			s.logger.Warnf("Error applying config system prompt after loading history: %v", err)
		}
		return nil
	} else if err != nil {
		s.logger.Warnf("YAML unmarshal failed: %v. Checking for legacy format.", err)
		// Fall through to check legacy format
	} else {
		s.logger.Debug("YAML unmarshal succeeded but yielded no messages.")
		// Fall through? Or treat as empty? Let's try legacy.
	}

	// Fallback: Try parsing as legacy plain text format (role: content\n)
	s.logger.Debug("Attempting legacy plain text parse for history.")
	lines := strings.Split(string(data), "\n")
	var legacyMessages []llms.MessageContent
	for i, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" {
			continue
		}
		parts := strings.SplitN(trimmedLine, ":", 2) // Split only on the first colon
		if len(parts) != 2 {
			s.logger.Warnf("Skipping invalid legacy history line %d: no colon found: %s", i+1, trimmedLine)
			continue
		}
		role := strings.ToLower(strings.TrimSpace(parts[0]))
		content := strings.TrimSpace(parts[1])
		var msgType llms.ChatMessageType
		switch role {
		case "user", "human":
			msgType = llms.ChatMessageTypeHuman
		case "ai", "assistant", "model": // Accept 'model' as well
			msgType = llms.ChatMessageTypeAI
		case "system":
			msgType = llms.ChatMessageTypeSystem
		default:
			s.logger.Warnf("Skipping legacy history line %d with unknown role '%s'", i+1, role)
			continue
		}
		if content != "" {
			legacyMessages = append(legacyMessages, llms.TextParts(msgType, content))
		} else {
			s.logger.Debugf("Skipping legacy history line %d with empty content (Role: %s)", i+1, role)
		}
	}

	if len(legacyMessages) > 0 {
		s.logger.Infof("Loaded %d messages from legacy history format.", len(legacyMessages))
		// Similar logic to YAML load for prepending/merging
		currentMessages := s.payload.Messages
		s.payload.Messages = legacyMessages
		historyHasSysPrompt := s.payload.HasSystemPrompt()
		s.logger.Debugf("Legacy history has system prompt: %t. Current initial messages: %d", historyHasSysPrompt, len(currentMessages))
		for _, msg := range currentMessages {
			if msg.Role == llms.ChatMessageTypeSystem && historyHasSysPrompt {
				s.logger.Debug("Skipping initial system prompt from args/config as legacy history has one.")
				continue
			}
			s.logger.Debugf("Appending initial message (Role: %s) after loaded legacy history.", msg.Role)
			s.payload.Messages = append(s.payload.Messages, msg)
		}
		if err := s.setupSystemPrompt(); err != nil {
			s.logger.Warnf("Error applying config system prompt after loading legacy history: %v", err)
		}

		// Recommend saving immediately in the new format if an output file is configured
		if s.historyOutFile != "" {
			s.logger.Info("Legacy history format loaded. Saving immediately in new YAML format.")
			if err := s.saveHistory(); err != nil {
				// Log failure but don't treat as critical load error
				s.logger.Warnf("Failed to save history in new format after loading legacy: %v", err)
				s.logger.Warnf("Failed to convert legacy history to new format: %v", err)
			} else {
				s.logger.Debug("Successfully saved history in YAML format after loading legacy.")
			}
		}
		return nil
	}

	// If neither YAML nor legacy parsing worked
	s.logger.Warn("Could not parse history file content (checked YAML and legacy formats). Proceeding without loaded history.")
	// Optionally inform user
	s.logger.Warn("Could not parse history file content. Ignoring history file.")
	return nil // Treat unparseable as non-fatal for now, allows startup
}

// saveHistory saves the history to the history file in YAML format.
func (s *Service) saveHistory() error {
	if s.historyOutFile == "" {
		s.logger.Debug("No history output file specified, skipping save.")
		return nil
	}

	// Ensure the path is fully resolved (tilde expansion should happen earlier, but check again)
	expandedPath, err := expandTilde(s.historyOutFile)
	if err != nil {
		// Log error but proceed with potentially unexpanded path
		s.logger.Warnf("Could not expand history output path '%s' during save: %v", s.historyOutFile, err)
		expandedPath = s.historyOutFile
	}

	if len(s.payload.Messages) == 0 {
		// Don't save an empty history file unless it doesn't exist yet (to create dir)
		// Let's attempt to create the directory anyway, then write if messages exist.
		s.logger.Debug("No messages in payload to save to history.")
		// return nil // Skip saving if no messages? Or save empty file?
	}

	// Ensure directory exists
	historyDir := filepath.Dir(expandedPath)
	s.logger.Debugf("Ensuring history directory exists: %s", historyDir)
	if err := os.MkdirAll(historyDir, 0750); err != nil { // Slightly more restrictive permissions
		return fmt.Errorf("failed to create history directory %s: %w", historyDir, err)
	}

	// Only proceed to write if there are messages
	if len(s.payload.Messages) == 0 {
		s.logger.Debug("Directory ensured, but no messages to write.")
		return nil
	}

	// Wrap messages for marshaling
	wrappedMessages := make([]MessageContentWrapper, len(s.payload.Messages))
	for i, msg := range s.payload.Messages {
		wrappedMessages[i] = MessageContentWrapper{Content: msg}
	}

	// Marshal wrapped messages to YAML
	s.logger.Debugf("Marshaling %d messages to YAML for history file: %s", len(wrappedMessages), expandedPath)
	data, err := yaml.Marshal(wrappedMessages)
	if err != nil {
		return fmt.Errorf("failed to marshal history to YAML: %w", err)
	}

	// Write to file using TempFile and Rename for atomicity
	tempFile, err := os.CreateTemp(historyDir, filepath.Base(expandedPath)+".*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temporary history file in %s: %w", historyDir, err)
	}
	tempFilePath := tempFile.Name()
	s.logger.Debugf("Writing history to temporary file: %s", tempFilePath)

	_, writeErr := tempFile.Write(data)
	// Close the temp file before attempting rename
	closeErr := tempFile.Close()

	if writeErr != nil {
		_ = os.Remove(tempFilePath) // Attempt cleanup on write error
		return fmt.Errorf("failed to write to temporary history file %s: %w", tempFilePath, writeErr)
	}
	if closeErr != nil {
		_ = os.Remove(tempFilePath) // Attempt cleanup on close error
		return fmt.Errorf("failed to close temporary history file %s: %w", tempFilePath, closeErr)
	}

	// Atomically replace the target file with the temporary file
	s.logger.Debugf("Renaming temporary history file %s to %s", tempFilePath, expandedPath)
	err = os.Rename(tempFilePath, expandedPath)
	if err != nil {
		// If rename fails, the temp file might still be around. Attempt removal.
		_ = os.Remove(tempFilePath)
		return fmt.Errorf("failed to rename temporary history file %s to %s: %w", tempFilePath, expandedPath, err)
	}

	// Set permissions explicitly after rename (optional, depends on umask)
	// if err := os.Chmod(expandedPath, 0640); err != nil {
	// 	s.logger.Warnf("Failed to set permissions on history file %s: %v", expandedPath, err)
	// }

	s.logger.Infof("Saved %d messages to history file: %s", len(s.payload.Messages), expandedPath)

	return nil
}

// --- YAML Marshaling/Unmarshaling ---

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
	// ImageURL string `yaml:"image_url,omitempty"`
	// ToolCalls []ToolCall `yaml:"tool_calls,omitempty"` // Assuming ToolCall is marshallable
}

// MessageContentWrapper wraps llms.MessageContent for YAML marshaling
type MessageContentWrapper struct {
	Content llms.MessageContent
}

// MarshalYAML implements yaml.Marshaler
func (w MessageContentWrapper) MarshalYAML() (interface{}, error) {
	yamlParts := make([]messagePartYAML, 0, len(w.Content.Parts))
	for _, part := range w.Content.Parts {
		switch p := part.(type) {
		case llms.TextContent:
			yamlParts = append(yamlParts, messagePartYAML{Type: "text", Text: p.Text})
		// --- Add cases for other supported types ---
		// case llms.ImageURLContent:
		//  yamlParts = append(yamlParts, messagePartYAML{Type: "image_url", ImageURL: p.ImageURL})
		// case llms.ToolCallContent:
		//  // Assuming p.ToolCalls is a slice of a marshallable ToolCall struct
		//  yamlParts = append(yamlParts, messagePartYAML{Type: "tool_calls", ToolCalls: p.ToolCalls})
		default:
			// Handle unknown part types or return an error
			// Using standard logger here as service logger might not be available
			// log.Printf("Skipping unknown message part type during YAML marshal: %T", p)
			// Let's return an error instead of skipping silently
			return nil, fmt.Errorf("cannot marshal unknown message part type: %T", p)
		}
	}
	return messageContentYAML{Role: w.Content.Role, Parts: yamlParts}, nil
}

// UnmarshalYAML implements the yaml.Unmarshaler interface.
func (w *MessageContentWrapper) UnmarshalYAML(node *yaml.Node) error {
	// Check node kind first
	if node.Kind != yaml.MappingNode {
		// Support legacy format where history was just a list of strings?
		// For now, require the map structure.
		return fmt.Errorf("expected YAML mapping node for message, got %v", node.Kind)
	}

	var helper messageContentYAML
	if err := node.Decode(&helper); err != nil {
		return fmt.Errorf("failed to decode message YAML structure: %w", err)
	}

	w.Content.Role = helper.Role
	w.Content.Parts = make([]llms.ContentPart, 0, len(helper.Parts))
	for _, yamlPart := range helper.Parts {
		switch yamlPart.Type {
		case "text":
			w.Content.Parts = append(w.Content.Parts, llms.TextContent{Text: yamlPart.Text})
		// --- Add cases for other supported types ---
		// case "image_url":
		//  w.Content.Parts = append(w.Content.Parts, llms.ImageURLContent{ImageURL: yamlPart.ImageURL})
		// case "tool_calls":
		//  w.Content.Parts = append(w.Content.Parts, llms.ToolCallContent{ToolCalls: yamlPart.ToolCalls})
		default:
			// Log or return error for unknown types found in the file
			// Using standard logger here as service logger might not be available
			// log.Printf("Skipping unknown message part type '%s' during YAML unmarshal", yamlPart.Type)
			// Return error to be stricter
			return fmt.Errorf("unknown message part type '%s' found in history YAML", yamlPart.Type)
		}
	}
	return nil
}

// Ensure our custom types work with the yaml package
var _ yaml.Marshaler = (*MessageContentWrapper)(nil)
var _ yaml.Unmarshaler = (*MessageContentWrapper)(nil)

// --- Spinner ---

// spinner provides a simple CLI spinner.
type spinner struct {
	frames     []string
	pos        int
	active     bool
	stopChan   chan struct{} // Renamed from 'done'
	output     io.Writer
	mu         sync.Mutex // Added mutex for thread safety
	ticker     *time.Ticker
	initialPos int
}

// newSpinner creates a spinner instance.
func newSpinner(output io.Writer) *spinner {
	if output == nil {
		output = os.Stderr // Default to stderr
	}
	return &spinner{
		// Simple frames: "-\|/"
		// Braille frames: []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		frames:   []string{"-", "\\", "|", "/"},
		stopChan: make(chan struct{}),
		output:   output,
	}
}

// start begins the spinner animation in a separate goroutine.
func (s *spinner) start(initialPos int) {
	s.mu.Lock()
	if s.active {
		s.mu.Unlock()
		return // Already active
	}
	s.active = true
	s.initialPos = initialPos
	s.pos = 0
	s.ticker = time.NewTicker(150 * time.Millisecond) // Slower ticker?
	s.mu.Unlock()

	go s.run()
}

// run is the internal loop for the spinner animation.
func (s *spinner) run() {
	s.mu.Lock()
	// Print initial spaces if needed
	if s.initialPos > 0 {
		fmt.Fprint(s.output, strings.Repeat(" ", s.initialPos))
	}
	fmt.Fprint(s.output, " ") // Print initial space for the spinner frame
	ticker := s.ticker
	s.mu.Unlock()

	defer ticker.Stop()

	for {
		select {
		case <-s.stopChan:
			s.mu.Lock()
			// Clear the spinner frame before returning
			fmt.Fprint(s.output, "\b \b")
			s.mu.Unlock()
			return
		case <-ticker.C:
			s.mu.Lock()
			if !s.active { // Check active flag inside loop
				fmt.Fprint(s.output, "\b \b") // Clear just in case
				s.mu.Unlock()
				return
			}
			// Update spinner frame
			fmt.Fprint(s.output, "\b"+s.frames[s.pos])
			s.pos = (s.pos + 1) % len(s.frames)
			s.mu.Unlock()
		}
	}
}

// stop signals the spinner goroutine to stop and cleans up.
func (s *spinner) stop() {
	s.mu.Lock()
	if !s.active {
		s.mu.Unlock()
		return // Already stopped
	}
	s.active = false
	// Stop the ticker first
	if s.ticker != nil {
		s.ticker.Stop()
	}
	s.mu.Unlock()

	// Signal the goroutine using non-blocking send
	select {
	case s.stopChan <- struct{}{}:
	default:
		// Goroutine might have already exited, which is fine.
	}
}

// spin starts a spinner at the given character position using the specified writer.
// It returns a function to stop the spinner.
func spin(pos int, writer io.Writer) func() {
	if writer == nil {
		writer = os.Stderr // Default to stderr if nil
	}
	// Check if writer is a terminal TTY before starting spinner
	isTerminal := false
	if f, ok := writer.(*os.File); ok {
		// Use golang.org/x/term for potentially better cross-platform check
		isTerminal = term.IsTerminal(int(f.Fd()))
	} else {
		// If not a file, assume not a terminal (e.g., buffer in tests)
		isTerminal = false
	}

	// Only start spinner if output is a terminal
	if !isTerminal {
		// log.Debug("Output is not a terminal, spinner disabled.") // Use service logger if available
		return func() {} // Return no-op function if not a terminal
	}

	// log.Debug("Output is a terminal, starting spinner.") // Use service logger
	s := newSpinner(writer)
	s.start(pos)
	return s.stop // Return the stop method
}

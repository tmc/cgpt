// Command cgpt is a command line tool for interacting with Large Language Models (LLMs).
//
// Usage:
//
//	cgpt [flags] [input]
//
// Input can be provided via:
//   - Command line arguments
//   - -i/--input flag (can be used multiple times)
//   - -f/--file flag (can be used multiple times, use '-' for stdin)
//   - Piped input
//
// Flags:
//
//	-b, --backend string             The backend to use (default "anthropic")
//	-m, --model string               The model to use (default "claude-3-7-sonnet-20250219")
//	-i, --input string               Direct string input (can be used multiple times)
//	-f, --file string                Input file path. Use '-' for stdin (can be used multiple times)
//	-c, --continuous                 Run in continuous mode (interactive)
//	-s, --system-prompt string       System prompt to use
//	-p, --prefill string             Prefill the assistant's response
//	-I, --history-load string        File to read completion history from
//	-O, --history-save string        File to store completion history in
//	    --config string              Path to the configuration file (default "config.yaml")
//	-v, --verbose                    Verbose output
//	    --debug                      Debug output
//	-n, --completions int            Number of completions (when running non-interactively with history)
//	-t, --max-tokens int             Maximum tokens to generate (default 8000)
//	    --completion-timeout duration Maximum time to wait for a response (default 2m0s)
//	-h, --help                       Display help information
//
// The -c/--continuous flag enables interactive mode, where the program runs in a loop,
// using the previous output as input for the next request. In this mode, inference
package main

import (
	"context"
	"errors" // Import errors package
	"fmt"
	"io"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/pflag"
	"github.com/tmc/cgpt/backends"
	"github.com/tmc/cgpt/completion"
	"github.com/tmc/cgpt/input"
	"github.com/tmc/cgpt/options"
	"github.com/tmc/langchaingo/httputil"
	"golang.org/x/term"
)

// defineFlags defines the command line flags for the cgpt command
func defineFlags(fs *pflag.FlagSet, opts *options.RunOptions) {
	// Runtime flags
	fs.StringArrayVarP(&opts.InputStrings, "input", "i", nil, "Direct string input (can be used multiple times)")
	// Default to using stdin by default, just like the original code.
	// This ensures stdin is read by default, and cleared when in a terminal.
	fs.StringArrayVarP(&opts.InputFiles, "file", "f", []string{"-"}, "Input file path. Use '-' for stdin (can be used multiple times)")
	// Default to false for continuous mode - if stdin is a terminal with no other inputs,
	// we'll set this to true automatically later
	fs.BoolVarP(&opts.Continuous, "continuous", "c", false, "Run in continuous mode (interactive)")
	fs.BoolVar(&opts.UseTUI, "tui", false, "Use terminal UI mode (BubbleTea) for interactive sessions")
	fs.BoolVarP(&opts.Verbose, "verbose", "v", false, "Verbose output")
	fs.BoolVar(&opts.DebugMode, "debug", false, "Debug output")
	fs.BoolVar(&opts.ShowSpinner, "show-spinner", true, "Show spinner while waiting for completion")
	fs.StringVarP(&opts.Prefill, "prefill", "p", "", "Prefill the assistant's response")
	fs.BoolVar(&opts.StreamOutput, "stream", true, "Use streaming output")

	fs.BoolVar(&opts.PrintUsage, "print-usage", false, "Print token usage information")

	fs.BoolVar(&opts.OpenAIUseLegacyMaxTokens, "openai-use-max-tokens", false, "If true, uses 'max_tokens' vs 'max_output_tokens' for openai backends")

	fs.BoolVar(&opts.EchoPrefill, "prefill-echo", true, "Print the prefill message")
	fs.DurationVar(&opts.CompletionTimeout, "completion-timeout", 2*time.Minute, "Maximum time to wait for a response")

	// History flags
	fs.StringVarP(&opts.HistoryIn, "history-in", "I", "", "File to read completion history from")
	fs.StringVarP(&opts.HistoryOut, "history-out", "O", "", "File to store completion history in")
	fs.StringVar(&opts.HistoryIn, "history-load", "", "File to read completion history from (deprecated)")
	fs.StringVar(&opts.HistoryOut, "history-save", "", "File to store completion history in (deprecated)")

	fs.StringVar(&opts.ReadlineHistoryFile, "readline-history-file", "~/.cgpt_history", "File to store readline history in")
	fs.IntVarP(&opts.NCompletions, "completions", "n", 0, "Number of completions (when running non-interactively with history)")

	// Config flags
	// Check if opts.Config is nil before accessing fields
	backend := "anthropic"
	model := "claude-3-7-sonnet-20250219"
	sysPrompt := ""
	maxTokens := 0
	temp := 0.05
	if opts.Config != nil {
		backend = opts.Config.Backend
		model = opts.Config.Model
		sysPrompt = opts.Config.SystemPrompt
		maxTokens = opts.Config.MaxTokens
		temp = opts.Config.Temperature
	}
	fs.StringVarP(&opts.Config.Backend, "backend", "b", backend, "The backend to use")
	fs.StringVarP(&opts.Config.Model, "model", "m", model, "The model to use")
	fs.StringVarP(&opts.Config.SystemPrompt, "system-prompt", "s", sysPrompt, "System prompt to use")
	fs.IntVarP(&opts.Config.MaxTokens, "max-tokens", "t", maxTokens, "Maximum tokens to generate")
	fs.Float64VarP(&opts.Config.Temperature, "temperature", "T", temp, "Temperature for sampling")

	// Config file path
	fs.StringVar(&opts.ConfigPath, "config", "config.yaml", "Path to the configuration file")
	
	// Test flags
	fs.BoolVar(&opts.Config.SlowResponses, "slow-responses", false, "Simulate slow response generation (for UX testing)")
	fs.StringVar(&opts.Config.HTTPRecordFile, "http-record", "", "Path to HTTP record/replay file for tests")
	// Add alias for http-record to match what's used in tests
	fs.StringVar(&opts.Config.HTTPRecordFile, "httprecord", "", "Path to HTTP record/replay file for tests (alias)")
}

func main() {
	opts, flagSet, err := initFlags(os.Args, os.Stdin)
	if err != nil {
		// Check specifically for pflag.ErrHelp to exit cleanly
		if errors.Is(err, pflag.ErrHelp) {
			os.Exit(0)
		}
		fmt.Fprintf(os.Stderr, "cgpt: flag error: %v\n", err)
		os.Exit(2)
	}

	// Start pprof if needed (keep as is)
	go func() {
		_ = http.ListenAndServe("localhost:6060", nil) // Ignore error for optional pprof
	}()

	ctx := context.Background()

	// Use a long-running context - the completion timeouts are handled at the LLM level
	// This allows the shell to remain responsive for user input in continuous mode

	// Create a context with signal handling
	// We'll use a custom signal handling approach for better control
	// Standard NotifyContext can be too aggressive with cancellation
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Set up custom signal handling that's more controlled
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)

	// Track interruption time to implement a "double-interrupt" feature
	var lastInterruptTime time.Time
	// Use a very short threshold to make it harder to trigger accidentally
	// Require a very quick double-tap to force exit (within 250ms)
	// Most users pressing Ctrl+C twice in different contexts won't trigger this
	const doubleInterruptThreshold = 250 * time.Millisecond

	// No longer needed - we'll use a simpler approach

	go func() {
		for _ = range sigChan {
			// Record current time
			now := time.Now()

			// Check if this is a rapid double-interrupt (Ctrl+C pressed twice quickly)
			if !lastInterruptTime.IsZero() && now.Sub(lastInterruptTime) < doubleInterruptThreshold {
				// Double interrupt - force exit
				fmt.Fprintln(os.Stderr, "\nReceived rapid double interrupt, exiting immediately.")
				// Log the double-interrupt for debugging
				if opts.DebugMode {
					fmt.Fprintf(os.Stderr, "Debug: Double interrupt detected (%v ms apart)\n", 
						now.Sub(lastInterruptTime).Milliseconds())
				}
				os.Exit(1)
			}

			// Update last interrupt time
			lastInterruptTime = now

			// Cancel the context but don't exit immediately
			// This allows our handlers to process the cancellation gracefully
			cancel()

			// For very long-running process that might hang, set a safety timeout
			// This is just a safeguard in case the cancellation isn't processed properly
			go func() {
				time.Sleep(10 * time.Second)
				fmt.Fprintln(os.Stderr, "\nForced exit after timeout.")
				os.Exit(1)
			}()

			// Removed case - simplified approach
		}
	}()
	// Clean up signal handling when done
	defer signal.Stop(sigChan)

	// Shutdown handling is now done in the signal handler above

	if err = run(ctx, opts, flagSet); err != nil {
		// Don't report expected ways to exit as errors (context canceled, interrupt, EOF, etc)
		exitError := false
		
		// Check for any of the "normal" ways to exit
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			// Context cancellation (Ctrl+C handling) is expected
			exitError = true
		} else if err.Error() == "Interrupt" { 
			// Readline's ErrInterrupt comes from the interactive session (Ctrl+C at prompt)
			exitError = true
		} else if errors.Is(err, io.EOF) {
			// EOF (Ctrl+D) is an expected way to exit
			exitError = true
		}
		
		if !exitError {
			// Only show error for unexpected issues
			fmt.Fprintf(os.Stderr, "cgpt: error: %T %v\n", err, err)
			os.Exit(1)
		}
		// For expected ways to exit, just exit cleanly (status 0)
	}
	// Exit 0 on success or context cancellation
}

// run function now takes the main context directly
func run(ctx context.Context, opts options.RunOptions, flagSet *pflag.FlagSet) error {
	// Ensure we have stderr output even if uninitialized
	stderr := opts.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}

	// Load the config file
	fileConfig, err := options.LoadConfig(opts.ConfigPath, stderr, flagSet)
	if err != nil {
		// Use Fprintf for stderr
		fmt.Fprintf(stderr, "Warning: failed to load config: %v. Proceeding with defaults/flags.\n", err)
		// Reset fileConfig to avoid using partially loaded data, rely on flags/defaults
		fileConfig = &options.Config{} // Or handle more gracefully if needed
	}
	// Apply flags over fileConfig (or vice versa, depending on desired precedence)
	// This merge logic might need refinement based on how flags vs config are intended to interact.
	// Assuming flags override config for now:
	if flagSet.Changed("backend") {
		fileConfig.Backend = opts.Config.Backend
	}
	if flagSet.Changed("model") {
		fileConfig.Model = opts.Config.Model
	}
	if flagSet.Changed("system-prompt") {
		fileConfig.SystemPrompt = opts.Config.SystemPrompt
	}
	if flagSet.Changed("max-tokens") {
		fileConfig.MaxTokens = opts.Config.MaxTokens
	}
	if flagSet.Changed("temperature") {
		fileConfig.Temperature = opts.Config.Temperature
	}
	opts.Config = fileConfig // Use the potentially merged/updated config

	// TODO: move to history (keep as is)
	if dir, _ := os.UserHomeDir(); dir != "" {
		err := os.MkdirAll(filepath.Join(dir, ".cgpt"), 0755)
		if err != nil {
			fmt.Fprintf(stderr, "Failed to create default save path: %v\n", err)
		}
	}

	// Process input using the new input package
	isStdinTerminal := false
	if f, ok := opts.Stdin.(*os.File); ok && f != nil { // Check if f is not nil
		isStdinTerminal = term.IsTerminal(int(f.Fd()))
	}

	// Store the flags that were explicitly set, in order they appeared on the command line
	var inputFileOrder, inputStringOrder []string
	flagSet.Visit(func(flag *pflag.Flag) {
		// Ensure flag.Value is not nil before calling String()
		if flag.Value == nil {
			return
		}
		if flag.Name == "file" || flag.Name == "f" {
			// Handle string slice flags correctly
			if files, ok := flag.Value.(pflag.SliceValue); ok {
				inputFileOrder = append(inputFileOrder, files.GetSlice()...)
			} else {
				inputFileOrder = append(inputFileOrder, flag.Value.String())
			}
		} else if flag.Name == "input" || flag.Name == "i" {
			if inputs, ok := flag.Value.(pflag.SliceValue); ok {
				inputStringOrder = append(inputStringOrder, inputs.GetSlice()...)
			} else {
				inputStringOrder = append(inputStringOrder, flag.Value.String())
			}
		}
	})

	// Pass the inputs and continuous flag to the processor
	// TTY reattachment logic depends on properly handling continuous mode
	inputProcessor := input.NewProcessor(
		opts.InputFiles,
		opts.InputStrings,
		opts.PositionalArgs,
		opts.Stdin,
		isStdinTerminal,
		opts.Continuous,
	).WithFileOrder(inputFileOrder).WithStringOrder(inputStringOrder)

	// Pass ctx directly to GetCombinedReader
	inputReader, _, tryReattachTTY, err := inputProcessor.GetCombinedReader(ctx)
	if err != nil {
		return fmt.Errorf("failed to prepare input reader: %w", err) // Error opening files etc.
	}

	inputBytes, err := io.ReadAll(inputReader)
	if err != nil {
		// Check for context cancellation during read
		if errors.Is(err, context.Canceled) {
			return err // Propagate cancellation
		}
		return fmt.Errorf("failed to read input: %w", err)
	}
	initialPrompt := strings.TrimSpace(string(inputBytes))

	// --- TTY Handling ---
	var ttyFile *os.File
	// We need to reattach to /dev/tty for interactive mode in these cases:
	// 1. Input processor explicitly requested reattachment (tryReattachTTY=true)
	//    This happens when stdin was consumed but was originally a terminal, or in continuous mode
	// 2. We're running in continuous mode AND stdin is still a terminal (direct terminal input)
	if tryReattachTTY || (opts.Continuous && isStdinTerminal) {
		var errTTY error
		ttyFile, errTTY = os.OpenFile("/dev/tty", os.O_RDWR, 0)
		if errTTY != nil {
			fmt.Fprintf(stderr, "Error: Could not reattach to terminal for interactive mode.\n")
			ttyFile = nil
		} else {
			defer func() {
				if ttyFile != nil {
					_ = ttyFile.Close() // Ignore close error on defer
				}
			}()
		}
	}

	// Initialize the model
	modelOpts := []backends.InferenceProviderOption{}
	if opts.DebugMode {
		fmt.Fprintln(stderr, "cgpt: Debug mode enabled")
		modelOpts = append(modelOpts, backends.WithHTTPClient(httputil.DebugHTTPClient))
	}
	if opts.OpenAIUseLegacyMaxTokens {
		modelOpts = append(modelOpts, backends.WithUseLegacyMaxTokens(true))
	}
	model, err := backends.InitializeModel(opts.Config, modelOpts...)
	if err != nil {
		return fmt.Errorf("failed to initialize model: %w", err)
	}

	// Determine whether to run in continuous mode
	// If -c is explicitly set, respect that and ensure it's properly handled
	isContinuousSet := flagSet.Changed("continuous") || flagSet.Changed("c")

	if isContinuousSet {
		opts.Continuous = true
		// When continuous mode is explicitly requested, ensure stdin is used
		if len(opts.InputFiles) == 0 {
			opts.InputFiles = []string{"-"}
		}
	} else if isStdinTerminal &&
		len(opts.InputFiles) == 0 && len(opts.InputStrings) == 0 && len(opts.PositionalArgs) == 0 {
		// Otherwise, if stdin is a tty, and no input files, strings, or args are provided,
		// then we should run in continuous mode (implicit continuous mode)
		opts.Continuous = true
		fmt.Fprintf(stderr, "No inputs provided and stdin is a terminal, switching to continuous mode\n")
	}

	// Spinner on TTY (keep as is)
	isStdoutTerminal := false
	if f, ok := opts.Stdout.(*os.File); ok && f != nil { // Check f not nil
		isStdoutTerminal = term.IsTerminal(int(f.Fd()))
	}
	opts.ShowSpinner = opts.ShowSpinner && isStdoutTerminal

	// Create the completion service config & service
	compCfg := NewCompletionConfig(opts)

	// Create service options
	svcOpts := completion.NewOptions()

	// Copy over relevant options
	svcOpts.Stdout = opts.Stdout
	svcOpts.Stderr = opts.Stderr
	svcOpts.ShowSpinner = opts.ShowSpinner
	svcOpts.EchoPrefill = opts.EchoPrefill
	svcOpts.PrintUsage = opts.PrintUsage
	svcOpts.StreamOutput = opts.StreamOutput
	svcOpts.Continuous = opts.Continuous
	svcOpts.UseTUI = opts.UseTUI
	svcOpts.Verbose = opts.Verbose
	svcOpts.DebugMode = opts.DebugMode
	svcOpts.HistoryIn = opts.HistoryIn
	svcOpts.HistoryOut = opts.HistoryOut
	svcOpts.ReadlineHistoryFile = opts.ReadlineHistoryFile
	svcOpts.Prefill = opts.Prefill
	if opts.CompletionTimeout > 0 {
		svcOpts.CompletionTimeout = opts.CompletionTimeout
	}

	// If we need to use the TTY, use it for stdout in the completion service
	// The stdin handling is managed separately through RunOptions.Stdin
	if ttyFile != nil && opts.Continuous {
		// For continuous mode with TTY, override both STDIN and STDOUT
		// This is crucial for the interactive prompt to work correctly
		svcOpts.Stdout = ttyFile

		// We need to explicitly override stdin in RunOptions to use the TTY file
		// Since RunOptions.Stdin may have been set to read from pipe/file already
		opts.Stdin = ttyFile
	}

	// Create a logger for the completion service
	logger, err := NewLogger(opts.Stderr, opts.Verbose, opts.DebugMode)
	if err != nil {
		return fmt.Errorf("failed to create logger: %w", err)
	}

	// Not needed anymore - simplified approach

	s, err := completion.New(compCfg, model,
		completion.WithOptions(svcOpts),
		completion.WithLogger(logger))
	if err != nil {
		return fmt.Errorf("cgpt: failed to create completion service: %w", err)
	}

	// Add initial prompt if we read one
	if initialPrompt != "" {
		s.AddUserMessage(initialPrompt)
	}

	// Handle case where we are NOT continuous and have NO initial prompt (even after loading history)
	if !opts.Continuous && initialPrompt == "" && opts.HistoryIn == "" {
		// Check payload message count *after* potential history loading inside New()
		if len(s.GetPayloadMessages()) == 0 { // Assuming GetPayloadMessages exists
			return fmt.Errorf("no input provided for non-continuous mode (use -i, -f, args, or pipe stdin)")
		}
	}

	// Add prefill if provided
	if opts.Prefill != "" {
		s.SetNextCompletionPrefill(opts.Prefill)
	}

	// Run the completion service
	runOpts := completion.RunOptions{
		Config:              compCfg, // This is completion.Config
		InputStrings:        opts.InputStrings,
		InputFiles:          opts.InputFiles,
		PositionalArgs:      opts.PositionalArgs,
		Prefill:             opts.Prefill,
		Continuous:          opts.Continuous,
		StreamOutput:        opts.StreamOutput,
		ShowSpinner:         opts.ShowSpinner,
		EchoPrefill:         opts.EchoPrefill,
		UseTUI:              opts.UseTUI,
		PrintUsage:          opts.PrintUsage,
		Verbose:             opts.Verbose,
		DebugMode:           opts.DebugMode,
		HistoryIn:           opts.HistoryIn,
		HistoryOut:          opts.HistoryOut,
		ReadlineHistoryFile: opts.ReadlineHistoryFile,
		NCompletions:        opts.NCompletions,
		Stdout:              opts.Stdout,
		Stderr:              opts.Stderr,
		// Pass the stdin directly to allow proper TTY handling in interactive mode
		// The ttyFile has already been set as opts.Stdin above if needed
		// Handle stdin specially to avoid wrapping TTY files in NopCloser
		Stdin:                    getReadCloser(opts.Stdin),
		MaximumTimeout:           opts.CompletionTimeout,
		ConfigPath:               opts.ConfigPath,
		OpenAIUseLegacyMaxTokens: opts.OpenAIUseLegacyMaxTokens,
	}
	// Pass the main ctx directly
	return s.Run(ctx, runOpts)
}

// getReadCloser returns an io.ReadCloser for the given reader.
// If the reader is already an io.ReadCloser or an *os.File, it is returned as is.
// Otherwise, it is wrapped in an io.NopCloser.
func getReadCloser(r io.Reader) io.ReadCloser {
	if rc, ok := r.(io.ReadCloser); ok {
		return rc
	}
	if f, ok := r.(*os.File); ok {
		return f // *os.File implements ReadCloser
	}
	return io.NopCloser(r)
}

// NewCompletionConfig creates a completion.Config from options.RunOptions
func NewCompletionConfig(cfg options.RunOptions) *completion.Config {
	// Ensure cfg.Config is not nil before accessing fields
	maxTokens := 0
	temp := 0.05 // Default temperature
	sysPrompt := ""
	if cfg.Config != nil {
		maxTokens = cfg.Config.MaxTokens
		temp = cfg.Config.Temperature
		sysPrompt = cfg.Config.SystemPrompt
	}
	return &completion.Config{
		MaxTokens:         maxTokens,
		Temperature:       temp,
		SystemPrompt:      sysPrompt,
		CompletionTimeout: cfg.CompletionTimeout, // Use timeout from RunOptions directly
	}
}

// initFlags defines and parses command line flags
func initFlags(args []string, stdin io.Reader) (options.RunOptions, *pflag.FlagSet, error) {
	opts := options.RunOptions{
		Config: &options.Config{}, // Initialize Config field
		Stdin:  stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
	// Default values for Config fields if not set by flags/config file
	opts.Config.Backend = "anthropic"
	opts.Config.Model = "claude-3-7-sonnet-20250219"
	opts.Config.Temperature = 0.05
	opts.CompletionTimeout = 2 * time.Minute

	if len(args) == 0 {
		// Provide a default pflag.FlagSet even on error for consistency
		fs := pflag.NewFlagSet("cgpt", pflag.ContinueOnError)
		defineFlags(fs, &opts) // Define flags even if no args are provided
		return opts, fs, fmt.Errorf("no arguments provided")
	}

	fs := pflag.NewFlagSet(args[0], pflag.ContinueOnError)
	fs.SortFlags = false
	defineFlags(fs, &opts) // Define flags on the new FlagSet

	// If stdin is a terminal, clear the default "-" stdin input to avoid blocking
	// UNLESS continuous mode (-c flag) is explicitly set, in which case we want
	// to keep the TTY for the interactive mode prompt
	if f, ok := stdin.(*os.File); ok && f != nil && term.IsTerminal(int(f.Fd())) {
		// Look for explicit continuous flag
		isContinuousSet := false
		fs.Visit(func(flag *pflag.Flag) {
			if flag.Name == "continuous" || flag.Name == "c" {
				isContinuousSet = true
			}
		})

		if !isContinuousSet {
			// Only clear InputFiles if continuous mode isn't explicitly requested
			opts.InputFiles = nil
		}
	}

	showAdvancedUsage := fs.String("show-advanced-usage", "", "Show advanced usage examples (comma separated list of sections, or 'all')")
	help := fs.BoolP("help", "h", false, "Display help information")

	// Keep hidden flags as they are
	fs.MarkHidden("stream") // Correct name for the stream flag
	fs.MarkHidden("readline-history-file")
	fs.MarkHidden("prefill-echo")
	fs.MarkHidden("show-spinner")
	// Mark deprecated history flags as hidden
	fs.MarkHidden("history-load")
	fs.MarkHidden("history-save")

	fs.Usage = func() {
		// Use Stderr for usage to match convention
		fmt.Fprintln(os.Stderr, "cgpt is a command line tool for interacting with generative AI models")
		fmt.Fprintln(os.Stderr) // Add a newline
		// Ensure showAdvancedUsage isn't nil before dereferencing
		if showAdvancedUsage != nil && *showAdvancedUsage != "" {
			printAdvancedUsage(*showAdvancedUsage) // Assumes this prints to stdout/stderr appropriately
			return
		}
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", args[0])
		// Use FlagUsagesWrapped for better wrapping
		fmt.Fprint(os.Stderr, fs.FlagUsagesWrapped(120)) // Adjust wrap width if needed
		printBasicUsage()                                // Assumes this prints to stdout/stderr appropriately
	}

	err := fs.Parse(args[1:])
	if err != nil {
		// Check for help flag explicitly even on parse error
		if help != nil && *help { // Check if help is not nil before dereferencing
			fs.Usage()
			return opts, fs, pflag.ErrHelp // Return specific help error
		}
		return opts, fs, err // Return the parse error
	}

	// Handle help and advanced usage after successful parsing
	if help != nil && *help { // Check if help is not nil
		fs.Usage()
		return opts, fs, pflag.ErrHelp
	}

	if showAdvancedUsage != nil && *showAdvancedUsage != "" { // Check if showAdvancedUsage is not nil
		printAdvancedUsage(*showAdvancedUsage)
		return opts, fs, pflag.ErrHelp // Also treat this as requesting help
	}

	opts.PositionalArgs = fs.Args()

	return opts, fs, nil
}

// Import usage examples file when available
// Usage examples are defined in usage_examples.go

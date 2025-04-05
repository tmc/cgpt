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
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
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
	fs.StringArrayVarP(&opts.InputFiles, "file", "f", nil, "Input file path. Use '-' for stdin (can be used multiple times)")
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
	fs.StringVarP(&opts.Config.Backend, "backend", "b", "anthropic", "The backend to use")
	fs.StringVarP(&opts.Config.Model, "model", "m", "claude-3-7-sonnet-20250219", "The model to use")
	fs.StringVarP(&opts.Config.SystemPrompt, "system-prompt", "s", "", "System prompt to use")
	fs.IntVarP(&opts.Config.MaxTokens, "max-tokens", "t", 0, "Maximum tokens to generate")
	fs.Float64VarP(&opts.Config.Temperature, "temperature", "T", 0.05, "Temperature for sampling")

	// Config file path
	fs.StringVar(&opts.ConfigPath, "config", "config.yaml", "Path to the configuration file")
}

func main() {
	opts, flagSet, err := initFlags(os.Args, os.Stdin)
	if err != nil {
		if err == pflag.ErrHelp {
			os.Exit(0)
		}
		fmt.Fprintf(os.Stderr, "cgpt: flag error: %v\n", err)
		os.Exit(2)
	}

	ctx := context.Background()
	if err := run(ctx, opts, flagSet); err != nil {
		fmt.Fprintf(os.Stderr, "cgpt: error: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, opts options.RunOptions, flagSet *pflag.FlagSet) error {
	// Ensure we have stderr output even if uninitialized
	stderr := opts.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}

	// Load the config file
	fileConfig, err := options.LoadConfig(opts.ConfigPath, stderr, flagSet)
	if err != nil {
		fmt.Fprintf(stderr, "Warning: failed to load config: %v. Proceeding with defaults/flags.\n", err)
	}
	opts.Config = fileConfig

	// TODO: move to history
	// Creates the default save path if it doesn't exist
	if dir, _ := os.UserHomeDir(); dir != "" {
		err := os.MkdirAll(filepath.Join(dir, ".cgpt"), 0755)
		if err != nil {
			fmt.Fprintf(stderr, "Failed to create default save path: %v\n", err)
		}
	}

	// Process input using the new input package
	isStdinTerminal := term.IsTerminal(int(os.Stdin.Fd())) // Check TTY status *once*

	// Store the flags that were explicitly set, in order they appeared on the command line
	var inputFileOrder, inputStringOrder []string
	flagSet.Visit(func(flag *pflag.Flag) {
		if flag.Name == "file" || flag.Name == "f" {
			inputFileOrder = append(inputFileOrder, flag.Value.String())
		} else if flag.Name == "input" || flag.Name == "i" {
			inputStringOrder = append(inputStringOrder, flag.Value.String())
		}
	})

	// Pass the continuous flag to the processor for TTY reattachment decision
	inputProcessor := input.NewProcessor(
		opts.InputFiles,
		opts.InputStrings,
		opts.PositionalArgs,
		opts.Stdin,
		isStdinTerminal,
		opts.Continuous, // Let the input processor know if -c flag was used
	).WithFileOrder(inputFileOrder).WithStringOrder(inputStringOrder)

	inputReader, _, tryReattachTTY, err := inputProcessor.GetCombinedReader(ctx)
	if err != nil {
		return fmt.Errorf("failed to prepare input reader: %w", err) // Error opening files etc.
	}

	inputBytes, err := io.ReadAll(inputReader)
	if err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}
	initialPrompt := strings.TrimSpace(string(inputBytes))

	// --- TTY Handling ---
	// If we need to reattach to TTY (continuous mode with piped input)
	var ttyFile *os.File
	// Skip debug message to keep output clean
	if tryReattachTTY {
		// This flag implies opts.Continuous is true and stdin was piped
		var errTTY error
		ttyFile, errTTY = os.OpenFile("/dev/tty", os.O_RDWR, 0)

		if errTTY != nil {
			// Don't fail hard, but log prominently - future TTY operations will likely fail
			fmt.Fprintf(stderr, "Error: Could not reattach to terminal for interactive mode.\n")
			ttyFile = nil // Ensure ttyFile is nil if open failed
		} else {
			// *** CRITICAL: Defer close until the run function exits ***
			defer func() {
				if ttyFile != nil {
					ttyFile.Close()
				}
			}()
		}
	}

	// Initialize the model (the llms.Model interface)
	modelOpts := []backends.InferenceProviderOption{}
	// if debug mode is on, attach the debug http client:
	if opts.DebugMode {
		fmt.Fprintln(stderr, "Debug mode enabled")
		modelOpts = append(modelOpts, backends.WithHTTPClient(httputil.DebugHTTPClient))
	}
	if opts.OpenAIUseLegacyMaxTokens {
		modelOpts = append(modelOpts, backends.WithUseLegacyMaxTokens(true))
	}
	model, err := backends.InitializeModel(opts.Config, modelOpts...)
	if err != nil {
		return fmt.Errorf("failed to initialize model: %w", err)
	}

	// Check if user *explicitly* provided input via args, -i, or -f (excluding '-')
	hasExplicitFileInput := false
	for _, f := range opts.InputFiles {
		if f != "-" {
			hasExplicitFileInput = true
			break
		}
	}
	hasExplicitInputOtherThanStdin := hasExplicitFileInput || len(opts.InputStrings) > 0 || len(opts.PositionalArgs) > 0

	// Launch implicitly into continuous mode if:
	// 1. No other input sources were given (args, -i, -f other than '-')
	// 2. Stdin is available and is a terminal (not piped/redirected)
	// 3. The user didn't already specify -c
	if opts.Stdin != nil && isStdinTerminal && !hasExplicitInputOtherThanStdin && !opts.Continuous {
		opts.Continuous = true
	}
	// --- End Continuous Mode ---

	// Only have spinner on if stdout is a tty:
	opts.ShowSpinner = opts.ShowSpinner && term.IsTerminal(int(os.Stdout.Fd()))

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
	if ttyFile != nil && opts.Continuous {
		svcOpts.Stdout = ttyFile
	}

	s, err := completion.New(compCfg, model, completion.WithOptions(svcOpts))
	if err != nil {
		return fmt.Errorf("failed to create completion service: %w", err)
	}

	// Add initial prompt if we read one
	if initialPrompt != "" {
		s.AddUserMessage(initialPrompt)
	}

	// Handle case where we are NOT continuous and have NO initial prompt (even after loading history)
	// The completion service's Run should handle history loading via Options
	if !opts.Continuous && initialPrompt == "" && opts.HistoryIn == "" {
		return fmt.Errorf("no input provided for non-continuous mode (use -i, -f, args, or pipe stdin)")
	}

	// Add prefill if provided
	if opts.Prefill != "" {
		s.SetNextCompletionPrefill(opts.Prefill)
	}

	// When running in continuous mode with a reattached TTY - no need for special messaging

	// Run the completion service - this now handles all cases including TTY reattachment
	runOpts := completion.RunOptions{
		Config: compCfg,
		InputStrings: opts.InputStrings,
		InputFiles: opts.InputFiles,
		PositionalArgs: opts.PositionalArgs,
		Prefill: opts.Prefill,
		Continuous: opts.Continuous,
		StreamOutput: opts.StreamOutput,
		ShowSpinner: opts.ShowSpinner,
		EchoPrefill: opts.EchoPrefill,
		UseTUI: opts.UseTUI,
		PrintUsage: opts.PrintUsage,
		Verbose: opts.Verbose,
		DebugMode: opts.DebugMode,
		HistoryIn: opts.HistoryIn,
		HistoryOut: opts.HistoryOut,
		ReadlineHistoryFile: opts.ReadlineHistoryFile,
		NCompletions: opts.NCompletions,
		Stdout: opts.Stdout,
		Stderr: opts.Stderr,
		Stdin: opts.Stdin,
		MaximumTimeout: opts.CompletionTimeout,
		ConfigPath: opts.ConfigPath,
		OpenAIUseLegacyMaxTokens: opts.OpenAIUseLegacyMaxTokens,
	}
	return s.Run(ctx, runOpts)
}

func NewCompletionConfig(cfg options.RunOptions) *completion.Config {
	return &completion.Config{
		MaxTokens:         cfg.Config.MaxTokens,
		Temperature:       cfg.Config.Temperature,
		SystemPrompt:      cfg.Config.SystemPrompt,
		CompletionTimeout: cfg.CompletionTimeout,
	}
}

func initFlags(args []string, stdin io.Reader) (options.RunOptions, *pflag.FlagSet, error) {
	opts := options.RunOptions{
		Config: &options.Config{},
		Stdin:  stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
	if len(args) == 0 {
		return opts, nil, fmt.Errorf("no arguments provided")
	}

	fs := pflag.NewFlagSet(args[0], pflag.ContinueOnError)
	fs.SortFlags = false
	defineFlags(fs, &opts)

	// We no longer reset InputFiles based on stdin being a terminal
	// This is now handled by the input processor

	showAdvancedUsage := fs.String("show-advanced-usage", "", "Show advanced usage examples (comma separated list of sections, or 'all')")
	help := fs.BoolP("help", "h", false, "Display help information")

	fs.MarkHidden("stream-output")
	fs.MarkHidden("readline-history-file")
	fs.MarkHidden("prefill-echo")
	fs.MarkHidden("show-spinner")

	fs.Usage = func() {
		fmt.Println("cgpt is a command line tool for interacting with generative AI models")
		fmt.Println()
		if *showAdvancedUsage != "" {
			printAdvancedUsage(*showAdvancedUsage)
			return
		}
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", args[0])
		fs.PrintDefaults()
		printBasicUsage()
	}

	err := fs.Parse(args[1:])
	if err != nil {
		return opts, fs, err
	}

	if *help {
		fs.Usage()
		return opts, fs, pflag.ErrHelp
	}

	if *showAdvancedUsage != "" {
		printAdvancedUsage(*showAdvancedUsage)
		return opts, fs, pflag.ErrHelp
	}

	opts.PositionalArgs = fs.Args()

	return opts, fs, nil
}

// These functions are implemented in usage_examples.go
// Ensuring we don't have duplicate implementations

// Expand ~ to home directory
func expandTilde(path string) string {
	if path == "" || path[0] != '~' {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, path[1:])
}

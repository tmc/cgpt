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
//	-m, --model string               The model to use (default "claude-3-5-sonnet-20240620")
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
// using the previous output as input for the next request. This mode is automatically
// enabled when receiving piped input or when running interactively with no inputs.
//
// When receiving piped input, the program will continue running in continuous mode
// until EOF is received, at which point it gracefully exits.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/pflag"
	"github.com/tmc/cgpt"
	"github.com/tmc/langchaingo/httputil"
	"golang.org/x/term"
)

// defineFlags defines the command line flags for the cgpt command
func defineFlags(fs *pflag.FlagSet, opts *cgpt.RunOptions) {
	// Runtime flags
	fs.StringSliceVarP(&opts.InputStrings, "input", "i", nil, "Direct string input (can be used multiple times)")
	fs.StringSliceVarP(&opts.InputFiles, "file", "f", []string{"-"}, "Input file path. Use '-' for stdin (can be used multiple times)")
	fs.BoolVarP(&opts.Continuous, "continuous", "c", false, "Run in continuous mode (interactive)")
	fs.BoolVarP(&opts.Verbose, "verbose", "v", false, "Verbose output")
	fs.BoolVar(&opts.DebugMode, "debug", false, "Debug output")
	fs.BoolVar(&opts.ShowSpinner, "show-spinner", true, "Show spinner while waiting for completion")
	fs.StringVarP(&opts.Prefill, "prefill", "p", "", "Prefill the assistant's response")
	fs.BoolVar(&opts.StreamOutput, "stream", true, "Use streaming output")

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
	fs.StringVarP(&opts.Config.Model, "model", "m", "claude-3-5-sonnet-20240620", "The model to use")
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

func run(ctx context.Context, opts cgpt.RunOptions, flagSet *pflag.FlagSet) error {
	// Load the config file
	fileConfig, err := cgpt.LoadConfig(opts.ConfigPath, opts.Stderr, flagSet)
	opts.Config = fileConfig
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize the model (the llms.Model interface)
	modelOpts := []cgpt.InferenceProviderOption{}
	// if debug mode is on, attach the debug http client:
	if opts.DebugMode {
		fmt.Fprintln(opts.Stderr, "Debug mode enabled")
		modelOpts = append(modelOpts, cgpt.WithHTTPClient(httputil.DebugHTTPClient))
	}
	if opts.OpenAIUseLegacyMaxTokens {
		modelOpts = append(modelOpts, cgpt.WithUseLegacyMaxTokens(true))
	}
	model, err := cgpt.InitializeModel(opts.Config, modelOpts...)
	if err != nil {
		return fmt.Errorf("failed to initialize model: %w", err)
	}

	// Enable continuous mode when:
	// 1. stdin is a tty with no inputs (interactive shell), or
	// 2. stdin is not a tty (piped input)
	if (term.IsTerminal(int(os.Stdin.Fd())) && len(opts.InputFiles) == 0 && len(opts.InputStrings) == 0 && len(opts.PositionalArgs) == 0) ||
		(!term.IsTerminal(int(os.Stdin.Fd())) && slices.Contains(opts.InputFiles, "-")) {
		opts.Continuous = true
	}
	// Only have spinner on if stdout is a tty:
	opts.ShowSpinner = opts.ShowSpinner && term.IsTerminal(int(os.Stdout.Fd()))

	// Create the completion service
	s, err := cgpt.NewCompletionService(opts.Config, model,
		cgpt.WithStdout(opts.Stdout),
		cgpt.WithStderr(opts.Stderr),
	)
	if err != nil {
		return fmt.Errorf("failed to create completion service: %w", err)
	}
	// Run the completion service
	return s.Run(ctx, opts)
}

func initFlags(args []string, stdin io.Reader) (cgpt.RunOptions, *pflag.FlagSet, error) {
	opts := cgpt.RunOptions{
		Config: &cgpt.Config{},
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

	if term.IsTerminal(int(os.Stdin.Fd())) {
		opts.InputFiles = nil
	}

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

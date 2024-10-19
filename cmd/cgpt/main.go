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
// using the previous output as input for the next request. In this mode, inference
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/spf13/pflag"
	"github.com/tmc/cgpt"
)

func defineFlags(fs *pflag.FlagSet, opts *cgpt.RunOptions) {
	// Runtime flags
	fs.StringSliceVarP(&opts.InputStrings, "input", "i", nil, "Direct string input (can be used multiple times)")
	fs.StringSliceVarP(&opts.InputFiles, "file", "f", nil, "Input file path. Use '-' for stdin (can be used multiple times)")
	fs.BoolVarP(&opts.Continuous, "continuous", "c", false, "Run in continuous mode (interactive)")
	fs.BoolVarP(&opts.Verbose, "verbose", "v", false, "Verbose output")
	fs.BoolVar(&opts.DebugMode, "debug", false, "Debug output")
	fs.BoolVar(&opts.StreamOutput, "stream-output", true, "Use streaming output")
	fs.BoolVar(&opts.ShowSpinner, "show-spinner", true, "Show spinner while waiting for completion")
	fs.BoolVar(&opts.EchoPrefill, "prefill-echo", true, "Print the prefill message")

	// History flags
	fs.StringVarP(&opts.HistoryIn, "history-in", "I", "", "File to read completion history from")
	fs.StringVarP(&opts.HistoryOut, "history-out", "O", "", "File to store completion history in")
	// mark these deprecated:
	fs.StringVar(&opts.HistoryIn, "history-load", "", "File to read completion history from")
	fs.StringVar(&opts.HistoryOut, "history-save", "", "File to store completion history in")

	fs.StringVar(&opts.ReadlineHistoryFile, "readline-history-file", "~/.cgpt_history", "File to store readline history in")
	fs.IntVarP(&opts.NCompletions, "completions", "n", 0, "Number of completions (when running non-interactively with history)")

	// Config flags (these can override values in the config file)
	fs.StringVarP(&opts.Config.Backend, "backend", "b", "", "The backend to use")
	fs.StringVarP(&opts.Config.Model, "model", "m", "", "The model to use")
	fs.IntVarP(&opts.Config.MaxTokens, "max-tokens", "t", 0, "Maximum tokens to generate")
	fs.Float64VarP(&opts.Config.Temperature, "temperature", "T", 0, "Temperature for sampling")
	fs.StringVar(&opts.Config.OpenAIAPIKey, "openai-api-key", "", "OpenAI API Key")
	fs.StringVar(&opts.Config.AnthropicAPIKey, "anthropic-api-key", "", "Anthropic API Key")
	fs.StringVar(&opts.Config.GoogleAPIKey, "google-api-key", "", "Google API Key")

	// Config file path
	fs.StringVar(&opts.ConfigPath, "config", "config.yaml", "Path to the configuration file")
}

func main() {
	opts, flagSet, err := parseFlags()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(2)
	}

	ctx := context.Background()
	if err := run(ctx, opts, flagSet); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, opts cgpt.RunOptions, flagSet *pflag.FlagSet) error {
	// Load the config file
	fileConfig, err := cgpt.LoadConfig(opts.ConfigPath, opts.Stderr, flagSet)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	// Merge the loaded config with the RunOptions.Config
	mergedConfig := cgpt.MergeConfigs(*fileConfig, *opts.Config)
	opts.Config = &mergedConfig

	// Initialize the model (the llms.Model interface)
	modelOpts := []cgpt.ModelOption{}
	// if debug mode is on, attach the debug http client:
	if opts.DebugMode {
		modelOpts = append(modelOpts, cgpt.WithHTTPClient(http.DefaultClient))
	}
	model, err := cgpt.InitializeModel(opts.Config, modelOpts...)
	if err != nil {
		return fmt.Errorf("failed to initialize model: %w", err)
	}

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

func parseFlags() (cgpt.RunOptions, *pflag.FlagSet, error) {
	opts := cgpt.RunOptions{
		Config: &cgpt.Config{}, // Initialize with default values
		Stdin:  os.Stdin,       // Set stdin by default
		Stdout: os.Stdout,      // Set stdout by default
		Stderr: os.Stderr,      // Set stderr by default
	}

	fs := pflag.NewFlagSet("cgpt", pflag.ContinueOnError)
	defineFlags(fs, &opts)

	// Help and usage flags
	help := fs.BoolP("help", "h", false, "Display help information")
	showAdvancedUsage := fs.String("show-advanced-usage", "", "Show advanced usage examples")

	err := fs.Parse(os.Args[1:])
	if err != nil {
		return opts, fs, err
	}

	if *help {
		fs.Usage()
		return opts, fs, fmt.Errorf("help requested")
	}

	if *showAdvancedUsage != "" {
		showAdvancedUsageExamples(*showAdvancedUsage)
		return opts, fs, fmt.Errorf("advanced usage examples requested")
	}

	// Handle non-flag arguments
	opts.PositionalArgs = fs.Args()

	// Check if stdin should be used
	for _, file := range opts.InputFiles {
		if file == "-" {
			stat, _ := os.Stdin.Stat()
			if (stat.Mode() & os.ModeCharDevice) == 0 {
				// Data is being piped to stdin
				opts.Stdin = os.Stdin
			} else {
				// No data is being piped, remove "-" from InputFiles
				opts.InputFiles = removeString(opts.InputFiles, "-")
			}
			break
		}
	}

	return opts, fs, nil
}

func removeString(slice []string, s string) []string {
	for i, v := range slice {
		if v == s {
			return append(slice[:i], slice[i+1:]...)
		}
	}
	return slice
}

func showAdvancedUsageExamples(examples string) {
	// Implement the logic to show advanced usage examples
	fmt.Println("Advanced usage examples:", examples)
}

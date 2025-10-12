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
//	-m, --model string               The model to use (default "claude-sonnet-4-20250514")
//	-i, --input string               Direct string input (can be used multiple times)
//	-f, --file string                Input file path. Use '-' for stdin (can be used multiple times)
//	-c, --continuous                 Run in continuous mode (interactive)
//	-s, --system-prompt string       System prompt to use
//	-p, --prefill string             Prefill the assistant's response
//	-I, --history-in string          File to read completion history from
//	-O, --history-out string         File to store completion history in (or - for stdout)
//	-H, --history string             Read and write same history file (or 'auto' for auto-generated)
//	-C, --continue                   Continue most recent session
//	    --config string              Path to the configuration file (default "config.yaml")
//	-v, --verbose                    Verbose output
//	    --debug                      Debug output
//	-n, --completions int            Number of completions (when running non-interactively with history)
//	-t, --max-tokens int             Maximum tokens to generate (default 8000)
//	    --completion-timeout duration Maximum time to wait for a response (default 2m0s)
//	-h, --help                       Display help information
//
// History Management:
//
// cgpt supports session history with automatic metadata tracking:
//   - Use -H auto to create timestamped session files in ~/.cgpt/history/sessions/
//   - Use -H filename to read from and write to the same file
//   - Use -I and -O for explicit input/output control
//   - Use -C to continue the most recent session
//   - Sessions include metadata: creation time, description, and fork tracking
//
// The -c/--continuous flag enables interactive mode, where the program runs in a loop,
// using the previous output as input for the next request. In this mode, inference
package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/pflag"
	"github.com/tmc/cgpt"
	"github.com/tmc/langchaingo/httputil"
	"golang.org/x/term"
)

// defineFlags defines the command line flags for the cgpt command
func defineFlags(fs *pflag.FlagSet, opts *cgpt.RunOptions) {
	// === INPUT/OUTPUT FLAGS ===
	fs.StringArrayVarP(&opts.InputStrings, "input", "i", nil, "Direct text input (can be used multiple times)")
	fs.StringArrayVarP(&opts.InputFiles, "file", "f", []string{"-"}, "Input file path, use '-' for stdin (multiple files allowed)")
	fs.StringVarP(&opts.Prefill, "prefill", "p", "", "Start the assistant's response with this text")
	fs.BoolVarP(&opts.Continuous, "continuous", "c", false, "Interactive mode - chat with the AI in a loop")
	fs.BoolVarP(&opts.Verbose, "verbose", "v", false, "Show detailed output and processing information")
	fs.BoolVar(&opts.DebugMode, "debug", false, "Enable debug mode with request/response details")

	// === HISTORY & SESSION MANAGEMENT ===
	fs.StringVarP(&opts.HistoryIn, "history-in", "I", "", "Load conversation history from this file")
	fs.StringVarP(&opts.HistoryOut, "history-out", "O", "", "Save conversation history to this file (use '-' for stdout)")
	fs.StringVarP(&opts.History, "history", "H", "", "Use same file for input/output history (use 'auto' for timestamped files)")
	fs.BoolVarP(&opts.Continue, "continue", "C", false, "Continue your most recent conversation session")
	fs.IntVarP(&opts.NCompletions, "completions", "n", 0, "Number of AI responses to generate (for batch processing)")

	// === CONVERSATION FORKING ===
	fs.StringVar(&opts.ForkFrom, "fork-from", "", "Fork from this conversation file")
	fs.IntVar(&opts.ForkPoint, "fork-point", -1, "Message index to fork from (-1 for current point)")
	fs.StringVar(&opts.ForkDescription, "fork-desc", "", "Description for the fork")
	fs.StringVar(&opts.ForkBranch, "fork-branch", "", "Git branch name for the fork")
	fs.BoolVar(&opts.ListForks, "list-forks", false, "List all conversation forks")
	fs.BoolVar(&opts.ShowTree, "show-tree", false, "Show conversation tree structure")

	// === AI MODEL & BEHAVIOR ===
	fs.StringVarP(&opts.Config.Backend, "backend", "b", "anthropic", "AI provider: anthropic, openai, gemini, ollama")
	fs.StringVarP(&opts.Config.Model, "model", "m", "claude-sonnet-4-20250514", "AI model to use (e.g., claude-haiku-3-20240307, gpt-4)")
	fs.StringVarP(&opts.Config.SystemPrompt, "system-prompt", "s", "", "System instructions to guide the AI's behavior")
	fs.IntVarP(&opts.Config.MaxTokens, "max-tokens", "t", 0, "Maximum response length in tokens (0 = model default)")
	fs.Float64VarP(&opts.Config.Temperature, "temperature", "T", 0.05, "Creativity level: 0.0 (focused) to 1.0 (creative)")

	// === ADVANCED FEATURES ===
	fs.BoolVar(&opts.Config.PromptCaching, "prompt-caching", false, "Cache prompts to reduce costs on repeated requests")
	fs.StringVar(&opts.Config.ThinkingMode, "thinking-mode", "none", "Reasoning depth: none, low, medium, high, auto (Claude 4+ only)")
	fs.IntVar(&opts.Config.ThinkingBudget, "thinking-budget", 0, "Token budget for reasoning (overrides thinking-mode)")
	fs.BoolVar(&opts.Config.ShowUsage, "usage", false, "Display token usage, cache statistics, and cost estimates")
	fs.BoolVar(&opts.Config.ShowReasoning, "show-reasoning", false, "Show the AI's reasoning process when available")
	fs.BoolVar(&opts.Config.InterleavedThinking, "interleaved-thinking", false, "Enable advanced reasoning mode (Claude 4+ only)")

	// === TECHNICAL OPTIONS ===
	fs.StringVar(&opts.Config.BaseURL, "base-url", "", "Custom base URL for the API endpoint")
	fs.BoolVar(&opts.Config.InsecureSkipVerify, "insecure-skip-verify", false, "Skip TLS certificate verification")
	fs.BoolVar(&opts.ShowSpinner, "show-spinner", true, "Show loading spinner while waiting")
	fs.BoolVar(&opts.StreamOutput, "stream", true, "Stream responses as they generate")
	fs.BoolVar(&opts.EchoPrefill, "prefill-echo", true, "Display prefill text in output")
	fs.DurationVar(&opts.CompletionTimeout, "completion-timeout", 2*time.Minute, "Maximum wait time for AI response")
	fs.BoolVar(&opts.OpenAIUseLegacyMaxTokens, "openai-use-max-tokens", false, "Use legacy max_tokens parameter for OpenAI")
	fs.StringVar(&opts.ReadlineHistoryFile, "readline-history-file", "~/.cgpt_history", "Command history file for interactive mode")

	// === RETRY OPTIONS ===
	fs.IntVar(&opts.Config.MaxRetries, "max-retries", 3, "Maximum number of retry attempts for failed requests (0 disables retries)")
	fs.DurationVar(&opts.Config.RetryDelay, "retry-delay", time.Second, "Initial delay before first retry (exponential backoff applied)")
	fs.BoolVar(&opts.Config.DisableRetry, "disable-retry", false, "Disable all retry attempts for failed requests")

	// === CONFIGURATION ===
	fs.StringVar(&opts.ConfigPath, "config", "config.yaml", "Configuration file path")
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

	// Validate thinking mode configuration and show warnings
	if warnings := opts.Config.ValidateThinkingConfig(); len(warnings) > 0 {
		for _, warning := range warnings {
			fmt.Fprintln(opts.Stderr, warning)
		}
	}

	// Creates the default save path if it doesn't exist
	if dir, _ := os.UserHomeDir(); dir != "" {
		cgptDir := filepath.Join(dir, ".cgpt")
		if _, err := os.Stat(cgptDir); os.IsNotExist(err) {
			err := os.MkdirAll(cgptDir, 0755)
			if err != nil {
				fmt.Fprintf(opts.Stderr, "Failed to create default save path: %v\n", err)
			} else {
				fmt.Fprintf(opts.Stderr, "Created default save path: %s\n", cgptDir)
			}
		}
	}

	// Initialize the model (the llms.Model interface)
	// Start with a clone of the default transport to inherit proxy settings.
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: opts.Config.InsecureSkipVerify}
	httpClient := &http.Client{Transport: transport}

	// For Google AI, wrap the transport with ApiKeyTransport.
	if opts.Config.Backend == "googleai" {
		httpClient.Transport = &httputil.ApiKeyTransport{
			Transport: httpClient.Transport,
			APIKey:    opts.Config.GoogleAPIKey,
		}
	}

	if opts.DebugMode {
		fmt.Fprintln(opts.Stderr, "Debug mode enabled")
		// Get the top-level wrapper transport from the global JSONDebugClient.
		if wrapperTransport, ok := httputil.JSONDebugClient.Transport.(*httputil.Transport); ok {
			// The actual logging transport is nested inside.
			type transportSetter interface {
				SetTransport(http.RoundTripper)
			}
			// Use an interface to duck-type our way to setting the inner transport.
			if setter, ok := wrapperTransport.Transport.(transportSetter); ok {
				setter.SetTransport(httpClient.Transport)
				httpClient.Transport = wrapperTransport
			} else {
				fmt.Fprintln(opts.Stderr, "Warning: could not set base transport on debug client, using basic logger")
				httpClient.Transport = &httputil.LoggingTransport{Transport: httpClient.Transport}
			}
		} else {
			fmt.Fprintln(opts.Stderr, "Warning: could not get JSON debug transport, using basic logger")
			httpClient.Transport = &httputil.LoggingTransport{Transport: httpClient.Transport}
		}
	}

	modelOpts := []cgpt.InferenceProviderOption{cgpt.WithHTTPClient(httpClient)}
	model, err := cgpt.InitializeModel(opts.Config, modelOpts...)
	if err != nil {
		return fmt.Errorf("failed to initialize model: %w", err)
	}

	// Handle fork-specific commands first
	if opts.ListForks || opts.ShowTree || opts.ForkFrom != "" {
		return handleForkCommands(ctx, opts)
	}

	// If stdin is a tty, and no input files, strings, or args are provided,
	// then we should run in continuous mode:
	if term.IsTerminal(int(os.Stdin.Fd())) && len(opts.InputFiles) == 0 && len(opts.InputStrings) == 0 && len(opts.PositionalArgs) == 0 {
		opts.Continuous = true
	}
	// Only have spinner on if stdout is a tty:
	opts.ShowSpinner = opts.ShowSpinner && term.IsTerminal(int(os.Stdout.Fd()))

	// Create the completion service
	completionOpts := []cgpt.CompletionServiceOption{
		cgpt.WithStdout(opts.Stdout),
		cgpt.WithStderr(opts.Stderr),
	}
	if opts.OpenAIUseLegacyMaxTokens {
		completionOpts = append(completionOpts, cgpt.WithUseLegacyMaxTokens(true))
	}
	s, err := cgpt.NewCompletionService(opts.Config, model, completionOpts...)
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
	examples := fs.Bool("examples", false, "Show quick usage examples and exit")
	help := fs.BoolP("help", "h", false, "Display help information")

	fs.MarkHidden("stream-output")
	fs.MarkHidden("readline-history-file")
	fs.MarkHidden("prefill-echo")
	fs.MarkHidden("show-spinner")

	fs.Usage = func() {
		if *examples {
			printQuickExamples()
			return
		}
		if *showAdvancedUsage != "" {
			printAdvancedUsage(*showAdvancedUsage)
			return
		}
		printEnhancedHelp(args[0], fs)
	}

	err := fs.Parse(args[1:])
	if err != nil {
		return opts, fs, err
	}

	if *help {
		fs.Usage()
		return opts, fs, pflag.ErrHelp
	}

	if *examples {
		printQuickExamples()
		return opts, fs, pflag.ErrHelp
	}

	if *showAdvancedUsage != "" {
		printAdvancedUsage(*showAdvancedUsage)
		return opts, fs, pflag.ErrHelp
	}

	opts.PositionalArgs = fs.Args()

	return opts, fs, nil
}

// handleForkCommands handles fork-specific operations
func handleForkCommands(ctx context.Context, opts cgpt.RunOptions) error {
	forkManager, err := cgpt.NewForkManager()
	if err != nil {
		return fmt.Errorf("failed to initialize fork manager: %w", err)
	}

	// Handle list forks
	if opts.ListForks {
		forks, err := forkManager.ListForks()
		if err != nil {
			return fmt.Errorf("failed to list forks: %w", err)
		}

		fmt.Fprintf(opts.Stdout, "Conversation Forks:\n")
		if len(forks) == 0 {
			fmt.Fprintf(opts.Stdout, "  No forks found\n")
		} else {
			for _, fork := range forks {
				fmt.Fprintf(opts.Stdout, "  %s: %s\n", fork.Branch, fork.Description)
			}
		}
		return nil
	}

	// Handle show tree
	if opts.ShowTree {
		tree, err := forkManager.GetConversationTree()
		if err != nil {
			return fmt.Errorf("failed to get conversation tree: %w", err)
		}

		fmt.Fprintf(opts.Stdout, "Conversation Tree:\n")
		fmt.Fprintf(opts.Stdout, "%s", tree.PrintTree())
		return nil
	}

	// Handle fork creation
	if opts.ForkFrom != "" {
		forkCmd := cgpt.ForkCommand{
			InputFile:   opts.ForkFrom,
			OutputFile:  opts.HistoryOut,
			ForkPoint:   opts.ForkPoint,
			Description: opts.ForkDescription,
			BranchName:  opts.ForkBranch,
		}

		if err := forkManager.ExecuteForkCommand(ctx, forkCmd); err != nil {
			return fmt.Errorf("failed to execute fork command: %w", err)
		}

		fmt.Fprintf(opts.Stderr, "Successfully forked conversation from %s\n", filepath.Base(opts.ForkFrom))
		if forkCmd.OutputFile != "" {
			fmt.Fprintf(opts.Stderr, "New conversation saved to: %s\n", forkCmd.OutputFile)
		}
	}

	return nil
}

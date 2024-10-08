// Command cgpt is a command line tool for interacting with Large Language Models (LLMs).
//
// Usage:
//
//	cgpt [flags]
//
// Flags:
//
//	-b, --backend string             The backend to use (default "anthropic")
//	-m, --model string               The model to use (default "claude-3-5-sonnet-20240620")
//	-i, --input string               Direct string input (overrides -f)
//	-f, --file string                Input file path. Use '-' for stdin (default), mutually exclusive with -i (default "-")
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
	"os"
	"strings"
	"time"

	flag "github.com/spf13/pflag"

	"github.com/tmc/cgpt"
)

var (
	flagBackend = flag.StringP("backend", "b", "anthropic", "The backend to use")
	flagModel   = flag.StringP("model", "m", "claude-3-5-sonnet-20240620", "The model to use")

	flagInputString = flag.StringP("input", "i", "", "Direct string input (overrides -f)")
	flagInputFile   = flag.StringP("file", "f", "-", "Input file path. Use '-' for stdin (default), mutually exclusive with -i")

	flagContinuous   = flag.BoolP("continuous", "c", false, "Run in continuous mode (interactive)")
	flagSystemPrompt = flag.StringP("system-prompt", "s", "", "System prompt to use")
	flagPrefill      = flag.StringP("prefill", "p", "", "Prefill the assistant's response")

	flagHistoryIn  = flag.StringP("history-load", "I", "", "File to read completion history from")
	flagHistoryOut = flag.StringP("history-save", "O", "", "File to store completion history in")

	flagConfig  = flag.String("config", "config.yaml", "Path to the configuration file")
	flagVerbose = flag.BoolP("verbose", "v", false, "Verbose output")
	flagDebug   = flag.BoolP("debug", "", false, "Debug output")

	flagNCompletions = flag.IntP("completions", "n", 0, "Number of completions (when running non-interactively with history)")

	flagMaxTokens      = flag.IntP("max-tokens", "t", 8000, "Maximum tokens to generate")
	flagTemp           = flag.Float64P("temperature", "T", 0.05, "Temperature for sampling")
	flagMaximumTimeout = flag.DurationP("completion-timeout", "", 2*time.Minute, "Maximum time to wait for a response")
	flagHelp           = flag.BoolP("help", "h", false, "")

	// hidden flags
	flagReadlineHistoryFile = flag.String("readline-history-file", "~/.cgpt_history", "File to store readline history in")
	flagEchoPrefill         = flag.Bool("prefill-echo", true, "Print the prefill message")
	flagShowSpinner         = flag.Bool("show-spinner", true, "Show spinner while waiting for completion (default true, auto-disabled when in continuous mode)")
	flagStreamingOutput     = flag.Bool("stream-output", true, "Use streaming output")

	flagShowAdvancedUsage = flag.String("show-advanced-usage", "", fmt.Sprintf("Show advanced usage examples (comma-separated list of: %s) - use 'all' to show them all", strings.Join(advancedUsageFiles, ", ")))
)

func main() {
	initFlags()
	ctx := context.Background()

	// Load configuration and flags.
	cfg, err := cgpt.LoadConfig(*flagConfig, flag.CommandLine)
	if err != nil && *flagVerbose {
		fmt.Fprintf(os.Stderr, "issue loading config: %v\n", err)
	}

	s, err := cgpt.NewCompletionService(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if err = s.Run(ctx, cgpt.RunConfig{
		InputString:  *flagInputString,
		InputFile:    *flagInputFile,
		Continuous:   *flagContinuous,
		Prefill:      *flagPrefill,
		HistoryIn:    *flagHistoryIn,
		HistoryOut:   *flagHistoryOut,
		NCompletions: *flagNCompletions,
		Verbose:      *flagVerbose,
		DebugMode:    *flagDebug,

		EchoPrefill: *flagEchoPrefill,
		ShowSpinner: *flagShowSpinner,

		StreamOutput: *flagStreamingOutput,

		ReadlineHistoryFile: *flagReadlineHistoryFile,
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func initFlags() {
	flag.CommandLine.SortFlags = false
	flag.CommandLine.MarkHidden("stream-output")
	flag.CommandLine.MarkHidden("readline-history-file")
	flag.CommandLine.MarkHidden("prefill-echo")
	flag.CommandLine.MarkHidden("show-spinner")
	flag.Usage = func() {
		fmt.Println("cgpt is a command line tool for interacting with generative AI models")
		fmt.Println()
		if *flagShowAdvancedUsage != "" {
			printAdvancedUsage(*flagShowAdvancedUsage)
			return
		}
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		flag.CommandLine.PrintDefaults()
		printBasicUsage()
	}
	flag.Parse()
	if *flagHelp || *flagShowAdvancedUsage != "" {
		flag.Usage()
		os.Exit(0)
	}
}

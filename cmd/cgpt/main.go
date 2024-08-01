// Command cgpt is a command line tool for interacting with LLMs.
//
// The -c/-continuous flag will run the completion API in a loop, using the previous output as the
// input for the next request. It will run inference after two newlines are entered.
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	flag "github.com/spf13/pflag"

	"github.com/tmc/cgpt"
)

var (
	flagBackend      = flag.StringP("backend", "b", "anthropic", "The backend to use")
	flagModel        = flag.StringP("model", "m", "claude-3-5-sonnet-20240620", "The model to use")
	flagInput        = flag.StringP("input", "i", "-", "The input file to use. Use - for stdin (default)")
	flagContinuous   = flag.BoolP("continuous", "c", false, "Run in continuous mode (interactive)")
	flagSystemPrompt = flag.StringP("system-prompt", "s", "", "System prompt to use")
	flagHistoryRepo  = flag.StringP("history-repo", "r", "", "Git repository to use for history")
	flagHistorySHA   = flag.StringP("history-sha", "", "", "Git SHA to load history from")
	flagStream       = flag.Bool("stream", true, "Stream results")
	flagConfig       = flag.String("config", "config.yaml", "Path to the configuration file")
	flagVerbose      = flag.BoolP("verbose", "v", false, "Verbose output")
	flagDebug        = flag.BoolP("debug", "", false, "Debug output")

	flagMaxTokens      = flag.IntP("max-tokens", "t", 8000, "Maximum tokens to generate")
	flagMaximumTimeout = flag.DurationP("completion-timeout", "", 2*time.Minute, "Maximum time to wait for a response")

	flagReadlineHistoryFile = flag.String("readline-history-file", "~/.cgpt_history", "File to store readline history in")
	flagHelp                = flag.BoolP("help", "h", false, "")
)

func main() {
	initFlags()
	ctx := context.Background()

	// Attempt to load config, but don't fail if it doesn't exist.
	cfg, err := cgpt.LoadConfig(*flagConfig, flag.CommandLine)
	if err != nil && *flagVerbose {
		fmt.Fprintf(os.Stderr, "issue loading config: %v\n", err)
	}

	s, err := cgpt.NewCompletionService(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if *flagHistoryRepo != "" {
		if err := s.InitGitRepo(*flagHistoryRepo); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to initialize git repo: %v\n", err)
			os.Exit(1)
		}
		if *flagHistorySHA != "" {
			if err := s.LoadHistoryBySHA(*flagHistorySHA); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to load history: %v\n", err)
				os.Exit(1)
			}
		}
	}

	if err = s.Run(ctx, cgpt.RunConfig{
		Input:        *flagInput,
		Continuous:   *flagContinuous,
		Stream:       *flagStream,
		Verbose:      *flagVerbose,
		DebugMode:    *flagDebug,

		ReadlineHistoryFile: *flagReadlineHistoryFile,
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func initFlags() {
	flag.CommandLine.SortFlags = false
	flag.CommandLine.MarkHidden("readline-history-file")
	flag.CommandLine.MarkHidden("stream")
	flag.Usage = func() {
		fmt.Println("cgpt is a command line tool for interacting with generative AI models")
		fmt.Println()
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		flag.CommandLine.PrintDefaults()
		fmt.Println(`
Examples:
	$ echo "how should I interpret the output of nvidia-smi?" | cgpt
	$ echo "explain plan 9 in one sentence" | cgpt
	$ cgpt --history-repo /path/to/repo -c  # Run in continuous mode with history`)
	}
	flag.Parse()
	if *flagHelp {
		flag.Usage()
		os.Exit(0)
	}
}

func checkGitAvailability() {
	_, err := exec.LookPath("git")
	if err != nil {
		fmt.Println("Warning: Git is not installed or not in PATH. History features will be limited.")
	}
}
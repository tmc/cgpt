// Command cgpt is a command line tool for interacting with LLMs.
//
// The -continuous flag will run the completion API in a loop, using the previous output as the
// input for the next request. It will run after two newlines are entered.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/tmc/cgpt"
)

var (
	flagBackend    = flag.String("backend", "openai", "The backend to use")
	flagModel      = flag.String("model", "gpt-4o", "The model to use")
	flagInput      = flag.String("input", "-", "The input text to complete. If '-', read from stdin.")
	flagConfig     = flag.String("config", "config.yaml", "Path to the configuration file")
	flagContinuous = flag.Bool("continuous", false, "Run in continuous mode")
	flagStream     = flag.Bool("stream", true, "Stream results")

	flagHistoryIn    = flag.String("hist-in", "", "File to read history from")
	flagHistoryOut   = flag.String("hist-out", "", "File to store history in")
	flagNCompletions = flag.Int("completions", 0, "Number of completions (when running with history)")
)

func main() {
	flag.Parse()
	ctx := context.Background()
	cfg, err := cgpt.LoadConfigFromPath(*flagConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "issue loading config: %v\n", err)
	}

	s, err := cgpt.NewCompletionService(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if err = s.Run(ctx, cgpt.RunConfig{
		Input:        *flagInput,
		Continuous:   *flagContinuous,
		Stream:       *flagStream,
		HistoryIn:    *flagHistoryIn,
		HistoryOut:   *flagHistoryOut,
		NCompletions: *flagNCompletions,
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

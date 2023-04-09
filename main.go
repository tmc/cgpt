// Command cgpt is a command line tool for interacting with the OpenAI completion APIs.
//
// The -continuous flag will run the completion API in a loop, using the previous output as the
// input for the next request. It will run after two newlines are entered.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
)

var (
	flagInput      = flag.String("input", "-", "The input text to complete. If '-', read from stdin.")
	flagConfig     = flag.String("config", "config.yaml", "Path to the configuration file")
	flagContinuous = flag.Bool("continuous", false, "Run in continuous mode")

	flagHistoryIn  = flag.String("hist-in", "", "File to read history from")
	flagHistoryOut = flag.String("hist-out", "", "File to store history in")
)

func main() {
	flag.Parse()
	ctx := context.Background()
	cfg, err := loadConfig(*flagConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}
	if cfg.APIKey == "" {
		fmt.Fprintln(os.Stderr, "Missing API key in config. Please set the OPENAI_API_KEY environment variable or add it to the config file.")
		os.Exit(1)
	}
	fmt.Println("config:", cfg)

	s, err := newCompletionService(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if err = s.run(ctx, runConfig{
		Input:      *flagInput,
		Continuous: *flagContinuous,
		HistoryIn:  *flagHistoryIn,
		HistoryOut: *flagHistoryOut,
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// struct to hold flag values for the run command
type runConfig struct {
	// Input is the input text to complete. If "-", read from stdin.
	Input string
	// Continuous will run the completion API in a loop, using the previous output as the input for the next request.
	Continuous bool

	// HistoryIn is the file to read history from.
	HistoryIn string
	// HistoryOut is the file to store history in.
	HistoryOut string
}
